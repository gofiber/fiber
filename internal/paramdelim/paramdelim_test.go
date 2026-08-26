package paramdelim

import "testing"

func TestPathEndChars(t *testing.T) {
	s := PathEndChars()
	for _, c := range []byte{'/', '-', '.', ':', '\\', '?'} {
		if !s[c] {
			t.Fatalf("expected %q to be a path-end char", c)
		}
	}
	if s['#'] {
		t.Fatal("# is a client-only delimiter, not part of the shared set")
	}
}
