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

	// ErrRedirectDowngrade is returned when a redirect leads from an HTTPS
	// origin to a plaintext HTTP target. Following one would leak any
	// credentials, cookies, or session tokens that the original HTTPS
	// handshake protected.
	//
	// It comes only from the load-balancing transport, which runs Fiber's own
	// redirect loop. A client built on a *fasthttp.Client or *fasthttp.HostClient
	// — what New returns — delegates DoRedirects to fasthttp, which follows a
	// downgrade the same way net/http does: the hop is taken, and credentials
	// survive it because the host did not change. Set MaxRedirects to 0 and
	// inspect the 3xx yourself if a downgrade must be refused on those
	// transports.
	ErrRedirectDowngrade = errors.New("client: HTTPS to HTTP redirect blocked")
)
