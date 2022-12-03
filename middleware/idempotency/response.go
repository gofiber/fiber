package idempotency

type Response struct {
	StatusCode int

	Headers map[string]string

	Body []byte
}
