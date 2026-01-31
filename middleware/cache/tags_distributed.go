package cache

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	tagKeyPrefix    = "__cache_tag__:"
	tagRevKeyPrefix = "__cache_tagrev__:"
)

// distributedTagStore backs the tag↔key index in a shared external storage
// (e.g. Redis) so that InvalidateTags propagates across every middleware
// instance that shares the same backend. A local tagIndex is kept in sync
// for the fast has() path used on cache hit.
type distributedTagStore struct {
	local   *tagIndex
	storage fiber.Storage
	mu      sync.Mutex    // serialises read-modify-write cycles on shared storage
	ttl     time.Duration // TTL for tag index entries in shared storage
}

func newDistributedTagStore(storage fiber.Storage, ttl time.Duration) *distributedTagStore {
	return &distributedTagStore{
		local:   newTagIndex(),
		storage: storage,
		ttl:     ttl,
	}
}

// add registers key under the given tags in both the local index and the
// shared storage backend.
func (d *distributedTagStore) add(key string, tags []string) {
	if len(tags) == 0 {
		return
	}
	d.local.add(key, tags)

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, tag := range tags {
		fwdKey := tagKeyPrefix + tag
		fwd := d.readSet(fwdKey)
		if !sliceContains(fwd, key) {
			d.writeSet(fwdKey, append(fwd, key))
		}
	}
	revKey := tagRevKeyPrefix + key
	rev := d.readSet(revKey)
	for _, tag := range tags {
		if !sliceContains(rev, tag) {
			rev = append(rev, tag)
		}
	}
	d.writeSet(revKey, rev)
}

// remove deletes key from all tags in both the local index and shared storage.
func (d *distributedTagStore) remove(key string) {
	d.local.remove(key)

	d.mu.Lock()
	defer d.mu.Unlock()

	revKey := tagRevKeyPrefix + key
	rev := d.readSet(revKey)
	if len(rev) == 0 {
		return
	}
	for _, tag := range rev {
		fwdKey := tagKeyPrefix + tag
		fwd := d.readSet(fwdKey)
		d.writeSet(fwdKey, sliceRemove(fwd, key))
	}
	_ = d.storage.Delete(revKey)
}

// has reports whether key is tracked locally. This is the fast-path check
// used by the middleware's cache-hit re-population guard.
func (d *distributedTagStore) has(key string) bool {
	return d.local.has(key)
}

// invalidate reads the authoritative forward index from shared storage,
// collects all keys for the requested tags, cleans up both shared and local
// indexes, and returns the collected keys so the caller can evict them.
func (d *distributedTagStore) invalidate(tags []string) []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	seen := make(map[string]struct{})
	for _, tag := range tags {
		fwdKey := tagKeyPrefix + tag
		for _, k := range d.readSet(fwdKey) {
			seen[k] = struct{}{}
		}
		_ = d.storage.Delete(fwdKey)
	}

	tagSet := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tagSet[tag] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
		revKey := tagRevKeyPrefix + key
		rev := d.readSet(revKey)
		filtered := make([]string, 0, len(rev))
		for _, t := range rev {
			if _, ok := tagSet[t]; !ok {
				filtered = append(filtered, t)
			}
		}
		d.writeSet(revKey, filtered)
		d.local.remove(key)
	}
	return result
}

// readSet reads a string set from shared storage. Returns nil when the key
// does not exist.
func (d *distributedTagStore) readSet(key string) []string {
	raw, err := d.storage.Get(key)
	if err != nil || raw == nil {
		return nil
	}
	return decodeStringSet(raw)
}

// writeSet persists a string set to shared storage. An empty slice causes the
// key to be deleted.
func (d *distributedTagStore) writeSet(key string, ss []string) {
	if len(ss) == 0 {
		_ = d.storage.Delete(key)
		return
	}
	_ = d.storage.Set(key, encodeStringSet(ss), d.ttl)
}

// encodeStringSet serialises a string slice into a compact binary format:
// [count uint32 LE] ([len uint32 LE] [bytes])*
func encodeStringSet(ss []string) []byte {
	size := 4
	for _, s := range ss {
		size += 4 + len(s)
	}
	buf := make([]byte, size)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(ss)))
	off := 4
	for _, s := range ss {
		binary.LittleEndian.PutUint32(buf[off:off+4], uint32(len(s)))
		off += 4
		copy(buf[off:], s)
		off += len(s)
	}
	return buf
}

// decodeStringSet deserialises a []byte produced by encodeStringSet. Returns
// nil when data is nil or too short to contain a valid header.
func decodeStringSet(data []byte) []string {
	if len(data) < 4 {
		return nil
	}
	count := binary.LittleEndian.Uint32(data[0:4])
	ss := make([]string, 0, count)
	off := 4
	for i := uint32(0); i < count; i++ {
		if off+4 > len(data) {
			return ss
		}
		n := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if off+n > len(data) {
			return ss
		}
		ss = append(ss, string(data[off:off+n]))
		off += n
	}
	return ss
}

func sliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// sliceRemove returns ss with all occurrences of s removed.
func sliceRemove(ss []string, s string) []string {
	n := 0
	for _, v := range ss {
		if v != s {
			ss[n] = v
			n++
		}
	}
	return ss[:n]
}
