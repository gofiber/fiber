package fiber

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3/internal/schema"
	"github.com/stretchr/testify/require"
)

func TestConversionError(t *testing.T) {
	t.Parallel()
	ok := errors.As(ConversionError{}, &schema.ConversionError{})
	require.True(t, ok)
}

func TestUnknownKeyError(t *testing.T) {
	t.Parallel()
	ok := errors.As(UnknownKeyError{}, &schema.UnknownKeyError{})
	require.True(t, ok)
}

func TestEmptyFieldError(t *testing.T) {
	t.Parallel()
	ok := errors.As(EmptyFieldError{}, &schema.EmptyFieldError{})
	require.True(t, ok)
}

func TestMultiError(t *testing.T) {
	t.Parallel()
	ok := errors.As(MultiError{}, &schema.MultiError{})
	require.True(t, ok)
}

func TestInvalidUnmarshalError(t *testing.T) {
	t.Parallel()
	var e *json.InvalidUnmarshalError
	ok := errors.As(&InvalidUnmarshalError{}, &e)
	require.True(t, ok)
}

func TestMarshalerError(t *testing.T) {
	t.Parallel()
	var e *json.MarshalerError
	ok := errors.As(&MarshalerError{}, &e)
	require.True(t, ok)
}

func TestSyntaxError(t *testing.T) {
	t.Parallel()
	var e *json.SyntaxError
	ok := errors.As(&SyntaxError{}, &e)
	require.True(t, ok)
}

func TestUnmarshalTypeError(t *testing.T) {
	t.Parallel()
	var e *json.UnmarshalTypeError
	ok := errors.As(&UnmarshalTypeError{}, &e)
	require.True(t, ok)
}

func TestUnsupportedTypeError(t *testing.T) {
	t.Parallel()
	var e *json.UnsupportedTypeError
	ok := errors.As(&UnsupportedTypeError{}, &e)
	require.True(t, ok)
}

func TestUnsupportedValeError(t *testing.T) {
	t.Parallel()
	var e *json.UnsupportedValueError
	ok := errors.As(&UnsupportedValueError{}, &e)
	require.True(t, ok)
}
