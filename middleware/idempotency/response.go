package idempotency

// response is a struct that represents the response of a request.
// generation tool `go install github.com/tinylib/msgp@latest`
//
// Idempotency payloads are stored in backing storage, so keep headers/bodies bounded.
//
//go:generate msgp -o=response_msgp.go -tests=true -unexported
//msgp:limit arrays:16384 maps:2048 marshal:true
type response struct {
	Headers map[string][]string `msg:"hs,limit=1024"` // HTTP header count norms are well below this.

	Body       []byte `msg:"b"` // Idempotency bodies are bounded by storage policy, not msgp limits.
	StatusCode int    `msg:"sc"`
}
