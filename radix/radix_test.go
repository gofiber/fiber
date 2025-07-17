package radix

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTreeInsertAndLookup(t *testing.T) {
	tree := New()
	tree.Insert("/", 0)
	tree.Insert("/foo", 1)
	tree.Insert("/foobar", 2)
	tree.Insert("/bar", 3)

	cases := []struct {
		path   string
		prefix string
		val    any
		ok     bool
	}{
		{"/foo", "/foo", 1, true},
		{"/foobar/baz", "/foobar", 2, true},
		{"/bar/baz", "/bar", 3, true},
		{"/unknown", "/", 0, true},
	}

	for _, c := range cases {
		p, v, ok := tree.LongestPrefix(c.path)
		if ok != c.ok {
			t.Fatalf("%s: expected ok %v, got %v", c.path, c.ok, ok)
		}
		if p != c.prefix {
			t.Fatalf("%s: expected prefix %s, got %s", c.path, c.prefix, p)
		}
		if v != c.val {
			t.Fatalf("%s: expected val %v, got %v", c.path, c.val, v)
		}
	}
}

func TestTreeOverwrite(t *testing.T) {
	tree := New()
	tree.Insert("/foo", 1)
	tree.Insert("/foo", 2)
	_, v, ok := tree.LongestPrefix("/foo")
	if !ok || v != 2 {
		t.Fatalf("overwrite failed: %v %v", ok, v)
	}
}

func TestLongestPrefixLen(t *testing.T) {
	t.Parallel()
	require.Equal(t, 4, longestPrefixLen("/foo", "/foobar"))
	require.Equal(t, 0, longestPrefixLen("abc", "xyz"))
	require.Equal(t, 2, longestPrefixLen("abcd", "ab"))
}

func TestNodeGetSetEdge(t *testing.T) {
	t.Parallel()
	n := &node{}
	child1 := &node{prefix: "a"}
	n.setEdge('a', child1)
	require.Equal(t, child1, n.getEdge('a'))

	child2 := &node{prefix: "b"}
	n.setEdge('a', child2)
	require.Equal(t, child2, n.getEdge('a'))
	require.Nil(t, n.getEdge('b'))
}

func TestTreeInsertSplitAndSearch(t *testing.T) {
	t.Parallel()
	tree := New()
	tree.Insert("", 0) // value on root node
	tree.Insert("/foo", 1)
	tree.Insert("/fo", 2) // causes split
	tree.Insert("/fob", 3)
	tree.Insert("/fuz", 4)

	p, v, ok := tree.LongestPrefix("/foo")
	require.True(t, ok)
	require.Equal(t, "/foo", p)
	require.Equal(t, 1, v)

	p, v, ok = tree.LongestPrefix("/fo")
	require.True(t, ok)
	require.Equal(t, "/fo", p)
	require.Equal(t, 2, v)

	p, v, ok = tree.LongestPrefix("/fob")
	require.True(t, ok)
	require.Equal(t, "/fob", p)
	require.Equal(t, 3, v)

	p, v, ok = tree.LongestPrefix("/fuz/extra")
	require.True(t, ok)
	require.Equal(t, "/fuz", p)
	require.Equal(t, 4, v)

	p, v, ok = tree.LongestPrefix("/")
	require.True(t, ok)
	require.Equal(t, "", p)
	require.Equal(t, 0, v)

	p, v, ok = tree.LongestPrefix("/unknown")
	require.True(t, ok)
	require.Equal(t, "", p)
	require.Equal(t, 0, v)
}

func TestTreeLongestPrefixNoMatch(t *testing.T) {
	t.Parallel()
	tree := New()
	tree.Insert("/foo", 1)
	p, v, ok := tree.LongestPrefix("/bar")
	require.False(t, ok)
	require.Equal(t, "", p)
	require.Nil(t, v)
}

func TestTreeLongestPrefixNilTree(t *testing.T) {
	t.Parallel()
	tree := New()
	tree.root = nil
	p, v, ok := tree.LongestPrefix("/foo")
	require.False(t, ok)
	require.Equal(t, "", p)
	require.Nil(t, v)
}

func TestTreeLongestPrefixEmpty(t *testing.T) {
	t.Parallel()
	tree := New()
	tree.Insert("", 42)
	p, v, ok := tree.LongestPrefix("")
	require.True(t, ok)
	require.Equal(t, "", p)
	require.Equal(t, 42, v)
}
