package fiber

import (
	"encoding/json"
	"testing"

	"github.com/gofiber/schema"
	"github.com/stretchr/testify/require"
)

// Test_ErrorAliases pins that each exported error type is an alias of the
// gofiber/schema or encoding/json type it re-exports, so errors.As finds the
// original type through the alias.
func Test_ErrorAliases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err    error
		target any // the pointer errors.As is given: it names the original type
		name   string
	}{
		{name: "ConversionError", err: ConversionError{}, target: &schema.ConversionError{}},
		{name: "UnknownKeyError", err: UnknownKeyError{}, target: &schema.UnknownKeyError{}},
		{name: "EmptyFieldError", err: EmptyFieldError{}, target: &schema.EmptyFieldError{}},
		{name: "MultiError", err: MultiError{}, target: &schema.MultiError{}},
		{name: "InvalidUnmarshalError", err: &InvalidUnmarshalError{}, target: new(*json.InvalidUnmarshalError)},
		{name: "MarshalerError", err: &MarshalerError{}, target: new(*json.MarshalerError)},
		{name: "SyntaxError", err: &SyntaxError{}, target: new(*json.SyntaxError)},
		{name: "UnmarshalTypeError", err: &UnmarshalTypeError{}, target: new(*json.UnmarshalTypeError)},
		{name: "UnsupportedTypeError", err: &UnsupportedTypeError{}, target: new(*json.UnsupportedTypeError)},
		{name: "UnsupportedValueError", err: &UnsupportedValueError{}, target: new(*json.UnsupportedValueError)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorAs(t, tc.err, tc.target)
		})
	}
}
