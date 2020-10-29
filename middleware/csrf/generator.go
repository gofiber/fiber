package csrf

import (
	"crypto/rand"
	"encoding/base64"
)

type Generator interface {
	Generate() string
}

type cryptoRandomGenerator struct {
	length int32
}

func (g cryptoRandomGenerator) Generate() string {
	value := make([]byte, 32)
	rand.Read(value)
	return base64.RawURLEncoding.EncodeToString(value)
}
