package session

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// A store takes the ID that a store sharing a source with it cached, if it names a stored session,
// and never that of one it shares none with. G reads the cookies of A and of B: it shares with A,
// whose ID was never saved, and with B, which found its own session, but not with E, whose ID was
// never saved either. G must find the session its b_sid cookie names, whichever of A and E cached
// an ID last.
func Test_Session_Store_FindsTheSessionItsCookieNamesWhateverOtherStoresCached(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: shared})
	storeE := NewStore(Config{Extractor: extractors.FromCookie("e_sid")})
	storeG := NewStore(Config{Extractor: extractors.Chain(extractors.FromCookie("a_sid"), extractors.FromCookie("b_sid")), Storage: shared})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("k", "v") })

	for _, tc := range []struct {
		name  string
		asked []*Store
	}{
		{"the newest ID is that of a store it shares a cookie with", []*Store{storeA}},
		{"the newest ID is that of a store it shares none with", []*Store{storeA, storeB, storeE}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := newRequest(t, app)
			c.Request().Header.SetCookie("b_sid", idB)

			var issued []string
			for _, store := range tc.asked {
				sess := mustGet(t, store, c)
				if store == storeB {
					require.False(t, sess.Fresh())
					require.Equal(t, idB, sess.ID(), "B finds its session, and caches nothing")
				} else {
					require.True(t, sess.Fresh(), "the store caches an ID that it does not save")
					issued = append(issued, sess.ID())
				}
			}

			g := mustGet(t, storeG, c)
			require.NotContains(t, issued, g.ID(), "G took an ID that another store cached")
			require.False(t, g.Fresh(), "G found the session its b_sid cookie names")
			require.Equal(t, idB, g.ID())
			require.Equal(t, "v", g.Get("k"))
		})
	}
}

// A new session whose cookie names an ID the storage does not know is cached with the others of the
// request, as the session of a cookie that names a stored one is not. W and X cache an ID first,
// unsaved; Y, whose cookie is unknown, caches one after them and saves it. R shares Y's cookie but
// not W's or X's, so it must take the ID Y cached, and not find its own session as it would if Y
// had cached nothing.
func Test_Session_Store_FreshSessionOfAnUnknownCookieIsCached(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	app := fiber.New()
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: shared})
	storeW := NewStore(Config{Extractor: extractors.FromCookie("w_sid")})
	storeX := NewStore(Config{Extractor: extractors.FromCookie("x_sid")})
	storeY := NewStore(Config{Extractor: extractors.FromCookie("y_sid"), Storage: shared})
	storeR := NewStore(Config{Extractor: extractors.Chain(extractors.FromCookie("b_sid"), extractors.FromCookie("y_sid")), Storage: shared})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("k", "v") })

	c := newRequest(t, app)
	c.Request().Header.SetCookie("b_sid", idB)
	c.Request().Header.SetCookie("y_sid", "names-no-session")

	for _, store := range []*Store{storeB, storeW, storeX} {
		mustGet(t, store, c)
	}
	y := mustGet(t, storeY, c)
	require.True(t, y.Fresh(), "Y's cookie names no stored session")
	require.NoError(t, y.Save())
	require.Same(t, storeY, requestIDs(c).store, "Y cached its ID after W and X")
	require.Equal(t, y.ID(), c.Locals(sessionIDContextKey), "and it is the ID the logger shows")

	r := mustGet(t, storeR, c)
	require.Equal(t, y.ID(), r.ID(), "R shares Y's cookie, and Y cached last")
	require.NotEqual(t, idB, r.ID(), "R went to its own cookie instead of the ID that Y cached")
	require.False(t, r.Fresh(), "R found the session that Y saved")
}

// Stores are told apart by every source they read, by kind and name, whether or not it can be
// written back to. Stores over the same query parameter, authentication scheme or custom extractor
// key share the ID cached on the request; a store over the query string and one over a cookie do
// not, so the ID a link carries reaches the stores that read the query string only.
func Test_Session_Store_SourcesAreToldApartByKindAndName(t *testing.T) {
	t.Parallel()

	custom := func(fiber.Ctx) (string, error) { return "", extractors.ErrNotFound }
	query, cookie := extractors.FromQuery("sid"), extractors.FromCookie("c_sid")
	bearer, legacy := extractors.FromAuthHeader("Bearer"), extractors.FromCustom("legacy", custom)
	form, param := extractors.FromForm("sid"), extractors.FromParam("sid")
	shares := func(a, b extractors.Extractor) bool {
		return NewStore(Config{Extractor: a}).sharesSource(NewStore(Config{Extractor: b}))
	}

	for _, pair := range [][2]extractors.Extractor{
		{query, extractors.FromQuery("sid")},
		{bearer, extractors.FromAuthHeader("bearer")},
		{legacy, extractors.FromCustom("legacy", custom)},
		{form, extractors.FromForm("sid")},
		{param, extractors.FromParam("sid")},
	} {
		require.True(t, shares(pair[0], pair[1]), "stores over the same source share")
	}
	for _, pair := range [][2]extractors.Extractor{
		{query, extractors.FromQuery("token")},
		{query, bearer},
		{query, cookie},
		{cookie, query},
		{bearer, cookie},
		{bearer, extractors.FromAuthHeader("Token")}, // the same header, another scheme
		{legacy, extractors.FromCustom("other", custom)},
		{legacy, query},
		{form, query}, // the same name in another kind is another source
		{param, form},
		{param, cookie},
	} {
		require.False(t, shares(pair[0], pair[1]), "stores over different sources are told apart")
	}

	// A new visitor: a store over the same source takes the ID the first store saved, a store over
	// another source starts a session of its own.
	shared := memory.New()
	app := fiber.New()
	byQuery := NewStore(Config{Extractor: query, Storage: shared})
	visitor := newRequest(t, app)
	first := mustGet(t, byQuery, visitor)
	first.Set("k", "v")
	require.NoError(t, first.Save())
	second := mustGet(t, NewStore(Config{Extractor: query, Storage: shared}), visitor)
	require.Equal(t, first.ID(), second.ID(), "a store over the same query parameter took the ID the first one saved")
	require.False(t, second.Fresh())
	third := mustGet(t, NewStore(Config{Extractor: cookie, Storage: shared}), visitor)
	require.NotEqual(t, first.ID(), third.ID(), "a cookie store next to them took the ID too")
	require.True(t, third.Fresh())

	// A session that a link names is served by the store that reads the query string; the cookie
	// store behind it keeps to its own cookie, and starts a session of its own for a visitor who
	// has none.
	id := seedSession(t, app, byQuery, func(s *Session) { s.Set("k", "v") })
	queryHandler, queryStore := NewWithStore(Config{Extractor: query, Storage: shared})
	cookieHandler, cookieStore := NewWithStore(Config{Extractor: cookie, Storage: shared})
	stacked := fiber.New()
	stacked.Use(queryHandler, cookieHandler)
	stacked.Get("/", func(c fiber.Ctx) error {
		q, m := queryStore.FromContext(c), cookieStore.FromContext(c)
		return c.SendString(fmt.Sprintf("%s|%t|%v|%t|%v", q.ID(), q.Fresh(), q.Get("k"), m.ID() == id, m.Get("k")))
	})
	resp, body := sendRequest(t, stacked, "/?sid="+id, nil)
	require.Equal(t, id+"|false|v|false|<nil>", body, "the cookie store served the session the link names")
	cookies := cookiesByName(resp)
	require.NotNil(t, cookies["c_sid"], "the cookie store set no cookie for its own session")
	require.NotEqual(t, id, cookies["c_sid"].Value, "the cookie carries the ID the link named")
}

// Config is embedded, so its fields can be assigned after NewStore. A store whose Extractor was
// replaced writes the ID to, and shares with, what it reads now.
func Test_Store_ExtractorReplacedAfterNewStore(t *testing.T) {
	t.Parallel()

	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	store.Extractor = extractors.FromCookie("b_sid")

	c := newRequest(t, fiber.New())
	require.NoError(t, mustGet(t, store, c).Save())

	var written []string
	for key := range c.Response().Header.Cookies() {
		written = append(written, string(key))
	}
	require.Equal(t, []string{"b_sid"}, written, "the ID goes to the cookie the store reads now")

	require.True(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("b_sid")})))
	require.False(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("a_sid")})))
	require.False(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("B_SID")})), "cookie names are case-sensitive")

	store.Extractor = extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C"))
	require.Equal(t, []string{"cookie:c_sid", "header:X-C"}, sinkNames(store.writableSinks()))
	require.True(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("x-c")})))
	require.False(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("b_sid")})))

	// A chain of the same length and with the same first extractor is another chain.
	chained := NewStore(Config{Extractor: extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C"))})
	require.Equal(t, []string{"cookie:c_sid", "header:X-C"}, sinkNames(chained.writableSinks()))
	chained.Extractor = extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-D"))
	require.Equal(t, []string{"cookie:c_sid", "header:X-D"}, sinkNames(chained.writableSinks()))
	require.True(t, chained.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("x-d")})))
	require.False(t, chained.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("x-c")})))

	// A chain reports the key and the source of its first child, so a cookie replaced by a header
	// of the same name, and a leaf replaced by a chain that starts with it, differ in one thing
	// each, and each is another extractor.
	swapped := NewStore(Config{Extractor: extractors.FromCookie("sid")})
	swapped.Extractor = extractors.FromHeader("sid")
	require.Equal(t, []string{"header:sid"}, sinkNames(swapped.writableSinks()), "a header, not the cookie of that name")
	require.True(t, swapped.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("SID")})))
	require.False(t, swapped.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("sid")})))

	grown := NewStore(Config{Extractor: extractors.FromCookie("a")})
	grown.Extractor = extractors.Chain(extractors.FromCookie("a"), extractors.FromHeader("X-A"))
	require.Equal(t, []string{"cookie:a", "header:X-A"}, sinkNames(grown.writableSinks()), "the chain has the header as well")
	require.True(t, grown.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("x-a")})))
}

// An ID that names no session of the store is passed over to the next one, and not only to the
// request: W, the newest to cache an ID, reads the cookie S reads but keeps its sessions in a
// storage of its own, so its ID is no session of S's; A cached an ID before and saved its session
// in the storage of S, and S must find that one.
func Test_Session_Store_PassesOverAnIDThatIsNoSessionToAnOlderOneThatIs(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("sid")})
	storeW := NewStore(Config{Extractor: extractors.FromCookie("sid")})
	storeS := NewStore(Config{Extractor: extractors.FromCookie("sid"), Storage: storeA.Storage})

	c := newRequest(t, app)

	a := mustGet(t, storeA, c)
	a.Set("k", "v")
	require.NoError(t, a.Save())

	w := mustGet(t, storeW, c)
	require.True(t, w.Fresh(), "A's ID is no session of the storage of W")
	require.Same(t, storeW, requestIDs(c).store, "W cached the newest ID")

	s := mustGet(t, storeS, c)
	require.False(t, s.Fresh(), "S stopped at the newest ID, which names no session, and lost the one A saved")
	require.Equal(t, a.ID(), s.ID())
	require.Equal(t, "v", s.Get("k"))
}

// Of the cached IDs that name a stored session the newest one is taken: A1 and A2 read different
// cookies, so each starts a session of its own and saves it, and S reads both.
func Test_Session_Store_TakesTheNewestOfTheSavedIDsItShares(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	storeA1 := NewStore(Config{Extractor: extractors.FromCookie("a1_sid"), Storage: storage})
	storeA2 := NewStore(Config{Extractor: extractors.FromCookie("a2_sid"), Storage: storage})
	storeS := NewStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("a1_sid"), extractors.FromCookie("a2_sid")),
		Storage:   storage,
	})

	c := newRequest(t, fiber.New())
	a1, a2 := saveNew(t, c, storeA1), saveNew(t, c, storeA2)
	require.NotEqual(t, a1.ID(), a2.ID())

	s := mustGet(t, storeS, c)
	require.False(t, s.Fresh())
	require.Equal(t, a2.ID(), s.ID(), "S took the older of the two sessions that were saved")
}

// Regenerate and Reset give the session a new ID, and a store that is asked again looks for the
// session under that one, not under the ID that was deleted: for a visitor who has no cookie yet
// and for one who has.
func Test_Session_Store_Get_AskedAgainFindsTheSessionUnderItsNewID(t *testing.T) {
	t.Parallel()

	for name, change := range map[string]func(*Session) error{"Regenerate": (*Session).Regenerate, "Reset": (*Session).Reset} {
		for _, returning := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, returning visitor %t", name, returning), func(t *testing.T) {
				t.Parallel()

				app := fiber.New()
				store := NewStore()
				c := newRequest(t, app)
				if returning {
					c.Request().Header.SetCookie("session_id", seedSession(t, app, store, func(s *Session) { s.Set("k", "v") }))
				}

				first := mustGet(t, store, c)
				was := first.ID()
				require.NoError(t, change(first))
				require.NotEqual(t, was, first.ID())
				first.Set("after", "change")
				require.NoError(t, first.Save())

				again := mustGet(t, store, c)
				require.Equal(t, first.ID(), again.ID(), "the session was lost under its new ID")
				require.False(t, again.Fresh())
				require.Equal(t, "change", again.Get("after"))
			})
		}
	}
}

// Regenerate of a session the store gave out first does not hide the session it gave out and saved
// since: the entry of the store names the newer one, and the older session is not the one that
// entry is about.
func Test_Session_Store_Get_RegenerateOfAnOlderSessionKeepsTheSavedOne(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore()
	c := newRequest(t, app)

	older := mustGet(t, store, c)
	newer := mustGet(t, store, c)
	require.NoError(t, newer.Save())
	require.NoError(t, older.Regenerate())

	again := mustGet(t, store, c)
	require.Equal(t, newer.ID(), again.ID(), "the saved session was hidden by the regenerated older one")
	require.False(t, again.Fresh())
}
