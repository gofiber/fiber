package fiber

import (
	"errors"
)

var (
	errBindPoolTypeAssertion  = errors.New("failed to type-assert to *Bind")
	errCustomCtxTypeAssertion = errors.New("failed to type-assert to CustomCtx")
	errTLSConfigTypeAssertion = errors.New("failed to type-assert to *tls.Config")
	errInvalidEscapeSequence  = errors.New("invalid escape sequence")
	errTCPAddrTypeAssertion   = errors.New("failed to type-assert to *net.TCPAddr")
	errRedirectTypeAssertion  = errors.New("failed to type-assert to *Redirect")
)
