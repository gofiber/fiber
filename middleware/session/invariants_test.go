package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// The security properties of several stores serving one request, checked against random requests
// instead of hand-picked ones. Whatever cookies, headers and session IDs the client sends, valid,
// foreign, crafted or garbage:
//
//   - a session ID is only ever written to a cookie or header that a store which
//     owns that session reads;
//   - a new session never takes an ID the client supplied (no session fixation);
//   - a store only loads records that a store with the same storage, key prefix
//     and key hashing wrote (no session confusion between stores);
//   - a hashed store keeps the session ID out of its storage key;
//   - stores asked through Store.Get that read a cookie or header in common agree on
//     one session, whatever other stores were asked in between.
//
// The number of seeds is 8, and SESSION_INVARIANT_SEEDS raises it.

// invariantLedger remembers which key policy wrote each storage key, and what the checks found
// wrong where they cannot fail the test themselves, in a handler.
type invariantLedger struct {
	writers    map[string]string
	violations []string
	mu         sync.Mutex
}

func (l *invariantLedger) violatef(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.violations = append(l.violations, fmt.Sprintf(msg, args...))
}

// requireClean fails the test if anything was found wrong so far.
func (l *invariantLedger) requireClean(t *testing.T, where string) {
	t.Helper()

	l.mu.Lock()
	defer l.mu.Unlock()
	require.Empty(t, l.violations, where)
}

// invariantStorage is the storage of one store: it passes everything to a storage that stores may
// share, and tells the ledger what it writes and reads.
type invariantStorage struct {
	fiber.Storage
	ledger *invariantLedger
	group  string
	policy string
}

func (s *invariantStorage) ledgerKey(key string) string {
	return s.group + "\x00" + key
}

func (s *invariantStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.ledger.mu.Lock()
	s.ledger.writers[s.ledgerKey(key)] = s.policy
	s.ledger.mu.Unlock()
	return s.Storage.SetWithContext(ctx, key, val, exp)
}

func (s *invariantStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	val, err := s.Storage.GetWithContext(ctx, key)
	if err == nil && val != nil {
		s.ledger.mu.Lock()
		writer := s.ledger.writers[s.ledgerKey(key)]
		s.ledger.mu.Unlock()
		if writer != s.policy {
			s.ledger.violatef("a store with policy %q loaded key %q, written under policy %q", s.policy, key, writer)
		}
	}
	return val, err
}

// invariantStore is one store of the world, with what the checks need to know.
type invariantStore struct {
	*Store
	cookies []string
	headers []string
}

// shares reports whether the two stores read a cookie or a header in common.
func (a *invariantStore) shares(b *invariantStore) bool {
	return slices.ContainsFunc(append(slices.Clone(a.cookies), a.headers...), func(x string) bool {
		return slices.Contains(b.cookies, x) || slices.Contains(b.headers, x)
	})
}

// invariantWorld is the stores of one run and the IDs of the sessions saved so far, which a client
// can send back.
type invariantWorld struct {
	stores map[string]*invariantStore
	saved  map[string][]string
	ledger *invariantLedger
	rng    *rand.Rand
	// storages are the memory storages by group: the one stores may share, and the one of the
	// stores that keep to themselves.
	storages map[string]*memory.Storage
}

// The stores a request is served with: asked through Store.Get, A twice over, or stacked as session
// middleware.
var (
	storeAPINames   = []string{"A", "A2", "B", "C"}
	middlewareNames = []string{"A", "B", "C"}
)

// newWorld returns a world without stores, which stops the garbage collection of its storages when
// the test ends. A memory storage runs a goroutine of its own until it is closed, so a world that
// is not closed costs two goroutines and what they hold for as long as the process runs, which a
// fuzz target that builds a world for every input adds up to megabytes per second.
func newWorld(tb testing.TB, seed uint64) *invariantWorld {
	tb.Helper()

	w := &invariantWorld{
		stores:   make(map[string]*invariantStore),
		saved:    make(map[string][]string),
		ledger:   &invariantLedger{writers: make(map[string]string)},
		rng:      rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		storages: map[string]*memory.Storage{"shared": memory.New(), "private": memory.New()},
	}
	tb.Cleanup(func() {
		for _, storage := range w.storages {
			require.NoError(tb, storage.Close())
		}
	})
	return w
}

// add makes the store name from cfg, over the storage of the group, and notes the cookies and
// headers it reads. The key policy it tells the ledger it writes under is the group, the prefix and
// the hashing.
func (w *invariantWorld) add(name, group string, cfg *Config, cookies, headers []string) {
	cfg.Storage = &invariantStorage{
		Storage: w.storages[group], ledger: w.ledger, group: group,
		policy: fmt.Sprintf("%s|%s|%t", group, cfg.StorageKeyPrefix, cfg.HashStorageKeys),
	}
	w.stores[name] = &invariantStore{Store: NewStore(*cfg), cookies: cookies, headers: headers}
}

// newInvariantWorld has A and A2, which read the same cookie and share everything, B, which shares
// a storage with them but neither their cookie nor their key prefix, and C, with two places to read
// and a storage of its own.
func newInvariantWorld(tb testing.TB, seed uint64) *invariantWorld {
	tb.Helper()

	w := newWorld(tb, seed)
	for _, name := range []string{"A", "A2"} {
		w.add(name, "shared", &Config{Extractor: extractors.FromCookie("a_sid"), StorageKeyPrefix: "a:"}, []string{"a_sid"}, nil)
	}
	w.add("B", "shared", &Config{Extractor: extractors.FromCookie("b_sid"), StorageKeyPrefix: "b:"}, []string{"b_sid"}, nil)
	// Store C keeps its sessions as records: everything the world checks holds for them too.
	w.add("C", "private", &Config{
		Extractor:       extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C")),
		HashStorageKeys: true,
		Codec:           JSONCodec{},
	}, []string{"c_sid"}, []string{"X-C"})
	return w
}

// newApp returns an app with a session middleware for each of the named stores, over the config the
// store was built with, in that order, and the handler for GET /.
func (w *invariantWorld) newApp(names []string, handler fiber.Handler) *fiber.App {
	app := fiber.New()
	for _, name := range names {
		cfg := w.stores[name].Config
		cfg.Store = w.stores[name].Store
		mw, _ := NewWithStore(cfg)
		app.Use(mw)
	}
	app.Get("/", handler)
	return app
}

// storeAPIApp is an app whose handler gets the session of each store named in the ops of the query
// through Store.Get, gives it data, saves it and reports it.
func (w *invariantWorld) storeAPIApp() *fiber.App {
	var visits atomic.Int32
	return w.newApp(nil, func(c fiber.Ctx) error {
		var rec invariantRecord
		for name := range strings.SplitSeq(c.Query("ops"), ",") {
			sess, err := w.stores[name].Get(c)
			if err != nil {
				return err
			}
			sess.Set("n", int(visits.Add(1)))
			if err := sess.Save(); err != nil {
				sess.Release()
				return err
			}
			rec.Sessions = append(rec.Sessions, invariantSession{Store: name, ID: sess.ID(), Fresh: sess.Fresh()})
			sess.Release()
		}
		return c.JSON(rec)
	})
}

// middlewareApp is an app with a session middleware for each of A, B and C. Its handler reports the
// session of every one of them through Store.FromContext, and gives the ones named in the ops of
// the query some data.
func (w *invariantWorld) middlewareApp() *fiber.App {
	return w.newApp(middlewareNames, func(c fiber.Ctx) error {
		// Every middleware of the stack runs and saves its session, whichever the handler touches:
		// all of them are reported, and the ops pick the ones that get data.
		touched := strings.Split(c.Query("ops"), ",")
		var rec invariantRecord
		for _, name := range middlewareNames {
			mw := w.stores[name].FromContext(c)
			if mw == nil {
				return fmt.Errorf("no middleware for store %s", name)
			}
			if slices.Contains(touched, name) {
				mw.Set("n", 1)
			}
			rec.Sessions = append(rec.Sessions, invariantSession{Store: name, ID: mw.ID(), Fresh: mw.Fresh()})
		}
		return c.JSON(rec)
	})
}

// clientValue is what the client sends for one cookie, header or query value: nothing, the ID of a
// session that store `own` saved, a crafted variation of it, or garbage.
func (w *invariantWorld) clientValue(own string) string {
	pick := func(store string) string {
		ids := w.saved[store]
		if len(ids) == 0 {
			return "unknown"
		}
		return ids[w.rng.IntN(len(ids))]
	}
	names := []string{"A", "B", "C"}
	switch w.rng.IntN(10) {
	case 0, 1, 2:
		return ""
	case 3, 4, 5:
		return pick(own)
	case 6:
		return pick(names[w.rng.IntN(len(names))]) // a session of some store, whoever's it is
	case 7:
		return "b:" + pick("B") // as another store's key would read
	case 8:
		return "a:" + pick("A")
	default:
		return strings.Repeat("g", 1+w.rng.IntN(300)) + ":" + strconv.Itoa(w.rng.IntN(10))
	}
}

// invariantRecord is what the handler reports about the sessions it used.
type invariantRecord struct {
	Sessions []invariantSession `json:"sessions"`
}

type invariantSession struct {
	Store string `json:"store"`
	ID    string `json:"id"`
	Fresh bool   `json:"fresh"`
}

// decodeRecord reads what the handler reported. A response that is not a 200 reports nothing.
func decodeRecord(t *testing.T, status int, body []byte) invariantRecord {
	t.Helper()

	var rec invariantRecord
	if status == fiber.StatusOK {
		require.NoError(t, json.Unmarshal(body, &rec), "%s", body)
	}
	return rec
}

// invariantRequest is one request of the client.
type invariantRequest struct {
	cookies map[string]string
	header  string
	ops     []string
}

func (w *invariantWorld) newRequest(opNames []string) invariantRequest {
	req := invariantRequest{cookies: make(map[string]string)}
	for _, store := range []string{"A", "B", "C"} {
		if v := w.clientValue(store); v != "" {
			req.cookies[w.stores[store].cookies[0]] = v
		}
	}
	req.header = w.clientValue("C")
	for range 1 + w.rng.IntN(5) {
		req.ops = append(req.ops, opNames[w.rng.IntN(len(opNames))])
	}
	return req
}

func (r invariantRequest) where(step int) string {
	return fmt.Sprintf("request %d (ops %v, cookies %v, header %q)", step, r.ops, r.cookies, r.header)
}

func (r invariantRequest) send(t *testing.T, app *fiber.App) (*http.Response, invariantRecord) {
	t.Helper()

	httpReq := httptest.NewRequest(fiber.MethodGet, "/?ops="+strings.Join(r.ops, ","), http.NoBody)
	for name, value := range r.cookies {
		httpReq.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	if r.header != "" {
		httpReq.Header.Set("X-C", r.header)
	}
	resp, err := app.Test(httpReq)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp, decodeRecord(t, resp.StatusCode, body)
}

// sendDirect is send without the network round trip app.Test makes: the request goes to the handler
// of the app as a fasthttp request, which is several times cheaper and what a fuzzer needs. Every
// Set-Cookie of the response has to be readable by fasthttp and by net/http, whose reading the
// checks rely on.
func (r invariantRequest) sendDirect(t *testing.T, handler fasthttp.RequestHandler) (*http.Response, invariantRecord) {
	t.Helper()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/?ops=" + strings.Join(r.ops, ","))
	for name, value := range r.cookies {
		fctx.Request.Header.SetCookie(name, value)
	}
	if r.header != "" {
		fctx.Request.Header.Set("X-C", r.header)
	}
	handler(fctx)

	resp := &http.Response{StatusCode: fctx.Response.StatusCode(), Header: make(http.Header)}
	for _, raw := range fctx.Response.Header.Cookies() {
		cookie := fasthttp.AcquireCookie()
		require.NoError(t, cookie.ParseBytes(raw), "unparsable Set-Cookie %q", raw)
		fasthttp.ReleaseCookie(cookie)
		resp.Header.Add("Set-Cookie", string(raw))
	}
	// The checks look at the cookies through net/http, which drops one it cannot read: it would escape them.
	require.Len(t, resp.Cookies(), len(resp.Header.Values("Set-Cookie")), "net/http cannot read a Set-Cookie of the response")
	resp.Header.Set("X-C", string(fctx.Response.Header.Peek("X-C")))

	return resp, decodeRecord(t, resp.StatusCode, fctx.Response.Body())
}

// requireNoFixation holds the sessions of the request against the property that a new session never
// takes an ID the client supplied.
func (r invariantRequest) requireNoFixation(t *testing.T, where string, rec invariantRecord) {
	t.Helper()

	supplied := make(map[string]bool)
	for _, v := range r.cookies {
		supplied[v] = true
	}
	if r.header != "" {
		supplied[r.header] = true
	}
	for _, s := range rec.Sessions {
		if s.Fresh {
			require.False(t, supplied[s.ID], "%s: a new session of store %s took the ID the client supplied: %q", where, s.Store, s.ID)
		}
	}
}

// requireOwned holds the cookies and the header X-C that a response sets against the sessions of
// the request: a session ID is only written to a cookie or header that a store which owns that
// session reads.
func (w *invariantWorld) requireOwned(t *testing.T, where string, rec invariantRecord, cookies []*http.Cookie, header string) {
	t.Helper()

	owns := func(name, value string, read func(*invariantStore) []string) bool {
		return slices.ContainsFunc(rec.Sessions, func(s invariantSession) bool {
			return s.ID == value && slices.Contains(read(w.stores[s.Store]), name)
		})
	}
	for _, cookie := range cookies {
		require.True(t, owns(cookie.Name, cookie.Value, func(s *invariantStore) []string { return s.cookies }),
			"%s: cookie %s=%q was written, but no store reading that cookie owns that session ID (sessions: %v)", where, cookie.Name, cookie.Value, rec.Sessions)
	}
	if header != "" {
		require.True(t, owns("X-C", header, func(s *invariantStore) []string { return s.headers }),
			"%s: header X-C=%q was written, but no store reading that header owns that session ID", where, header)
	}
}

// check holds the response and what the handler saw against the properties above.
func (w *invariantWorld) check(t *testing.T, step int, req invariantRequest, resp *http.Response, rec invariantRecord) {
	t.Helper()
	where := req.where(step)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, where)

	// A session the client names with a valid ID in the cookie its store reads is found, whatever
	// else the request carries: another store's cached ID does not take its place.
	for _, sess := range rec.Sessions {
		store := w.stores[sess.Store]
		expected := req.cookies[store.cookies[0]]
		if expected == "" && len(store.headers) > 0 {
			expected = req.header
		}
		if expected == "" || !slices.Contains(w.saved[strings.TrimSuffix(sess.Store, "2")], expected) {
			continue
		}
		require.False(t, sess.Fresh, "%s: store %s did not find the session its client named: %q", where, sess.Store, expected)
		require.Equal(t, expected, sess.ID, where)
	}

	req.requireNoFixation(t, where, rec)

	// Stores that read a cookie or a header in common are one visitor's session, whatever other
	// stores were asked in between: a store asked again, or after one that shares with it, finds
	// the session that one saved, and does not start another. Stores that read none in common keep
	// sessions of their own: they never come out with the same ID, which is what a store adopting
	// the ID another one cached on the request would do.
	for i, later := range rec.Sessions {
		for _, earlier := range rec.Sessions[:i] {
			if !w.stores[earlier.Store].shares(w.stores[later.Store]) {
				require.NotEqual(t, earlier.ID, later.ID, "%s: stores %s and %s, which share no cookie or header, ended up with the same ID %q", where, earlier.Store, later.Store, earlier.ID)
				continue
			}
			require.Equal(t, earlier.ID, later.ID, "%s: store %s, asked after store %s which shares a cookie or header with it, did not find the session that one saved", where, later.Store, earlier.Store)
			require.False(t, later.Fresh, "%s: store %s started a new session, though store %s had saved one", where, later.Store, earlier.Store)
		}
	}

	w.requireOwned(t, where, rec, resp.Cookies(), resp.Header.Get("X-C"))

	// A store only loads what a store with its key policy wrote.
	w.ledger.requireClean(t, where)

	// What was saved reaches the client: the last session saved for a cookie has its ID in that
	// cookie, or in the header of the store, since every save writes it.
	last := make(map[string]invariantSession)
	for _, sess := range rec.Sessions {
		last[w.stores[sess.Store].cookies[0]] = sess
	}
	for cookieName, sess := range last {
		written := slices.ContainsFunc(resp.Cookies(), func(c *http.Cookie) bool { return c.Name == cookieName && c.Value == sess.ID }) ||
			slices.ContainsFunc(w.stores[sess.Store].headers, func(h string) bool { return resp.Header.Get(h) == sess.ID })
		require.True(t, written, "%s: the session %q of store %s was saved, but its ID reached the client in none of the cookies and headers of the store", where, sess.ID, sess.Store)
	}

	// A hashed store keeps the ID out of its storage key.
	for _, sess := range rec.Sessions {
		if sess.Store == "C" {
			key := w.stores["C"].storageKey(sess.ID)
			require.NotContains(t, key, sess.ID, where)
			require.Len(t, key, len(keyNamespace)+hashedKeyLen, where)
		}
	}

	// What was saved can be sent back.
	for _, sess := range rec.Sessions {
		owner := strings.TrimSuffix(sess.Store, "2")
		w.saved[owner] = append(w.saved[owner], sess.ID)
	}
}

func invariantSeeds() uint64 {
	if n, err := strconv.ParseUint(os.Getenv("SESSION_INVARIANT_SEEDS"), 10, 64); err == nil && n > 0 {
		return n
	}
	return 8
}

// randomRequests sends 150 random requests for each of the seeds, to an app of its own that newApp
// makes over a world of its own, and holds every response against the properties. The ops of a
// request are drawn from the names.
func randomRequests(t *testing.T, newApp func(*invariantWorld) *fiber.App, names []string) {
	t.Helper()

	for seed := range invariantSeeds() {
		t.Run("seed="+strconv.FormatUint(seed, 10), func(t *testing.T) {
			t.Parallel()

			w := newInvariantWorld(t, seed)
			app := newApp(w)

			for step := range 150 {
				req := w.newRequest(names)
				resp, rec := req.send(t, app)
				w.check(t, step, req, resp, rec)
			}
		})
	}
}

// Several stores used through Store.Get, in random order and repeatedly, within one request.
func Test_Session_Invariants_StoreAPI(t *testing.T) {
	t.Parallel()

	randomRequests(t, (*invariantWorld).storeAPIApp, storeAPINames)
}

// The same stores as stacked session middleware, reached through Store.FromContext.
func Test_Session_Invariants_Middleware(t *testing.T) {
	t.Parallel()

	randomRequests(t, (*invariantWorld).middlewareApp, middlewareNames)
}
