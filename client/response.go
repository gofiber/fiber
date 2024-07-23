package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Response is the result of a request. This object is used to access the response data.
type Response struct {
	client  *Client
	request *Request

	RawResponse *fasthttp.Response
	cookie      []*fasthttp.Cookie
}

// setClient method sets client object in response instance.
// Use core object in the client.
func (r *Response) setClient(c *Client) {
	r.client = c
}

// setRequest method sets Request object in response instance.
// The request will be released when the Response.Close is called.
func (r *Response) setRequest(req *Request) {
	r.request = req
}

// Status method returns the HTTP status string for the executed request.
func (r *Response) Status() string {
	return string(r.RawResponse.Header.StatusMessage())
}

// StatusCode method returns the HTTP status code for the executed request.
func (r *Response) StatusCode() int {
	return r.RawResponse.StatusCode()
}

// Protocol method returns the HTTP response protocol used for the request.
func (r *Response) Protocol() string {
	return string(r.RawResponse.Header.Protocol())
}

// Header method returns the response headers.
func (r *Response) Header(key string) string {
	return utils.UnsafeString(r.RawResponse.Header.Peek(key))
}

// Cookies method to access all the response cookies.
func (r *Response) Cookies() []*fasthttp.Cookie {
	return r.cookie
}

// Body method returns HTTP response as []byte array for the executed request.
func (r *Response) Body() []byte {
	return r.RawResponse.Body()
}

// String method returns the body of the server response as String.
func (r *Response) String() string {
	return strings.TrimSpace(string(r.Body()))
}

// JSON method will unmarshal body to json.
func (r *Response) JSON(v any) error {
	return r.client.jsonUnmarshal(r.Body(), v)
}

// XML method will unmarshal body to xml.
func (r *Response) XML(v any) error {
	return r.client.xmlUnmarshal(r.Body(), v)
}

// Save method will save the body to a file or io.Writer.
func (r *Response) Save(v any) error {
	switch p := v.(type) {
	case string:
		file := filepath.Clean(p)
		dir := filepath.Dir(file)

		// create directory
		if _, err := os.Stat(dir); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("failed to check directory: %w", err)
			}

			if err = os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		// create file
		outFile, err := os.Create(file)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer func() { _ = outFile.Close() }() //nolint:errcheck // not needed

		_, err = io.Copy(outFile, bytes.NewReader(r.Body()))
		if err != nil {
			return fmt.Errorf("failed to write response body to file: %w", err)
		}

		return nil
	case io.Writer:
		_, err := io.Copy(p, bytes.NewReader(r.Body()))
		if err != nil {
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

// Reset clears the Response object.
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

// Close method will release Request object and Response object,
// after call Close please don't use these object.
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

// AcquireResponse returns an empty response object from the pool.
//
// The returned response may be returned to the pool with ReleaseResponse when no longer needed.
// This allows reducing GC load.
func AcquireResponse() *Response {
	resp, ok := responsePool.Get().(*Response)
	if !ok {
		panic("unexpected type from responsePool.Get()")
	}
	return resp
}

// ReleaseResponse returns the object acquired via AcquireResponse to the pool.
//
// Do not access the released Response object, otherwise data races may occur.
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}
