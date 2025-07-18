package radix

import (
	"container/list"
	"sync"
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
	key    string
	prefix string
	value  V
}

// Tree is the exported radix tree structure. It contains an optional
// LRU cache to speed up repeated lookups of the same path.
// Tree is the exported radix tree structure. It contains an optional
// LRU cache to speed up repeated lookups of the same path.
type Tree[V any] struct {
	root       *node[V]
	cacheSize  int
	cache      map[string]*list.Element
	order      *list.List
	cacheMutex sync.RWMutex
}

// New creates a new empty radix tree.
// New creates a new empty radix tree.
func New[V any]() *Tree[V] {
	return &Tree[V]{
		root:      &node[V]{},
		cacheSize: 1024,
		cache:     make(map[string]*list.Element),
		order:     list.New(),
	}
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
	t.cacheMutex.Lock()
	// clear cache on modifications
	if len(t.cache) > 0 {
		t.cache = make(map[string]*list.Element)
		t.order.Init()
	}
	t.cacheMutex.Unlock()

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
	t.cacheMutex.RLock()
	if elem, ok := t.cache[s]; ok {
		ce := elem.Value.(*cacheEntry[V])
		t.order.MoveToFront(elem)
		t.cacheMutex.RUnlock()
		return ce.prefix, ce.value, true
	}
	t.cacheMutex.RUnlock()

	prefix, val, ok := t.longestPrefixNoCache(s)
	if ok {
		t.cacheMutex.Lock()
		if len(t.cache) >= t.cacheSize {
			back := t.order.Back()
			if back != nil {
				be := back.Value.(*cacheEntry[V])
				delete(t.cache, be.key)
				t.order.Remove(back)
			}
		}
		ce := &cacheEntry[V]{key: s, prefix: prefix, value: val}
		elem := t.order.PushFront(ce)
		t.cache[s] = elem
		t.cacheMutex.Unlock()
	}
	return prefix, val, ok
}
