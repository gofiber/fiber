package fiber

import (
	"errors"
	"testing"

	jerrors "encoding/json"

	"github.com/gofiber/fiber/v3/internal/schema"
	"github.com/stretchr/testify/require"
)

func TestConversionError(t *testing.T) {
	ok := errors.As(ConversionError{}, &schema.ConversionError{})
	require.True(t, ok)
}

func TestUnknownKeyError(t *testing.T) {
	ok := errors.As(UnknownKeyError{}, &schema.UnknownKeyError{})
	require.True(t, ok)
}

func TestEmptyFieldError(t *testing.T) {
	ok := errors.As(EmptyFieldError{}, &schema.EmptyFieldError{})
	require.True(t, ok)
}

func TestMultiError(t *testing.T) {
	ok := errors.As(MultiError{}, &schema.MultiError{})
	require.True(t, ok)
}

func TestInvalidUnmarshalError(t *testing.T) {
	var e *jerrors.InvalidUnmarshalError
	ok := errors.As(&InvalidUnmarshalError{}, &e)
	require.True(t, ok)
}

func TestMarshalerError(t *testing.T) {
	var e *jerrors.MarshalerError
	ok := errors.As(&MarshalerError{}, &e)
	require.True(t, ok)
}

func TestSyntaxError(t *testing.T) {
	var e *jerrors.SyntaxError
	ok := errors.As(&SyntaxError{}, &e)
	require.True(t, ok)
}

func TestUnmarshalTypeError(t *testing.T) {
	var e *jerrors.UnmarshalTypeError
	ok := errors.As(&UnmarshalTypeError{}, &e)
	require.True(t, ok)
}

func TestUnsupportedTypeError(t *testing.T) {
	var e *jerrors.UnsupportedTypeError
	ok := errors.As(&UnsupportedTypeError{}, &e)
	require.True(t, ok)
}

func TestUnsupportedValeError(t *testing.T) {
	var e *jerrors.UnsupportedValueError
	ok := errors.As(&UnsupportedValueError{}, &e)
	require.True(t, ok)
}
