package session

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
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

type flowSession struct {
	ID    string `json:"id"`
	Fresh bool   `json:"fresh"`
}

// flowOps are what a handler may do with the session of a store, picked at random.
var flowOps = []string{"", "", "set", "set", "reset", "resetempty", "regenerate", "destroy"}

func Test_Session_Invariants_Flows(t *testing.T) {
	t.Parallel()

	for seed := range invariantSeeds() {
		t.Run("seed="+strconv.FormatUint(seed, 10), func(t *testing.T) {
			t.Parallel()

			rng := rand.New(rand.NewPCG(seed, seed*7919))
			ledger := &invariantLedger{writers: make(map[string]string)}
			shared, private := memory.New(), memory.New()
			t.Cleanup(func() {
				require.NoError(t, shared.Close())
				require.NoError(t, private.Close())
			})
			ledgered := func(storage fiber.Storage, group, prefix string, hashed bool) fiber.Storage {
				return &invariantStorage{Storage: storage, ledger: ledger, group: group, policy: fmt.Sprintf("%s|%s|%t", group, prefix, hashed)}
			}

			// The deadlines are short enough for sessions to expire while the test runs.
			handlerA, storeA := NewWithStore(Config{
				Extractor:         extractors.FromCookie("a_sid"),
				Storage:           ledgered(shared, "shared", "a:", false),
				StorageKeyPrefix:  "a:",
				SkipEmptySessions: true,
				IdleTimeout:       20 * time.Millisecond,
				AbsoluteTimeout:   40 * time.Millisecond,
			})
			handlerB, storeB := NewWithStore(Config{
				Extractor:        extractors.FromCookie("b_sid"),
				Storage:          ledgered(shared, "shared", "b:", true),
				StorageKeyPrefix: "b:",
				HashStorageKeys:  true,
			})
			handlerC, storeC := NewWithStore(Config{
				Extractor:         extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C")),
				Storage:           ledgered(private, "private", "", true),
				HashStorageKeys:   true,
				SkipEmptySessions: true,
			})
			names := []string{"A", "B", "C"}
			stores := map[string]*Store{"A": storeA, "B": storeB, "C": storeC}
			cookies := map[string]string{"A": "a_sid", "B": "b_sid", "C": "c_sid"}

			var mu sync.Mutex
			var violations []string
			violate := func(format string, args ...any) {
				mu.Lock()
				defer mu.Unlock()
				violations = append(violations, fmt.Sprintf(format, args...))
			}

			app := fiber.New()
			app.Use(handlerA, handlerB, handlerC)
			app.Get("/", func(c fiber.Ctx) error {
				report := make(map[string]flowSession)
				for _, name := range names {
					m := stores[name].FromContext(c)
					if m == nil {
						violate("the handler has no middleware of store %s", name)
						continue
					}
					if owner := m.Get("owner"); owner != nil && owner != name {
						violate("store %s loaded a session that holds owner=%v, the data of another store", name, owner)
					}
					var err error
					switch c.Query(name) {
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
						violate("store %s: %s failed: %v", name, c.Query(name), err)
					}
					report[name] = flowSession{ID: m.ID(), Fresh: m.Fresh()}
				}
				return c.JSON(report)
			})

			saved := make(map[string][]string) // store -> IDs its sessions had
			pick := func(name string) string {
				if ids := saved[name]; len(ids) > 0 {
					return ids[rng.IntN(len(ids))]
				}
				return ""
			}
			for step := range flowRequests {
				supplied := make(map[string]bool)
				httpReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
				query := make([]string, 0, len(names))
				for _, name := range names {
					var value string
					switch rng.IntN(9) {
					case 3, 4, 5:
						value = pick(name)
					case 6:
						value = pick(names[rng.IntN(len(names))]) // a session of some store, whoever's it is
					case 7:
						value = []string{"a:", "b:"}[rng.IntN(2)] + pick([]string{"A", "B"}[rng.IntN(2)]) // as another store's key would read
					case 8:
						value = strings.Repeat("g", 1+rng.IntN(100))
					default: // nothing
					}
					if value != "" {
						httpReq.AddCookie(&http.Cookie{Name: cookies[name], Value: value})
						supplied[value] = true
						if name == "C" && rng.IntN(2) == 0 {
							httpReq.Header.Set("X-C", value)
						}
					}
					query = append(query, name+"="+flowOps[rng.IntN(len(flowOps))])
				}
				httpReq.URL.RawQuery = strings.Join(query, "&")
				if rng.IntN(40) == 0 {
					time.Sleep(45 * time.Millisecond) // lets the deadlines of A pass
				}

				resp, err := app.Test(httpReq)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				where := fmt.Sprintf("request %d (%s)", step, httpReq.URL.RawQuery)
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "%s: %s", where, body)

				var report map[string]flowSession
				require.NoError(t, json.Unmarshal(body, &report), where)
				for name, session := range report {
					if session.Fresh && supplied[session.ID] {
						violate("%s: a new session of store %s took the ID the client supplied: %q", where, name, session.ID)
					}
					if session.ID != "" && !strings.Contains(httpReq.URL.RawQuery, name+"=destroy") {
						saved[name] = append(saved[name], session.ID)
					}
				}

				// What the response sets belongs to a session of a store that reads it.
				owns := func(cookieOrHeader, value string) bool {
					for name, session := range report {
						if session.ID != value {
							continue
						}
						if cookieOrHeader == cookies[name] || (name == "C" && cookieOrHeader == "X-C") {
							return true
						}
					}
					return false
				}
				for _, cookie := range resp.Cookies() {
					if cookie.MaxAge < 0 { // expires the cookie
						continue
					}
					if !owns(cookie.Name, cookie.Value) {
						violate("%s: the cookie %s=%q is not the ID of a session of a store that reads it (report %v)", where, cookie.Name, cookie.Value, report)
					}
				}
				if value := resp.Header.Get("X-C"); value != "" && !owns("X-C", value) {
					violate("%s: the header X-C=%q is not the ID of a session of the store that reads it (report %v)", where, value, report)
				}
			}

			ledger.mu.Lock()
			defer ledger.mu.Unlock()
			mu.Lock()
			defer mu.Unlock()
			violations = append(violations, ledger.violations...)
			require.Empty(t, violations, "%d violations", len(violations))
		})
	}
}
