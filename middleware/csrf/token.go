package csrf

import (
	"time"
)

// Token represents a CSRF token with expiration metadata.
// This is used internally for token storage and validation.
type Token struct {
	Expiration time.Time `json:"expiration"`
	Key        string    `json:"key"`
	Raw        []byte    `json:"raw"`
}
