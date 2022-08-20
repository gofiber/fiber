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
	require.Equal(t, true, ok)
}

func TestUnknownKeyError(t *testing.T) {
	ok := errors.As(UnknownKeyError{}, &schema.UnknownKeyError{})
	require.Equal(t, true, ok)
}

func TestEmptyFieldError(t *testing.T) {
	ok := errors.As(EmptyFieldError{}, &schema.EmptyFieldError{})
	require.Equal(t, true, ok)
}

func TestMultiError(t *testing.T) {
	ok := errors.As(MultiError{}, &schema.MultiError{})
	require.Equal(t, true, ok)
}

func TestInvalidUnmarshalError(t *testing.T) {
	var e *jerrors.InvalidUnmarshalError
	ok := errors.As(&InvalidUnmarshalError{}, &e)
	require.Equal(t, true, ok)
}

func TestMarshalerError(t *testing.T) {
	var e *jerrors.MarshalerError
	ok := errors.As(&MarshalerError{}, &e)
	require.Equal(t, true, ok)
}

func TestSyntaxError(t *testing.T) {
	var e *jerrors.SyntaxError
	ok := errors.As(&SyntaxError{}, &e)
	require.Equal(t, true, ok)
}

func TestUnmarshalTypeError(t *testing.T) {
	var e *jerrors.UnmarshalTypeError
	ok := errors.As(&UnmarshalTypeError{}, &e)
	require.Equal(t, true, ok)
}

func TestUnsupportedTypeError(t *testing.T) {
	var e *jerrors.UnsupportedTypeError
	ok := errors.As(&UnsupportedTypeError{}, &e)
	require.Equal(t, true, ok)
}

func TestUnsupportedValeError(t *testing.T) {
	var e *jerrors.UnsupportedValueError
	ok := errors.As(&UnsupportedValueError{}, &e)
	require.Equal(t, true, ok)
}
