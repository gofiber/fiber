package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// WithStruct is implemented by types that allow data to be stored from a struct via reflection.
type WithStruct interface {
	Add(name, obj string)
	Del(name string)
}

// bodyType defines the type of request body.
type bodyType int

// Enumeration of request body types.
const (
	noBody bodyType = iota
	jsonBody
	xmlBody
	formBody
	filesBody
	rawBody
	cborBody
)

var ErrClientNil = errors.New("client cannot be nil")

// Request contains all data related to an HTTP request.
type Request struct {
	ctx context.Context //nolint:containedctx // Context is needed to be stored in the request.

	body    any
	header  *Header
	params  *QueryParam
	cookies *Cookie
	path    *PathParam

	client *Client

	formData *FormData

	RawRequest *fasthttp.Request
	url        string
	method     string
	userAgent  string
	boundary   string
	referer    string
	files      []*File

	timeout      time.Duration
	maxRedirects int

	bodyType bodyType
}

// Method returns the HTTP method set in the Request.
func (r *Request) Method() string {
	return r.method
}

// SetMethod sets the HTTP method for the Request.
// It is recommended to use the specialized methods (e.g., Get, Post) instead.
func (r *Request) SetMethod(method string) *Request {
	r.method = method
	return r
}

// URL returns the URL set in the Request.
func (r *Request) URL() string {
	return r.url
}

// SetURL sets the URL for the Request.
func (r *Request) SetURL(url string) *Request {
	r.url = url
	return r
}

// Client returns the Client instance associated with this Request.
func (r *Request) Client() *Client {
	return r.client
}

// SetClient sets the Client instance for the Request.
func (r *Request) SetClient(c *Client) *Request {
	if c == nil {
		panic(ErrClientNil)
	}

	r.client = c
	return r
}

// Context returns the context associated with the Request.
// If not set, a background context is returned.
func (r *Request) Context() context.Context {
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

// SetContext sets the context for the Request, allowing request cancellation if ctx is done.
// See https://blog.golang.org/context article and the "context" package documentation.
func (r *Request) SetContext(ctx context.Context) *Request {
	r.ctx = ctx
	return r
}

// Header returns all values associated with the given header key.
func (r *Request) Header(key string) []string {
	return r.header.PeekMultiple(key)
}

type pair struct {
	k []string
	v []string
}

func (p *pair) Len() int {
	return len(p.k)
}

func (p *pair) Swap(i, j int) {
	p.k[i], p.k[j] = p.k[j], p.k[i]
	p.v[i], p.v[j] = p.v[j], p.v[i]
}

func (p *pair) Less(i, j int) bool {
	return p.k[i] < p.k[j]
}

// Headers returns an iterator over all headers in the Request.
// Use maps.Collect() to gather them into a map if needed.
//
// The returned values are only valid until the request object is released.
// Do not store references to returned values; make copies instead.
func (r *Request) Headers() iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		peekKeys := r.header.PeekKeys()
		keys := make([][]byte, len(peekKeys))
		copy(keys, peekKeys) // It is necessary to have immutable byte slice.

		for _, key := range keys {
			vals := r.header.PeekAll(utils.UnsafeString(key))
			valsStr := make([]string, len(vals))
			for i, v := range vals {
				valsStr[i] = utils.UnsafeString(v)
			}

			if !yield(utils.UnsafeString(key), valsStr) {
				return
			}
		}
	}
}

// AddHeader adds a single header field and value to the Request.
func (r *Request) AddHeader(key, val string) *Request {
	r.header.Add(key, val)
	return r
}

// SetHeader sets a single header field and value in the Request, overriding any previously set value.
func (r *Request) SetHeader(key, val string) *Request {
	r.header.Del(key)
	r.header.Set(key, val)
	return r
}

// AddHeaders adds multiple header fields and values at once.
func (r *Request) AddHeaders(h map[string][]string) *Request {
	r.header.AddHeaders(h)
	return r
}

// SetHeaders sets multiple header fields and values at once, overriding previously set values.
func (r *Request) SetHeaders(h map[string]string) *Request {
	r.header.SetHeaders(h)
	return r
}

// Param returns all values associated with the given query parameter.
func (r *Request) Param(key string) []string {
	var res []string
	tmp := r.params.PeekMulti(key)
	for _, v := range tmp {
		res = append(res, utils.UnsafeString(v))
	}
	return res
}

// Params returns an iterator over all query parameters in the Request.
// Use maps.Collect() to gather them into a map if needed.
//
// The returned values are only valid until the request object is released.
// Do not store references to returned values; make copies instead.
func (r *Request) Params() iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		vals := r.params.Len()
		p := pair{
			k: make([]string, 0, vals),
			v: make([]string, 0, vals),
		}
		for k, v := range r.params.All() {
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
				j = i + 1
			}
		}
	}
}

// AddParam adds a single query parameter and value to the Request.
func (r *Request) AddParam(key, val string) *Request {
	r.params.Add(key, val)
	return r
}

// SetParam sets a single query parameter and value in the Request, overriding any previously set value.
func (r *Request) SetParam(key, val string) *Request {
	r.params.Set(key, val)
	return r
}

// AddParams adds multiple query parameters and their values at once.
func (r *Request) AddParams(m map[string][]string) *Request {
	r.params.AddParams(m)
	return r
}

// SetParams sets multiple query parameters and their values at once, overriding previously set values.
func (r *Request) SetParams(m map[string]string) *Request {
	r.params.SetParams(m)
	return r
}

// SetParamsWithStruct sets multiple query parameters from a struct, overriding previously set values.
func (r *Request) SetParamsWithStruct(v any) *Request {
	r.params.SetParamsWithStruct(v)
	return r
}

// DelParams deletes one or more query parameters.
func (r *Request) DelParams(key ...string) *Request {
	for _, v := range key {
		r.params.Del(v)
	}
	return r
}

// UserAgent returns the User-Agent header set in the Request.
func (r *Request) UserAgent() string {
	return r.userAgent
}

// SetUserAgent sets the User-Agent header, overriding any previously set value.
func (r *Request) SetUserAgent(ua string) *Request {
	r.userAgent = ua
	return r
}

// Boundary returns the multipart boundary used by the Request.
func (r *Request) Boundary() string {
	return r.boundary
}

// SetBoundary sets the multipart boundary.
func (r *Request) SetBoundary(b string) *Request {
	r.boundary = b
	return r
}

// Referer returns the Referer header set in the Request.
func (r *Request) Referer() string {
	return r.referer
}

// SetReferer sets the Referer header, overriding any previously set value.
func (r *Request) SetReferer(referer string) *Request {
	r.referer = referer
	return r
}

// Cookie returns the value of a named cookie.
// If the cookie does not exist, an empty string is returned.
func (r *Request) Cookie(key string) string {
	if val, ok := (*r.cookies)[key]; ok {
		return val
	}
	return ""
}

// Cookies returns an iterator over all cookies.
// Use maps.Collect() to gather them into a map if needed.
func (r *Request) Cookies() iter.Seq2[string, string] {
	return r.cookies.All()
}

// SetCookie sets a single cookie, overriding any previously set value.
func (r *Request) SetCookie(key, val string) *Request {
	r.cookies.SetCookie(key, val)
	return r
}

// SetCookies sets multiple cookies at once, overriding previously set values.
func (r *Request) SetCookies(m map[string]string) *Request {
	r.cookies.SetCookies(m)
	return r
}

// SetCookiesWithStruct sets multiple cookies from a struct, overriding previously set values.
func (r *Request) SetCookiesWithStruct(v any) *Request {
	r.cookies.SetCookiesWithStruct(v)
	return r
}

// DelCookies deletes one or more cookies.
func (r *Request) DelCookies(key ...string) *Request {
	r.cookies.DelCookies(key...)
	return r
}

// PathParam returns the value of a named path parameter.
// If the parameter does not exist, an empty string is returned.
func (r *Request) PathParam(key string) string {
	if val, ok := (*r.path)[key]; ok {
		return val
	}
	return ""
}

// PathParams returns an iterator over all path parameters.
// Use maps.Collect() to gather them into a map if needed.
func (r *Request) PathParams() iter.Seq2[string, string] {
	return r.path.All()
}

// SetPathParam sets a single path parameter and value, overriding any previously set value.
func (r *Request) SetPathParam(key, val string) *Request {
	r.path.SetParam(key, val)
	return r
}

// SetPathParams sets multiple path parameters and values at once, overriding previously set values.
func (r *Request) SetPathParams(m map[string]string) *Request {
	r.path.SetParams(m)
	return r
}

// SetPathParamsWithStruct sets multiple path parameters from a struct, overriding previously set values.
func (r *Request) SetPathParamsWithStruct(v any) *Request {
	r.path.SetParamsWithStruct(v)
	return r
}

// DelPathParams deletes one or more path parameters.
func (r *Request) DelPathParams(key ...string) *Request {
	r.path.DelParams(key...)
	return r
}

// ResetPathParams deletes all path parameters.
func (r *Request) ResetPathParams() *Request {
	r.path.Reset()
	return r
}

// SetJSON sets the request body to a JSON-encoded value.
func (r *Request) SetJSON(v any) *Request {
	r.body = v
	r.bodyType = jsonBody
	return r
}

// SetXML sets the request body to an XML-encoded value.
func (r *Request) SetXML(v any) *Request {
	r.body = v
	r.bodyType = xmlBody
	return r
}

// SetCBOR sets the request body to a CBOR-encoded value.
func (r *Request) SetCBOR(v any) *Request {
	r.body = v
	r.bodyType = cborBody
	return r
}

// SetRawBody sets the request body to raw bytes.
func (r *Request) SetRawBody(v []byte) *Request {
	r.body = v
	r.bodyType = rawBody
	return r
}

// resetBody clears the existing body. If the current body type is filesBody and
// the new type is formBody, the formBody setting is ignored to preserve files.
func (r *Request) resetBody(t bodyType) {
	r.body = nil

	// If bodyType is filesBody and we attempt to set formBody, ignore the change.
	if r.bodyType == filesBody && t == formBody {
		return
	}
	r.bodyType = t
}

// FormData returns all values associated with a form field.
func (r *Request) FormData(key string) []string {
	var res []string
	tmp := r.formData.PeekMulti(key)
	for _, v := range tmp {
		res = append(res, utils.UnsafeString(v))
	}
	return res
}

// AllFormData returns an iterator over all form fields.
// Use maps.Collect() to gather them into a map if needed.
//
// The returned values are only valid until the request object is released.
// Do not store references to returned values; make copies instead.
func (r *Request) AllFormData() iter.Seq2[string, []string] {
	return func(yield func(string, []string) bool) {
		vals := r.formData.Len()
		p := pair{
			k: make([]string, 0, vals),
			v: make([]string, 0, vals),
		}
		for k, v := range r.formData.All() {
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
				j = i + 1
			}
		}
	}
}

// AddFormData adds a single form field and value to the Request.
func (r *Request) AddFormData(key, val string) *Request {
	r.formData.Add(key, val)
	r.resetBody(formBody)
	return r
}

// SetFormData sets a single form field and value, overriding any previously set value.
func (r *Request) SetFormData(key, val string) *Request {
	r.formData.Set(key, val)
	r.resetBody(formBody)
	return r
}

// AddFormDataWithMap adds multiple form fields and values to the Request.
func (r *Request) AddFormDataWithMap(m map[string][]string) *Request {
	r.formData.AddWithMap(m)
	r.resetBody(formBody)
	return r
}

// SetFormDataWithMap sets multiple form fields and values at once, overriding previously set values.
func (r *Request) SetFormDataWithMap(m map[string]string) *Request {
	r.formData.SetWithMap(m)
	r.resetBody(formBody)
	return r
}

// SetFormDataWithStruct sets multiple form fields from a struct, overriding previously set values.
func (r *Request) SetFormDataWithStruct(v any) *Request {
	r.formData.SetWithStruct(v)
	r.resetBody(formBody)
	return r
}

// DelFormData deletes one or more form fields.
func (r *Request) DelFormData(key ...string) *Request {
	r.formData.DelData(key...)
	r.resetBody(formBody)
	return r
}

// File returns the file associated with the given name.
// If no name was provided during addition, it attempts to match by the file's base name.
func (r *Request) File(name string) *File {
	for _, v := range r.files {
		if v.name == "" {
			if filepath.Base(v.path) == name {
				return v
			}
		} else if v.name == name {
			return v
		}
	}
	return nil
}

// Files returns all files added to the Request.
//
// The returned values are only valid until the request object is released.
// Do not store references to returned values; make copies instead.
func (r *Request) Files() []*File {
	return r.files
}

// FileByPath returns the file associated with the given file path.
func (r *Request) FileByPath(path string) *File {
	for _, v := range r.files {
		if v.path == path {
			return v
		}
	}
	return nil
}

// AddFile adds a single file by its path.
func (r *Request) AddFile(path string) *Request {
	r.files = append(r.files, AcquireFile(SetFilePath(path)))
	r.resetBody(filesBody)
	return r
}

// AddFileWithReader adds a file using an io.ReadCloser.
func (r *Request) AddFileWithReader(name string, reader io.ReadCloser) *Request {
	r.files = append(r.files, AcquireFile(SetFileName(name), SetFileReader(reader)))
	r.resetBody(filesBody)
	return r
}

// AddFiles adds multiple files at once.
func (r *Request) AddFiles(files ...*File) *Request {
	r.files = append(r.files, files...)
	r.resetBody(filesBody)
	return r
}

// Timeout returns the timeout duration set in the Request.
func (r *Request) Timeout() time.Duration {
	return r.timeout
}

// SetTimeout sets the timeout for the Request, overriding any previously set value.
func (r *Request) SetTimeout(t time.Duration) *Request {
	r.timeout = t
	return r
}

// MaxRedirects returns the maximum number of redirects configured for the Request.
func (r *Request) MaxRedirects() int {
	return r.maxRedirects
}

// SetMaxRedirects sets the maximum number of redirects, overriding any previously set value.
func (r *Request) SetMaxRedirects(count int) *Request {
	r.maxRedirects = count
	return r
}

// checkClient ensures that a Client is set. If none is set, it defaults to the global defaultClient.
func (r *Request) checkClient() {
	if r.client == nil {
		r.SetClient(defaultClient)
	}
}

// Get sends a GET request to the given URL.
func (r *Request) Get(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodGet).Send()
}

// Post sends a POST request to the given URL.
func (r *Request) Post(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodPost).Send()
}

// Head sends a HEAD request to the given URL.
func (r *Request) Head(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodHead).Send()
}

// Put sends a PUT request to the given URL.
func (r *Request) Put(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodPut).Send()
}

// Delete sends a DELETE request to the given URL.
func (r *Request) Delete(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodDelete).Send()
}

// Options sends an OPTIONS request to the given URL.
func (r *Request) Options(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodOptions).Send()
}

// Patch sends a PATCH request to the given URL.
func (r *Request) Patch(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodPatch).Send()
}

// Custom sends a request with a custom HTTP method to the given URL.
func (r *Request) Custom(url, method string) (*Response, error) {
	return r.SetURL(url).SetMethod(method).Send()
}

// Send executes the Request.
func (r *Request) Send() (*Response, error) {
	r.checkClient()
	return newCore().execute(r.Context(), r.Client(), r)
}

// Reset clears the Request object, returning it to its default state.
// Used by ReleaseRequest to recycle the object.
func (r *Request) Reset() {
	r.url = ""
	r.method = fiber.MethodGet
	r.userAgent = ""
	r.referer = ""
	r.ctx = nil
	r.body = nil
	r.timeout = 0
	r.maxRedirects = 0
	r.bodyType = noBody
	r.boundary = boundary

	for len(r.files) != 0 {
		t := r.files[0]
		r.files = r.files[1:]
		ReleaseFile(t)
	}

	r.formData.Reset()
	r.path.Reset()
	r.cookies.Reset()
	r.header.Reset()
	r.params.Reset()
	r.RawRequest.Reset()
}

// Header wraps fasthttp.RequestHeader, storing headers for both client and request.
type Header struct {
	*fasthttp.RequestHeader
}

// PeekMultiple returns multiple values of a header field with the same key.
func (h *Header) PeekMultiple(key string) []string {
	var res []string
	byteKey := []byte(key)
	for k, value := range h.RequestHeader.All() {
		if bytes.EqualFold(k, byteKey) {
			res = append(res, utils.UnsafeString(value))
		}
	}
	return res
}

// AddHeaders adds multiple headers from a map.
func (h *Header) AddHeaders(r map[string][]string) {
	for k, v := range r {
		for _, vv := range v {
			h.Add(k, vv)
		}
	}
}

// SetHeaders sets multiple headers from a map, overriding previously set values.
func (h *Header) SetHeaders(r map[string]string) {
	for k, v := range r {
		h.Del(k)
		h.Set(k, v)
	}
}

// QueryParam wraps fasthttp.Args for query parameters.
type QueryParam struct {
	*fasthttp.Args
}

// Keys returns all keys from the query parameters.
func (p *QueryParam) Keys() []string {
	keys := make([]string, 0, p.Len())
	for key := range p.All() {
		keys = append(keys, utils.UnsafeString(key))
	}
	return slices.Compact(keys)
}

// AddParams adds multiple parameters from a map.
func (p *QueryParam) AddParams(r map[string][]string) {
	for k, v := range r {
		for _, vv := range v {
			p.Add(k, vv)
		}
	}
}

// SetParams sets multiple parameters from a map, overriding previously set values.
func (p *QueryParam) SetParams(r map[string]string) {
	for k, v := range r {
		p.Set(k, v)
	}
}

// SetParamsWithStruct sets multiple parameters from a struct.
// Nested structs are not currently supported.
func (p *QueryParam) SetParamsWithStruct(v any) {
	SetValWithStruct(p, "param", v)
}

// Cookie is a map used to store cookies.
type Cookie map[string]string

// Add adds a cookie key-value pair.
func (c Cookie) Add(key, val string) {
	c[key] = val
}

// Del deletes a cookie by key.
func (c Cookie) Del(key string) {
	delete(c, key)
}

// SetCookie sets a single cookie value.
func (c Cookie) SetCookie(key, val string) {
	c[key] = val
}

// SetCookies sets multiple cookies from a map.
func (c Cookie) SetCookies(m map[string]string) {
	maps.Copy(c, m)
}

// SetCookiesWithStruct sets cookies from a struct.
// Nested structs are not currently supported.
func (c Cookie) SetCookiesWithStruct(v any) {
	SetValWithStruct(c, "cookie", v)
}

// DelCookies deletes multiple cookies by keys.
func (c Cookie) DelCookies(key ...string) {
	for _, v := range key {
		c.Del(v)
	}
}

// All returns an iterator over cookie key-value pairs.
//
// The returned key and value should not be retained after the iteration loop.
func (c Cookie) All() iter.Seq2[string, string] {
	return maps.All(c)
}

// Reset clears the Cookie map.
func (c Cookie) Reset() {
	clear(c)
}

// PathParam is a map used to store path parameters.
type PathParam map[string]string

// Add adds a path parameter key-value pair.
func (p PathParam) Add(key, val string) {
	p[key] = val
}

// Del deletes a path parameter by key.
func (p PathParam) Del(key string) {
	delete(p, key)
}

// SetParam sets a single path parameter.
func (p PathParam) SetParam(key, val string) {
	p[key] = val
}

// SetParams sets multiple path parameters from a map.
func (p PathParam) SetParams(m map[string]string) {
	maps.Copy(p, m)
}

// SetParamsWithStruct sets multiple path parameters from a struct.
// Nested structs are not currently supported.
func (p PathParam) SetParamsWithStruct(v any) {
	SetValWithStruct(p, "path", v)
}

// DelParams deletes multiple path parameters.
func (p PathParam) DelParams(key ...string) {
	for _, v := range key {
		p.Del(v)
	}
}

// All returns an iterator over path parameter key-value pairs.
//
// The returned key and value should not be retained after the iteration loop.
func (p PathParam) All() iter.Seq2[string, string] {
	return maps.All(p)
}

// Reset clears the PathParam map.
func (p PathParam) Reset() {
	clear(p)
}

// FormData wraps fasthttp.Args for URL-encoded bodies and form data.
type FormData struct {
	*fasthttp.Args
}

// Keys returns all keys from the form data.
func (f *FormData) Keys() []string {
	keys := make([]string, 0, f.Len())
	for key := range f.All() {
		keys = append(keys, utils.UnsafeString(key))
	}
	return slices.Compact(keys)
}

// Add adds a single form field.
func (f *FormData) Add(key, val string) {
	f.Args.Add(key, val)
}

// Set sets a single form field, overriding previously set values.
func (f *FormData) Set(key, val string) {
	f.Args.Set(key, val)
}

// AddWithMap adds multiple form fields from a map.
func (f *FormData) AddWithMap(m map[string][]string) {
	for k, v := range m {
		for _, vv := range v {
			f.Add(k, vv)
		}
	}
}

// SetWithMap sets multiple form fields from a map, overriding previously set values.
func (f *FormData) SetWithMap(m map[string]string) {
	for k, v := range m {
		f.Set(k, v)
	}
}

// SetWithStruct sets multiple form fields from a struct.
// Nested structs are not currently supported.
func (f *FormData) SetWithStruct(v any) {
	SetValWithStruct(f, "form", v)
}

// DelData deletes multiple form fields.
func (f *FormData) DelData(key ...string) {
	for _, v := range key {
		f.Args.Del(v)
	}
}

// Reset clears the FormData object.
func (f *FormData) Reset() {
	f.Args.Reset()
}

// File represents a file to be sent with the request.
type File struct {
	reader    io.ReadCloser
	name      string
	fieldName string
	path      string
}

// SetName sets the file's name.
func (f *File) SetName(n string) {
	f.name = n
}

// SetFieldName sets the key associated with the file in the body.
func (f *File) SetFieldName(n string) {
	f.fieldName = n
}

// SetPath sets the file's path.
func (f *File) SetPath(p string) {
	f.path = p
}

// SetReader sets the file's reader, which will be closed in the parserBody hook.
func (f *File) SetReader(r io.ReadCloser) {
	f.reader = r
}

// Reset clears the File object.
func (f *File) Reset() {
	f.name = ""
	f.fieldName = ""
	f.path = ""
	f.reader = nil
}

var requestPool = &sync.Pool{
	New: func() any {
		return &Request{
			header:     &Header{RequestHeader: &fasthttp.RequestHeader{}},
			params:     &QueryParam{Args: fasthttp.AcquireArgs()},
			cookies:    &Cookie{},
			path:       &PathParam{},
			boundary:   "--FiberFormBoundary",
			formData:   &FormData{Args: fasthttp.AcquireArgs()},
			files:      make([]*File, 0),
			RawRequest: fasthttp.AcquireRequest(),
		}
	},
}

// AcquireRequest returns a new (pooled) Request object.
func AcquireRequest() *Request {
	req, ok := requestPool.Get().(*Request)
	if !ok {
		panic(errors.New("failed to type-assert to *Request"))
	}
	return req
}

// ReleaseRequest returns the Request object to the pool.
// Do not use the released Request afterward to avoid data races.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

var filePool sync.Pool

// SetFileFunc defines a function that modifies a File object.
type SetFileFunc func(f *File)

// SetFileName sets the file name.
func SetFileName(n string) SetFileFunc {
	return func(f *File) {
		f.SetName(n)
	}
}

// SetFileFieldName sets the file's field name.
func SetFileFieldName(p string) SetFileFunc {
	return func(f *File) {
		f.SetFieldName(p)
	}
}

// SetFilePath sets the file path.
func SetFilePath(p string) SetFileFunc {
	return func(f *File) {
		f.SetPath(p)
	}
}

// SetFileReader sets the file's reader.
func SetFileReader(r io.ReadCloser) SetFileFunc {
	return func(f *File) {
		f.SetReader(r)
	}
}

// AcquireFile returns a (pooled) File object and applies the provided SetFileFunc functions to it.
func AcquireFile(setter ...SetFileFunc) *File {
	fv := filePool.Get()
	if fv != nil {
		f, ok := fv.(*File)
		if !ok {
			panic(errors.New("failed to type-assert to *File"))
		}
		for _, v := range setter {
			v(f)
		}
		return f
	}
	f := &File{}
	for _, v := range setter {
		v(f)
	}
	return f
}

// ReleaseFile returns the File object to the pool.
// Do not use the released File afterward to avoid data races.
func ReleaseFile(f *File) {
	f.Reset()
	filePool.Put(f)
}

// SetValWithStruct sets values using a struct. The struct's fields are examined via reflection.
// `p` is a type that implements WithStruct. `tagName` defines the struct tag to look for.
// `v` is the struct containing data.
//
// Fields in `v` should be string, int, int8, int16, int32, int64, uint,
// uint8, uint16, uint32, uint64, float32, float64, complex64,
// complex128 or bool. Arrays or slices are inserted sequentially with the
// same key. Other types are ignored.
func SetValWithStruct(p WithStruct, tagName string, v any) {
	valueOfV := reflect.ValueOf(v)
	typeOfV := reflect.TypeOf(v)

	// The value should be a struct or a pointer to a struct.
	if typeOfV.Kind() == reflect.Pointer && typeOfV.Elem().Kind() == reflect.Struct {
		valueOfV = valueOfV.Elem()
		typeOfV = typeOfV.Elem()
	} else if typeOfV.Kind() != reflect.Struct {
		return
	}

	// A helper function to set values.
	var setVal func(name string, val reflect.Value)
	setVal = func(name string, val reflect.Value) {
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			p.Add(name, strconv.Itoa(int(val.Int())))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			p.Add(name, strconv.FormatUint(val.Uint(), 10))
		case reflect.Float32, reflect.Float64:
			p.Add(name, strconv.FormatFloat(val.Float(), 'f', -1, 64))
		case reflect.Complex64, reflect.Complex128:
			p.Add(name, strconv.FormatComplex(val.Complex(), 'f', -1, 128))
		case reflect.Bool:
			if val.Bool() {
				p.Add(name, "true")
			} else {
				p.Add(name, "false")
			}
		case reflect.String:
			p.Add(name, val.String())
		case reflect.Slice, reflect.Array:
			for i := 0; i < val.Len(); i++ {
				setVal(name, val.Index(i))
			}
		default:
			return
		}
	}

	for i := 0; i < typeOfV.NumField(); i++ {
		field := typeOfV.Field(i)
		if !field.IsExported() {
			continue
		}

		name := field.Tag.Get(tagName)
		if name == "" {
			name = field.Name
		}
		val := valueOfV.Field(i)
		// To cover slice and array, we delete the val then add it.
		p.Del(name)
		setVal(name, val)
	}
}
