package fiber

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/gofiber/schema"
	"github.com/stretchr/testify/require"
)

func Test_ConversionError(t *testing.T) {
	t.Parallel()
	ok := errors.As(ConversionError{}, &schema.ConversionError{})
	require.True(t, ok)
}

func Test_UnknownKeyError(t *testing.T) {
	t.Parallel()
	ok := errors.As(UnknownKeyError{}, &schema.UnknownKeyError{})
	require.True(t, ok)
}

func Test_EmptyFieldError(t *testing.T) {
	t.Parallel()
	ok := errors.As(EmptyFieldError{}, &schema.EmptyFieldError{})
	require.True(t, ok)
}

func Test_MultiError(t *testing.T) {
	t.Parallel()
	ok := errors.As(MultiError{}, &schema.MultiError{})
	require.True(t, ok)
}

func Test_InvalidUnmarshalError(t *testing.T) {
	t.Parallel()
	var e *json.InvalidUnmarshalError
	ok := errors.As(&InvalidUnmarshalError{}, &e)
	require.True(t, ok)
}

func Test_MarshalerError(t *testing.T) {
	t.Parallel()
	var e *json.MarshalerError
	ok := errors.As(&MarshalerError{}, &e)
	require.True(t, ok)
}

func Test_SyntaxError(t *testing.T) {
	t.Parallel()
	var e *json.SyntaxError
	ok := errors.As(&SyntaxError{}, &e)
	require.True(t, ok)
}

func Test_UnmarshalTypeError(t *testing.T) {
	t.Parallel()
	var e *json.UnmarshalTypeError
	ok := errors.As(&UnmarshalTypeError{}, &e)
	require.True(t, ok)
}

func Test_UnsupportedTypeError(t *testing.T) {
	t.Parallel()
	var e *json.UnsupportedTypeError
	ok := errors.As(&UnsupportedTypeError{}, &e)
	require.True(t, ok)
}

func Test_UnsupportedValeError(t *testing.T) {
	t.Parallel()
	var e *json.UnsupportedValueError
	ok := errors.As(&UnsupportedValueError{}, &e)
	require.True(t, ok)
}
