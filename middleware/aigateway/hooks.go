package aigateway

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

// RelayRequest is the mutable view of a request handed to Config.OnRequest
// before any policy check. The hook may rewrite Path and Body; both feed the
// allow-lists, the model sniff, and the relay, so a hook cannot bypass
// policy — its output is what gets policed.
type RelayRequest struct {
	// Path is the request path after PathPrefix stripping. Reassign it to
	// relay a different upstream path.
	Path string

	// Body is nil initially (read the current body via c.Body()). Assign a
	// new body to replace it; the replacement is relayed identity-encoded,
	// so any Content-Encoding header is dropped with it.
	Body []byte
}

// RelayResponse is the mutable view of a buffered upstream response handed to
// Config.OnResponse before it is sent to the client. Streaming responses are
// relayed pass-through and never produce one.
type RelayResponse struct {
	// Body is the full upstream response body, still content-encoded when
	// the upstream compressed it.
	Body []byte

	// Status is the response status relayed to the client. It starts as the
	// upstream's status; UsageEvent.StatusCode keeps reporting the upstream
	// value even when a hook changes this.
	Status int
}

// hookStatus maps an OnRequest hook error to a response status: a
// *fiber.Error chooses its own code, anything else is a 403 policy veto.
func hookStatus(err error) int {
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return fe.Code
	}
	return fiber.StatusForbidden
}
