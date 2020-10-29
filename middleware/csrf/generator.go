package csrf

import (
	"crypto/rand"
	"encoding/base64"
)

type Generator interface {
	Generate() string
}

type cryptoRandomGenerator struct {
	length int
}

func (g cryptoRandomGenerator) Generate() string {
	value := make([]byte, g.length)
	n, err := rand.Read(value)

	if err != nil || n != g.length {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(value)
}
