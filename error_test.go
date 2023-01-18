package fiber

import (
	"errors"
	"testing"

	jerrors "encoding/json"

	"github.com/gofiber/fiber/v2/internal/schema"
	"github.com/gofiber/fiber/v2/utils"
)

func TestConversionError(t *testing.T) {
	t.Parallel()
	ok := errors.As(ConversionError{}, &schema.ConversionError{})
	utils.AssertEqual(t, true, ok)
}

func TestUnknownKeyError(t *testing.T) {
	t.Parallel()
	ok := errors.As(UnknownKeyError{}, &schema.UnknownKeyError{})
	utils.AssertEqual(t, true, ok)
}

func TestEmptyFieldError(t *testing.T) {
	t.Parallel()
	ok := errors.As(EmptyFieldError{}, &schema.EmptyFieldError{})
	utils.AssertEqual(t, true, ok)
}

func TestMultiError(t *testing.T) {
	t.Parallel()
	ok := errors.As(MultiError{}, &schema.MultiError{})
	utils.AssertEqual(t, true, ok)
}

func TestInvalidUnmarshalError(t *testing.T) {
	t.Parallel()
	var e *jerrors.InvalidUnmarshalError
	ok := errors.As(&InvalidUnmarshalError{}, &e)
	utils.AssertEqual(t, true, ok)
}

func TestMarshalerError(t *testing.T) {
	t.Parallel()
	var e *jerrors.MarshalerError
	ok := errors.As(&MarshalerError{}, &e)
	utils.AssertEqual(t, true, ok)
}

func TestSyntaxError(t *testing.T) {
	t.Parallel()
	var e *jerrors.SyntaxError
	ok := errors.As(&SyntaxError{}, &e)
	utils.AssertEqual(t, true, ok)
}

func TestUnmarshalTypeError(t *testing.T) {
	t.Parallel()
	var e *jerrors.UnmarshalTypeError
	ok := errors.As(&UnmarshalTypeError{}, &e)
	utils.AssertEqual(t, true, ok)
}

func TestUnsupportedTypeError(t *testing.T) {
	t.Parallel()
	var e *jerrors.UnsupportedTypeError
	ok := errors.As(&UnsupportedTypeError{}, &e)
	utils.AssertEqual(t, true, ok)
}

func TestUnsupportedValeError(t *testing.T) {
	t.Parallel()
	var e *jerrors.UnsupportedValueError
	ok := errors.As(&UnsupportedValueError{}, &e)
	utils.AssertEqual(t, true, ok)
}
