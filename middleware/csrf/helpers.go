package csrf

import (
	"crypto/subtle"
)

func compareTokens(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
