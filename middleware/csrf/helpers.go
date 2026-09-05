package csrf

import (
	"crypto/subtle"

	"github.com/gofiber/utils/v2"
)

const schemeHTTPS = "https"

func compareTokens(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

func compareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare(utils.UnsafeBytes(a), utils.UnsafeBytes(b)) == 1
}
