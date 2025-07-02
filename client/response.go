package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Response represents the result of a request. It provides access to the response data.
type Response struct {
	client  *Client
	request *Request

	RawResponse *fasthttp.Response
	cookie      []*fasthttp.Cookie
}

// setClient sets the client instance in the response. The client object is used by core functionalities.
func (r *Response) setClient(c *Client) {
	r.client = c
}

// setRequest sets the request object in the response. The request is released when Response.Close is called.
func (r *Response) setRequest(req *Request) {
	r.request = req
}

// Status returns the HTTP status message of the executed request.
func (r *Response) Status() string {
	return string(r.RawResponse.Header.StatusMessage())
}

// StatusCode returns the HTTP status code of the executed request.
func (r *Response) StatusCode() int {
	return r.RawResponse.StatusCode()
}

// Protocol returns the HTTP protocol used for the request.
func (r *Response) Protocol() string {
	return string(r.RawResponse.Header.Protocol())
}

// Header returns the value of the specified response header field.
func (r *Response) Header(key string) string {
	return utils.UnsafeString(r.RawResponse.Header.Peek(key))
}

// Headers returns all headers in the response using an iterator.
// Use maps.Collect() to gather them into a map if needed.
//
// The returned values are valid only until the response object is released.
// Do not store references to returned values; make copies instead.
func (r *Response) Headers() iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		vals := r.RawResponse.Header.Len()
		p := pair{
			k: make([]string, 0, vals),
			v: make([]string, 0, vals),
		}
		for k, v := range r.RawResponse.Header.All() {
			p.k = append(p.k, utils.UnsafeString(k))
			p.v = append(p.v, utils.UnsafeString(v))
		}
		sort.Sort(&p)

		j := 0
		for i := 0; i < vals; i++ {
			if i == vals-1 || p.k[i] != p.k[i+1] {
				if !yield(p.k[i], p.v[j:i+1]) {
					break
				}
				j = i
			}
		}
	}
}

// Cookies returns all cookies set by the response.
//
// The returned values are valid only until the response object is released.
// Do not store references to returned values; make copies instead.
func (r *Response) Cookies() []*fasthttp.Cookie {
	return r.cookie
}

// Body returns the HTTP response body as a byte slice.
func (r *Response) Body() []byte {
	return r.RawResponse.Body()
}

// String returns the response body as a trimmed string.
func (r *Response) String() string {
	return utils.Trim(string(r.Body()), ' ')
}

// JSON unmarshals the response body into the given interface{} using JSON.
func (r *Response) JSON(v any) error {
	return r.client.jsonUnmarshal(r.Body(), v)
}

// CBOR unmarshals the response body into the given interface{} using CBOR.
func (r *Response) CBOR(v any) error {
	return r.client.cborUnmarshal(r.Body(), v)
}

// XML unmarshals the response body into the given interface{} using XML.
func (r *Response) XML(v any) error {
	return r.client.xmlUnmarshal(r.Body(), v)
}

// Save writes the response body to a file or io.Writer.
// If a string path is provided, it creates directories if needed, then writes to a file.
// If an io.Writer is provided, it writes directly to it.
func (r *Response) Save(v any) error {
	switch p := v.(type) {
	case string:
		file := filepath.Clean(p)
		dir := filepath.Dir(file)

		// Create directory if it doesn't exist
		if _, err := os.Stat(dir); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("failed to check directory: %w", err)
			}

			if err = os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		// Create and write to file
		outFile, err := os.Create(file)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer func() { _ = outFile.Close() }() //nolint:errcheck // not needed

		if _, err = io.Copy(outFile, bytes.NewReader(r.Body())); err != nil {
			return fmt.Errorf("failed to write response body to file: %w", err)
		}

		return nil

	case io.Writer:
		if _, err := io.Copy(p, bytes.NewReader(r.Body())); err != nil {
			return fmt.Errorf("failed to write response body to io.Writer: %w", err)
		}
		defer func() {
			if pc, ok := p.(io.WriteCloser); ok {
				_ = pc.Close() //nolint:errcheck // not needed
			}
		}()
		return nil

	default:
		return ErrNotSupportSaveMethod
	}
}

// Reset clears the Response object, making it ready for reuse.
func (r *Response) Reset() {
	r.client = nil
	r.request = nil

	for len(r.cookie) != 0 {
		t := r.cookie[0]
		r.cookie = r.cookie[1:]
		fasthttp.ReleaseCookie(t)
	}

	r.RawResponse.Reset()
}

// Close releases both the Request and Response objects back to their pools.
// After calling Close, do not use these objects.
func (r *Response) Close() {
	if r.request != nil {
		tmp := r.request
		r.request = nil
		ReleaseRequest(tmp)
	}
	ReleaseResponse(r)
}

var responsePool = &sync.Pool{
	New: func() any {
		return &Response{
			cookie:      []*fasthttp.Cookie{},
			RawResponse: fasthttp.AcquireResponse(),
		}
	},
}

// AcquireResponse returns a new (pooled) Response object.
// When done, release it with ReleaseResponse to reduce GC load.
func AcquireResponse() *Response {
	resp, ok := responsePool.Get().(*Response)
	if !ok {
		panic("unexpected type from responsePool.Get()")
	}
	return resp
}

// ReleaseResponse returns the Response object to the pool.
// Do not use the released Response afterward to avoid data races.
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}
