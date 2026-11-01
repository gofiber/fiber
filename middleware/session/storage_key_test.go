package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// keyLogStorage logs the operation and the key of every read, write and delete a store makes, then
// delegates to an in-memory backend.
type keyLogStorage struct {
	fiber.Storage
	ops []string
	mu  sync.Mutex
}

func (s *keyLogStorage) record(op, key string) {
	s.mu.Lock()
	s.ops = append(s.ops, op+":"+key)
	s.mu.Unlock()
}

func (s *keyLogStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	s.record("get", key)
	return s.Storage.GetWithContext(ctx, key)
}

func (s *keyLogStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.record("set", key)
	return s.Storage.SetWithContext(ctx, key, val, exp)
}

func (s *keyLogStorage) DeleteWithContext(ctx context.Context, key string) error {
	s.record("delete", key)
	return s.Storage.DeleteWithContext(ctx, key)
}

// seen returns the operations logged so far, as "op:key".
func (s *keyLogStorage) seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.ops)
}

func digestOf(id string) string {
	sum := sha256.Sum256([]byte(id))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// storageKeyPolicies are the ways a store can key its records.
var storageKeyPolicies = []struct {
	name   string
	prefix string
	hash   bool
}{
	{name: "default"},
	{name: "prefix", prefix: "app:session:"},
	{name: "hash", hash: true},
	{name: "prefix and hash", prefix: "app:session:", hash: true},
}

// A key costs the one allocation of the key itself: without a prefix or hashing it is the namespace
// and the ID in one string, and the digest is built on the stack.
//
// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
func Test_Store_StorageKey_Allocations(t *testing.T) {
	const id = "wuyjDTdyBzOvL4mUnQ6qiM0nsSYUdyAxnXn0X2VgbX4"

	for _, tc := range storageKeyPolicies {
		store := NewStore(Config{StorageKeyPrefix: tc.prefix, HashStorageKeys: tc.hash})
		require.LessOrEqual(t, testing.AllocsPerRun(100, func() { _ = store.storageKey(id) }), 1.0, tc.name)
	}
}

// Every way a session reaches the storage must use the same key policy: a single call site left on
// the raw ID would read, write or delete a record the others never find.
func Test_Session_StorageKeyPolicy_EveryCallSite(t *testing.T) {
	t.Parallel()

	for _, tc := range storageKeyPolicies {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := &keyLogStorage{Storage: memory.New()}
			handler, store := NewWithStore(Config{Storage: log, StorageKeyPrefix: tc.prefix, HashStorageKeys: tc.hash})

			app := fiber.New()
			app.Use(handler)
			addSessionRoutes(app)
			app.Get("/reset", func(c fiber.Ctx) error {
				if err := FromContext(c).Reset(); err != nil {
					return err
				}
				FromContext(c).Set("again", true)
				return c.SendString(FromContext(c).ID())
			})

			ids := make([]string, 0, 8)
			do := func(path, id string) string {
				var cookies map[string]string
				if id != "" {
					cookies = map[string]string{"session_id": id}
				}
				_, body := sendRequest(t, app, path, cookies)
				if body != "" {
					ids = append(ids, body)
				}
				return body
			}

			id := do("/login", "")
			id = do("/regenerate", id) // Get, Delete, Set
			id = do("/reset", id)      // Get, Delete, Set
			do("/destroy", id)         // Get, Delete

			// The store pattern's own entry points.
			id = do("/login", "")
			loaded, err := store.GetByID(context.Background(), id)
			require.NoError(t, err)
			require.Equal(t, "alice", loaded.Get("user"))
			loaded.Release()
			require.NoError(t, store.Delete(context.Background(), id))

			operations := log.seen()
			for _, op := range []string{"get:", "set:", "delete:"} {
				require.True(t, slices.ContainsFunc(operations, func(s string) bool { return strings.HasPrefix(s, op) }),
					"the scenario never made a %q call, so it proves nothing about it", op)
			}
			for _, op := range operations {
				_, key, _ := strings.Cut(op, ":")
				require.True(t, strings.HasPrefix(key, tc.prefix), "%s lacks the prefix %q", op, tc.prefix)
				rest := strings.TrimPrefix(key, tc.prefix)
				require.True(t, strings.HasPrefix(rest, keyNamespace), "%s lacks the namespace %q", op, keyNamespace)
				rest = strings.TrimPrefix(rest, keyNamespace)
				if tc.hash {
					require.Len(t, rest, hashedKeyLen, "%s is not a digest", op)
					require.NotContains(t, ids, rest, "%s used a session ID as the key", op)
					require.True(t, slices.ContainsFunc(ids, func(id string) bool { return digestOf(id) == rest }),
						"%s is the digest of no session this test created", op)
				} else {
					require.Contains(t, ids, rest, "%s is the key of no session this test created", op)
				}
			}
		})
	}
}

// The point of hashing: what is in the storage is not something a client can send back as its
// cookie.
func Test_Session_HashStorageKeys_StoredKeyIsNotAnID(t *testing.T) {
	t.Parallel()

	mem := memory.New()
	handler, store := NewWithStore(Config{Storage: mem, HashStorageKeys: true})

	app := fiber.New()
	app.Use(handler)
	addSessionRoutes(app)

	_, id := sendRequest(t, app, "/login", nil)
	_, body := sendRequest(t, app, "/read", map[string]string{"session_id": id})
	require.Equal(t, "alice", body, "the client's ID still finds its session")

	keys, err := mem.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	stored := string(keys[0])
	require.Equal(t, keyNamespace+digestOf(id), stored)
	require.NotEqual(t, id, stored)

	// Whoever reads the storage and sends what they found gets nothing.
	_, body = sendRequest(t, app, "/read", map[string]string{"session_id": stored})
	require.Equal(t, "<nil>", body)
	_, err = store.GetByID(context.Background(), stored)
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
	_, err = store.GetByID(context.Background(), digestOf(id))
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore, "nor is the digest without its namespace")

	sess, err := store.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "alice", sess.Get("user"))
	sess.Release()
}

// Two stores over one storage and the same session ID stay apart when their prefixes differ.
func Test_Session_StorageKeyPrefix_SeparatesStores(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	same := func() string { return "same-id" }
	storeA := NewStore(Config{Storage: shared, StorageKeyPrefix: "a:", KeyGenerator: same})
	storeB := NewStore(Config{Storage: shared, StorageKeyPrefix: "b:", KeyGenerator: same})
	require.Equal(t, "same-id", seedSession(t, fiber.New(), storeA, func(s *Session) { s.Set("owner", "a") }))

	ctx := context.Background()
	_, err := storeB.GetByID(ctx, "same-id")
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore, "the other prefix does not see the record")

	sess, err := storeA.GetByID(ctx, "same-id")
	require.NoError(t, err)
	require.Equal(t, "a", sess.Get("owner"))
	sess.Release()

	require.NoError(t, storeB.Delete(ctx, "same-id"))
	sess, err = storeA.GetByID(ctx, "same-id")
	require.NoError(t, err, "deleting under the other prefix left the record alone")
	sess.Release()

	require.NoError(t, storeA.Delete(ctx, "same-id"))
	_, err = storeA.GetByID(ctx, "same-id")
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
}

// A prefix does not scope Reset: the storage interface cannot list keys, so Reset clears the whole
// storage, as the option documents.
func Test_Store_Reset_ClearsTheWholeStorageDespiteAPrefix(t *testing.T) {
	t.Parallel()

	shared := memory.New()
	store := NewStore(Config{Storage: shared, StorageKeyPrefix: "app:"})
	require.NoError(t, shared.Set("unrelated", []byte("x"), time.Minute))
	require.NoError(t, shared.Set("app:session", []byte("x"), time.Minute))

	require.NoError(t, store.Reset(context.Background()))
	require.Zero(t, storedSessions(t, shared))
}

// Benchmark_Store_StorageKey measures the key every read, write and delete of a session goes
// through. Without a prefix or hashing it costs one allocation, the key itself.
func Benchmark_Store_StorageKey(b *testing.B) {
	const id = "wuyjDTdyBzOvL4mUnQ6qiM0nsSYUdyAxnXn0X2VgbX4"

	for _, tc := range storageKeyPolicies {
		b.Run(tc.name, func(b *testing.B) {
			store := NewStore(Config{StorageKeyPrefix: tc.prefix, HashStorageKeys: tc.hash})

			b.ReportAllocs()
			for b.Loop() {
				_ = store.storageKey(id)
			}
		})
	}
}
