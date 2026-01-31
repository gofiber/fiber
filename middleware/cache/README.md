# Cache Middleware

HTTP response caching middleware for Fiber. Stores responses keyed by request
path (or a custom key), serves them on subsequent matching requests, and
provides tag-based invalidation, conditional request validation, and
Cache-Control header management.

---

## Tag Strategies

Tags associate cache entries with logical groups. Entries sharing a tag can be
invalidated together, making it possible to flush related content (e.g. all
pages for a given user or product) with a single call.

### Associating Tags with Entries

Two configuration hooks control tag assignment:

| Hook | When it runs | Signature |
|---|---|---|
| `Tags` | Before the handler, using only request data | `func(c fiber.Ctx) []string` |
| `ResponseTags` | After the handler, with access to the response body | `func(c fiber.Ctx, body []byte) []string` |

Tags from both hooks are merged into a single set and persisted alongside the
cached entry. Either hook can be omitted.

```go
app.Use(cache.New(cache.Config{
    Tags: func(c fiber.Ctx) []string {
        return []string{"region:" + c.Query("region")}
    },
    ResponseTags: func(c fiber.Ctx, body []byte) []string {
        if strings.Contains(string(body), "draft") {
            return []string{"draft"}
        }
        return nil
    },
}))
```

### Tag Index Backends

The middleware automatically selects the tag index backend based on whether an
external `Storage` is configured:

| `Config.Storage` | Backend | Scope |
|---|---|---|
| `nil` (default) | In-memory `tagIndex` | Single process |
| non-nil (e.g. Redis) | `distributedTagStore` | Shared across all instances using the same backend |

**In-memory (`tagIndex`)** — a bidirectional map: a forward index
(`tag → set of cache keys`) supports invalidation lookups and a reverse index
(`cache key → set of tags`) supports cleanup on eviction or expiry. Both
directions provide O(1) lookup.

**Distributed (`distributedTagStore`)** — persists the same bidirectional
index in the shared storage backend using reserved key prefixes:

| Key pattern | Contains |
|---|---|
| `__cache_tag__:<tag>` | Forward index: all cache keys under this tag |
| `__cache_tagrev__:<key>` | Reverse index: all tags for this cache key |

Each instance also maintains a local `tagIndex` as a fast path for the
`has()` check performed on every cache hit. See
[Cross-Instance Invalidation](#cross-instance-invalidation) for how this
affects invalidation semantics.

### Tag Re-population

When an external storage backend is used, a process restart clears the local
tag index. The middleware recovers transparently: on every cache hit it checks
whether the key is already tracked locally, and if not, re-adds the tags that
were persisted with the cached entry. No explicit warm-up step is required.

### Tag Rejection

`RejectTags` is a list of glob patterns. If any tag computed by `Tags` or
`ResponseTags` matches a reject pattern, the response is **not stored** and the
middleware reports `X-Cache: unreachable`.

```go
app.Use(cache.New(cache.Config{
    Tags: func(c fiber.Ctx) []string { ... },
    RejectTags: []string{
        "internal",   // exact match
        "user:*",     // prefix — matches "user:", "user:42", …
        "*:secret",   // suffix — matches "api:secret", …
    },
}))
```

Patterns are pre-classified at middleware creation into three runtime-efficient
buckets:

| Pattern shape | Matching strategy |
|---|---|
| No `*` | O(1) map lookup |
| Single trailing `*` | `strings.HasPrefix` |
| All other `*` patterns | Full glob match |

The only wildcard character is `*`. It matches any sequence of characters
including the empty string.

---

## Invalidation Patterns

### Basic Invalidation

`cache.InvalidateTags` removes all cached responses associated with any of the
provided tags. It must be called from a handler that has the cache middleware in
its chain.

```go
app.Post("/flush", func(c fiber.Ctx) error {
    tag := c.Query("tag")
    return cache.InvalidateTags(c, tag)
})
```

Multiple tags can be passed in a single call. Entries sharing any of the
listed tags are collected and removed. Duplicate keys — entries that match more
than one of the provided tags — are deduplicated internally.

### What Happens During Invalidation

1. **Collect keys** — the tag store reads the forward index for each requested
   tag and merges the results into a unique key set.
2. **Remove forward entries** — the forward index entries for the invalidated
   tags are deleted.
3. **Update reverse entries** — for each collected key, the invalidated tags are
   stripped from its reverse index entry. Keys whose reverse entry becomes empty
   are removed entirely.
4. **Evict from cache** — the middleware removes each collected key from the
   expiration heap (when `MaxBytes` is configured) and deletes it from storage.

### Partial Invalidation

A cache entry can carry multiple tags. Invalidating one tag removes the entry
from the cache, but other tags' forward index entries retain the key until those
tags are invalidated in turn or the entry is re-cached.

```
Entry "k1" tagged ["product:1", "region:us"]

InvalidateTags(c, "region:us")
  → k1 is evicted from the cache
  → forward index for "region:us" is cleared
  → reverse index for k1 retains ["product:1"]
```

### Cross-Instance Invalidation

When `Storage` is set, the forward index lives in the shared backend. An
`InvalidateTags` call on **any** instance reads that shared index and therefore
discovers keys cached by other instances. Those keys are deleted from the shared
storage, making the invalidation effective cluster-wide.

Each instance maintains a local tag index for the fast `has()` path. A remote
invalidation does not update other instances' local indexes; those entries become
stale. The stale local entries are silently overwritten the next time the same
key is cached, so no manual synchronisation is required.

### Prerequisite

The cache middleware must be registered on the route or a parent group before
the handler that calls `InvalidateTags`. If the middleware is absent,
`InvalidateTags` returns an error:

```
cache: InvalidateTags requires the cache middleware to be registered on this route
```

---

## Conditional Requests

The middleware supports RFC 9110 conditional request validation. Conditional
checks run on both the cache-hit path (using values stored with the cached
entry) and the cache-miss / revalidation path (using values the handler or
generators produced during the current request).

### ETag

**Auto-generation** (`EnableETag`, default `true`): when no `ETag` header has
been set by the handler or by `ETagGenerator`, the middleware computes a strong
ETag as the hex-encoded SHA-256 hash of the response body.

**Custom generation** (`ETagGenerator`): runs after the handler and before
auto-generation. When it returns a non-empty string that value is set as the
`ETag` header; auto-generation is then skipped.

```go
app.Use(cache.New(cache.Config{
    ETagGenerator: func(c fiber.Ctx, body []byte) string {
        return fmt.Sprintf(`"%s-v2"`, myHash(body))
    },
}))
```

**Validation on cache hit**: when a stored entry has an ETag and the request
carries `If-None-Match`, the middleware performs a weak comparison. A match
returns `304 Not Modified` without loading the response body from storage.

### Last-Modified

**Auto-generation** (`EnableLastModified`, default `true`): when no
`Last-Modified` header has been set by the handler or by
`LastModifiedGenerator`, the middleware sets it to the current time.

**Custom generation** (`LastModifiedGenerator`): runs after the handler. A zero
`time.Time` is treated as "not set" and does not suppress auto-generation.

```go
app.Use(cache.New(cache.Config{
    LastModifiedGenerator: func(c fiber.Ctx) time.Time {
        return db.LastModifiedForResource(c.Param("id"))
    },
}))
```

**Validation on cache hit**: when a stored entry has a `lastModified` timestamp
and the request carries `If-Modified-Since`, the middleware compares the two. If
the stored modification time is not after the client's timestamp, a
`304 Not Modified` is returned.

### Precedence (RFC 9110 §8.3)

When a request includes both `If-None-Match` and `If-Modified-Since`, only
`If-None-Match` is evaluated. `If-Modified-Since` is ignored. This rule applies
on both the cache-hit path and the miss-path conditional checks.

### Miss-Path Conditional Evaluation

On a cache miss or when revalidation forces the handler to re-run, the
middleware evaluates conditional headers against the ETag and Last-Modified
values produced during that request (by the handler, a generator, or
auto-generation). A match still produces `304 Not Modified`, preventing an
unnecessary body transfer even when the cache entry does not yet exist or has
just been replaced.

---

## Cache-Control Configuration

### Middleware-Generated Cache-Control

By default (`DisableCacheControl: false`) the middleware writes a
`Cache-Control` response header whenever the handler has not already set one.
The generated format is:

```
public, max-age=<seconds>[, must-revalidate]
```

- **`max-age`** is the remaining freshness lifetime of the entry in seconds.
- **`must-revalidate`** is appended only when the original handler response
  included `must-revalidate` or `proxy-revalidate` in its `Cache-Control`
  header. The flag is stored with the cached entry and replayed on every
  subsequent hit.

This header is produced on both the miss path (first response) and the hit path
(responses served from cache), including `304 Not Modified` replies.

### Handler-Set Cache-Control Is Preserved

When a handler sets its own `Cache-Control` header, the middleware stores the
raw header bytes with the cached entry and restores them verbatim on cache hits
and `304` responses. The automatic `public, max-age=…` generation is skipped
whenever a stored `Cache-Control` value is present.

```go
app.Get("/custom", func(c fiber.Ctx) error {
    c.Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=60")
    return c.SendString("ok")
})
// On subsequent hits the exact header above is replayed unchanged.
```

### Response Directives That Influence Caching Decisions

| Directive | Effect |
|---|---|
| `no-store` | Response is not cached. |
| `no-cache` | Any existing cached entry for this key is deleted. Response is not stored. |
| `private` | Same effect as `no-cache` in this shared-cache context. |
| `s-maxage=N` | Sets the cache lifetime to N seconds. Takes priority over `max-age`. |
| `max-age=N` | Sets the cache lifetime to N seconds when `s-maxage` is absent. |
| `must-revalidate` | Stored with the entry; appended to middleware-generated `Cache-Control` on hit. Stale entries with this flag trigger revalidation instead of being served. |
| `proxy-revalidate` | Treated identically to `must-revalidate`. |

### Expires Header

When neither `s-maxage` nor `max-age` is present in the response, the
middleware falls back to the `Expires` header. The cache lifetime is computed as
`Expires − now`. A malformed `Expires` value causes the entry to be stored with
a very short effective TTL; the next request triggers revalidation.

### ExpirationGenerator

`ExpirationGenerator` overrides all other expiration sources (`s-maxage`,
`max-age`, `Expires`, and `Config.Expiration`). When set, its return value
becomes the cache lifetime unconditionally.

```go
app.Use(cache.New(cache.Config{
    ExpirationGenerator: func(c fiber.Ctx, cfg *cache.Config) time.Duration {
        if c.Path() == "/static" {
            return 24 * time.Hour
        }
        return cfg.Expiration
    },
}))
```

### Request Directives That Influence Cache Behaviour

| Directive | Effect |
|---|---|
| `no-store` | Bypasses the cache entirely — no lookup, no storage. |
| `no-cache` (or `Pragma: no-cache`) | Cached response is not served; the handler runs. The fresh response may still be stored. |
| `max-age=N` | A cached entry older than N seconds is not served; the handler re-runs (revalidation). |
| `min-fresh=N` | A cached entry whose remaining freshness is less than N seconds triggers revalidation. |
| `max-stale[=N]` | Allows a stale entry to be served. Without a value, any degree of staleness is accepted. |
| `only-if-cached` | Returns `504 Gateway Timeout` instead of running the handler when no fresh cached response is available. |

### Disabling Cache-Control Generation

Set `DisableCacheControl: true` to suppress all middleware-generated
`Cache-Control` headers. Handler-set `Cache-Control` values are still stored
and replayed verbatim — this option only prevents the automatic
`public, max-age=…` generation.
