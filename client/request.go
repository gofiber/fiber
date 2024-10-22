package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// WithStruct Implementing this interface allows data to
// be stored from a struct via reflect.
type WithStruct interface {
	Add(name, obj string)
	Del(name string)
}

// Types of request bodies.
type bodyType int

// Enumeration definition of the request body type.
const (
	noBody bodyType = iota
	jsonBody
	xmlBody
	formBody
	filesBody
	rawBody
	cborBody
)

var ErrClientNil = errors.New("client can not be nil")

// Request is a struct which contains the request data.
type Request struct {
	ctx context.Context //nolint:containedctx // It's needed to be stored in the request.

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

// Method returns http method in request.
func (r *Request) Method() string {
	return r.method
}

// SetMethod will set method for Request object,
// user should use request method to set method.
func (r *Request) SetMethod(method string) *Request {
	r.method = method
	return r
}

// URL returns request url in Request instance.
func (r *Request) URL() string {
	return r.url
}

// SetURL will set url for Request object.
func (r *Request) SetURL(url string) *Request {
	r.url = url
	return r
}

// Client get Client instance in Request.
func (r *Request) Client() *Client {
	return r.client
}

// SetClient method sets client in request instance.
func (r *Request) SetClient(c *Client) *Request {
	if c == nil {
		panic(ErrClientNil)
	}

	r.client = c
	return r
}

// Context returns the Context if its already set in request
// otherwise it creates new one using `context.Background()`.
func (r *Request) Context() context.Context {
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

// SetContext sets the context.Context for current Request. It allows
// to interrupt the request execution if ctx.Done() channel is closed.
// See https://blog.golang.org/context article and the "context" package
// documentation.
func (r *Request) SetContext(ctx context.Context) *Request {
	r.ctx = ctx
	return r
}

// Header method returns header value via key,
// this method will visit all field in the header.
func (r *Request) Header(key string) []string {
	return r.header.PeekMultiple(key)
}

// AddHeader method adds a single header field and its value in the request instance.
func (r *Request) AddHeader(key, val string) *Request {
	r.header.Add(key, val)
	return r
}

// SetHeader method sets a single header field and its value in the request instance.
// It will override header which has been set in client instance.
func (r *Request) SetHeader(key, val string) *Request {
	r.header.Del(key)
	r.header.Set(key, val)
	return r
}

// AddHeaders method adds multiple header fields and its values at one go in the request instance.
func (r *Request) AddHeaders(h map[string][]string) *Request {
	r.header.AddHeaders(h)
	return r
}

// SetHeaders method sets multiple header fields and its values at one go in the request instance.
// It will override header which has been set in client instance.
func (r *Request) SetHeaders(h map[string]string) *Request {
	r.header.SetHeaders(h)
	return r
}

// Param method returns params value via key,
// this method will visit all field in the query param.
func (r *Request) Param(key string) []string {
	var res []string
	tmp := r.params.PeekMulti(key)
	for _, v := range tmp {
		res = append(res, utils.UnsafeString(v))
	}

	return res
}

// AddParam method adds a single param field and its value in the request instance.
func (r *Request) AddParam(key, val string) *Request {
	r.params.Add(key, val)
	return r
}

// SetParam method sets a single param field and its value in the request instance.
// It will override param which has been set in client instance.
func (r *Request) SetParam(key, val string) *Request {
	r.params.Set(key, val)
	return r
}

// AddParams method adds multiple param fields and its values at one go in the request instance.
func (r *Request) AddParams(m map[string][]string) *Request {
	r.params.AddParams(m)
	return r
}

// SetParams method sets multiple param fields and its values at one go in the request instance.
// It will override param which has been set in client instance.
func (r *Request) SetParams(m map[string]string) *Request {
	r.params.SetParams(m)
	return r
}

// SetParamsWithStruct method sets multiple param fields and its values at one go in the request instance.
// It will override param which has been set in client instance.
func (r *Request) SetParamsWithStruct(v any) *Request {
	r.params.SetParamsWithStruct(v)
	return r
}

// DelParams method deletes single or multiple param fields ant its values.
func (r *Request) DelParams(key ...string) *Request {
	for _, v := range key {
		r.params.Del(v)
	}
	return r
}

// UserAgent returns user agent in request instance.
func (r *Request) UserAgent() string {
	return r.userAgent
}

// SetUserAgent method sets user agent in request.
// It will override user agent which has been set in client instance.
func (r *Request) SetUserAgent(ua string) *Request {
	r.userAgent = ua
	return r
}

// Boundary returns boundary in multipart boundary.
func (r *Request) Boundary() string {
	return r.boundary
}

// SetBoundary method sets multipart boundary.
func (r *Request) SetBoundary(b string) *Request {
	r.boundary = b

	return r
}

// Referer returns referer in request instance.
func (r *Request) Referer() string {
	return r.referer
}

// SetReferer method sets referer in request.
// It will override referer which set in client instance.
func (r *Request) SetReferer(referer string) *Request {
	r.referer = referer
	return r
}

// Cookie returns the cookie be set in request instance.
// if cookie doesn't exist, return empty string.
func (r *Request) Cookie(key string) string {
	if val, ok := (*r.cookies)[key]; ok {
		return val
	}
	return ""
}

// SetCookie method sets a single cookie field and its value in the request instance.
// It will override cookie which set in client instance.
func (r *Request) SetCookie(key, val string) *Request {
	r.cookies.SetCookie(key, val)
	return r
}

// SetCookies method sets multiple cookie fields and its values at one go in the request instance.
// It will override cookie which set in client instance.
func (r *Request) SetCookies(m map[string]string) *Request {
	r.cookies.SetCookies(m)
	return r
}

// SetCookiesWithStruct method sets multiple cookie fields and its values at one go in the request instance.
// It will override cookie which set in client instance.
func (r *Request) SetCookiesWithStruct(v any) *Request {
	r.cookies.SetCookiesWithStruct(v)
	return r
}

// DelCookies method deletes single or multiple cookie fields ant its values.
func (r *Request) DelCookies(key ...string) *Request {
	r.cookies.DelCookies(key...)
	return r
}

// PathParam returns the path param be set in request instance.
// if path param doesn't exist, return empty string.
func (r *Request) PathParam(key string) string {
	if val, ok := (*r.path)[key]; ok {
		return val
	}

	return ""
}

// SetPathParam method sets a single path param field and its value in the request instance.
// It will override path param which set in client instance.
func (r *Request) SetPathParam(key, val string) *Request {
	r.path.SetParam(key, val)
	return r
}

// SetPathParams method sets multiple path param fields and its values at one go in the request instance.
// It will override path param which set in client instance.
func (r *Request) SetPathParams(m map[string]string) *Request {
	r.path.SetParams(m)
	return r
}

// SetPathParamsWithStruct method sets multiple path param fields and its values at one go in the request instance.
// It will override path param which set in client instance.
func (r *Request) SetPathParamsWithStruct(v any) *Request {
	r.path.SetParamsWithStruct(v)
	return r
}

// DelPathParams method deletes single or multiple path param fields ant its values.
func (r *Request) DelPathParams(key ...string) *Request {
	r.path.DelParams(key...)
	return r
}

// ResetPathParams deletes all path params.
func (r *Request) ResetPathParams() *Request {
	r.path.Reset()
	return r
}

// SetJSON method sets JSON body in request.
func (r *Request) SetJSON(v any) *Request {
	r.body = v
	r.bodyType = jsonBody
	return r
}

// SetXML method sets XML body in request.
func (r *Request) SetXML(v any) *Request {
	r.body = v
	r.bodyType = xmlBody
	return r
}

func (r *Request) SetCBOR(v any) *Request {
	r.body = v
	r.bodyType = cborBody
	return r
}

// SetRawBody method sets body with raw data in request.
func (r *Request) SetRawBody(v []byte) *Request {
	r.body = v
	r.bodyType = rawBody
	return r
}

// resetBody will clear body object and set bodyType
// if body type is formBody and filesBody, the new body type will be ignored.
func (r *Request) resetBody(t bodyType) {
	r.body = nil

	// Set form data after set file ignore.
	if r.bodyType == filesBody && t == formBody {
		return
	}
	r.bodyType = t
}

// FormData method returns form data value via key,
// this method will visit all field in the form data.
func (r *Request) FormData(key string) []string {
	var res []string
	tmp := r.formData.PeekMulti(key)
	for _, v := range tmp {
		res = append(res, utils.UnsafeString(v))
	}

	return res
}

// AddFormData method adds a single form data field and its value in the request instance.
func (r *Request) AddFormData(key, val string) *Request {
	r.formData.AddData(key, val)
	r.resetBody(formBody)
	return r
}

// SetFormData method sets a single form data field and its value in the request instance.
func (r *Request) SetFormData(key, val string) *Request {
	r.formData.SetData(key, val)
	r.resetBody(formBody)
	return r
}

// AddFormDatas method adds multiple form data fields and its values in the request instance.
func (r *Request) AddFormDatas(m map[string][]string) *Request {
	r.formData.AddDatas(m)
	r.resetBody(formBody)
	return r
}

// SetFormDatas method sets multiple form data fields and its values in the request instance.
func (r *Request) SetFormDatas(m map[string]string) *Request {
	r.formData.SetDatas(m)
	r.resetBody(formBody)
	return r
}

// SetFormDatasWithStruct method sets multiple form data fields
// and its values in the request instance via struct.
func (r *Request) SetFormDatasWithStruct(v any) *Request {
	r.formData.SetDatasWithStruct(v)
	r.resetBody(formBody)
	return r
}

// DelFormDatas method deletes multiple form data fields and its value in the request instance.
func (r *Request) DelFormDatas(key ...string) *Request {
	r.formData.DelDatas(key...)
	r.resetBody(formBody)
	return r
}

// File returns file ptr store in request obj by name.
// If name field is empty, it will try to match path.
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

// FileByPath returns file ptr store in request obj by path.
func (r *Request) FileByPath(path string) *File {
	for _, v := range r.files {
		if v.path == path {
			return v
		}
	}

	return nil
}

// AddFile method adds single file field
// and its value in the request instance via file path.
func (r *Request) AddFile(path string) *Request {
	r.files = append(r.files, AcquireFile(SetFilePath(path)))
	r.resetBody(filesBody)
	return r
}

// AddFileWithReader method adds single field
// and its value in the request instance via reader.
func (r *Request) AddFileWithReader(name string, reader io.ReadCloser) *Request {
	r.files = append(r.files, AcquireFile(SetFileName(name), SetFileReader(reader)))
	r.resetBody(filesBody)
	return r
}

// AddFiles method adds multiple file fields
// and its value in the request instance via File instance.
func (r *Request) AddFiles(files ...*File) *Request {
	r.files = append(r.files, files...)
	r.resetBody(filesBody)
	return r
}

// Timeout returns the length of timeout in request.
func (r *Request) Timeout() time.Duration {
	return r.timeout
}

// SetTimeout method sets timeout field and its values at one go in the request instance.
// It will override timeout which set in client instance.
func (r *Request) SetTimeout(t time.Duration) *Request {
	r.timeout = t
	return r
}

// MaxRedirects returns the max redirects count in request.
func (r *Request) MaxRedirects() int {
	return r.maxRedirects
}

// SetMaxRedirects method sets the maximum number of redirects at one go in the request instance.
// It will override max redirect which set in client instance.
func (r *Request) SetMaxRedirects(count int) *Request {
	r.maxRedirects = count
	return r
}

// checkClient method checks whether the client has been set in request.
func (r *Request) checkClient() {
	if r.client == nil {
		r.SetClient(defaultClient)
	}
}

// Get Send get request.
func (r *Request) Get(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodGet).Send()
}

// Post Send post request.
func (r *Request) Post(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodPost).Send()
}

// Head Send head request.
func (r *Request) Head(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodHead).Send()
}

// Put Send put request.
func (r *Request) Put(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodPut).Send()
}

// Delete Send Delete request.
func (r *Request) Delete(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodDelete).Send()
}

// Options Send Options request.
func (r *Request) Options(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodOptions).Send()
}

// Patch Send patch request.
func (r *Request) Patch(url string) (*Response, error) {
	return r.SetURL(url).SetMethod(fiber.MethodPatch).Send()
}

// Custom Send custom request.
func (r *Request) Custom(url, method string) (*Response, error) {
	return r.SetURL(url).SetMethod(method).Send()
}

// Send a request.
func (r *Request) Send() (*Response, error) {
	r.checkClient()

	return newCore().execute(r.Context(), r.Client(), r)
}

// Reset clear Request object, used by ReleaseRequest method.
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

// Header is a wrapper which wrap http.Header,
// the header in client and request will store in it.
type Header struct {
	*fasthttp.RequestHeader
}

// PeekMultiple methods returns multiple field in header with same key.
func (h *Header) PeekMultiple(key string) []string {
	var res []string
	byteKey := []byte(key)
	h.RequestHeader.VisitAll(func(key, value []byte) {
		if bytes.EqualFold(key, byteKey) {
			res = append(res, utils.UnsafeString(value))
		}
	})

	return res
}

// AddHeaders receives a map and add each value to header.
func (h *Header) AddHeaders(r map[string][]string) {
	for k, v := range r {
		for _, vv := range v {
			h.Add(k, vv)
		}
	}
}

// SetHeaders will override all headers.
func (h *Header) SetHeaders(r map[string]string) {
	for k, v := range r {
		h.Del(k)
		h.Set(k, v)
	}
}

// QueryParam is a wrapper which wrap url.Values,
// the query string and formdata in client and request will store in it.
type QueryParam struct {
	*fasthttp.Args
}

// AddParams receive a map and add each value to param.
func (p *QueryParam) AddParams(r map[string][]string) {
	for k, v := range r {
		for _, vv := range v {
			p.Add(k, vv)
		}
	}
}

// SetParams will override all params.
func (p *QueryParam) SetParams(r map[string]string) {
	for k, v := range r {
		p.Set(k, v)
	}
}

// SetParamsWithStruct will override all params with struct or pointer of struct.
// Now nested structs are not currently supported.
func (p *QueryParam) SetParamsWithStruct(v any) {
	SetValWithStruct(p, "param", v)
}

// Cookie is a map which to store the cookies.
type Cookie map[string]string

// Add method impl the method in WithStruct interface.
func (c Cookie) Add(key, val string) {
	c[key] = val
}

// Del method impl the method in WithStruct interface.
func (c Cookie) Del(key string) {
	delete(c, key)
}

// SetCookie method sets a single val in Cookie.
func (c Cookie) SetCookie(key, val string) {
	c[key] = val
}

// SetCookies method sets multiple val in Cookie.
func (c Cookie) SetCookies(m map[string]string) {
	for k, v := range m {
		c[k] = v
	}
}

// SetCookiesWithStruct method sets multiple val in Cookie via a struct.
func (c Cookie) SetCookiesWithStruct(v any) {
	SetValWithStruct(c, "cookie", v)
}

// DelCookies method deletes multiple val in Cookie.
func (c Cookie) DelCookies(key ...string) {
	for _, v := range key {
		c.Del(v)
	}
}

// VisitAll method receive a function which can travel the all val.
func (c Cookie) VisitAll(f func(key, val string)) {
	for k, v := range c {
		f(k, v)
	}
}

// Reset clears the Cookie object.
func (c Cookie) Reset() {
	for k := range c {
		delete(c, k)
	}
}

// PathParam is a map which to store path params.
type PathParam map[string]string

// Add method impl the method in WithStruct interface.
func (p PathParam) Add(key, val string) {
	p[key] = val
}

// Del method impl the method in WithStruct interface.
func (p PathParam) Del(key string) {
	delete(p, key)
}

// SetParam method sets a single val in PathParam.
func (p PathParam) SetParam(key, val string) {
	p[key] = val
}

// SetParams method sets multiple val in PathParam.
func (p PathParam) SetParams(m map[string]string) {
	for k, v := range m {
		p[k] = v
	}
}

// SetParamsWithStruct method sets multiple val in PathParam via a struct.
func (p PathParam) SetParamsWithStruct(v any) {
	SetValWithStruct(p, "path", v)
}

// DelParams method deletes multiple val in PathParams.
func (p PathParam) DelParams(key ...string) {
	for _, v := range key {
		p.Del(v)
	}
}

// VisitAll method receive a function which can travel the all val.
func (p PathParam) VisitAll(f func(key, val string)) {
	for k, v := range p {
		f(k, v)
	}
}

// Reset clear the PathParam object.
func (p PathParam) Reset() {
	for k := range p {
		delete(p, k)
	}
}

// FormData is a wrapper of fasthttp.Args,
// and it is used for url encode body and file body.
type FormData struct {
	*fasthttp.Args
}

// AddData method is a wrapper of Args's Add method.
func (f *FormData) AddData(key, val string) {
	f.Add(key, val)
}

// SetData method is a wrapper of Args's Set method.
func (f *FormData) SetData(key, val string) {
	f.Set(key, val)
}

// AddDatas method supports add multiple fields.
func (f *FormData) AddDatas(m map[string][]string) {
	for k, v := range m {
		for _, vv := range v {
			f.Add(k, vv)
		}
	}
}

// SetDatas method supports set multiple fields.
func (f *FormData) SetDatas(m map[string]string) {
	for k, v := range m {
		f.Set(k, v)
	}
}

// SetDatasWithStruct method supports set multiple fields via a struct.
func (f *FormData) SetDatasWithStruct(v any) {
	SetValWithStruct(f, "form", v)
}

// DelDatas method deletes multiple fields.
func (f *FormData) DelDatas(key ...string) {
	for _, v := range key {
		f.Del(v)
	}
}

// Reset clear the FormData object.
func (f *FormData) Reset() {
	f.Args.Reset()
}

// File is a struct which support send files via request.
type File struct {
	reader    io.ReadCloser
	name      string
	fieldName string
	path      string
}

// SetName method sets file name.
func (f *File) SetName(n string) {
	f.name = n
}

// SetFieldName method sets key of file in the body.
func (f *File) SetFieldName(n string) {
	f.fieldName = n
}

// SetPath method set file path.
func (f *File) SetPath(p string) {
	f.path = p
}

// SetReader method can receive a io.ReadCloser
// which will be closed in parserBody hook.
func (f *File) SetReader(r io.ReadCloser) {
	f.reader = r
}

// Reset clear the File object.
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

// AcquireRequest returns an empty request object from the pool.
//
// The returned request may be returned to the pool with ReleaseRequest when no longer needed.
// This allows reducing GC load.
func AcquireRequest() *Request {
	req, ok := requestPool.Get().(*Request)
	if !ok {
		panic(errors.New("failed to type-assert to *Request"))
	}

	return req
}

// ReleaseRequest returns the object acquired via AcquireRequest to the pool.
//
// Do not access the released Request object, otherwise data races may occur.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

var filePool sync.Pool

// SetFileFunc The methods as follows is used by AcquireFile method.
// You can set file field via these method.
type SetFileFunc func(f *File)

// SetFileName method sets file name.
func SetFileName(n string) SetFileFunc {
	return func(f *File) {
		f.SetName(n)
	}
}

// SetFileFieldName method sets key of file in the body.
func SetFileFieldName(p string) SetFileFunc {
	return func(f *File) {
		f.SetFieldName(p)
	}
}

// SetFilePath method set file path.
func SetFilePath(p string) SetFileFunc {
	return func(f *File) {
		f.SetPath(p)
	}
}

// SetFileReader method can receive a io.ReadCloser
func SetFileReader(r io.ReadCloser) SetFileFunc {
	return func(f *File) {
		f.SetReader(r)
	}
}

// AcquireFile returns an File object from the pool.
// And you can set field in the File with SetFileFunc.
//
// The returned file may be returned to the pool with ReleaseFile when no longer needed.
// This allows reducing GC load.
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

// ReleaseFile returns the object acquired via AcquireFile to the pool.
//
// Do not access the released File object, otherwise data races may occur.
func ReleaseFile(f *File) {
	f.Reset()
	filePool.Put(f)
}

// SetValWithStruct Set some values using structs.
// `p` is a structure that implements the WithStruct interface,
// The field name can be specified by `tagName`.
// `v` is a struct include some data.
// Note: This method only supports simple types and nested structs are not currently supported.
func SetValWithStruct(p WithStruct, tagName string, v any) {
	valueOfV := reflect.ValueOf(v)
	typeOfV := reflect.TypeOf(v)

	// The v should be struct or point of struct
	if typeOfV.Kind() == reflect.Pointer && typeOfV.Elem().Kind() == reflect.Struct {
		valueOfV = valueOfV.Elem()
		typeOfV = typeOfV.Elem()
	} else if typeOfV.Kind() != reflect.Struct {
		return
	}

	// Boring type judge.
	// TODO: cover more types and complex data structure.
	var setVal func(name string, value reflect.Value)
	setVal = func(name string, val reflect.Value) {
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			p.Add(name, strconv.Itoa(int(val.Int())))
		case reflect.Bool:
			if val.Bool() {
				p.Add(name, "true")
			}
		case reflect.String:
			p.Add(name, val.String())
		case reflect.Float32, reflect.Float64:
			p.Add(name, strconv.FormatFloat(val.Float(), 'f', -1, 64))
		case reflect.Slice, reflect.Array:
			for i := 0; i < val.Len(); i++ {
				setVal(name, val.Index(i))
			}
		default:
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
		if val.IsZero() {
			continue
		}
		// To cover slice and array, we delete the val then add it.
		p.Del(name)
		setVal(name, val)
	}
}
