package fiber

import "github.com/valyala/fasthttp"

type Request struct {
	app      *App
	fasthttp *fasthttp.Request
}

func (r *Request) App() *App {
	return r.app
}

func (r *Request) Append(header, value string) {
	r.fasthttp.Header.Add(header, value)
}

func (r *Request) OriginalURL() string {
	return r.app.getString(r.fasthttp.Header.RequestURI())
}
