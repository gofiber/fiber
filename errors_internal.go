package fiber

import (
	"errors"
)

var (
	errBindPoolTypeAssertion   = errors.New("failed to type-assert to *Bind")
	errCustomCtxTypeAssertion  = errors.New("failed to type-assert to CustomCtx")
	errDefaultCtxTypeAssertion = errors.New("failed to type-assert to *DefaultCtx")
	errInvalidEscapeSequence   = errors.New("invalid escape sequence")
	errRedirectTypeAssertion   = errors.New("failed to type-assert to *Redirect")
)
