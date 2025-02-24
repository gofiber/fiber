package fiber

import (
	"bufio"
)

//go:generate ifacemaker --file res.go --struct DefaultRes --iface Res --pkg fiber --output res_interface_gen.go --not-exported true --iface-comment "Res"
type DefaultRes struct {
	ctx *DefaultCtx
}

func (r *DefaultRes) Append(field string, values ...string) {
	r.ctx.Append(field, values...)
}

func (r *DefaultRes) Attachment(filename ...string) {
	r.ctx.Attachment(filename...)
}

func (r *DefaultRes) AutoFormat(body any) error {
	return r.ctx.AutoFormat(body)
}

func (r *DefaultRes) CBOR(body any, ctype ...string) error {
	return r.ctx.CBOR(body, ctype...)
}

func (r *DefaultRes) ClearCookie(key ...string) {
	r.ctx.ClearCookie(key...)
}

func (r *DefaultRes) Cookie(cookie *Cookie) {
	r.ctx.Cookie(cookie)
}

func (r *DefaultRes) Download(file string, filename ...string) error {
	return r.ctx.Download(file, filename...)
}

func (r *DefaultRes) Format(handlers ...ResFmt) error {
	return r.ctx.Format(handlers...)
}

func (r *DefaultRes) Get(key string, defaultValue ...string) string {
	return r.ctx.GetRespHeader(key, defaultValue...)
}

func (r *DefaultRes) JSON(body any, ctype ...string) error {
	return r.ctx.JSON(body, ctype...)
}

func (r *DefaultRes) JSONP(data any, callback ...string) error {
	return r.ctx.JSONP(data, callback...)
}

func (r *DefaultRes) Links(link ...string) {
	r.ctx.Links(link...)
}

func (r *DefaultRes) Location(path string) {
	r.ctx.Location(path)
}

func (r *DefaultRes) Render(name string, bind any, layouts ...string) error {
	return r.ctx.Render(name, bind, layouts...)
}

func (r *DefaultRes) Send(body []byte) error {
	return r.ctx.Send(body)
}

func (r *DefaultRes) SendFile(file string, config ...SendFile) error {
	return r.ctx.SendFile(file, config...)
}

func (r *DefaultRes) SendStatus(status int) error {
	return r.ctx.SendStatus(status)
}

func (r *DefaultRes) SendString(body string) error {
	return r.ctx.SendString(body)
}

func (r *DefaultRes) SendStreamWriter(streamWriter func(*bufio.Writer)) error {
	return r.ctx.SendStreamWriter(streamWriter)
}

func (r *DefaultRes) Set(key, val string) {
	r.ctx.Set(key, val)
}

func (r *DefaultRes) Status(status int) Ctx {
	return r.ctx.Status(status)
}

func (r *DefaultRes) Type(extension string, charset ...string) Ctx {
	return r.ctx.Type(extension, charset...)
}

func (r *DefaultRes) Vary(fields ...string) {
	r.ctx.Vary(fields...)
}

func (r *DefaultRes) Write(p []byte) (int, error) {
	return r.ctx.Write(p)
}

func (r *DefaultRes) Writef(f string, a ...any) (int, error) {
	return r.ctx.Writef(f, a...)
}

func (r *DefaultRes) WriteString(s string) (int, error) {
	return r.ctx.WriteString(s)
}

func (r *DefaultRes) XML(data any) error {
	return r.ctx.XML(data)
}
