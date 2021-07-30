package fiber

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2/internal/schema"
	"github.com/gofiber/fiber/v2/utils"
)

func TestConversionError(t *testing.T) {
	ok := errors.As(ConversionError{}, &schema.ConversionError{})
	utils.AssertEqual(t, true, ok)
}

func TestUnknownKeyError(t *testing.T) {
	ok := errors.As(UnknownKeyError{}, &schema.UnknownKeyError{})
	utils.AssertEqual(t, true, ok)
}

func TestEmptyFieldError(t *testing.T) {
	ok := errors.As(EmptyFieldError{}, &schema.EmptyFieldError{})
	utils.AssertEqual(t, true, ok)
}

func TestMultiError(t *testing.T) {
	ok := errors.As(MultiError{}, &schema.MultiError{})
	utils.AssertEqual(t, true, ok)
}
