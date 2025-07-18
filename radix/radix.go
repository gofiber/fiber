package radix

import (
	"sync"
	"sync/atomic"
)

// Tree implements a simple radix tree optimized for prefix lookups.
// It supports inserting string keys and searching the longest matching prefix.

// edge connects a node to its child identified by the label byte.
// The value type is generic so the tree can store any payload without
// relying on interface{} and the overhead it introduces.
type edge[V any] struct {
	label byte
	node  *node[V]
}

// node represents a single node in the radix tree. For small fan-out a slice
// of edges is used. Once the number of edges grows above a threshold the edges
// are stored in a map which provides O(1) lookups similar to the larger node
// types described in the ART paper.
type node[V any] struct {
	prefix  string
	edges   []edge[V]
	edgeMap map[byte]*node[V]
	value   V
	leaf    bool
}

// cacheEntry stores a cached lookup result.
type cacheEntry[V any] struct {
	prefix string
	value  V
}

// Tree is the exported radix tree structure. It optionally maintains a
// small lookup cache for repeated prefix queries. The cache is implemented
// using sync.Map for lock-free reads once populated.
type Tree[V any] struct {
	root      *node[V]
	cacheSize int
	cache     atomic.Value // map[string]cacheEntry[V]
	mu        sync.Mutex
}

// New creates a new radix tree. When cacheSize is 0, the lookup cache is
// disabled to avoid any synchronization overhead.
func New[V any](cacheSize ...int) *Tree[V] {
	size := 0
	if len(cacheSize) > 0 {
		size = cacheSize[0]
	}
	t := &Tree[V]{
		root:      &node[V]{},
		cacheSize: size,
	}
	if size > 0 {
		t.cache.Store(make(map[string]cacheEntry[V]))
	}
	return t
}

// getEdge returns the child edge for the given label.
func (n *node[V]) getEdge(b byte) *node[V] {
	if n.edgeMap != nil {
		return n.edgeMap[b]
	}
	for i := range n.edges {
		if n.edges[i].label == b {
			return n.edges[i].node
		}
	}
	return nil
}

// setEdge sets or replaces the child edge for the given label.
func (n *node[V]) setEdge(b byte, child *node[V]) {
	if n.edgeMap != nil {
		n.edgeMap[b] = child
		return
	}
	for i := range n.edges {
		if n.edges[i].label == b {
			n.edges[i].node = child
			return
		}
	}
	n.edges = append(n.edges, edge[V]{label: b, node: child})
	// Promote to map once node becomes dense.
	if len(n.edges) > 16 {
		n.edgeMap = make(map[byte]*node[V], len(n.edges))
		for i := range n.edges {
			n.edgeMap[n.edges[i].label] = n.edges[i].node
		}
		n.edges = nil
	}
}

// longestPrefixLen returns the length of the common prefix between a and b.
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

// Insert adds the key with its value to the tree, replacing existing values.
func (t *Tree[V]) Insert(key string, val V) {

	n := t.root
	for {
		if len(key) == 0 {
			n.value = val
			n.leaf = true
			return
		}
		c := key[0]
		child := n.getEdge(c)
		if child == nil {
			n.setEdge(c, &node[V]{prefix: key, leaf: true, value: val})
			return
		}
		l := longestPrefixLen(child.prefix, key)
		if l == len(child.prefix) {
			n = child
			key = key[l:]
			continue
		}
		split := &node[V]{
			prefix: child.prefix[l:],
			edges:  child.edges,
			value:  child.value,
			leaf:   child.leaf,
		}
		child.prefix = child.prefix[:l]
		child.edges = []edge[V]{{label: split.prefix[0], node: split}}
		var zero V
		child.value = zero
		child.leaf = false
		if l == len(key) {
			child.value = val
			child.leaf = true
			return
		}
		newChild := &node[V]{prefix: key[l:], leaf: true, value: val}
		child.edges = append(child.edges, edge[V]{label: newChild.prefix[0], node: newChild})
		return
	}
}

// LongestPrefix finds the value for the longest key that is a prefix of s.
// It returns the matched prefix, the value, and whether a match was found.
// longestPrefixNoCache contains the core search algorithm without consulting the cache.
func (t *Tree[V]) longestPrefixNoCache(s string) (string, V, bool) {
	n := t.root
	idx := 0
	var (
		lastPrefix string
		lastVal    V
		found      bool
	)
	for {
		if n == nil {
			break
		}
		if len(n.prefix) > 0 {
			if len(s[idx:]) < len(n.prefix) || s[idx:idx+len(n.prefix)] != n.prefix {
				break
			}
			idx += len(n.prefix)
		}
		if n.leaf {
			lastPrefix = s[:idx]
			lastVal = n.value
			found = true
		}
		if idx >= len(s) {
			break
		}
		n = n.getEdge(s[idx])
		if n == nil {
			break
		}
	}
	if !found {
		var zero V
		return "", zero, false
	}
	return lastPrefix, lastVal, true
}

// LongestPrefix finds the value for the longest key that is a prefix of s. It
// first checks the internal LRU cache and falls back to walking the tree on a
// miss. Cached entries are promoted to the front on access.
func (t *Tree[V]) LongestPrefix(s string) (string, V, bool) {
	if t.cacheSize == 0 {
		return t.longestPrefixNoCache(s)
	}

	if v := t.cache.Load(); v != nil {
		m := v.(map[string]cacheEntry[V])
		if ce, ok := m[s]; ok {
			return ce.prefix, ce.value, true
		}
	}

	prefix, val, ok := t.longestPrefixNoCache(s)
	if ok {
		t.mu.Lock()
		defer t.mu.Unlock()
		m, _ := t.cache.Load().(map[string]cacheEntry[V])
		if m == nil {
			m = make(map[string]cacheEntry[V])
		}
		if len(m) < t.cacheSize {
			newM := make(map[string]cacheEntry[V], len(m)+1)
			for k, v := range m {
				newM[k] = v
			}
			newM[s] = cacheEntry[V]{prefix: prefix, value: val}
			t.cache.Store(newM)
		}
	}
	return prefix, val, ok
}
