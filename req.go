package fiber

import (
	"crypto/tls"
	"mime/multipart"
)

//go:generate ifacemaker --file req.go --struct DefaultReq --iface Req --pkg fiber --output req_interface_gen.go --not-exported true --iface-comment "Req"
type DefaultReq struct {
	ctx *DefaultCtx
}

func (r *DefaultReq) Accepts(offers ...string) string {
	return r.ctx.Accepts(offers...)
}

func (r *DefaultReq) AcceptsCharsets(offers ...string) string {
	return r.ctx.AcceptsCharsets(offers...)
}

func (r *DefaultReq) AcceptsEncodings(offers ...string) string {
	return r.ctx.AcceptsEncodings(offers...)
}

func (r *DefaultReq) AcceptsLanguages(offers ...string) string {
	return r.ctx.AcceptsLanguages(offers...)
}

func (r *DefaultReq) BaseURL() string {
	return r.ctx.BaseURL()
}

func (r *DefaultReq) Body() []byte {
	return r.ctx.Body()
}

func (r *DefaultReq) BodyRaw() []byte {
	return r.ctx.BodyRaw()
}

func (r *DefaultReq) ClientHelloInfo() *tls.ClientHelloInfo {
	return r.ctx.ClientHelloInfo()
}

func (r *DefaultReq) Cookies(key string, defaultValue ...string) string {
	return r.ctx.Cookies(key, defaultValue...)
}

func (r *DefaultReq) FormFile(key string) (*multipart.FileHeader, error) {
	return r.ctx.FormFile(key)
}

func (r *DefaultReq) FormValue(key string, defaultValue ...string) string {
	return r.ctx.FormValue(key, defaultValue...)
}

func (r *DefaultReq) Fresh() bool {
	return r.ctx.Fresh()
}

func (r *DefaultReq) Get(key string, defaultValue ...string) string {
	return r.ctx.Get(key, defaultValue...)
}

func (r *DefaultReq) Host() string {
	return r.ctx.Host()
}

func (r *DefaultReq) Hostname() string {
	return r.ctx.Hostname()
}

func (r *DefaultReq) IP() string {
	return r.ctx.IP()
}

func (r *DefaultReq) IPs() []string {
	return r.ctx.IPs()
}

func (r *DefaultReq) Is(extension string) bool {
	return r.ctx.Is(extension)
}

func (r *DefaultReq) IsFromLocal() bool {
	return r.ctx.IsFromLocal()
}

func (r *DefaultReq) IsProxyTrusted() bool {
	return r.ctx.IsProxyTrusted()
}

func (r *DefaultReq) Method(override ...string) string {
	return r.ctx.Method(override...)
}

func (r *DefaultReq) MultipartForm() (*multipart.Form, error) {
	return r.ctx.MultipartForm()
}

func (r *DefaultReq) OriginalURL() string {
	return r.ctx.OriginalURL()
}

func (r *DefaultReq) Params(key string, defaultValue ...string) string {
	return r.ctx.Params(key, defaultValue...)
}

func (r *DefaultReq) Path(override ...string) string {
	return r.ctx.Path(override...)
}

func (r *DefaultReq) Port() string {
	return r.ctx.Port()
}

func (r *DefaultReq) Protocol() string {
	return r.ctx.Protocol()
}

func (r *DefaultReq) Queries() map[string]string {
	return r.ctx.Queries()
}

func (r *DefaultReq) Query(key string, defaultValue ...string) string {
	return r.ctx.Query(key, defaultValue...)
}

func (r *DefaultReq) Range(size int) (Range, error) {
	return r.ctx.Range(size)
}

func (r *DefaultReq) Route() *Route {
	return r.ctx.Route()
}

func (r *DefaultReq) SaveFile(fileheader *multipart.FileHeader, path string) error {
	return r.ctx.SaveFile(fileheader, path)
}

func (r *DefaultReq) SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error {
	return r.ctx.SaveFileToStorage(fileheader, path, storage)
}

func (r *DefaultReq) Secure() bool {
	return r.ctx.Secure()
}

func (r *DefaultReq) Stale() bool {
	return r.ctx.Stale()
}

func (r *DefaultReq) Subdomains(offset ...int) []string {
	return r.ctx.Subdomains(offset...)
}

func (r *DefaultReq) XHR() bool {
	return r.ctx.XHR()
}
