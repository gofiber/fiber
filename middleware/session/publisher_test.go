package session

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// A store takes the ID that a store sharing a source with it cached, and never that
// of one it shares none with. A and E cache a new ID each, B finds a session through
// its cookie; G reads the cookies of A and of B, so it shares with A, whose ID was
// never saved and so names no session, and with the store that found its own, but not
// with E, and must find its own session instead of taking the ID that E cached.
func Test_Session_Store_TakesNoIDOfAStoreItSharesNothingWith(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: shared})
	storeE := NewStore(Config{Extractor: extractors.FromCookie("e_sid")})
	storeG := NewStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("a_sid"), extractors.FromCookie("b_sid")),
		Storage:   shared,
	})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("k", "v") })

	c := newRequest(t, app)
	c.Request().Header.SetCookie("b_sid", idB)

	a, err := storeA.Get(c)
	require.NoError(t, err)
	require.True(t, a.Fresh(), "A caches an ID first")
	b, err := storeB.Get(c)
	require.NoError(t, err)
	require.False(t, b.Fresh())
	require.Equal(t, idB, b.ID(), "B finds its session, and caches nothing")
	e, err := storeE.Get(c)
	require.NoError(t, err)
	require.True(t, e.Fresh(), "E caches an ID after A")

	g, err := storeG.Get(c)
	require.NoError(t, err)
	require.NotEqual(t, e.ID(), g.ID(), "G took the ID that E cached, though it reads none of E's cookies")
	require.False(t, g.Fresh(), "G found the session its b_sid cookie names")
	require.Equal(t, idB, g.ID())
}

// An ID that a store sharing a source with it cached and never saved names no
// session, and does not hide the one the request names: A caches an ID it does not
// save, and G, which reads A's cookie and B's, finds the session that the cookie of B
// names, where taking the ID of A, the newest, would start a session of its own.
func Test_Session_Store_PassesOverAnUnsavedIDOfAStoreItShares(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: shared})
	storeG := NewStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("a_sid"), extractors.FromCookie("b_sid")),
		Storage:   shared,
	})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("k", "v") })

	c := newRequest(t, app)
	c.Request().Header.SetCookie("b_sid", idB)

	a := mustGet(t, storeA, c)
	require.True(t, a.Fresh(), "A caches an ID, and does not save it")

	g := mustGet(t, storeG, c)
	require.NotEqual(t, a.ID(), g.ID(), "G took the ID that A cached, which names no session")
	require.False(t, g.Fresh(), "G found the session its b_sid cookie names")
	require.Equal(t, idB, g.ID())
	require.Equal(t, "v", g.Get("k"))
}

// The IDs of a request are kept newest first, and a store passes over the ones that
// name no session. W, X and Y cache an ID each, none of them saved, and R, which shares
// a cookie with X but none with Y, takes neither: it finds its own session.
func Test_Session_Store_CachedIDsAreKeptNewestFirst(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	app := fiber.New()
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: shared})
	storeW := NewStore(Config{Extractor: extractors.FromCookie("w_sid")})
	storeX := NewStore(Config{Extractor: extractors.FromCookie("x_sid")})
	storeY := NewStore(Config{Extractor: extractors.FromCookie("y_sid")})
	storeR := NewStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("b_sid"), extractors.FromCookie("x_sid")),
		Storage:   shared,
	})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("k", "v") })

	c := newRequest(t, app)
	c.Request().Header.SetCookie("b_sid", idB)

	for _, store := range []*Store{storeB, storeW, storeX, storeY} {
		sess, err := store.Get(c)
		require.NoError(t, err)
		sess.Release()
	}
	newest := requestIDs(c)
	require.NotNil(t, newest)
	require.Same(t, storeY, newest.store, "the later one comes first")
	require.Same(t, storeX, newest.next.store)
	require.Same(t, storeW, newest.next.next.store)
	require.Nil(t, newest.next.next.next, "B found its session and cached nothing")

	r := mustGet(t, storeR, c)
	require.False(t, r.Fresh(), "R shares X's cookie but not Y's, and must find its own session")
	require.Equal(t, idB, r.ID())
}

// A new session whose cookie names an ID the storage does not know is cached with the
// others of the request, as the session of a cookie that names a stored one is not.
// W and X cache an ID first, none of them saved; Y, whose cookie is unknown, caches
// one after them and saves it; R shares Y's cookie but neither W's nor X's, so it must
// take the ID that Y cached, and not find its own session as it would if Y had
// cached nothing.
func Test_Session_Store_FreshSessionOfAnUnknownCookieIsCached(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	app := fiber.New()
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: shared})
	storeW := NewStore(Config{Extractor: extractors.FromCookie("w_sid")})
	storeX := NewStore(Config{Extractor: extractors.FromCookie("x_sid")})
	storeY := NewStore(Config{Extractor: extractors.FromCookie("y_sid"), Storage: shared})
	storeR := NewStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("b_sid"), extractors.FromCookie("y_sid")),
		Storage:   shared,
	})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("k", "v") })

	c := newRequest(t, app)
	c.Request().Header.SetCookie("b_sid", idB)
	c.Request().Header.SetCookie("y_sid", "names-no-session")

	for _, store := range []*Store{storeB, storeW, storeX} {
		sess, err := store.Get(c)
		require.NoError(t, err)
		sess.Release()
	}
	y := mustGet(t, storeY, c)
	require.True(t, y.Fresh(), "Y's cookie names no stored session")
	require.NoError(t, y.Save())
	require.Same(t, storeY, requestIDs(c).store, "Y cached its ID after W and X")

	r := mustGet(t, storeR, c)
	require.Equal(t, y.ID(), r.ID(), "R shares Y's cookie, and Y cached last")
	require.NotEqual(t, idB, r.ID(), "R went to its own cookie instead of the ID that Y cached")
	require.False(t, r.Fresh(), "R found the session that Y saved")
}

// A middleware that starts after a Store.Get cached an ID that was never saved finds
// the session its own cookie names, though the store of that ID shares a cookie with
// it: the ID names no session.
func Test_Session_Store_MiddlewarePassesOverAnIDThatNamesNoSession(t *testing.T) {
	t.Parallel()

	earlier := NewStore(Config{Extractor: extractors.FromCookie("s2a_sid")})
	handler1, _ := NewWithStore(Config{Extractor: extractors.FromCookie("s1_sid")})
	handler2, store2 := NewWithStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("s2a_sid"), extractors.FromCookie("s2b_sid")),
	})

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		sess, err := earlier.Get(c) // fresh: it caches the new ID, which is never saved
		if err != nil {
			return err
		}
		sess.Release()
		return c.Next()
	})
	app.Use(handler1, handler2)
	app.Get("/seed", func(c fiber.Ctx) error {
		store2.FromContext(c).Set("who", "s2")
		return c.SendString(store2.FromContext(c).ID())
	})
	app.Get("/read", func(c fiber.Ctx) error {
		m := store2.FromContext(c)
		return c.SendString(fmt.Sprintf("%s|%t", m.ID(), m.Fresh()))
	})

	_, id := sendRequest(t, app, "/seed", nil)
	_, body := sendRequest(t, app, "/read", map[string]string{"s2b_sid": id})
	require.Equal(t, id+"|false", body, "the second middleware found its own session instead of taking the ID that the first cached")
}

// Store.Get says which store cached a new ID, with the ID and where the request named
// it, whether or not the client sent an ID, and a store that caches again replaces its
// entry and comes first.
func Test_Session_Store_Get_CachesTheNewIDWithItsStore(t *testing.T) {
	t.Parallel()

	store := NewStore()
	other := NewStore(Config{Extractor: extractors.FromCookie("other_sid")})
	app := fiber.New()

	withID := newRequest(t, app)
	withID.Request().Header.SetCookie("session_id", "names-no-session")
	sess := mustGet(t, store, withID)
	require.True(t, sess.Fresh())
	cached := requestIDs(withID)
	require.NotNil(t, cached)
	require.Same(t, store, cached.store)
	require.Equal(t, sess.ID(), cached.id)
	require.Equal(t, extractors.Result{Value: "names-no-session", Key: "session_id", Source: extractors.SourceCookie, Resolved: true}, cached.res,
		"the provenance of the ID the client sent, which the new one is written back to")
	require.Nil(t, cached.next)
	require.Equal(t, sess.ID(), withID.Locals(sessionIDContextKey), "and it is the ID the logger shows")

	without := newRequest(t, app)
	first := mustGet(t, store, without)
	require.True(t, first.Fresh())
	require.Zero(t, requestIDs(without).res, "the client sent no ID")
	mustGet(t, other, without)
	again := mustGet(t, store, without)
	require.True(t, again.Fresh(), "the first ID was never saved")
	require.NotEqual(t, first.ID(), again.ID())

	newest := requestIDs(without)
	require.Same(t, store, newest.store, "a store that caches again comes first")
	require.Equal(t, again.ID(), newest.id)
	require.Same(t, other, newest.next.store)
	require.Nil(t, newest.next.next, "and has one entry")
}

// The list of IDs goes back to a pool shared by every request, whole: it must hold
// nothing of the request it served.
//
// Not parallel: it swaps the shared pool for its own, to see what Close hands back.
func Test_CachedID_CloseForgetsEverything(t *testing.T) {
	shared := cachedIDPool
	cachedIDPool = &sync.Pool{New: func() any { return new(cachedID) }}
	t.Cleanup(func() { cachedIDPool = shared })

	res := extractors.Result{Value: "v", Key: "k", Source: extractors.SourceCookie, Resolved: true}
	oldest := &cachedID{store: &Store{}, id: "a", res: res}
	middle := &cachedID{next: oldest, store: &Store{}, id: "b", res: res}
	newest := &cachedID{next: middle, store: &Store{}, id: "c", res: res}
	require.NoError(t, newest.Close())
	for _, entry := range []*cachedID{oldest, middle, newest} {
		require.Equal(t, cachedID{}, *entry)
	}
}

// Stores are told apart by every source they read, by kind and name, whether or
// not it can be written back to. Stores over the same query parameter, the same
// authentication scheme or custom extractors with the same key share the ID cached
// on the request; a store over the query string and a store over a cookie do not,
// so the ID a link carries reaches the stores that read the query string and no
// cookie store next to them.
func Test_Session_Store_SourcesAreToldApartByKindAndName(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	app := fiber.New()
	custom := func(fiber.Ctx) (string, error) { return "", extractors.ErrNotFound }
	byQuery := NewStore(Config{Extractor: extractors.FromQuery("sid"), Storage: shared})
	bySameQuery := NewStore(Config{Extractor: extractors.FromQuery("sid"), Storage: shared})
	byOtherQuery := NewStore(Config{Extractor: extractors.FromQuery("token"), Storage: shared})
	byAuth := NewStore(Config{Extractor: extractors.FromAuthHeader("Bearer"), Storage: shared})
	byAuthLower := NewStore(Config{Extractor: extractors.FromAuthHeader("bearer"), Storage: shared})
	byToken := NewStore(Config{Extractor: extractors.FromAuthHeader("Token"), Storage: shared})
	byCookie := NewStore(Config{Extractor: extractors.FromCookie("c_sid"), Storage: shared})
	byCustom := NewStore(Config{Extractor: extractors.FromCustom("legacy", custom), Storage: shared})
	bySameCustom := NewStore(Config{Extractor: extractors.FromCustom("legacy", custom), Storage: shared})
	byOtherCustom := NewStore(Config{Extractor: extractors.FromCustom("other", custom), Storage: shared})
	byForm := NewStore(Config{Extractor: extractors.FromForm("sid"), Storage: shared})
	bySameForm := NewStore(Config{Extractor: extractors.FromForm("sid"), Storage: shared})
	byParam := NewStore(Config{Extractor: extractors.FromParam("sid"), Storage: shared})
	bySameParam := NewStore(Config{Extractor: extractors.FromParam("sid"), Storage: shared})

	for _, pair := range [][2]*Store{
		{byQuery, bySameQuery},
		{byAuth, byAuthLower},
		{byCustom, bySameCustom},
		{byForm, bySameForm},
		{byParam, bySameParam},
	} {
		require.True(t, pair[0].sharesSource(pair[1]), "stores over the same source share")
	}
	for _, pair := range [][2]*Store{
		{byQuery, byOtherQuery},
		{byQuery, byAuth},
		{byQuery, byCookie},
		{byCookie, byQuery},
		{byAuth, byCookie},
		{byAuth, byToken}, // the same header, another scheme
		{byCustom, byOtherCustom},
		{byCustom, byQuery},
		{byForm, byQuery}, // the same name in another kind is another source
		{byParam, byForm},
		{byParam, byCookie},
	} {
		require.False(t, pair[0].sharesSource(pair[1]), "stores over different sources are told apart")
	}

	// A new visitor: a store over the same source takes the ID the first store
	// saved, a store over another source starts a session of its own.
	visitor := newRequest(t, app)
	first := mustGet(t, byQuery, visitor)
	first.Set("k", "v")
	require.NoError(t, first.Save())
	second := mustGet(t, bySameQuery, visitor)
	require.Equal(t, first.ID(), second.ID(), "a store over the same query parameter took the ID the first one saved")
	require.False(t, second.Fresh())
	third := mustGet(t, byCookie, visitor)
	require.NotEqual(t, first.ID(), third.ID(), "a cookie store next to them took the ID too")
	require.True(t, third.Fresh())

	// A session that a link names is served by the store that reads the query
	// string; the cookie store behind it keeps to its own cookie, and starts a
	// session of its own for a visitor who has none.
	id := seedSession(t, app, byQuery, func(s *Session) { s.Set("k", "v") })
	queryHandler, queryStore := NewWithStore(Config{Extractor: extractors.FromQuery("sid"), Storage: shared})
	cookieHandler, cookieStore := NewWithStore(Config{Extractor: extractors.FromCookie("c_sid"), Storage: shared})
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

// Config is embedded, so its fields can be assigned after NewStore. A store whose
// Extractor was replaced writes the ID to, and shares with, what it reads now.
func Test_Store_ExtractorReplacedAfterNewStore(t *testing.T) {
	t.Parallel()

	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	store.Extractor = extractors.FromCookie("b_sid")

	app := fiber.New()
	c := newRequest(t, app)
	sess := mustGet(t, store, c)
	require.NoError(t, sess.Save())

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

	// A chain reports the key and the source of its first child, so a cookie replaced
	// by a header of the same name, and a leaf replaced by a chain that starts with it,
	// differ in one thing each, and each is another extractor.
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

// An ID that names no session of the store is passed over to the next one, and not only
// to the request: W, the newest to cache an ID, reads the cookie S reads but keeps its
// sessions in a storage of its own, so its ID is no session of S's; A cached an ID
// before and saved its session in the storage of S, and S must find that one.
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

// Of the cached IDs that name a stored session the newest one is taken: A1 and A2 read
// different cookies, so each starts a session of its own and saves it, and S reads both.
func Test_Session_Store_TakesTheNewestOfTheSavedIDsItShares(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	app := fiber.New()
	storeA1 := NewStore(Config{Extractor: extractors.FromCookie("a1_sid"), Storage: storage})
	storeA2 := NewStore(Config{Extractor: extractors.FromCookie("a2_sid"), Storage: storage})
	storeS := NewStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("a1_sid"), extractors.FromCookie("a2_sid")),
		Storage:   storage,
	})

	c := newRequest(t, app)

	saved := func(store *Store) *Session {
		sess, err := store.Get(c)
		require.NoError(t, err)
		t.Cleanup(sess.Release)
		require.True(t, sess.Fresh())
		require.NoError(t, sess.Save())
		return sess
	}
	a1, a2 := saved(storeA1), saved(storeA2)
	require.NotEqual(t, a1.ID(), a2.ID())

	s := mustGet(t, storeS, c)
	require.False(t, s.Fresh())
	require.Equal(t, a2.ID(), s.ID(), "S took the older of the two sessions that were saved")
}

// Regenerate and Reset give the session a new ID, and a store that is asked again looks
// for the session under that one, not under the ID that was deleted: for a visitor who
// has no cookie yet and for one who has.
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
