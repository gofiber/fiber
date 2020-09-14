package iso8601

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		value string
		flags ValidFlags
		valid bool
	}{
		// valid
		{"2018-01-01T23:42:59.123456789Z", Strict, true},
		{"2018-01-01T23:42:59.123456789+07:00", Strict, true},
		{"2018-01-01T23:42:59.123456789-07:00", Strict, true},
		{"2018-01-01T23:42:59.000+07:00", Strict, true},

		{"2018-01-01", Flexible, true},
		{"2018-01-01 23:42:59", Flexible, true},
		{"2018-01-01T23:42:59.123-0700", Flexible, true},

		// invalid
		{"", Flexible, false},                                 // empty string
		{"whatever", Flexible, false},                         // not a time
		{"2018-01-01", Strict, false},                         // missing time
		{"2018-01-01 23:42:59-0700", Strict, false},           // missing subsecond
		{"2018-01-01T23:42:59.123456789+0700", Strict, false}, // don't allow numeric time zone
		{"2018_01-01T23:42:59.123456789Z", Strict, false},     // invalid date separator (first)
		{"2018-01_01T23:42:59.123456789Z", Strict, false},     // invalid date separator (second)
		{"2018-01-01 23:42:59.123456789Z", Strict, false},     // invalid date-time separator
		{"2018-01-01T23-42:59.123456789Z", Strict, false},     // invalid time separator (first)
		{"2018-01-01T23:42-59.123456789Z", Strict, false},     // invalid time separator (second)
		{"2018-01-01T23:42:59,123456789Z", Strict, false},     // invalid decimal separator
		{"2018-01-01T23:42:59.123456789", Strict, false},      // missing timezone
		{"18-01-01T23:42:59.123456789Z", Strict, false},       // 2-digit year
		{"2018-1-01T23:42:59.123456789Z", Strict, false},      // 1-digit month
		{"2018-01-1T23:42:59.123456789Z", Strict, false},      // 1-digit day
		{"2018-01-01T3:42:59.123456789Z", Strict, false},      // 1-digit hour
		{"2018-01-01T23:2:59.123456789Z", Strict, false},      // 1-digit minute
		{"2018-01-01T23:42:9.123456789Z", Strict, false},      // 1-digit second
		{"2018-01-01T23:42:59.Z", Strict, false},              // not enough subsecond digits
		{"2018-01-01T23:42:59.1234567890Z", Strict, false},    // too many subsecond digits
		{"2018-01-01T23:42:59.123456789+7:00", Strict, false}, // 1-digit timezone hour
		{"2018-01-01T23:42:59.123456789+07:0", Strict, false}, // 1-digit timezone minute
		{"2018-01-01_23:42:59", Flexible, false},              // invalid date-time separator (not a space)
	}

	for _, test := range tests {
		if test.valid != Valid(test.value, test.flags) {
			t.Errorf("%q expected Valid to return %t", test.value, test.valid)
		} else if test.valid {
			if !isIsoString(test.value) {
				t.Errorf("behavior mismatch, isIsoString says %q must not be a valid date", test.value)
			}
		} else if test.flags != Strict {
			if isIsoString(test.value) {
				t.Errorf("behavior mismatch, isIsoString says %q must be a valid date", test.value)
			}
		}
	}
}

func BenchmarkValidate(b *testing.B) {
	b.Run("success", benchmarkValidateSuccess)
	b.Run("failure", benchmarkValidateFailure)
}

func benchmarkValidateSuccess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if !Valid("2018-01-01T23:42:59.123456789Z", Flexible) {
			b.Fatal("not valid")
		}
	}
}

func benchmarkValidateFailure(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if Valid("2018-01-01T23:42:59 oops!", Flexible) {
			b.Fatal("valid but should not")
		}
	}
}

func BenchmarkTimeParse(b *testing.B) {
	b.Run("success", benchmarkTimeParseSuccess)
	b.Run("failure", benchmarkTimeParseFailure)
}

func benchmarkTimeParseSuccess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := time.Parse(time.RFC3339Nano, "2018-01-01T23:42:59.123456789Z"); err != nil {
			b.Fatal("not valid")
		}
	}
}

func benchmarkTimeParseFailure(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := time.Parse(time.RFC3339Nano, "2018-01-01T23:42:59 oops!"); err == nil {
			b.Fatal("valid but should not")
		}
	}
}

// =============================================================================
// This code is extracted from a library we had that we are replacing with this
// package, we use it to verify that the behavior matches.
// =============================================================================
var validDates = [...]string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999-0700",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func isIsoString(str string) bool {
	// Per RFC3339Nano Spec a date should never be more than 35 chars.
	if len(str) > 36 {
		return false
	}

	for _, format := range validDates {
		if _, err := time.Parse(format, str); err == nil {
			return true
		}
	}

	return false
}
