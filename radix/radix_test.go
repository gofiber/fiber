package radix

import "testing"

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
