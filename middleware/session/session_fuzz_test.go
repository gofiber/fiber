package session

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// base64URLAlphabet is what a hashed storage key is made of after its prefix.
const base64URLAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// Whatever ID a client sends, the key a session is stored under follows the policy
// of the store and nothing else: the prefix and then the ID itself, or the prefix
// and then a fixed-length digest from which the ID cannot be read back and in which
// it cannot inject anything.
func Fuzz_Store_StorageKey(f *testing.F) {
	f.Add("abc", "", false)
	f.Add("abc", "app:", false)
	f.Add("abc", "app:", true)
	f.Add("", "app:", true)
	f.Add("b:abc", "a:", false)
	f.Add("a\x00b\r\n:*", "", true)
	f.Add(strings.Repeat("x", 4096), "p", true)

	f.Fuzz(func(t *testing.T, id, prefix string, hashed bool) {
		store := &Store{Config: Config{StorageKeyPrefix: prefix, HashStorageKeys: hashed}}
		key := store.storageKey(id)
		require.Equal(t, key, store.storageKey(id), "the key of an ID is stable")

		if !hashed {
			require.Equal(t, prefix+storageKeyPrefix+id, key)
			return
		}
		require.True(t, strings.HasPrefix(key, prefix+storageKeyPrefix))
		digest := key[len(prefix)+len(storageKeyPrefix):]
		require.Len(t, digest, hashedKeyLen)
		for _, r := range digest {
			require.Contains(t, base64URLAlphabet, string(r), "a digest is made of the URL-safe alphabet only")
		}
		require.NotEqual(t, key, store.storageKey(id+"x"), "two IDs share a key")
	})
}

// Arbitrary bytes in the cookies and the header of a request, for stores over several
// cookies and a header, never panic the middleware, never make it write a session ID
// to a cookie that no store of that ID reads, and never make a new session take what
// the client sent.
func Fuzz_Session_ClientInput(f *testing.F) {
	f.Add("", "", "", "")
	f.Add("a", "b", "c", "d")
	f.Add("b:abc", "a:abc", "unknown", "unknown")
	f.Add(strings.Repeat("g", 512), "\x00\x01\x02", "\"quoted\"", "x y")

	f.Fuzz(func(t *testing.T, a, b, c, header string) {
		// A world of its own for every input, closed with it: nothing carries over from
		// one input to the next, so an input that fails fails alone, and neither the
		// storage nor the goroutines of the storages grow for as long as the fuzzing
		// runs.
		w := newInvariantWorld(1)
		t.Cleanup(func() { w.shutdown(t) })
		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			var rec invariantRecord
			for name := range strings.SplitSeq(c.Query("ops"), ",") {
				sess, err := w.stores[name].Get(c)
				if err != nil {
					return err
				}
				sess.Set("n", 1)
				if err := sess.Save(); err != nil {
					sess.Release()
					return err
				}
				rec.Sessions = append(rec.Sessions, invariantSession{Store: name, ID: sess.ID(), Fresh: sess.Fresh()})
				sess.Release()
			}
			return c.JSON(rec)
		})
		handler := app.Handler()

		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI("/?ops=A,B,C,A2,B")
		fctx.Request.Header.SetCookie("a_sid", a)
		fctx.Request.Header.SetCookie("b_sid", b)
		fctx.Request.Header.SetCookie("c_sid", c)
		fctx.Request.Header.Set("X-C", header)

		handler(fctx)
		if fctx.Response.StatusCode() != fiber.StatusOK {
			return
		}

		var rec invariantRecord
		require.NoError(t, json.Unmarshal(fctx.Response.Body(), &rec))
		supplied := map[string]bool{a: true, b: true, c: true, header: true}
		owned := make(map[string]map[string]bool) // store -> IDs
		for _, s := range rec.Sessions {
			if owned[s.Store] == nil {
				owned[s.Store] = make(map[string]bool)
			}
			owned[s.Store][s.ID] = true
			if s.Fresh {
				require.False(t, supplied[s.ID], "a new session of %s took the ID the client sent", s.Store)
			}
		}
		reads := func(cookie, value string) bool {
			for store, ids := range owned {
				if !ids[value] {
					continue
				}
				if slices.Contains(w.stores[store].cookies, cookie) {
					return true
				}
			}
			return false
		}
		for _, raw := range fctx.Response.Header.Cookies() {
			cookie := fasthttp.AcquireCookie()
			if err := cookie.ParseBytes(raw); err != nil {
				t.Errorf("unparsable Set-Cookie %q: %v", raw, err)
			} else if !reads(string(cookie.Key()), string(cookie.Value())) {
				t.Errorf("cookie %s=%q was written, but no store reading it owns that ID", cookie.Key(), cookie.Value())
			}
			fasthttp.ReleaseCookie(cookie)
		}
	})
}
