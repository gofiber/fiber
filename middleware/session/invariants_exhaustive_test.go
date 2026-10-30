package session

import (
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// The properties of the invariant world, checked for every request of a small space
// instead of random ones. The client sends, in each of the cookies a_sid and b_sid,
// nothing, the ID of a session of the store itself, the ID of a session of the other
// one, the other's ID as its key would read it, or garbage; in the cookie c_sid and
// the header X-C of the store with two places, nothing, its own ID or garbage: 225
// combinations. Each one is sent with every sequence of up to three Store.Get calls
// over the four stores (84 sequences; SESSION_INVARIANT_EXHAUSTIVE=2 makes it 20
// and 4 makes it 340), and once with every subset of three stacked middleware (7).
// So a failure does not depend on what a seed happened to pick, and a combination of
// the cookies and the order of the stores that breaks a property is found, not hoped
// for.

// exhaustiveChunks is the number of subtests the combinations are shared out to, which
// run in parallel, each over a world of its own.
const exhaustiveChunks = 8

// exhaustiveOps is the longest sequence of Store.Get calls that is sent: three unless
// SESSION_INVARIANT_EXHAUSTIVE says from one to four.
func exhaustiveOps() int {
	if raw := os.Getenv("SESSION_INVARIANT_EXHAUSTIVE"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 4 {
			return n
		}
	}
	return 3
}

// seed saves a session for each store named in ops with a request that carries nothing,
// freezes the IDs the world sends back from then on, and returns the ID of each store.
func (w *invariantWorld) seed(t *testing.T, app *fiber.App, ops []string) map[string]string {
	t.Helper()

	req := invariantRequest{cookies: make(map[string]string), ops: ops}
	resp, rec, err := req.send(app)
	require.NoError(t, err)
	w.check(t, 0, req, resp, rec)
	w.frozen = true

	ids := make(map[string]string)
	for store, saved := range w.saved {
		require.NotEmpty(t, saved, store)
		ids[store] = saved[0]
	}
	return ids
}

// sendDirect is send without the network round trip app.Test makes: the request goes to
// the handler of the app as a fasthttp request, which is several times cheaper and what
// tens of thousands of requests need. Every Set-Cookie of the response has to be readable by
// fasthttp and by net/http, whose reading the checks rely on.
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

	var rec invariantRecord
	if resp.StatusCode == fiber.StatusOK {
		require.NoError(t, json.Unmarshal(fctx.Response.Body(), &rec), "%s", fctx.Response.Body())
	}
	return resp, rec
}

// exhaustiveClients returns every combination of what a client may send, given the IDs
// of the sessions of stores A, B and C, in the order cookie a_sid, b_sid, c_sid, header.
func exhaustiveClients(ids map[string]string) []invariantRequest {
	aValues := []string{"", ids["A"], ids["B"], "b:" + ids["B"], "garbage"}
	bValues := []string{"", ids["B"], ids["A"], "a:" + ids["A"], "garbage"}
	cValues := []string{"", ids["C"], "garbage"}
	headers := []string{"", ids["C"], "garbage"}

	var out []invariantRequest
	for _, a := range aValues {
		for _, b := range bValues {
			for _, c := range cValues {
				for _, header := range headers {
					req := invariantRequest{cookies: make(map[string]string), header: header}
					for name, value := range map[string]string{"a_sid": a, "b_sid": b, "c_sid": c} {
						if value != "" {
							req.cookies[name] = value
						}
					}
					out = append(out, req)
				}
			}
		}
	}
	return out
}

// opSequences returns every sequence of one to maxLen of the names.
func opSequences(names []string, maxLen int) [][]string {
	var out [][]string
	var extend func(prefix []string)
	extend = func(prefix []string) {
		if len(prefix) > 0 {
			out = append(out, prefix)
		}
		if len(prefix) == maxLen {
			return
		}
		for _, name := range names {
			extend(append(slices.Clone(prefix), name))
		}
	}
	extend(nil)
	return out
}

// subsets returns every non-empty subset of the names, in the order of the names.
func subsets(names []string) [][]string {
	var out [][]string
	for mask := 1; mask < 1<<len(names); mask++ {
		var subset []string
		for i, name := range names {
			if mask&(1<<i) != 0 {
				subset = append(subset, name)
			}
		}
		out = append(out, subset)
	}
	return out
}

func Test_Session_Invariants_Exhaustive_StoreAPI(t *testing.T) {
	t.Parallel()

	names := []string{"A", "A2", "B", "C"}
	longest := exhaustiveOps()
	sequences := opSequences(names, longest)
	want := 0
	for length, count := 1, len(names); length <= longest; length, count = length+1, count*len(names) {
		want += count
	}
	require.Len(t, sequences, want)

	for chunk := range exhaustiveChunks {
		t.Run("chunk="+strconv.Itoa(chunk), func(t *testing.T) {
			t.Parallel()

			w := newInvariantWorld(1)
			t.Cleanup(func() { w.shutdown(t) })
			app := w.storeAPIApp()
			clients := exhaustiveClients(w.seed(t, app, names))
			require.Len(t, clients, 5*5*3*3)
			handler := app.Handler()

			step := 0
			for i, client := range clients {
				if i%exhaustiveChunks != chunk {
					continue
				}
				for _, ops := range sequences {
					req := client
					req.ops = ops
					resp, rec := req.sendDirect(t, handler)
					w.check(t, step, req, resp, rec)
					step++
				}
			}
		})
	}
}

func Test_Session_Invariants_Exhaustive_Middleware(t *testing.T) {
	t.Parallel()

	names := []string{"A", "B", "C"}
	sets := subsets(names)
	require.Len(t, sets, 7)

	for chunk := range exhaustiveChunks {
		t.Run("chunk="+strconv.Itoa(chunk), func(t *testing.T) {
			t.Parallel()

			w := newInvariantWorld(1)
			t.Cleanup(func() { w.shutdown(t) })
			app := w.middlewareApp(names)
			clients := exhaustiveClients(w.seed(t, app, names))
			handler := app.Handler()

			step := 0
			for i, client := range clients {
				if i%exhaustiveChunks != chunk {
					continue
				}
				for _, ops := range sets {
					req := client
					req.ops = ops
					resp, rec := req.sendDirect(t, handler)
					w.check(t, step, req, resp, rec)
					step++
				}
			}
		})
	}
}
