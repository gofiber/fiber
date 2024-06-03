package idempotency

// response is a struct that represents the response of a request.
// generation tool `go install github.com/tinylib/msgp@latest`
//
//go:generate msgp -o=response_msgp.go -io=false -unexported
type response struct {
	StatusCode int `msg:"sc"`

	Headers map[string][]string `msg:"hs"`

	Body []byte `msg:"b"`
}
