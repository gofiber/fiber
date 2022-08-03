package client

import (
	"context"
	"reflect"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// Implementing this interface allows data to be passed through the structure.
type WithStruct interface {
	Add(string, string)
	Del(string)
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
)

type Request struct {
	url       string
	method    string
	ctx       context.Context
	userAgent string
	header    *Header
	params    *QueryParam
	cookies   *Cookie
	path      *PathParam

	body     any
	bodyType bodyType

	rawRequest *fasthttp.Request
}

// setMethod will set method for Request object,
// user should use request method to set method.
func (r *Request) setMethod(method string) *Request {
	r.method = method
	return r
}

// SetURL will set url for Request object.
func (r *Request) SetURL(url string) *Request {
	r.url = url
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

// AddHeader method adds a single header field and its value in the request instance.
// It will override header which set in client instance.
func (r *Request) AddHeader(key, val string) *Request {
	r.header.Add(key, val)
	return r
}

// SetHeader method sets a single header field and its value in the request instance.
// It will override header which set in client instance.
func (r *Request) SetHeader(key, val string) *Request {
	r.header.Set(key, val)
	return r
}

// AddHeaders method adds multiple headers field and its values at one go in the request instance.
// It will override header which set in client instance.
func (r *Request) AddHeaders(h map[string][]string) *Request {
	r.header.AddHeaders(h)
	return r
}

// SetHeaders method sets multiple headers field and its values at one go in the request instance.
// It will override header which set in client instance.
func (r *Request) SetHeaders(h map[string]string) *Request {
	r.header.SetHeaders(h)
	return r
}

// AddParam method adds a single param field and its value in the request instance.
// It will override param which set in client instance.
func (r *Request) AddParam(key, val string) *Request {
	r.params.Add(key, val)
	return r
}

// SetParam method sets a single param field and its value in the request instance.
// It will override param which set in client instance.
func (r *Request) SetParam(key, val string) *Request {
	r.params.Set(key, val)
	return r
}

// AddParams method adds multiple params field and its values at one go in the request instance.
// It will override param which set in client instance.
func (r *Request) AddParams(m map[string][]string) *Request {
	r.params.AddParams(m)
	return r
}

// SetParams method sets multiple params field and its values at one go in the request instance.
// It will override param which set in client instance.
func (r *Request) SetParams(m map[string]string) *Request {
	r.params.SetParams(m)
	return r
}

// SetParamWithStruct method sets multiple params field and its values at one go in the request instance.
// It will override param which set in client instance.
func (r *Request) SetParamsWithStruct(v any) *Request {
	r.params.SetParamsWithStruct(v)
	return r
}

// DelParams method deletes single or multiple params field ant its values.
func (r *Request) DelParams(key ...string) *Request {
	for _, v := range key {
		r.params.Del(v)
	}
	return r
}

// SetUserAgent method sets user agent in request.
// It will override user agent which set in client instance.
func (r *Request) SetUserAgent(ua string) *Request {
	r.userAgent = ua
	return r
}

// SetCookie method sets a single cookie field and its value in the request instance.
// It will override cookie which set in client instance.
func (r *Request) SetCookie(key, val string) *Request {
	r.cookies.SetCookie(key, val)
	return r
}

// SetCookies method sets multiple cookie field and its values at one go in the request instance.
// It will override cookie which set in client instance.
func (r *Request) SetCookies(m map[string]string) *Request {
	r.cookies.SetCookies(m)
	return r
}

// SetCookiesWithStruct method sets multiple cookies field and its values at one go in the request instance.
// It will override cookie which set in client instance.
func (r *Request) SetCookiesWithStruct(v any) *Request {
	r.cookies.SetCookiesWithStruct(v)
	return r
}

// DelCookies method deletes single or multiple cookies field ant its values.
func (r *Request) DelCookies(key ...string) *Request {
	r.cookies.DelCookies(key...)
	return r
}

// SetPathParam method sets a single path param field and its value in the request instance.
// It will override path param which set in client instance.
func (r *Request) SetPathParam(key, val string) *Request {
	r.path.SetParam(key, val)
	return r
}

// SetPathParams method sets multiple path params field and its values at one go in the request instance.
// It will override path param which set in client instance.
func (r *Request) SetPathParams(m map[string]string) *Request {
	r.path.SetParams(m)
	return r
}

// SetParamsWithStruct method sets multiple path params field and its values at one go in the request instance.
// It will override path param which set in client instance.
func (r *Request) SetPathParamsWithStruct(v any) *Request {
	r.path.SetParamsWithStruct(v)
	return r
}

// DelPathParams method deletes single or multiple path params field ant its values.
func (r *Request) DelPathParams(key ...string) *Request {
	r.path.DelParams(key...)
	return r
}

// SetJSON method sets json body in request.
func (r *Request) SetJSON(v any) *Request {
	r.body = v
	r.bodyType = jsonBody
	return r
}

// SetXML method sets xml body in request.
func (r *Request) SetXML(v any) *Request {
	r.body = v
	r.bodyType = xmlBody
	return r
}

// SetRawBody method sets body with raw data in request.
func (r *Request) SetRawBody(v []byte) *Request {
	r.body = v
	r.bodyType = rawBody
	return r
}

// Reset clear Request object, used by ReleaseRequest method.
func (r *Request) Reset() {
	r.url = ""
	r.method = fiber.MethodGet
	r.ctx = nil
	r.userAgent = ""
	r.body = nil
	r.bodyType = noBody

	r.path.Reset()
	r.cookies.Reset()
	r.header.Reset()
	r.params.Reset()
	r.rawRequest.Reset()
}

// Header is a wrapper which wrap http.Header,
// the header in client and request will store in it.
type Header struct {
	*fasthttp.RequestHeader
}

// AddHeaders receive a map and add each value to header.
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

// SetCookie method sets a signle val in Cookie.
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

// DelCookies method deletes mutiple val in Cookie.
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

// Reset clear the Cookie object.
func (c Cookie) Reset() {
	for k := range c {
		delete(c, k)
	}
}

// PathParam is a map which to store the cookies.
type PathParam map[string]string

// Add method impl the method in WithStruct interface.
func (p PathParam) Add(key, val string) {
	p[key] = val
}

// Del method impl the method in WithStruct interface.
func (p PathParam) Del(key string) {
	delete(p, key)
}

// SetParam method sets a signle val in PathParam.
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

// DelParams method deletes mutiple val in PathParams.
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

// Reset clear the PathParams object.
func (p PathParam) Reset() {
	for k := range p {
		delete(p, k)
	}
}

var requestPool sync.Pool

// AcquireRequest returns an empty request object from the pool.
//
// The returned request may be returned to the pool with ReleaseRequest when no longer needed.
// This allows reducing GC load.
func AcquireRequest() (req *Request) {
	reqv := requestPool.Get()
	if reqv != nil {
		req = reqv.(*Request)
		return
	}

	req = &Request{
		header:     &Header{RequestHeader: &fasthttp.RequestHeader{}},
		params:     &QueryParam{Args: fasthttp.AcquireArgs()},
		cookies:    &Cookie{},
		path:       &PathParam{},
		rawRequest: fasthttp.AcquireRequest(),
	}
	return
}

// ReleaseRequest returns the object acquired via AcquireRequest to the pool.
//
// Do not access the released Request object, otherwise data races may occur.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

// Set some values using structs.
// `p` is a structure that implements the WithStruct interface,
// The field name can be specified by `tagName`.
// `v` is a struct include some data.
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
