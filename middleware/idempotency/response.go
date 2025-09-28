package idempotency

import "sync"

// response is a struct that represents the response of a request.
// generation tool `go install github.com/tinylib/msgp@latest`
//
//go:generate msgp -o=response_msgp.go -tests=true -unexported
type response struct {
	Headers map[string][]string `msg:"hs"`

	Body       []byte `msg:"b"`
	StatusCode int    `msg:"sc"`
}

const (
	cachedResponseBodyDefaultCap = 4 << 10   // 4 KiB default body buffer
	cachedResponseBodyMaxCap     = 256 << 10 // 256 KiB maximum retained buffer
	cachedResponseHeaderHint     = 8
)

var cachedResponsePool = sync.Pool{
	New: func() any {
		return &response{
			Headers: make(map[string][]string, cachedResponseHeaderHint),
			Body:    make([]byte, 0, cachedResponseBodyDefaultCap),
		}
	},
}

func acquireCachedResponse() *response {
	res, ok := cachedResponsePool.Get().(*response)
	if !ok {
		panic("failed to type-assert to *response")
	}
	if res.Headers == nil {
		res.Headers = make(map[string][]string, cachedResponseHeaderHint)
	}
	if res.Body == nil {
		res.Body = make([]byte, 0, cachedResponseBodyDefaultCap)
	}
	return res
}

func releaseCachedResponse(res *response) {
	if res == nil {
		return
	}
	res.StatusCode = 0
	if res.Body != nil {
		res.Body = resetCachedResponseBody(res.Body)
	}
	if res.Headers != nil {
		resetCachedResponseHeaders(res.Headers)
	}
	cachedResponsePool.Put(res)
}

func resetCachedResponseHeaders(headers map[string][]string) {
	if headers == nil {
		return
	}
	for key, values := range headers {
		for i := range values {
			values[i] = ""
		}
		delete(headers, key)
	}
}

func resetCachedResponseBody(body []byte) []byte {
	if cap(body) > cachedResponseBodyMaxCap {
		return make([]byte, 0, cachedResponseBodyDefaultCap)
	}
	return body[:0]
}
