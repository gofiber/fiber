package client

import (
	"errors"
)

var (
	errResponseChanTypeAssertion = errors.New("failed to type-assert to *Response")
	errChanErrorTypeAssertion    = errors.New("failed to type-assert to chan error")
	errRequestTypeAssertion      = errors.New("failed to type-assert to *Request")
	errFileTypeAssertion         = errors.New("failed to type-assert to *File")
	errCookieJarTypeAssertion    = errors.New("failed to type-assert to *CookieJar")
	errSyncPoolBuffer            = errors.New("failed to retrieve buffer from a sync.Pool")

	// ErrRedirectDowngrade is returned when DoRedirects encounters a
	// redirect from an HTTPS origin to a plaintext HTTP target.
	// Following such a redirect would leak any credentials, cookies, or
	// session tokens that the original HTTPS handshake protected.
	ErrRedirectDowngrade = errors.New("client: HTTPS to HTTP redirect blocked")
)
