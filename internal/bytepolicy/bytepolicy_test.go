package bytepolicy

import (
	"testing"
)

var policies = []struct {
	mask   func(uint64) uint64
	is     func(byte) bool
	reject func(byte) bool
	name   string
}{
	{
		name: "non-visible ASCII", mask: NonVisibleASCIIMask, is: IsNonVisibleASCII,
		reject: func(c byte) bool { return c < ' ' || c > '~' },
	},
	{
		name: "non-visible ASCII or tab", mask: NonVisibleASCIIOrTabMask, is: IsNonVisibleASCIIOrTab,
		reject: func(c byte) bool { return c != '\t' && (c < ' ' || c > '~') },
	},
	{
		name: "quoted string escape", mask: QuotedStringEscapeMask, is: IsQuotedStringEscape,
		reject: func(c byte) bool { return c < ' ' || c == 127 || c == '"' || c == '\\' },
	},
}

// referenceMask marks bit 7 of each lane of w that reject matches, lane i
// being byte i of the little-endian word swar.Load8 returns.
func referenceMask(w uint64, reject func(byte) bool) uint64 {
	var m uint64
	for lane := range 8 {
		if reject(byte(w >> (8 * lane))) {
			m |= 0x80 << (8 * lane)
		}
	}
	return m
}

func Test_Policies_AllBytes(t *testing.T) {
	t.Parallel()
	for _, p := range policies {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			for c := range 256 {
				if got, want := p.is(byte(c)), p.reject(byte(c)); got != want {
					t.Fatalf("byte %02x: got %v, want %v", c, got, want)
				}
			}
			// Every byte in every lane, next to every other byte in the rest.
			for a := range uint64(256) {
				for b := range uint64(256) {
					for lane := range 8 {
						w := b*0x0101010101010101&^(0xff<<(8*lane)) | a<<(8*lane)
						if got, want := p.mask(w), referenceMask(w, p.reject); got != want {
							t.Fatalf("word %016x: got %016x, want %016x", w, got, want)
						}
					}
				}
			}
		})
	}
}

func FuzzPolicies(f *testing.F) {
	for _, seed := range []uint64{0, 0x0909090909090909, 0x7f7f7f7f7f7f7f7f, 0x8080808080808080, 0x2220225c7e1f8000} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, w uint64) {
		for _, p := range policies {
			if got, want := p.mask(w), referenceMask(w, p.reject); got != want {
				t.Fatalf("%s word %016x: got %016x, want %016x", p.name, w, got, want)
			}
		}
	})
}
