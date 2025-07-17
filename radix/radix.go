package radix

// Tree implements a simple radix tree optimized for prefix lookups.
// It supports inserting string keys and searching the longest matching prefix.

type edge struct {
	label byte
	node  *node
}

type node struct {
	prefix string
	edges  []edge
	value  any
	leaf   bool
}

// Tree is the exported radix tree structure.
type Tree struct {
	root *node
}

// New creates a new empty radix tree.
func New() *Tree {
	return &Tree{root: &node{}}
}

// getEdge returns the child edge for the given label.
func (n *node) getEdge(b byte) *node {
	for i := range n.edges {
		if n.edges[i].label == b {
			return n.edges[i].node
		}
	}
	return nil
}

// setEdge sets or replaces the child edge for the given label.
func (n *node) setEdge(b byte, child *node) {
	for i := range n.edges {
		if n.edges[i].label == b {
			n.edges[i].node = child
			return
		}
	}
	n.edges = append(n.edges, edge{label: b, node: child})
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
func (t *Tree) Insert(key string, val any) {
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
			n.setEdge(c, &node{prefix: key, leaf: true, value: val})
			return
		}
		l := longestPrefixLen(child.prefix, key)
		if l == len(child.prefix) {
			n = child
			key = key[l:]
			continue
		}
		split := &node{
			prefix: child.prefix[l:],
			edges:  child.edges,
			value:  child.value,
			leaf:   child.leaf,
		}
		child.prefix = child.prefix[:l]
		child.edges = []edge{{label: split.prefix[0], node: split}}
		child.value = nil
		child.leaf = false
		if l == len(key) {
			child.value = val
			child.leaf = true
			return
		}
		newChild := &node{prefix: key[l:], leaf: true, value: val}
		child.edges = append(child.edges, edge{label: newChild.prefix[0], node: newChild})
		return
	}
}

// LongestPrefix finds the value for the longest key that is a prefix of s.
// It returns the matched prefix, the value, and whether a match was found.
func (t *Tree) LongestPrefix(s string) (string, any, bool) {
	n := t.root
	idx := 0
	var (
		lastPrefix string
		lastVal    any
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
		}
		if idx >= len(s) {
			break
		}
		n = n.getEdge(s[idx])
		if n == nil {
			break
		}
	}
	if lastVal == nil {
		return "", nil, false
	}
	return lastPrefix, lastVal, true
}
