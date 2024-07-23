package csrf

import (
	"time"
)

type Token struct {
	Expiration time.Time `json:"expiration"`
	Key        string    `json:"key"`
	Raw        []byte    `json:"raw"`
}
