package session

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// seedStoredSession saves a session with data in store and returns its ID.
func seedStoredSession(t *testing.T, app *fiber.App, store *Store) string {
	t.Helper()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	sess, err := store.Get(c)
	require.NoError(t, err)
	defer sess.Release()
	sess.Set("k", "v")
	require.NoError(t, sess.Save())
	return sess.ID()
}

// The newest publication of a cached session ID decides who may reuse it, whether
// it went to the provenance box of the request or, while the request has none, to a
// local of its own. A has no box to say so in, B's cookie then creates one, and E
// publishes into it; G reads the cookies of A and of B, so it shares with the older
// publisher and with the store that resolved its own ID, but not with E, and must
// find its own session instead of taking the ID that E cached.
func Test_Session_Store_NewestPublisherDecidesWhoMayReuseTheID(t *testing.T) {
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
	idB := seedStoredSession(t, app, storeB)

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("b_sid", idB)

	a, err := storeA.Get(c)
	require.NoError(t, err)
	require.True(t, a.Fresh(), "A publishes first, with no box in the request")
	b, err := storeB.Get(c)
	require.NoError(t, err)
	require.False(t, b.Fresh())
	require.Equal(t, idB, b.ID(), "B's cookie creates the box")
	e, err := storeE.Get(c)
	require.NoError(t, err)
	require.True(t, e.Fresh(), "E publishes into the box")

	g, err := storeG.Get(c)
	require.NoError(t, err)
	require.NotEqual(t, e.ID(), g.ID(), "G took the ID that E cached, though it reads none of E's cookies")
	require.False(t, g.Fresh(), "G found the session its b_sid cookie names")
	require.Equal(t, idB, g.ID())
}

// Of two publications that go to the box, the later one counts. W caches the first
// ID, which a request with a box but no cached ID leaves in the owner local; X and Y
// then publish into the box, and R, which shares a cookie with X but none with Y,
// must not take the ID that Y cached.
func Test_Session_Store_LaterPublicationToTheBoxCounts(t *testing.T) {
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
	idB := seedStoredSession(t, app, storeB)

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("b_sid", idB)

	for _, store := range []*Store{storeB, storeW, storeX, storeY} {
		sess, err := store.Get(c)
		require.NoError(t, err)
		sess.Release()
	}
	box, ok := c.Locals(sessionExtractorContextKey).(*provenanceBox)
	require.True(t, ok)
	require.Same(t, storeY, box.publisher, "the later publication is the one recorded")

	r, err := storeR.Get(c)
	require.NoError(t, err)
	defer r.Release()
	require.False(t, r.Fresh(), "R shares X's cookie but not Y's, and must find its own session")
	require.Equal(t, idB, r.ID())
}

// A new session whose cookie names an ID the storage does not know is published to
// the box the request already has, as the session of a cookie that names a stored
// one is. W and X publish into the box first; Y, whose cookie is unknown, publishes
// after them; R shares Y's cookie but neither W's nor X's, so it must take the ID
// that Y cached, and not find its own session as it would if the box still named X.
func Test_Session_Store_FreshSessionOfAnUnknownCookiePublishesToTheBox(t *testing.T) {
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
	idB := seedStoredSession(t, app, storeB)

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("b_sid", idB)
	c.Request().Header.SetCookie("y_sid", "names-no-session")

	for _, store := range []*Store{storeB, storeW, storeX} {
		sess, err := store.Get(c)
		require.NoError(t, err)
		sess.Release()
	}
	y, err := storeY.Get(c)
	require.NoError(t, err)
	defer y.Release()
	require.True(t, y.Fresh(), "Y's cookie names no stored session")
	require.NoError(t, y.Save())
	box, ok := c.Locals(sessionExtractorContextKey).(*provenanceBox)
	require.True(t, ok)
	require.Same(t, storeY, box.publisher, "Y published into the box, after W and X")

	r, err := storeR.Get(c)
	require.NoError(t, err)
	defer r.Release()
	require.Equal(t, y.ID(), r.ID(), "R shares Y's cookie, and Y published last")
	require.NotEqual(t, idB, r.ID(), "R went to its own cookie instead of the ID that Y cached")
	require.False(t, r.Fresh(), "R found the session that Y saved")
}

// A middleware that starts after a Store.Get published without a box is the
// publisher of the ID it caches, not the store of the earlier one.
func Test_Session_Store_MiddlewareIsThePublisherOverTheOwnerLocal(t *testing.T) {
	t.Parallel()

	earlier := NewStore(Config{Extractor: extractors.FromCookie("s2a_sid")})
	handler1, _ := NewWithStore(Config{Extractor: extractors.FromCookie("s1_sid")})
	handler2, store2 := NewWithStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("s2a_sid"), extractors.FromCookie("s2b_sid")),
	})

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		sess, err := earlier.Get(c) // fresh, with no box: it leaves its store in the owner local
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

// Store.Get says which store cached a new ID in the provenance box of the request
// when the request has one, as it has when the client sent an ID, and in a local of
// its own when it has none.
func Test_Session_Store_Get_RecordsThePublisherWhereTheRequestCanHoldIt(t *testing.T) {
	t.Parallel()

	store := NewStore()
	app := fiber.New()

	withID := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(withID)
	withID.Request().Header.SetCookie("session_id", "names-no-session")
	sess, err := store.Get(withID)
	require.NoError(t, err)
	defer sess.Release()
	require.True(t, sess.Fresh())
	box, ok := withID.Locals(sessionExtractorContextKey).(*provenanceBox)
	require.True(t, ok)
	require.Same(t, store, box.publisher)
	require.Nil(t, withID.Locals(sessionOwnerContextKey), "the box says it already")

	without := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(without)
	other, err := store.Get(without)
	require.NoError(t, err)
	defer other.Release()
	require.True(t, other.Fresh())
	_, hasBox := without.Locals(sessionExtractorContextKey).(*provenanceBox)
	require.False(t, hasBox)
	require.Same(t, store, without.Locals(sessionOwnerContextKey))
}

// The box goes back to a pool shared by every request: it must hold nothing of the
// request it served.
//
// Not parallel: it swaps the shared pool for its own, to see what Close hands back.
func Test_ProvenanceBox_CloseForgetsEverything(t *testing.T) {
	shared := provenancePool
	provenancePool = &sync.Pool{New: func() any { return new(provenanceBox) }}
	t.Cleanup(func() { provenancePool = shared })

	box := &provenanceBox{
		owner:     &Store{},
		publisher: &Store{},
		res:       extractors.Result{Value: "v", Key: "k", Source: extractors.SourceCookie, Resolved: true},
	}
	require.NoError(t, box.Close())
	require.Equal(t, provenanceBox{}, *box)
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
	visitor := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(visitor)
	first, err := byQuery.Get(visitor)
	require.NoError(t, err)
	defer first.Release()
	first.Set("k", "v")
	require.NoError(t, first.Save())
	second, err := bySameQuery.Get(visitor)
	require.NoError(t, err)
	defer second.Release()
	require.Equal(t, first.ID(), second.ID(), "a store over the same query parameter took the ID the first one saved")
	require.False(t, second.Fresh())
	third, err := byCookie.Get(visitor)
	require.NoError(t, err)
	defer third.Release()
	require.NotEqual(t, first.ID(), third.ID(), "a cookie store next to them took the ID too")
	require.True(t, third.Fresh())

	// A session that a link names is served by the store that reads the query
	// string; the cookie store behind it keeps to its own cookie, and starts a
	// session of its own for a visitor who has none.
	id := seedStoredSession(t, app, byQuery)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	sess, err := store.Get(c)
	require.NoError(t, err)
	defer sess.Release()
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
