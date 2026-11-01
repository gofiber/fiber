package session

import (
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/stretchr/testify/require"
)

// The security properties of the invariant world hold for the flows it leaves out as
// well: SkipEmptySessions, a session that outlives its AbsoluteTimeout, and handlers
// that Reset, Regenerate or Destroy a session. Three session middleware are nested in
// one app, over a storage they share and one of their own, and random requests carry
// valid, foreign, crafted and garbage IDs. Whatever the client sends:
//
//   - no store loads a session that another store wrote: a session holds the name of
//     its store, when it holds anything;
//   - a cookie or header that the response sets carries the ID of a session of a store
//     that reads it;
//   - a new session never takes an ID the client supplied;
//   - a store only loads records that a store with the same key policy wrote.
//
// The number of seeds is the one of the other invariant tests.

// flowRequests is the number of requests of one seed.
const flowRequests = 300

// flowOps are what a handler may do with the session of a store, picked at random.
var flowOps = []string{"", "", "set", "set", "reset", "resetempty", "regenerate", "destroy"}

// newFlowWorld has A, B and C as the middleware stack of the invariant world, with the
// options that make for flows: A and C skip the sessions that stay empty, B hashes its
// keys, and the deadlines of A are short enough for its sessions to expire while the test runs.
func newFlowWorld(tb testing.TB, seed uint64) *invariantWorld {
	tb.Helper()

	w := newWorld(tb, seed)
	w.rng = rand.New(rand.NewPCG(seed, seed*7919)) // the flows draw their requests from a stream of their own
	w.add("A", "shared", &Config{
		Extractor:         extractors.FromCookie("a_sid"),
		StorageKeyPrefix:  "a:",
		SkipEmptySessions: true,
		IdleTimeout:       20 * time.Millisecond,
		AbsoluteTimeout:   40 * time.Millisecond,
	}, []string{"a_sid"}, nil)
	w.add("B", "shared", &Config{
		Extractor:        extractors.FromCookie("b_sid"),
		StorageKeyPrefix: "b:",
		HashStorageKeys:  true,
	}, []string{"b_sid"}, nil)
	w.add("C", "private", &Config{
		Extractor:         extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C")),
		HashStorageKeys:   true,
		SkipEmptySessions: true,
		Codec:             JSONCodec{},
	}, []string{"c_sid"}, []string{"X-C"})
	return w
}

// flowsApp is an app with a session middleware for each of A, B and C. Its handler does
// to the session of each store the operation that the ops of the query name for it, in
// the order of the stores, and reports the sessions. What it finds wrong it tells the ledger.
func (w *invariantWorld) flowsApp() *fiber.App {
	return w.newApp(middlewareNames, func(c fiber.Ctx) error {
		ops := strings.Split(c.Query("ops"), ",")
		var rec invariantRecord
		for i, name := range middlewareNames {
			m := w.stores[name].FromContext(c)
			if m == nil {
				w.ledger.violatef("the handler has no middleware of store %s", name)
				continue
			}
			if owner := m.Get("owner"); owner != nil && owner != name {
				w.ledger.violatef("store %s loaded a session that holds owner=%v, the data of another store", name, owner)
			}
			var err error
			switch ops[i] {
			case "set":
				m.Set("owner", name)
			case "reset":
				if err = m.Reset(); err == nil {
					m.Set("owner", name)
				}
			case "resetempty":
				err = m.Reset()
			case "regenerate":
				err = m.Regenerate()
			case "destroy":
				err = m.Destroy()
			}
			if err != nil {
				w.ledger.violatef("store %s: %s failed: %v", name, ops[i], err)
			}
			rec.Sessions = append(rec.Sessions, invariantSession{Store: name, ID: m.ID(), Fresh: m.Fresh()})
		}
		return c.JSON(rec)
	})
}

// flowRequest is a request whose cookie of each store holds nothing, an ID or garbage,
// and whose ops name what the handler does with the session of each of the stores.
func (w *invariantWorld) flowRequest() invariantRequest {
	pick := func(name string) string {
		if ids := w.saved[name]; len(ids) > 0 {
			return ids[w.rng.IntN(len(ids))]
		}
		return ""
	}
	req := invariantRequest{cookies: make(map[string]string)}
	for _, name := range middlewareNames {
		var value string
		switch w.rng.IntN(9) {
		case 3, 4, 5:
			value = pick(name)
		case 6:
			value = pick(middlewareNames[w.rng.IntN(len(middlewareNames))]) // a session of some store, whoever's it is
		case 7:
			value = []string{"a:", "b:"}[w.rng.IntN(2)] + pick([]string{"A", "B"}[w.rng.IntN(2)]) // as another store's key would read
		case 8:
			value = strings.Repeat("g", 1+w.rng.IntN(100))
		default: // nothing
		}
		if value != "" {
			req.cookies[w.stores[name].cookies[0]] = value
			if name == "C" && w.rng.IntN(2) == 0 {
				req.header = value
			}
		}
		req.ops = append(req.ops, flowOps[w.rng.IntN(len(flowOps))])
	}
	return req
}

// checkFlow holds the response and what the handler saw against the properties above.
func (w *invariantWorld) checkFlow(t *testing.T, step int, req invariantRequest, resp *http.Response, rec invariantRecord) {
	t.Helper()
	where := req.where(step)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, where)

	req.requireNoFixation(t, where, rec)

	// What the response sets belongs to a session of a store that reads it. A cookie that
	// expires sets nothing.
	set := slices.DeleteFunc(resp.Cookies(), func(c *http.Cookie) bool { return c.MaxAge < 0 })
	w.requireOwned(t, where, rec, set, resp.Header.Get("X-C"))
	w.ledger.requireClean(t, where)

	for i, sess := range rec.Sessions {
		if sess.ID != "" && req.ops[i] != "destroy" {
			w.saved[sess.Store] = append(w.saved[sess.Store], sess.ID)
		}
	}
}

func Test_Session_Invariants_Flows(t *testing.T) {
	t.Parallel()

	for seed := range invariantSeeds() {
		t.Run("seed="+strconv.FormatUint(seed, 10), func(t *testing.T) {
			t.Parallel()

			w := newFlowWorld(t, seed)
			app := w.flowsApp()

			for step := range flowRequests {
				req := w.flowRequest()
				if w.rng.IntN(40) == 0 {
					time.Sleep(45 * time.Millisecond) // lets the deadlines of A pass
				}
				resp, rec := req.send(t, app)
				w.checkFlow(t, step, req, resp, rec)
			}
		})
	}
}
