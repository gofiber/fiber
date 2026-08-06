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

	// ErrRedirectDowngrade is returned when a redirect leads from an HTTPS origin
	// to plaintext HTTP. Only the load-balancing transport raises it: the others
	// delegate to fasthttp, which takes the hop. Set MaxRedirects to 0 to refuse.
	ErrRedirectDowngrade = errors.New("client: HTTPS to HTTP redirect blocked")
)
