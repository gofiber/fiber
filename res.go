package fiber

import "github.com/valyala/fasthttp"

type Res struct {
	app      *App
	fasthttp *fasthttp.Response
}

// TODO:
// nodejs removeHeader()?
// StatusCode() int
// FastHTTP().SetStatusCode() => Status() ?
// cookies
// Get/Peek, nil to string?? XXXXXX
// Middleware Proxy Action?

// FastHTTP returns the *fasthttp.Response object
// This allows you to use all fasthttp response methods
// https://godoc.org/github.com/valyala/fasthttp#Response
func (res *Res) FastHTTP() *fasthttp.Response {
	return res.fasthttp
}

func (res *Res) App() *App {
	return res.app
}

func (res *Res) Append(key string, values ...string) {
	for _, val := range values {
		res.fasthttp.Header.Add(key, val)
	}
}

func (res *Res) Get(key string) string {
	return res.app.getString(res.fasthttp.Header.Peek(key))
}

func (res *Res) Set(key, val string) {
	res.fasthttp.Header.Set(key, val)
}
