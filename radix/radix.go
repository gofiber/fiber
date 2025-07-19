package radix

import (
	"sync"
	"sync/atomic"
)

// node implements a compact radix tree node with optional promotion to a
// dense 256-entry child table. It stores up to 16 children directly in two
// parallel arrays before switching to the dense table.
type node[V any] struct {
	prefix string
	value  V
	leaf   bool

	smallKeys     [16]byte
	smallChildren [16]*node[V]
	smallCount    uint8

	big *[256]*node[V]
}

func (n *node[V]) getEdge(b byte) *node[V]        { return n.get(b) }
func (n *node[V]) setEdge(b byte, child *node[V]) { n.set(b, child) }

func (n *node[V]) get(b byte) *node[V] {
	if n.big != nil {
		return n.big[b]
	}
	for i := uint8(0); i < n.smallCount; i++ {
		if n.smallKeys[i] == b {
			return n.smallChildren[i]
		}
	}
	return nil
}

func (n *node[V]) set(b byte, child *node[V]) {
	if n.big != nil {
		n.big[b] = child
		return
	}
	for i := uint8(0); i < n.smallCount; i++ {
		if n.smallKeys[i] == b {
			n.smallChildren[i] = child
			return
		}
	}
	if n.smallCount < 16 {
		n.smallKeys[n.smallCount] = b
		n.smallChildren[n.smallCount] = child
		n.smallCount++
		return
	}
	tbl := new([256]*node[V])
	for i := uint8(0); i < n.smallCount; i++ {
		tbl[n.smallKeys[i]] = n.smallChildren[i]
		n.smallChildren[i] = nil
	}
	n.big = tbl
	n.smallCount = 0
	n.smallKeys = [16]byte{}
	n.set(b, child)
}

// cacheEntry holds a cached lookup result.
type cacheEntry[V any] struct {
	prefix string
	value  V
}

// Tree is a radix tree optimized for prefix lookups. It keeps a small
// copy-on-write cache for hot paths when enabled.
type Tree[V any] struct {
	root      *node[V]
	cacheSize int
	cache     atomic.Value // map[string]cacheEntry[V]
	mu        sync.Mutex
	frozen    bool
}

// New creates a new tree. When cacheSize is 0, caching is disabled.
func New[V any](cacheSize ...int) *Tree[V] {
	size := 0
	if len(cacheSize) > 0 {
		size = cacheSize[0]
	}
	t := &Tree[V]{root: &node[V]{}, cacheSize: size}
	if size > 0 {
		t.cache.Store(make(map[string]cacheEntry[V]))
	}
	return t
}

// longestPrefixLen returns the length of the common prefix of a and b.
func longestPrefixLen(a, b string) int {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}
	i := 0
	for i < max && a[i] == b[i] {
		i++
	}
	return i
}

// Insert adds key/value to the tree.
func (t *Tree[V]) Insert(key string, val V) {
	if t.frozen {
		return
	}
	n := t.root
	for {
		if len(key) == 0 {
			n.value = val
			n.leaf = true
			return
		}
		c := key[0]
		child := n.get(c)
		if child == nil {
			n.set(c, &node[V]{prefix: key, leaf: true, value: val})
			return
		}
		l := longestPrefixLen(child.prefix, key)
		if l == len(child.prefix) {
			n = child
			key = key[l:]
			continue
		}
		split := &node[V]{
			prefix:        child.prefix[l:],
			value:         child.value,
			leaf:          child.leaf,
			smallKeys:     child.smallKeys,
			smallChildren: child.smallChildren,
			smallCount:    child.smallCount,
			big:           child.big,
		}
		child.prefix = child.prefix[:l]
		child.smallKeys = [16]byte{}
		child.smallChildren = [16]*node[V]{}
		child.smallCount = 0
		child.big = nil
		child.value = *new(V)
		child.leaf = false
		child.set(split.prefix[0], split)
		if l == len(key) {
			child.value = val
			child.leaf = true
			return
		}
		child.set(key[l], &node[V]{prefix: key[l:], leaf: true, value: val})
		return
	}
}

// longestPrefixNoCache searches the tree without consulting the cache.
func (t *Tree[V]) longestPrefixNoCache(s string) (string, V, bool) {
	n := t.root
	idx := 0
	lastIdx := 0
	var lastVal V
	found := false
	for n != nil {
		if len(n.prefix) > 0 {
			if idx+len(n.prefix) > len(s) || s[idx:idx+len(n.prefix)] != n.prefix {
				break
			}
			idx += len(n.prefix)
		}
		if n.leaf {
			lastIdx = idx
			lastVal = n.value
			found = true
		}
		if idx >= len(s) {
			break
		}
		n = n.get(s[idx])
	}
	if !found {
		var zero V
		return "", zero, false
	}
	return s[:lastIdx], lastVal, true
}

// LongestPrefix returns the longest prefix match for s.
func (t *Tree[V]) LongestPrefix(s string) (string, V, bool) {
	if t.cacheSize > 0 {
		if v := t.cache.Load(); v != nil {
			if ce, ok := v.(map[string]cacheEntry[V])[s]; ok {
				return ce.prefix, ce.value, true
			}
		}
	}
	p, v, ok := t.longestPrefixNoCache(s)
	if ok && t.cacheSize > 0 {
		t.mu.Lock()
		m, _ := t.cache.Load().(map[string]cacheEntry[V])
		if m == nil {
			m = make(map[string]cacheEntry[V])
		}
		if len(m) < t.cacheSize {
			nmap := make(map[string]cacheEntry[V], len(m)+1)
			for k, v := range m {
				nmap[k] = v
			}
			nmap[s] = cacheEntry[V]{prefix: p, value: v}
			t.cache.Store(nmap)
		}
		t.mu.Unlock()
	}
	return p, v, ok
}

// Lookup is like LongestPrefix but only returns the value.
func (t *Tree[V]) Lookup(s string) (V, bool) {
	if t.cacheSize > 0 {
		if v := t.cache.Load(); v != nil {
			if ce, ok := v.(map[string]cacheEntry[V])[s]; ok {
				return ce.value, true
			}
		}
	}
	_, v, ok := t.longestPrefixNoCache(s)
	if ok && t.cacheSize > 0 {
		t.mu.Lock()
		m, _ := t.cache.Load().(map[string]cacheEntry[V])
		if m == nil {
			m = make(map[string]cacheEntry[V])
		}
		if len(m) < t.cacheSize {
			nmap := make(map[string]cacheEntry[V], len(m)+1)
			for k, v := range m {
				nmap[k] = v
			}
			nmap[s] = cacheEntry[V]{value: v}
			t.cache.Store(nmap)
		}
		t.mu.Unlock()
	}
	return v, ok
}

// Freeze makes the tree read-only and releases dense tables.
func (t *Tree[V]) Freeze() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.frozen {
		return
	}
	freezeNode(t.root)
	t.frozen = true
}

func freezeNode[V any](n *node[V]) {
	if n == nil {
		return
	}
	for i := uint8(0); i < n.smallCount; i++ {
		freezeNode(n.smallChildren[i])
		n.smallChildren[i] = nil
	}
	if n.big != nil {
		for i := 0; i < 256; i++ {
			if n.big[i] != nil {
				freezeNode(n.big[i])
			}
		}
		n.big = nil
	}
}
