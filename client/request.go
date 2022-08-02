package client

import (
	"context"
	"reflect"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

type Request struct {
	url        string
	method     string
	ctx        context.Context
	header     *Header
	params     *Params
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

// Reset clear Request object, used by ReleaseRequest method.
func (r *Request) Reset() {
	r.url = ""
	r.method = fiber.MethodGet
	r.ctx = nil

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

// Params is a wrapper which wrap url.Values,
// the query string and formdata in client and request will store in it.
type Params struct {
	*fasthttp.Args
}

// AddParams receive a map and add each value to param.
func (p *Params) AddParams(r map[string][]string) {
	for k, v := range r {
		for _, vv := range v {
			p.Add(k, vv)
		}
	}
}

// SetParams will override all params.
func (p *Params) SetParams(r map[string]string) {
	for k, v := range r {
		p.Set(k, v)
	}
}

func (p *Params) SetParamsWithStruct(v any) {
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
			} else {
				p.Add(name, "false")
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

		name := field.Tag.Get("param")
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
		params:     &Params{Args: fasthttp.AcquireArgs()},
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
