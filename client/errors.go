package client

import (
	"errors"
)

var (
	errResponseChanTypeAssertion = errors.New("failed to type-assert to *Response")
	errErrorChanTypeAssertion    = errors.New("failed to type-assert to chan error")
	errRequestTypeAssertion      = errors.New("failed to type-assert to *Request")
	errFileTypeAssertion         = errors.New("failed to type-assert to *File")
	errCookieJarTypeAssertion    = errors.New("failed to type-assert to *CookieJar")
	errSyncPoolBuffer            = errors.New("failed to retrieve buffer from a sync.Pool")
)
