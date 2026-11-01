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
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// The security properties of several stores serving one request, checked against
// random requests instead of hand-picked ones. Whatever cookies, headers and
// session IDs the client sends, valid, foreign, crafted or garbage:
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

// invariantLedger remembers which key policy wrote each storage key.
type invariantLedger struct {
	writers    map[string]string
	violations []string
	mu         sync.Mutex
}

// invariantStorage is the storage of one store: it passes everything to a storage
// that stores may share, and tells the ledger what it writes and reads.
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
		if writer := s.ledger.writers[s.ledgerKey(key)]; writer != s.policy {
			s.ledger.violations = append(s.ledger.violations,
				fmt.Sprintf("a store with policy %q loaded key %q, written under policy %q", s.policy, key, writer))
		}
		s.ledger.mu.Unlock()
	}
	return val, err
}

// invariantStore is one store of the world, with what the checks need to know.
type invariantStore struct {
	*Store
	cookies []string
	headers []string
}

// invariantWorld is the stores of one run and the IDs of the sessions saved so far,
// which a client can send back.
type invariantWorld struct {
	stores   map[string]*invariantStore
	saved    map[string][]string
	ledger   *invariantLedger
	rng      *rand.Rand
	storages []*memory.Storage
	// frozen stops check from adding the sessions it sees to saved, so that a test
	// that sends the same fixed set of IDs again and again keeps them few.
	frozen bool
}

// shutdown stops the garbage collection of the storages of the world. A memory storage
// runs a goroutine of its own until it is closed, so a world that is not closed
// costs two goroutines and what they hold for as long as the process runs, which a
// fuzz target that builds a world for every input adds up to megabytes per second.
func (w *invariantWorld) shutdown(tb testing.TB) {
	tb.Helper()

	for _, storage := range w.storages {
		require.NoError(tb, storage.Close())
	}
}

func newInvariantWorld(seed uint64) *invariantWorld {
	ledger := &invariantLedger{writers: make(map[string]string)}
	shared, private := memory.New(), memory.New()
	stores := make(map[string]*invariantStore)
	add := func(name string, ext extractors.Extractor, storage fiber.Storage, group, prefix string, hashed bool, codec Codec, cookies, headers []string) {
		policy := fmt.Sprintf("%s|%s|%t", group, prefix, hashed)
		stores[name] = &invariantStore{
			Store: NewStore(Config{
				Extractor:        ext,
				Storage:          &invariantStorage{Storage: storage, ledger: ledger, group: group, policy: policy},
				StorageKeyPrefix: prefix,
				HashStorageKeys:  hashed,
				Codec:            codec,
			}),
			cookies: cookies,
			headers: headers,
		}
	}
	add("A", extractors.FromCookie("a_sid"), shared, "shared", "a:", false, nil, []string{"a_sid"}, nil)
	add("A2", extractors.FromCookie("a_sid"), shared, "shared", "a:", false, nil, []string{"a_sid"}, nil)
	add("B", extractors.FromCookie("b_sid"), shared, "shared", "b:", false, nil, []string{"b_sid"}, nil)
	// Store C keeps its sessions as records: everything the world checks holds for them too.
	add("C", extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C")), private, "private", "", true, JSONCodec{},
		[]string{"c_sid"}, []string{"X-C"})

	return &invariantWorld{
		stores:   stores,
		saved:    make(map[string][]string),
		ledger:   ledger,
		rng:      rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		storages: []*memory.Storage{shared, private},
	}
}

// storeAPIApp is an app whose handler gets the session of each store named in the ops
// of the query through Store.Get, gives it data, saves it and reports it.
func (w *invariantWorld) storeAPIApp() *fiber.App {
	app := fiber.New()
	var visits int
	var mu sync.Mutex
	app.Get("/", func(c fiber.Ctx) error {
		var rec invariantRecord
		for name := range strings.SplitSeq(c.Query("ops"), ",") {
			sess, err := w.stores[name].Get(c)
			if err != nil {
				return err
			}
			mu.Lock()
			visits++
			n := visits
			mu.Unlock()
			sess.Set("n", n)
			if err := sess.Save(); err != nil {
				sess.Release()
				return err
			}
			rec.Sessions = append(rec.Sessions, invariantSession{Store: name, ID: sess.ID(), Fresh: sess.Fresh()})
			sess.Release()
		}
		return c.JSON(rec)
	})
	return app
}

// middlewareApp is an app with a session middleware for each of the named stores,
// over the config the store was built with, in that order. The handler reports the
// session of every one of them through Store.FromContext, and gives the ones named in
// the ops of the query some data.
func (w *invariantWorld) middlewareApp(names []string) *fiber.App {
	app := fiber.New()
	for _, name := range names {
		cfg := w.stores[name].Config
		cfg.Store = w.stores[name].Store
		handler, _ := NewWithStore(cfg)
		app.Use(handler)
	}
	app.Get("/", func(c fiber.Ctx) error {
		// Every middleware of the stack runs and saves its session, whichever the
		// handler touches: all of them are reported, and the ops pick the ones that
		// get data.
		touched := make(map[string]bool)
		for name := range strings.SplitSeq(c.Query("ops"), ",") {
			touched[name] = true
		}
		var rec invariantRecord
		for _, name := range names {
			mw := w.stores[name].FromContext(c)
			if mw == nil {
				return fmt.Errorf("no middleware for store %s", name)
			}
			if touched[name] {
				mw.Set("n", 1)
			}
			rec.Sessions = append(rec.Sessions, invariantSession{Store: name, ID: mw.ID(), Fresh: mw.Fresh()})
		}
		return c.JSON(rec)
	})
	return app
}

// clientValue is what the client sends for one cookie, header or query value: nothing,
// the ID of a session that store `from` saved, a crafted variation of it, or garbage.
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

// invariantRequest is one request of the client.
type invariantRequest struct {
	cookies map[string]string
	header  string
	ops     []string
}

func (w *invariantWorld) newRequest(opNames []string) invariantRequest {
	req := invariantRequest{cookies: make(map[string]string)}
	if v := w.clientValue("A"); v != "" {
		req.cookies["a_sid"] = v
	}
	if v := w.clientValue("B"); v != "" {
		req.cookies["b_sid"] = v
	}
	if v := w.clientValue("C"); v != "" {
		req.cookies["c_sid"] = v
	}
	if v := w.clientValue("C"); v != "" {
		req.header = v
	}
	for range 1 + w.rng.IntN(5) {
		req.ops = append(req.ops, opNames[w.rng.IntN(len(opNames))])
	}
	return req
}

func (r invariantRequest) supplied() map[string]bool {
	out := make(map[string]bool)
	for _, v := range r.cookies {
		out[v] = true
	}
	if r.header != "" {
		out[r.header] = true
	}
	return out
}

func (r invariantRequest) send(app *fiber.App) (*http.Response, invariantRecord, error) {
	httpReq := httptest.NewRequest(fiber.MethodGet, "/?ops="+strings.Join(r.ops, ","), http.NoBody)
	for name, value := range r.cookies {
		httpReq.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	if r.header != "" {
		httpReq.Header.Set("X-C", r.header)
	}
	resp, err := app.Test(httpReq)
	if err != nil {
		return nil, invariantRecord{}, fmt.Errorf("send: %w", err)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, invariantRecord{}, fmt.Errorf("read: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, invariantRecord{}, fmt.Errorf("close: %w", err)
	}
	var rec invariantRecord
	if resp.StatusCode == fiber.StatusOK {
		if err := json.Unmarshal(raw, &rec); err != nil {
			return nil, invariantRecord{}, fmt.Errorf("decode %q: %w", raw, err)
		}
	}
	return resp, rec, nil
}

// check holds the response and what the handler saw against the properties above.
func (w *invariantWorld) check(t *testing.T, step int, req invariantRequest, resp *http.Response, rec invariantRecord) {
	t.Helper()
	where := fmt.Sprintf("request %d (ops %v, cookies %v, header %q)", step, req.ops, req.cookies, req.header)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, where)

	idsOf := make(map[string]map[string]bool)
	for _, s := range rec.Sessions {
		if idsOf[s.Store] == nil {
			idsOf[s.Store] = make(map[string]bool)
		}
		idsOf[s.Store][s.ID] = true
	}

	// A session the client names with a valid ID in the cookie its store reads is found,
	// whatever else the request carries: another store's cached ID does not take its place.
	for _, sess := range rec.Sessions {
		expected := req.cookies[w.stores[sess.Store].cookies[0]]
		if expected == "" && len(w.stores[sess.Store].headers) > 0 {
			expected = req.header
		}
		if expected == "" || !slices.Contains(w.saved[strings.TrimSuffix(sess.Store, "2")], expected) {
			continue
		}
		require.False(t, sess.Fresh, "%s: store %s did not find the session its client named: %q", where, sess.Store, expected)
		require.Equal(t, expected, sess.ID, where)
	}

	// A new session never takes an ID the client supplied.
	supplied := req.supplied()
	for _, s := range rec.Sessions {
		if s.Fresh {
			require.False(t, supplied[s.ID], "%s: a new session of store %s took the ID the client supplied: %q", where, s.Store, s.ID)
		}
	}

	// Stores that read no cookie or header in common keep sessions of their own: they
	// never come out with the same ID, which is what a store adopting the ID another one
	// cached on the request would do.
	for first, firstIDs := range idsOf {
		for second, secondIDs := range idsOf {
			if first >= second || w.stores[first].shares(w.stores[second]) {
				continue
			}
			for id := range firstIDs {
				require.False(t, secondIDs[id], "%s: stores %s and %s, which share no cookie or header, ended up with the same ID %q", where, first, second, id)
			}
		}
	}

	// A session ID is only written to a cookie or header a store that owns it reads.
	owns := func(name, value string, read func(*invariantStore) []string) bool {
		for storeName, ids := range idsOf {
			if !ids[value] {
				continue
			}
			if slices.Contains(read(w.stores[storeName]), name) {
				return true
			}
		}
		return false
	}
	for _, cookie := range resp.Cookies() {
		require.True(t, owns(cookie.Name, cookie.Value, func(s *invariantStore) []string { return s.cookies }),
			"%s: cookie %s=%q was written, but no store reading that cookie owns that session ID (sessions: %v)", where, cookie.Name, cookie.Value, rec.Sessions)
	}
	if value := resp.Header.Get("X-C"); value != "" {
		require.True(t, owns("X-C", value, func(s *invariantStore) []string { return s.headers }),
			"%s: header X-C=%q was written, but no store reading that header owns that session ID", where, value)
	}

	// A store only loads what a store with its key policy wrote.
	w.ledger.mu.Lock()
	violations := append([]string(nil), w.ledger.violations...)
	w.ledger.mu.Unlock()
	require.Empty(t, violations, where)

	// What was saved reaches the client: the last session saved for a cookie has its ID
	// in that cookie, or in the header of the store, since every save writes it.
	last := make(map[string]invariantSession)
	for _, sess := range rec.Sessions {
		last[w.stores[sess.Store].cookies[0]] = sess
	}
	for cookieName, sess := range last {
		written := false
		for _, cookie := range resp.Cookies() {
			written = written || (cookie.Name == cookieName && cookie.Value == sess.ID)
		}
		for _, header := range w.stores[sess.Store].headers {
			written = written || resp.Header.Get(header) == sess.ID
		}
		require.True(t, written, "%s: the session %q of store %s was saved, but its ID reached the client in none of the cookies and headers of the store", where, sess.ID, sess.Store)
	}

	// A hashed store keeps the ID out of its storage key.
	for id := range idsOf["C"] {
		key := w.stores["C"].storageKey(id)
		require.NotEqual(t, id, key, where)
		require.Len(t, key, len(keyNamespace)+hashedKeyLen, where)
	}

	// What was saved can be sent back.
	if w.frozen {
		return
	}
	for _, s := range rec.Sessions {
		w.saved[strings.TrimSuffix(s.Store, "2")] = append(w.saved[strings.TrimSuffix(s.Store, "2")], s.ID)
	}
}

// shares reports whether the two stores read a cookie or a header in common.
func (a *invariantStore) shares(b *invariantStore) bool {
	for _, x := range append(slices.Clone(a.cookies), a.headers...) {
		if slices.Contains(b.cookies, x) || slices.Contains(b.headers, x) {
			return true
		}
	}
	return false
}

// checkAskedAgain holds the sessions of a request that asked its stores through
// Store.Get, each of which gave its session data and saved it, against the property
// the cached IDs of a request are for: stores that read a cookie or a header in common
// are one visitor's session, whatever other stores were asked in between. A store
// asked again, or a store that shares with one that was asked before, finds the
// session that one saved, and does not start another.
func (w *invariantWorld) checkAskedAgain(t *testing.T, where string, rec invariantRecord) {
	t.Helper()

	for i, later := range rec.Sessions {
		for _, earlier := range rec.Sessions[:i] {
			if !w.stores[earlier.Store].shares(w.stores[later.Store]) {
				continue
			}
			require.Equal(t, earlier.ID, later.ID, "%s: store %s, asked after store %s which shares a cookie or header with it, did not find the session that one saved", where, later.Store, earlier.Store)
			require.False(t, later.Fresh, "%s: store %s started a new session, though store %s had saved one", where, later.Store, earlier.Store)
		}
	}
}

func invariantSeeds() uint64 {
	if raw := os.Getenv("SESSION_INVARIANT_SEEDS"); raw != "" {
		if n, err := strconv.ParseUint(raw, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 8
}

// Several stores used through Store.Get, in random order and repeatedly, within
// one request.
func Test_Session_Invariants_StoreAPI(t *testing.T) {
	t.Parallel()

	for seed := range invariantSeeds() {
		t.Run("seed="+strconv.FormatUint(seed, 10), func(t *testing.T) {
			t.Parallel()

			w := newInvariantWorld(seed)
			t.Cleanup(func() { w.shutdown(t) })
			app := w.storeAPIApp()

			for step := range 150 {
				req := w.newRequest([]string{"A", "A2", "B", "C"})
				resp, rec, err := req.send(app)
				require.NoError(t, err)
				w.check(t, step, req, resp, rec)
				w.checkAskedAgain(t, fmt.Sprintf("request %d (ops %v, cookies %v, header %q)", step, req.ops, req.cookies, req.header), rec)
			}
		})
	}
}

// The same stores as stacked session middleware, reached through Store.FromContext.
func Test_Session_Invariants_Middleware(t *testing.T) {
	t.Parallel()

	for seed := range invariantSeeds() {
		t.Run("seed="+strconv.FormatUint(seed, 10), func(t *testing.T) {
			t.Parallel()

			w := newInvariantWorld(seed)
			t.Cleanup(func() { w.shutdown(t) })
			names := []string{"A", "B", "C"}
			app := w.middlewareApp(names)

			for step := range 150 {
				req := w.newRequest(names)
				resp, rec, err := req.send(app)
				require.NoError(t, err)
				w.check(t, step, req, resp, rec)
			}
		})
	}
}
