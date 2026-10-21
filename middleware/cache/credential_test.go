package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
)

// ttlStorage keeps the TTL each key was last written with.
type ttlStorage struct {
	*failingCacheStorage
	ttls  map[string]time.Duration
	ttlMu sync.Mutex
}

func newTTLStorage() *ttlStorage {
	return &ttlStorage{failingCacheStorage: newFailingCacheStorage(), ttls: map[string]time.Duration{}}
}

func (s *ttlStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.ttlMu.Lock()
	s.ttls[key] = exp
	s.ttlMu.Unlock()
	return s.failingCacheStorage.SetWithContext(ctx, key, val, exp)
}

func (s *ttlStorage) ttl(key string) time.Duration {
	s.ttlMu.Lock()
	defer s.ttlMu.Unlock()
	return s.ttls[key]
}

func Test_carriesCredential(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{KeyCredentialHeaders: []string{"X-Company-Token"}})
	for name, want := range map[string]bool{
		"Authorization":       true,
		"authorization":       true,
		"COOKIE":              true,
		"Proxy-Authorization": true,
		"X-API-Key":           true,
		"x-auth-token":        true,
		"X-Access-Token":      true,
		"X-Company-Token":     true,
		"x-company-token":     true,
		"X-Tenant":            false,
		"Accept-Language":     false,
		"Set-Cookie":          false,
		"X-Authorization":     false,
		"":                    false,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, cfg.carriesCredential(name))
		})
	}
}

// pathKey is the smallest custom KeyGenerator: the key is the path.
func pathKey(c fiber.Ctx) string { return c.Path() }

// requestWith returns the context of a request that carries the headers.
func requestWith(headers map[string]string) fiber.Ctx {
	return requestAt("/", headers)
}

// requestAt is requestWith for another path.
func requestAt(path string, headers map[string]string) fiber.Ctx {
	c, _ := buildKeygenCtx(&keygenCase{uri: path, headers: headers})
	c.Path(path)
	return c
}

// recordKeyOf is the key under which the refusal of the request would be recorded.
func recordKeyOf(r *refusalRecords, c fiber.Ctx, base string, vary []string, hasAuthorization bool) string { //nolint:revive // flag-parameter: a property of the request, not a mode of operation
	return r.key(c, base, vary, r.cfg.credentialsOf(c, vary, hasAuthorization))
}

func Test_requestCredentials_scope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		headers          map[string]string
		name             string
		vary             []string
		cfg              Config
		hasAuthorization bool
		wantEntry        bool
		wantKeyed        bool
	}{
		// A refusal reaches the entry found only when the key holds the credential or the Vary names it.
		{name: "nothing in the key"},
		{name: "Authorization", hasAuthorization: true, wantEntry: true, wantKeyed: true},
		{
			name: "KeyCookies cookie present", cfg: Config{KeyCookies: []string{"session"}},
			headers: map[string]string{"Cookie": "session=abc"}, wantEntry: true, wantKeyed: true,
		},
		{name: "KeyCookies cookie absent", cfg: Config{KeyCookies: []string{"session"}}},
		{
			name: "recognized header in KeyHeaders present", cfg: Config{KeyHeaders: []string{"X-API-Key"}},
			headers: map[string]string{"X-API-Key": "k"}, wantEntry: true, wantKeyed: true,
		},
		{name: "recognized header in KeyHeaders absent", cfg: Config{KeyHeaders: []string{"X-API-Key"}}},
		{
			name: "header in KeyHeaders that is not a credential", cfg: Config{KeyHeaders: []string{"X-Tenant"}},
			headers: map[string]string{"X-Tenant": "acme"},
		},
		{
			name: "declared header present", cfg: Config{KeyCredentialHeaders: []string{"X-Company-Token"}},
			headers: map[string]string{"X-Company-Token": "t"}, wantEntry: true, wantKeyed: true,
		},
		// The entry is shared by clients the credential does not tell apart.
		{name: "a cookie the key does not hold", headers: map[string]string{"Cookie": "a=b"}},
		{name: "a credential header the key does not hold", headers: map[string]string{"X-API-Key": "k"}},
		{
			name: "Vary names a credential the request sends", vary: []string{"cookie"},
			headers: map[string]string{"Cookie": "a=b"}, wantEntry: true,
		},
		{name: "Vary names a credential the request does not send", vary: []string{"cookie"}},
		{name: "Vary names a header that is not a credential", vary: []string{"x-tenant"}, headers: map[string]string{"X-Tenant": "acme"}},
		// A custom key is opaque: the refusal needs a credential it was told the key holds (Authorization is in every key).
		{
			name: "custom KeyGenerator, request with an undeclared cookie", cfg: Config{KeyGenerator: pathKey},
			headers: map[string]string{"Cookie": "sid=1"},
		},
		{
			name: "custom KeyGenerator, request with a declared cookie",
			cfg:  Config{KeyGenerator: pathKey, KeyCredentialHeaders: []string{"Cookie"}}, headers: map[string]string{"Cookie": "sid=1"}, wantEntry: true,
		},
		{
			name: "custom KeyGenerator, request with a built-in credential header it does not declare", cfg: Config{KeyGenerator: pathKey},
			headers: map[string]string{"X-API-Key": "junk"},
		},
		{
			name: "custom KeyGenerator, request with Proxy-Authorization", cfg: Config{KeyGenerator: pathKey},
			headers: map[string]string{"Proxy-Authorization": "Basic junk"},
		},
		{
			name:    "custom KeyGenerator, request with a built-in credential header it declares",
			cfg:     Config{KeyGenerator: pathKey, KeyCredentialHeaders: []string{"X-API-Key"}},
			headers: map[string]string{"X-API-Key": "k"}, wantEntry: true,
		},
		{
			name:    "custom KeyGenerator, request with a declared header",
			cfg:     Config{KeyGenerator: pathKey, KeyCredentialHeaders: []string{"X-Company-Token"}},
			headers: map[string]string{"X-Company-Token": "t"}, wantEntry: true,
		},
		{name: "custom KeyGenerator, Authorization", cfg: Config{KeyGenerator: pathKey}, hasAuthorization: true, wantEntry: true, wantKeyed: true},
		{
			name: "custom KeyGenerator, Vary names the cookie the request sends", cfg: Config{KeyGenerator: pathKey},
			vary: []string{"cookie"}, headers: map[string]string{"Cookie": "sid=1"}, wantEntry: true,
		},
		// Nothing the key could hold: a client turned away for its own reasons must not end the shared entry.
		{name: "custom KeyGenerator, anonymous request", cfg: Config{KeyGenerator: pathKey}},
		{
			name: "custom KeyGenerator, request with a header that is not a credential", cfg: Config{KeyGenerator: pathKey},
			headers: map[string]string{"X-Tenant": "acme"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := configDefault(tc.cfg)
			c := requestWith(tc.headers)
			scope := cfg.credentialsOf(c, tc.vary, tc.hasAuthorization).scope()
			require.Equal(t, tc.wantEntry, scope.entry, "entry")
			require.Equal(t, tc.wantKeyed, scope.keyed, "keyed")
		})
	}
}

func Test_sendsCredential(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		headers map[string]string
		name    string
		want    bool
	}{
		{name: "nothing"},
		{name: "a cookie it names", headers: map[string]string{"Cookie": "session=abc"}, want: true},
		{name: "a cookie it names, empty", headers: map[string]string{"Cookie": "session="}},
		{name: "a cookie it does not name", headers: map[string]string{"Cookie": "other=abc"}},
		{name: "a header it names", headers: map[string]string{"X-API-Key": "k"}, want: true},
		{name: "a header it does not name", headers: map[string]string{"X-Tenant": "acme"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, sendsCredential(requestWith(tc.headers), []string{"session"}, []string{"x-api-key"}))
		})
	}
}

func Test_keyLog(t *testing.T) {
	t.Parallel()

	t.Run("forgets the oldest past its limit, with the second it lapses", func(t *testing.T) {
		t.Parallel()

		l := &keyLog{limit: 2}
		require.Empty(t, l.add("a", 10).key)
		require.Empty(t, l.add("b", 20).key)
		require.Equal(t, loggedKey{key: "a", until: 10}, l.add("c", 30))
		require.Equal(t, loggedKey{key: "b", until: 20}, l.add("d", 40))
	})

	t.Run("a key seen again is the newest, and lapses at the later second", func(t *testing.T) {
		t.Parallel()

		l := &keyLog{limit: 2}
		l.add("a", 10)
		l.add("b", 20)
		require.Empty(t, l.add("a", 50).key)
		require.Empty(t, l.add("a", 5).key)
		require.Equal(t, loggedKey{key: "b", until: 20}, l.add("c", 30))
		require.Equal(t, loggedKey{key: "a", until: 50}, l.add("d", 40))
	})

	t.Run("a removed key leaves room", func(t *testing.T) {
		t.Parallel()

		l := &keyLog{limit: 2}
		l.add("a", 10)
		l.add("b", 20)
		l.remove("a")
		l.remove("never seen")
		require.Empty(t, l.add("c", 30).key)
		require.Equal(t, "b", l.add("d", 40).key)
	})

	t.Run("no limit forgets nothing, so keeps nothing", func(t *testing.T) {
		t.Parallel()

		l := &keyLog{}
		for i := range 100 {
			require.Empty(t, l.add(strconv.Itoa(i), 10).key)
		}
		require.Empty(t, l.items)
	})

	t.Run("a foreign value in the order forgets no key", func(t *testing.T) {
		t.Parallel()

		l := &keyLog{limit: 1}
		l.order.PushBack("not a logged key")
		require.Empty(t, l.add("a", 10).key)
		require.Contains(t, l.items, "a")
	})
}

func Test_refusalLimit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cfg  Config
		want int
	}{
		{name: "sized by MaxBytes", cfg: Config{MaxBytes: 1 << 20}, want: 4096},
		{name: "never fewer than the minimum", cfg: Config{MaxBytes: 1}, want: minRefusalRecords},
		{name: "a default when MaxBytes sets no limit", cfg: Config{}, want: unlimitedRefusalRecords},
		{name: "overridden", cfg: Config{MaxBytes: 1 << 20, refusalRecordLimit: 7}, want: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.cfg.refusalLimit())
		})
	}
}

// A record can exist for a URL until the last one of its group expires, or, in a Storage, once another instance announces one.
func Test_refusalRecords_mayHold(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := requestAt("/page", nil)
	elsewhere := requestAt("/dash", nil)
	require.NotEqual(t, groupOf("/page"), groupOf("/dash"))

	t.Run("in memory, from the first record until the last has expired", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault(Config{})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		require.False(t, records.mayHold(ctx, c, "base"))

		records.record(ctx, records.group(c, "base"), "base|refused|vary|digest", 0)
		require.True(t, records.mayHold(ctx, c, "base"))

		// The store drops its records at its own clock, which is the one read here.
		now := uint64(utils.Timestamp())
		records.all.until.Store(now)
		records.gates[records.group(c, "base")].until.Store(now)
		require.False(t, records.mayHold(ctx, c, "base"))
	})

	t.Run("only for the URLs of the group of the record", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault(Config{})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		records.record(ctx, records.group(c, "base"), "base|refused", 0)
		require.True(t, records.mayHold(ctx, c, "base"))
		require.False(t, records.mayHold(ctx, elsewhere, "base"))
	})

	t.Run("a custom key is grouped by the key", func(t *testing.T) {
		t.Parallel()

		require.NotEqual(t, groupOf("key-a"), groupOf("key-b"))
		cfg := configDefault(Config{KeyGenerator: pathKey})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		records.record(ctx, records.group(c, "key-a"), "key-a|refused", 0)
		require.True(t, records.mayHold(ctx, c, "key-a"))
		require.False(t, records.mayHold(ctx, c, "key-b"))
	})

	t.Run("a record is kept for Expiration, or as long as the refused entry had left", func(t *testing.T) {
		t.Parallel()

		now := time.Unix(1_700_000_000, 0)
		cfg := configDefault(Config{Expiration: time.Minute, clock: func() time.Time { return now }})
		kept := func(entryExp uint64) uint64 {
			records := newRefusalRecords(&cfg, newManager(nil, true))
			before := uint64(utils.Timestamp())
			records.record(ctx, records.group(c, "base"), "base|refused|vary|digest", entryExp)
			return records.all.until.Load() - before
		}
		// Whole seconds, plus the two the gate adds to cover the store's rounding and a tick.
		within := func(got, seconds uint64) {
			t.Helper()
			require.GreaterOrEqual(t, got, seconds+2)
			require.LessOrEqual(t, got, seconds+6)
		}

		within(kept(0), 60)
		within(kept(uint64(now.Unix())+10), 60)
		within(kept(uint64(now.Unix())+3600), 3600)
	})

	t.Run("in a Storage, when the markers of all URLs and of the group say so", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		require.False(t, records.mayHold(ctx, c, "base"))

		// Another instance announces a record for the group. It is noticed once the verdict is stale.
		storage.data[refusalMarker] = []byte{1}
		storage.data[groupMarkers[groupOf("/page")]] = []byte{1}
		records.all.seen.Store(0)
		require.True(t, records.mayHold(ctx, c, "base"))
		require.False(t, records.mayHold(ctx, elsewhere, "base"), "the group of another URL")
	})

	t.Run("in a Storage, a marker is read at most once per poll", func(t *testing.T) {
		t.Parallel()

		storage := newRefusalStorage()
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		records.poll = time.Hour.Milliseconds()
		for range 100 {
			require.False(t, records.mayHold(ctx, c, "base"))
		}
		require.Equal(t, int32(1), storage.markerReads.Load())
	})

	t.Run("in a Storage, a record another instance wrote is noticed within the poll", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		records.poll = 10
		require.False(t, records.mayHold(ctx, c, "base"))

		storage.data[refusalMarker] = []byte{1}
		storage.data[groupMarkers[groupOf("/page")]] = []byte{1}
		require.Eventually(t, func() bool { return records.mayHold(ctx, c, "base") }, 5*time.Second, time.Millisecond)
		require.LessOrEqual(t, markerPoll, 250*time.Millisecond, "the window a refusal can go unseen in")
	})

	t.Run("in a Storage that cannot answer, a record is taken to exist", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		storage.errs["get|"+refusalMarker] = errors.New("storage unavailable")
		storage.errs["get|"+groupMarkers[groupOf("/page")]] = errors.New("storage unavailable")
		cfg := configDefault(Config{Storage: storage})
		require.True(t, newRefusalRecords(&cfg, newManager(storage, true)).mayHold(ctx, c, "base"))
	})

	t.Run("a record written to a Storage is announced for the other instances", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		records.record(ctx, records.group(c, "base"), "base|refused|vary|digest", 0)
		require.Contains(t, storage.data, "base|refused|vary|digest")
		require.Contains(t, storage.data, refusalMarker)
		require.Contains(t, storage.data, groupMarkers[groupOf("/page")])
		require.True(t, records.mayHold(ctx, c, "base"))
	})

	t.Run("a record is kept as long as a credential's entry of the group has been", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault(Config{Expiration: time.Minute})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		group := records.group(c, "base")
		records.noteLifetime(group, time.Hour)
		records.noteLifetime(group, time.Minute)
		records.noteLifetime((group+1)%refusalGroups, 24*time.Hour)
		before := uint64(utils.Timestamp())
		records.record(ctx, group, "k", 0)
		kept := records.all.until.Load() - before
		require.GreaterOrEqual(t, kept, uint64(3602))
		require.LessOrEqual(t, kept, uint64(3606))
	})

	t.Run("an entry's lifetime counts only until it has passed", func(t *testing.T) {
		t.Parallel()

		now := time.Unix(1_700_000_000, 0)
		cfg := configDefault(Config{Expiration: time.Minute, clock: func() time.Time { return now }})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		group := records.group(c, "base")
		records.noteLifetime(group, time.Hour)
		now = now.Add(30 * time.Minute)
		before := uint64(utils.Timestamp())
		records.record(ctx, group, "k", 0)
		kept := records.all.until.Load() - before
		require.GreaterOrEqual(t, kept, uint64(1802), "half the hour is left")
		require.LessOrEqual(t, kept, uint64(1806))

		now = now.Add(time.Hour)
		records.forget(ctx, "k")
		records.record(ctx, group, "k", 0)
		kept = records.all.until.Load() - before
		require.LessOrEqual(t, kept, uint64(1806), "the hour has passed, so Expiration is all the record gets")
	})

	t.Run("a record is not shortened by a later refusal with less left", func(t *testing.T) {
		t.Parallel()

		now := time.Unix(1_700_000_000, 0)
		storage := newTTLStorage()
		cfg := configDefault(Config{Storage: storage, Expiration: time.Minute, clock: func() time.Time { return now }})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		group := records.group(c, "base")
		records.record(ctx, group, "k", uint64(now.Unix())+3600)
		lapse := lapseOf(storage.data["k"])
		require.GreaterOrEqual(t, lapse, uint64(utils.Timestamp())+3598)

		records.record(ctx, group, "k", uint64(now.Unix())+10)
		require.GreaterOrEqual(t, storage.ttl("k"), 3596*time.Second)
		require.Equal(t, lapse, lapseOf(storage.data["k"]))
	})

	t.Run("a marker announced for later stays", func(t *testing.T) {
		t.Parallel()

		storage := newTTLStorage()
		cfg := configDefault(Config{Storage: storage, Expiration: time.Minute})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		later := uint64(utils.Timestamp()) + 3600
		storage.data[refusalMarker] = lapseValue(later) // another instance's, for an hour
		records.record(ctx, records.group(c, "base"), "k", 0)
		require.GreaterOrEqual(t, storage.ttl(refusalMarker), 3596*time.Second)
		require.Equal(t, later, lapseOf(storage.data[refusalMarker]))
		require.GreaterOrEqual(t, records.all.until.Load(), later, "and this instance looks until then")
	})

	t.Run("a marker is not announced for a second that has passed", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		records.all.until.Store(uint64(utils.Timestamp()))
		records.announce(ctx, &records.all, refusalMarker)
		require.NotContains(t, storage.data, refusalMarker, "a TTL of zero would mean no expiry")
	})

	t.Run("in a Storage, a marker read says until when to look, so it is not polled again", func(t *testing.T) {
		t.Parallel()

		storage := newRefusalStorage()
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		later := uint64(utils.Timestamp()) + 3600
		storage.data[refusalMarker] = lapseValue(later)
		storage.data[groupMarkers[groupOf("/page")]] = lapseValue(later)
		for range 10 {
			records.all.seen.Store(0)
			require.True(t, records.mayHold(ctx, c, "base"))
		}
		require.Equal(t, int32(1), storage.markerReads.Load())
		require.Equal(t, later, records.all.until.Load())
	})

	t.Run("in a Storage, a clock set back does not put off the poll", func(t *testing.T) {
		t.Parallel()

		storage := newRefusalStorage()
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		require.False(t, records.mayHold(ctx, c, "base"))

		// The last poll looks an hour ahead once the clock is set back by an hour.
		records.all.seen.Store(time.Now().UnixMilli() + time.Hour.Milliseconds())
		storage.data[refusalMarker] = []byte{1}
		storage.data[groupMarkers[groupOf("/page")]] = []byte{1}
		require.True(t, records.mayHold(ctx, c, "base"))
		require.Equal(t, int32(2), storage.markerReads.Load())
	})

	t.Run("a write that takes a while keeps hits looking past when the store drops the record", func(t *testing.T) {
		t.Parallel()

		var once sync.Once
		storage := &hookedStorage{failingCacheStorage: newFailingCacheStorage(), onSet: func(string, time.Duration) {
			once.Do(func() { clocktest.SleepPast(t, 3*time.Second) })
		}}
		cfg := configDefault(Config{Storage: storage, Expiration: time.Minute})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		records.record(ctx, records.group(c, "base"), "k", 0)
		require.GreaterOrEqual(t, records.all.until.Load(), uint64(utils.Timestamp())+60)
	})
}

// The group of a URL is the same on every instance, since each reads the markers of the others.
func Test_groupOf(t *testing.T) {
	t.Parallel()

	require.Equal(t, 30, groupOf("/"))
	require.Equal(t, 37, groupOf("/page"))
	require.Equal(t, 6, groupOf("/dash"))

	seen := map[int]bool{}
	for i := range 4096 {
		group := groupOf("/page/" + strconv.Itoa(i))
		require.GreaterOrEqual(t, group, 0)
		require.Less(t, group, refusalGroups)
		seen[group] = true
	}
	require.Len(t, seen, refusalGroups, "the paths spread over every group")
}

func Test_droppedRecords(t *testing.T) {
	t.Parallel()

	t.Run("remembers what it is given, and forgets the oldest once full", func(t *testing.T) {
		t.Parallel()

		d := newDroppedRecords(2)
		d.add("a", 100)
		d.add("b", 200)
		require.True(t, d.has("a", 10))
		require.True(t, d.has("b", 10))
		require.False(t, d.has("c", 10))

		d.add("c", 300)
		require.False(t, d.has("a", 10), "a was the oldest")
		require.True(t, d.has("b", 10))
		require.True(t, d.has("c", 10))
		d.add("d", 400)
		require.False(t, d.has("b", 10), "then b")
		require.True(t, d.has("c", 10))
		require.True(t, d.has("d", 10))
	})

	t.Run("a record is remembered until the second it would have lapsed", func(t *testing.T) {
		t.Parallel()

		d := newDroppedRecords(2)
		d.add("a", 100)
		require.True(t, d.has("a", 99))
		require.False(t, d.has("a", 100))
	})

	t.Run("a key seen again keeps the later second", func(t *testing.T) {
		t.Parallel()

		d := newDroppedRecords(2)
		d.add("a", 100)
		d.add("a", 50)
		require.True(t, d.has("a", 99))
		d.add("a", 300)
		require.True(t, d.has("a", 299))
	})

	t.Run("a removed key is forgotten, and its slot is taken over before the oldest", func(t *testing.T) {
		t.Parallel()

		d := newDroppedRecords(2)
		d.add("a", 100)
		d.add("b", 200)
		d.remove("a")
		d.remove("never seen")
		require.False(t, d.has("a", 10))
		require.True(t, d.has("b", 10))
		d.add("c", 300)
		require.True(t, d.has("b", 10), "the slot of a was vacant")
		require.True(t, d.has("c", 10))
		d.add("d", 400)
		require.False(t, d.has("b", 10), "then b is the oldest")
		require.True(t, d.has("c", 10))
		require.True(t, d.has("d", 10))
	})

	t.Run("with no room it remembers nothing", func(t *testing.T) {
		t.Parallel()

		d := newDroppedRecords(0)
		d.add("a", 100)
		require.False(t, d.has("a", 10))
		d.remove("a")
	})
}

// An in-memory store keeps no more records than its limit; an external Storage expires its own.
func Test_refusalRecords_bound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := requestAt("/page", nil)
	keys := []string{"k1", "k2", "k3", "k4"}

	t.Run("an in-memory store forgets the oldest", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault(Config{refusalRecordLimit: 2})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		for _, key := range keys {
			records.record(ctx, records.group(c, "base"), key, 0)
		}
		held := func(key string) bool {
			_, err := records.m.getRaw(ctx, key)
			return err == nil
		}
		require.False(t, held("k1"))
		require.False(t, held("k2"))
		require.True(t, held("k3"))
		require.True(t, held("k4"))
	})

	t.Run("a record dropped while live is still refused, and nothing else is", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault(Config{refusalRecordLimit: 2})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		for _, key := range keys {
			records.record(ctx, records.group(c, "base"), key, 0)
		}

		// k1 and k2 are gone from the store, yet still refused; a key that was never recorded is not.
		require.True(t, records.refused(ctx, "k1"))
		require.True(t, records.refused(ctx, "k2"))
		require.False(t, records.refused(ctx, "unknown"))
		require.True(t, records.mayHold(ctx, c, "base"))
	})

	t.Run("forgetting a record forgets it dropped too", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault(Config{refusalRecordLimit: 1})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		records.record(ctx, records.group(c, "base"), "k1", 0)
		records.record(ctx, records.group(c, "base"), "k2", 0)
		require.True(t, records.refused(ctx, "k1"))
		records.forget(ctx, "k1")
		require.False(t, records.refused(ctx, "k1"))
	})

	t.Run("past what it can remember, the oldest dropped is forgotten", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault(Config{refusalRecordLimit: 1})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		for i := range 1 + droppedPerRecord {
			records.record(ctx, records.group(c, "base"), "k"+strconv.Itoa(i), 0)
		}
		for i := range 1 + droppedPerRecord {
			require.True(t, records.refused(ctx, "k"+strconv.Itoa(i)), "every one dropped so far is remembered")
		}

		records.record(ctx, records.group(c, "base"), "last", 0)
		require.False(t, records.refused(ctx, "k0"), "the oldest dropped made room")
		require.True(t, records.refused(ctx, "k1"))
		require.True(t, records.refused(ctx, "k"+strconv.Itoa(droppedPerRecord)))
		require.True(t, records.refused(ctx, "last"))
		require.False(t, records.refused(ctx, "unknown"))
	})

	t.Run("a record dropped after it lapsed costs nothing", func(t *testing.T) {
		t.Parallel()

		cfg := configDefault(Config{refusalRecordLimit: 1})
		records := newRefusalRecords(&cfg, newManager(nil, true))
		records.keys.add("old", uint64(utils.Timestamp())-10)
		records.record(ctx, records.group(c, "base"), "new", 0)
		require.Empty(t, records.dropped.ring)
		require.False(t, records.refused(ctx, "old"))
		require.True(t, records.refused(ctx, "new"))
	})

	t.Run("an external Storage keeps every record", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		cfg := configDefault(Config{Storage: storage, refusalRecordLimit: 2})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		for _, key := range keys {
			records.record(ctx, records.group(c, "base"), key, 0)
		}
		for _, key := range keys {
			require.True(t, records.refused(ctx, key), key)
		}
		require.Empty(t, records.dropped.ring)
	})
}

// A Storage failing a write or delete of a record does not fail the request; the other bookkeeping still happens.
func Test_refusalRecords_storageFailures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := requestAt("/page", nil)
	failure := errors.New("storage unavailable")

	t.Run("a record that cannot be written is remembered here, and the credential's hits fail closed until it would have lapsed", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		storage.errs["set|k"] = failure
		cfg := configDefault(Config{Storage: storage, Expiration: time.Minute})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		require.False(t, records.refused(ctx, "k"))

		before := uint64(utils.Timestamp())
		records.record(ctx, records.group(c, "base"), "k", 0)
		require.Empty(t, storage.data)
		require.True(t, records.mayHold(ctx, c, "base"), "the gates are open, so hits look")
		require.True(t, records.refused(ctx, "k"))
		require.False(t, records.refused(ctx, "other"), "no other credential pays for it")

		// For as long as the record would have been kept, and a moment more.
		require.True(t, records.dropped.has("k", before+61))
		require.False(t, records.dropped.has("k", before+63))
	})

	t.Run("a record or marker that cannot be read before the write is written as if there were none", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		storage.errs["get|k"] = failure
		storage.errs["get|"+refusalMarker] = failure
		cfg := configDefault(Config{Storage: storage, Expiration: time.Minute})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		before := uint64(utils.Timestamp())
		records.record(ctx, records.group(c, "base"), "k", 0)
		require.Contains(t, storage.data, "k")
		require.Contains(t, storage.data, refusalMarker)
		require.GreaterOrEqual(t, lapseOf(storage.data[refusalMarker]), before+62)
		require.Empty(t, records.dropped.ring)
	})

	t.Run("a marker that cannot be written leaves the record and the other marker", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		storage.errs["set|"+refusalMarker] = failure
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		records.record(ctx, records.group(c, "base"), "k", 0)
		require.Contains(t, storage.data, "k")
		require.NotContains(t, storage.data, refusalMarker)
		require.Contains(t, storage.data, groupMarkers[groupOf("/page")])
		require.Empty(t, records.dropped.ring, "the record is there to find")
	})

	t.Run("a record is announced in the group it is given, not the group of the path now", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		require.NotEqual(t, groupOf("/page"), groupOf("/dash"))
		records.record(ctx, groupOf("/dash"), "k", 0)
		require.Contains(t, storage.data, groupMarkers[groupOf("/dash")])
		require.NotContains(t, storage.data, groupMarkers[groupOf("/page")])
	})

	t.Run("a record that cannot be dropped to make room is still remembered as dropped", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		storage.errs["del|k1"] = failure
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		// Only an in-memory store is bounded, and deleting from it cannot fail.
		records.keys.limit = 1
		records.record(ctx, records.group(c, "base"), "k1", 0)
		records.record(ctx, records.group(c, "base"), "k2", 0)
		require.Contains(t, storage.data, "k1")
		require.True(t, records.dropped.has("k1", uint64(utils.Timestamp())))
	})

	t.Run("a record that cannot be cleared stays until the Storage expires it", func(t *testing.T) {
		t.Parallel()

		storage := newFailingCacheStorage()
		storage.errs["del|k"] = failure
		cfg := configDefault(Config{Storage: storage})
		records := newRefusalRecords(&cfg, newManager(storage, true))
		records.record(ctx, records.group(c, "base"), "k", 0)
		records.forget(ctx, "k")
		require.Contains(t, storage.data, "k")
	})
}

func Test_refusesCredential(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		want   bool
	}{
		{status: fiber.StatusUnauthorized, want: true},
		{status: fiber.StatusForbidden, want: true},
		{status: fiber.StatusProxyAuthRequired, want: true},
		{status: fiber.StatusOK},
		{status: fiber.StatusFound},
		{status: fiber.StatusNotFound},
		{status: fiber.StatusTooManyRequests},
		{status: fiber.StatusServiceUnavailable},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, refusesCredential(tc.status))
		})
	}
}

func Test_acceptsCredential(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		want   bool
	}{
		{status: fiber.StatusOK, want: true},
		{status: fiber.StatusNoContent, want: true},
		{status: fiber.StatusNotModified, want: true},
		// A redirect is how an app sends an ended session to its login page.
		{status: fiber.StatusMovedPermanently},
		{status: fiber.StatusFound},
		{status: fiber.StatusSeeOther},
		{status: fiber.StatusTemporaryRedirect},
		{status: fiber.StatusUnauthorized},
		{status: fiber.StatusNotFound},
		{status: fiber.StatusServiceUnavailable},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, acceptsCredential(tc.status))
		})
	}
}

// The gates read utils.Timestamp, which is zero until the updater runs; only the in-memory store starts it,
// so New must. Checked in a process of its own, since here every other test has long started it.
func Test_New_StartsTheTimestampUpdater(t *testing.T) {
	t.Parallel()

	const child = "FIBER_CACHE_TEST_TIMESTAMP_CHILD"
	if os.Getenv(child) == "1" {
		require.Zero(t, utils.Timestamp(), "something else in this process started the updater")
		New(Config{Storage: newFailingCacheStorage()})
		require.NotZero(t, utils.Timestamp())
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^Test_New_StartsTheTimestampUpdater$")
	cmd.Env = append(os.Environ(), child+"=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
}

func Test_errorRefusesCredential(t *testing.T) {
	t.Parallel()

	// A nil *fiber.Error held in an error matches errors.As, and must not be read.
	var typedNil *fiber.Error

	for _, tc := range []struct {
		err  error
		name string
		want bool
	}{
		{name: "nil"},
		{name: "401", err: fiber.ErrUnauthorized, want: true},
		{name: "403", err: fiber.ErrForbidden, want: true},
		{name: "407", err: fiber.ErrProxyAuthRequired, want: true},
		{name: "custom message", err: fiber.NewError(fiber.StatusForbidden, "tenant suspended"), want: true},
		{name: "wrapped", err: fmt.Errorf("session: %w", fiber.ErrUnauthorized), want: true},
		{name: "joined", err: errors.Join(errors.New("audit failed"), fiber.ErrForbidden), want: true},
		{name: "503", err: fiber.ErrServiceUnavailable},
		{name: "404", err: fiber.ErrNotFound},
		{name: "not a fiber error", err: errors.New("unauthorized")},
		{name: "typed nil", err: typedNil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, errorRefusesCredential(tc.err))
		})
	}
}

func Test_varyListsCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		vary     string
		declared []string
		want     bool
	}{
		{vary: "", want: false},
		{vary: "Accept", want: false},
		{vary: "Accept-Encoding, Accept-Language", want: false},
		{vary: "*", want: false},
		{vary: "Cookie", want: true},
		{vary: "cookie", want: true},
		{vary: "COOKIE", want: true},
		{vary: " Cookie ", want: true},
		{vary: "Accept-Encoding, Cookie", want: true},
		{vary: "Accept-Encoding,Cookie", want: true},
		{vary: "Authorization", want: true},
		{vary: "authorization", want: true},
		// The other headers that carry nothing but a credential.
		{vary: "Proxy-Authorization", want: true},
		{vary: "X-API-Key", want: true},
		{vary: "Accept, x-auth-token", want: true},
		{vary: "X-Access-Token", want: true},
		// Names that merely contain one of them are other fields.
		{vary: "X-Cookie", want: false},
		{vary: "Cookie2", want: false},
		{vary: "Set-Cookie", want: false},
		{vary: "X-Authorization", want: false},
		// A header is a credential only when it is declared.
		{vary: "X-Company-Token", want: false},
		{vary: "X-Company-Token", declared: []string{"x-company-token"}, want: true},
		{vary: "Accept, X-COMPANY-TOKEN", declared: []string{"x-company-token"}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.vary, func(t *testing.T) {
			t.Parallel()
			cfg := Config{KeyCredentialHeaders: tc.declared}
			require.Equal(t, tc.want, cfg.varyListsCredential([]byte(tc.vary)), "Vary: %q", tc.vary)
		})
	}
}

func Test_refusalRecords_key(t *testing.T) {
	t.Parallel()

	records := func(cfg Config) *refusalRecords {
		cfg = configDefault(cfg)
		return newRefusalRecords(&cfg, newManager(nil, true))
	}
	session := map[string]string{"Cookie": "session=abc"}

	t.Run("there is nothing to record unless the credential is the entry's own", func(t *testing.T) {
		t.Parallel()

		r := records(Config{KeyCredentialHeaders: []string{"X-Company-Token"}})
		key := func(vary []string, headers map[string]string) string {
			return recordKeyOf(r, requestWith(headers), "base", vary, false)
		}
		require.Empty(t, key(nil, session), "no Vary, and the key holds no cookie")
		require.Empty(t, key([]string{"x-tenant"}, session), "a Vary on no credential")
		require.Empty(t, key([]string{"cookie"}, nil), "no credential sent")
		require.Empty(t, key([]string{"x-api-key"}, session), "another credential sent")
		require.NotEmpty(t, key([]string{"cookie"}, session))

		// The declared header is in the key, so that is the credential recorded.
		held := key([]string{"cookie"}, map[string]string{"X-Company-Token": "t"})
		require.Contains(t, held, "x-company-token:1sha256:")
		require.NotContains(t, held, "|vary|", "the Vary selects by a cookie the request does not send")
	})

	t.Run("a Vary on a credential is recorded under the credential and not the variant", func(t *testing.T) {
		t.Parallel()

		r := records(Config{})
		key := func(vary []string, headers map[string]string) string {
			return recordKeyOf(r, requestWith(headers), "base", vary, false)
		}
		got := key([]string{"cookie", "x-variant"}, session)
		require.Equal(t, got, key([]string{"cookie"}, session), "the other Vary names are not part of the record")
		require.Equal(t, got, key([]string{"cookie", "x-variant"}, map[string]string{"Cookie": "session=abc", "X-Variant": "two"}))
		require.NotEqual(t, got, key([]string{"cookie"}, map[string]string{"Cookie": "session=other"}))
		require.Equal(t, got, recordKeyOf(r, requestWith(session), "other base", []string{"cookie"}, false), "the key is built from the request")
		require.True(t, strings.HasPrefix(got, refusalMarker+"|GET|"), "%q", got)
		require.Contains(t, got, "|vary|")
	})

	t.Run("a credential the key holds is recorded under the key alone, whatever the variant varies on", func(t *testing.T) {
		t.Parallel()

		r := records(Config{KeyCookies: []string{"session"}})
		key := func(vary []string, headers map[string]string) string {
			return recordKeyOf(r, requestWith(headers), "base", vary, false)
		}
		got := key(nil, session)
		require.NotEmpty(t, got)
		require.NotContains(t, got, "|vary|")
		require.Equal(t, got, key([]string{"cookie"}, session), "a variant varying on the cookie")
		require.Equal(t, got, key([]string{"x-api-key"}, map[string]string{"Cookie": "session=abc", "X-API-Key": "k"}), "a variant varying on another credential")
	})

	t.Run("a credential the key holds is recorded without the other dimensions of the key", func(t *testing.T) {
		t.Parallel()

		withAccept := func(cookie, accept string) map[string]string {
			return map[string]string{"Cookie": "session=" + cookie, "Accept": accept}
		}
		for name, cfg := range map[string]Config{
			"KeyCookies":     {KeyCookies: []string{"session"}},
			"KeyHeaders":     {KeyHeaders: []string{"Accept", "X-API-Key"}},
			"KeyCredentialH": {KeyHeaders: []string{"Accept"}, KeyCredentialHeaders: []string{"X-Company-Token"}},
		} {
			r := records(cfg)
			credential := func(value, accept string) map[string]string {
				switch name {
				case "KeyCookies":
					return withAccept(value, accept)
				case "KeyHeaders":
					return map[string]string{"X-API-Key": value, "Accept": accept}
				default:
					return map[string]string{"X-Company-Token": value, "Accept": accept}
				}
			}
			key := func(value, accept string) string {
				return recordKeyOf(r, requestWith(credential(value, accept)), "base", nil, false)
			}

			got := key("abc", "text/html")
			require.NotEmpty(t, got, name)
			require.Equal(t, got, key("abc", "application/json"), "%s: a variant of another Accept", name)
			require.NotEqual(t, got, key("other", "text/html"), "%s: another credential", name)
			require.NotContains(t, got, "abc", name)
		}
		require.Empty(t, recordKeyOf(records(Config{KeyCookies: []string{"session"}}), requestWith(nil), "base", nil, false), "no cookie sent")
	})

	t.Run("Authorization is recorded under its digest", func(t *testing.T) {
		t.Parallel()

		r := records(Config{})
		key := func(auth string) string {
			return recordKeyOf(r, requestWith(map[string]string{"Authorization": auth, "Accept": "text/html"}), "base", nil, true)
		}
		got := key("Bearer one")
		require.NotEmpty(t, got)
		require.Contains(t, got, "|auth="+authDigest("Bearer one"))
		require.NotEqual(t, got, key("Bearer two"))
	})

	t.Run("a declared header is a credential held by a custom key, or not", func(t *testing.T) {
		t.Parallel()

		token := map[string]string{"X-Company-Token": "t"}
		record := func(cfg Config, vary []string, headers map[string]string) string {
			return recordKeyOf(records(cfg), requestWith(headers), "base", vary, false)
		}
		varyToken := []string{"x-company-token"}
		declared := Config{KeyCredentialHeaders: []string{"X-Company-Token"}}
		custom := Config{KeyCredentialHeaders: []string{"X-Company-Token"}, KeyGenerator: pathKey}

		// The default key is built from it, so the record is under the credential, not the base.
		got := record(declared, varyToken, token)
		require.NotEmpty(t, got)
		require.NotContains(t, got, "base")

		// A custom key ignores it, so the manifest is shared and the refusal gets a record under the key it built.
		got = record(custom, varyToken, token)
		require.True(t, strings.HasPrefix(got, refusalMarker+"|GET|base|vary|"), "%q", got)
		require.NotEqual(t, got, record(custom, varyToken, map[string]string{"X-Company-Token": "u"}))
		require.Empty(t, record(custom, varyToken, nil))
		require.NotEmpty(t, record(custom, nil, token))
	})

	t.Run("a custom key holds Authorization too, so the record is under the key it built", func(t *testing.T) {
		t.Parallel()

		r := records(Config{KeyGenerator: pathKey})
		bearer := map[string]string{"Authorization": "Bearer one"}
		want := refusalMarker + "|GET|base"
		require.Equal(t, want, recordKeyOf(r, requestWith(bearer), "base", nil, true))
		require.Equal(t, want, recordKeyOf(r, requestWith(bearer), "base", []string{"x-tenant"}, true), "a Vary on no credential")
		require.Empty(t, recordKeyOf(r, requestWith(nil), "base", nil, false), "nothing sent")

		declared := records(Config{KeyGenerator: pathKey, KeyCredentialHeaders: []string{"X-Company-Token"}})
		both := map[string]string{"Authorization": "Bearer one", "X-Company-Token": "t"}
		require.Equal(t, want, recordKeyOf(declared, requestWith(both), "base", nil, true), "the key holds Authorization")
	})

	t.Run("a HEAD is recorded as the GET it mirrors", func(t *testing.T) {
		t.Parallel()

		head := func(headers map[string]string) fiber.Ctx {
			c := requestWith(headers)
			c.Method(fiber.MethodHead)
			return c
		}
		bearer := map[string]string{"Authorization": "Bearer one"}
		for name, cfg := range map[string]Config{
			"default key": {},
			"custom key":  {KeyGenerator: pathKey},
		} {
			r := records(cfg)
			get := recordKeyOf(r, requestWith(bearer), "base", nil, true)
			require.NotEmpty(t, get, name)
			require.Equal(t, get, recordKeyOf(r, head(bearer), "base", nil, true), name)
		}

		// Another method is its own: a refusal of a POST says nothing about the GET.
		r := records(Config{Methods: []string{fiber.MethodGet, fiber.MethodPost}})
		post := requestWith(bearer)
		post.Method(fiber.MethodPost)
		require.NotEqual(t, recordKeyOf(r, requestWith(bearer), "base", nil, true), recordKeyOf(r, post, "base", nil, true))
	})
}
