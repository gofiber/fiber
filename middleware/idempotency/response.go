package idempotency

//go:generate msgp -o=response_msgp.go -unexported
type response struct {
	StatusCode int `msg:"sc"`

	Headers map[string]string `msg:"hs"`

	Body []byte `msg:"b"`
}
