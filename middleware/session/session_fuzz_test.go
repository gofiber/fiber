package session

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
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
			require.Equal(t, prefix+keyNamespace+id, key)
			return
		}
		require.True(t, strings.HasPrefix(key, prefix+keyNamespace))
		digest := key[len(prefix)+len(keyNamespace):]
		require.Len(t, digest, hashedKeyLen)
		for _, r := range digest {
			require.Contains(t, base64URLAlphabet, string(r), "a digest is made of the URL-safe alphabet only")
		}
		require.NotEqual(t, key, store.storageKey(id+"x"), "two IDs share a key")
	})
}

// Arbitrary bytes in the cookies and the header of a request, for stores over several
// cookies and a header, never panic the middleware and never break a property of the
// invariant world: a new session never takes what the client sent, an ID is only written
// to a cookie or header that a store which owns it reads, and the rest of what check holds.
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
		w := newInvariantWorld(t, 1)
		app := w.storeAPIApp()

		req := invariantRequest{
			cookies: map[string]string{"a_sid": a, "b_sid": b, "c_sid": c},
			header:  header,
			ops:     []string{"A", "B", "C", "A2", "B"},
		}
		resp, rec := req.sendDirect(t, app.Handler())
		if resp.StatusCode != fiber.StatusOK {
			return
		}
		w.check(t, 0, req, resp, rec)
	})
}
