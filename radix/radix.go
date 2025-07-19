package radix

// node represents a path-compressed radix tree node. It stores up to 16
// children inline before promoting to a dense 256-entry table.
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

// Tree implements a simple radix tree optimized for prefix lookups.
type Tree[V any] struct {
	root *node[V]
}

// New returns an empty tree.
func New[V any]() *Tree[V] {
	return &Tree[V]{root: &node[V]{}}
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

// Insert adds the key with its value to the tree.
func (t *Tree[V]) Insert(key string, val V) {
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
		var zero V
		child.value = zero
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

// longestPrefixNoCache walks the tree to find the longest prefix match.
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
	return t.longestPrefixNoCache(s)
}

// Lookup returns the value associated with the longest prefix of s.
func (t *Tree[V]) Lookup(s string) (V, bool) {
	_, v, ok := t.longestPrefixNoCache(s)
	return v, ok
}
