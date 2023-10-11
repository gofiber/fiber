package csrf

import (
	"time"
)

type Token struct {
	Key        string    `json:"key"`
	Raw        []byte    `json:"raw"`
	Expiration time.Time `json:"expiration"`
}
