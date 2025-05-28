package cache

import (
	"testing"
	"time"
)

func TestParseMaxAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		expect time.Duration
		ok     bool
	}{
		{"max-age=60", 60 * time.Second, true},
		{"public, max-age=86400", 86400 * time.Second, true},
		{"no-store", 0, false},
		{"max-age=invalid", 0, false},
		{"public, s-maxage=100, max-age=50", 50 * time.Second, true},
		{"MAX-AGE=20", 20 * time.Second, true},
		{"public , max-age=0", 0, true},
		{"public , max-age", 0, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.header, func(t *testing.T) {
			t.Parallel()
			d, ok := parseMaxAge(tt.header)
			if tt.ok != ok {
				t.Fatalf("expected ok=%v got %v", tt.ok, ok)
			}
			if ok && d != tt.expect {
				t.Fatalf("expected %v got %v", tt.expect, d)
			}
		})
	}
}
