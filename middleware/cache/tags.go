package cache

import (
	"strings"
	"sync"
)

// tagStore abstracts the tag↔cache-key index. When no external storage is
// configured the lightweight in-memory tagIndex is used. When the middleware
// shares an external storage backend (e.g. Redis) a distributedTagStore
// persists the index there so that InvalidateTags propagates across every
// instance that uses the same backend.
type tagStore interface {
	// add registers key under the given tags.
	add(key string, tags []string)
	// remove deletes key from every tag it belongs to.
	remove(key string)
	// has reports whether key is currently tracked.
	has(key string) bool
	// invalidate collects all keys associated with the given tags, removes the
	// tag entries, and returns the collected keys so the caller can delete them
	// from the cache.
	invalidate(tags []string) []string
}

// tagIndex maintains a bidirectional mapping between tags and cache keys,
// enabling O(1) lookup of all keys for a given tag. The forward index
// (tags → keys) supports invalidation; the reverse index (keys → tags)
// supports cleanup when entries are evicted or expire.
type tagIndex struct {
	mu sync.Mutex
	// tag name → set of cache keys
	tags map[string]map[string]struct{}
	// cache key → set of tag names (reverse index for cleanup on eviction)
	keys map[string]map[string]struct{}
}

func newTagIndex() *tagIndex {
	return &tagIndex{
		tags: make(map[string]map[string]struct{}),
		keys: make(map[string]map[string]struct{}),
	}
}

// add registers a cache key under the given tags and records the reverse mapping.
func (ti *tagIndex) add(key string, tags []string) {
	if len(tags) == 0 {
		return
	}
	ti.mu.Lock()
	defer ti.mu.Unlock()

	for _, tag := range tags {
		if ti.tags[tag] == nil {
			ti.tags[tag] = make(map[string]struct{})
		}
		ti.tags[tag][key] = struct{}{}
	}
	if ti.keys[key] == nil {
		ti.keys[key] = make(map[string]struct{}, len(tags))
	}
	for _, tag := range tags {
		ti.keys[key][tag] = struct{}{}
	}
}

// has reports whether key is already tracked in the reverse index.
func (ti *tagIndex) has(key string) bool {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	_, ok := ti.keys[key]
	return ok
}

// remove deletes a cache key from all tags it belongs to. Called during
// eviction or expiration to keep the index consistent.
func (ti *tagIndex) remove(key string) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	tags, ok := ti.keys[key]
	if !ok {
		return
	}
	for tag := range tags {
		if keySet := ti.tags[tag]; keySet != nil {
			delete(keySet, key)
			if len(keySet) == 0 {
				delete(ti.tags, tag)
			}
		}
	}
	delete(ti.keys, key)
}

// invalidate collects all cache keys associated with any of the given tags,
// removes those tag entries from the forward index, and cleans up the
// reverse index for keys that have no remaining tags. It acquires and
// releases its own lock; callers must not hold mux simultaneously.
func (ti *tagIndex) invalidate(tags []string) []string {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	// Collect unique keys across all requested tags
	seen := make(map[string]struct{})
	for _, tag := range tags {
		for key := range ti.tags[tag] {
			seen[key] = struct{}{}
		}
		delete(ti.tags, tag)
	}

	// Build result slice and clean up reverse index
	tagSet := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tagSet[tag] = struct{}{}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
		if keyTags := ti.keys[key]; keyTags != nil {
			for tag := range tagSet {
				delete(keyTags, tag)
			}
			if len(keyTags) == 0 {
				delete(ti.keys, key)
			}
		}
	}
	return keys
}

// rejectMatcher pre-classifies tag reject patterns into three buckets
// for efficient runtime matching: exact strings use a map (O(1)),
// single trailing-* patterns use prefix checks, and everything else
// falls back to full glob matching.
type rejectMatcher struct {
	exact   map[string]struct{} // no wildcards
	prefix  []string            // single trailing *, stored without the *
	general []string            // all other glob patterns
}

func newRejectMatcher(patterns []string) *rejectMatcher {
	m := &rejectMatcher{
		exact: make(map[string]struct{}),
	}
	for _, p := range patterns {
		switch {
		case !strings.Contains(p, "*"):
			m.exact[p] = struct{}{}
		case strings.Count(p, "*") == 1 && p[len(p)-1] == '*':
			m.prefix = append(m.prefix, p[:len(p)-1])
		default:
			m.general = append(m.general, p)
		}
	}
	return m
}

// matches reports whether tag matches any reject pattern.
func (m *rejectMatcher) matches(tag string) bool {
	if _, ok := m.exact[tag]; ok {
		return true
	}
	for _, prefix := range m.prefix {
		if strings.HasPrefix(tag, prefix) {
			return true
		}
	}
	for _, pattern := range m.general {
		if globMatch(pattern, tag) {
			return true
		}
	}
	return false
}

// matchesAny reports whether any tag in the slice matches a reject pattern.
func (m *rejectMatcher) matchesAny(tags []string) bool {
	for _, tag := range tags {
		if m.matches(tag) {
			return true
		}
	}
	return false
}

// globMatch reports whether s matches the glob pattern.
// '*' matches any sequence of characters (including none).
// All other characters match literally.
func globMatch(pattern, s string) bool {
	for len(pattern) > 0 {
		if pattern[0] == '*' {
			// Collapse consecutive stars
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true // trailing * matches everything remaining
			}
			// Try matching the rest of the pattern at every position in s
			for i := range len(s) + 1 {
				if globMatch(pattern, s[i:]) {
					return true
				}
			}
			return false
		}
		if len(s) == 0 || pattern[0] != s[0] {
			return false
		}
		pattern = pattern[1:]
		s = s[1:]
	}
	return len(s) == 0
}
