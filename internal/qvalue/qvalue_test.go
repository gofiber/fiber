package qvalue

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		want  float64
		ok    bool
	}{
		{"0", 0, true},
		{"1", 1, true},
		{"0.5", 0.5, true},
		{"0.001", 0.001, true},
		{"0.000", 0, true},
		{"1.000", 1, true},
		{"1.0", 1, true},
		{"", 0, false},
		{"-0", 0, false},
		{"nan", 0, false},
		{"inf", 0, false},
		{"1.5", 0, false},
		{"+0.5", 0, false},
		{"0.0001", 0, false},
		{"bogus", 0, false},
		{".5", 0, false},
		{"00", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			got, ok := Parse([]byte(tt.value))
			if ok != tt.ok || (ok && got != tt.want) {
				t.Errorf("Parse(%q) = %v, %v; want %v, %v", tt.value, got, ok, tt.want, tt.ok)
			}
		})
	}
}
