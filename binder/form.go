package binder

import (
	"mime/multipart"
	"sync"

	"github.com/gofiber/fiber/v3/internal/mediatype"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

const MIMEMultipartForm string = "multipart/form-data"

var (
	dataMapPool = sync.Pool{
		New: func() any {
			return make(map[string][]string, 8)
		},
	}
	formFileMapPool = sync.Pool{
		New: func() any {
			return make(map[string][]*multipart.FileHeader)
		},
	}
)

// Keep oversized maps out of the pool so a rare large bind doesn't get retained
// and reused across subsequent requests.
const maxPoolableDataMapSize = 256

// FormBinding is the form binder for form request body.
type FormBinding struct {
	EnableSplitting bool
	MaxBodySize     int
}

// Name returns the binding name.
func (*FormBinding) Name() string {
	return "form"
}

// Bind parses the request body and returns the result.
func (b *FormBinding) Bind(req *fasthttp.Request, out any) error {
	// Callers that reach the binder directly do not go through Fiber's header
	// normalization, and comparing case-insensitively here is not enough on its
	// own: the parsers this method hands the request to — Request.PostArgs and
	// Request.MultipartForm — match the media type and the "boundary=" parameter
	// name case-sensitively, so a legal "Multipart/Form-Data" would still bind
	// nothing (or fail outright once the comparison routed it to bindMultipart).
	// Fold the case-insensitive parts first, then compare.
	//
	// Guarded on the media type actually being a form, because the fold lands
	// on the request's own bytes: running it on, say, a JSON request would
	// rewrite a header the caller may still be holding a view into, to no
	// purpose — neither parser below is going to look at it.
	if mediatype.IsForm(req.Header.ContentType()) {
		mediatype.NormalizeRequestContentType(&req.Header)
	}

	// Handle multipart form. Media types are case-insensitive
	// (RFC 9110 Section 8.3.1), so compare accordingly.
	if utils.EqualFold(FilterFlags(utils.UnsafeString(req.Header.ContentType())), MIMEMultipartForm) {
		return b.bindMultipart(req, out)
	}

	data := acquireDataMap()
	defer releaseDataMap(data)

	for key, val := range req.PostArgs().All() {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		if err := formatBindData(b.Name(), out, data, k, v, b.EnableSplitting, true); err != nil {
			return err
		}
	}

	return parse(b.Name(), out, data)
}

// bindMultipart parses the request body and returns the result.
func (b *FormBinding) bindMultipart(req *fasthttp.Request, out any) error {
	multipartForm, err := req.MultipartFormWithLimit(b.MaxBodySize)
	if err != nil {
		return err
	}

	data := acquireDataMap()
	defer releaseDataMap(data)

	for key, values := range multipartForm.Value {
		err = formatBindData(b.Name(), out, data, key, values, b.EnableSplitting, true)
		if err != nil {
			return err
		}
	}

	files := acquireFileHeaderMap()
	defer releaseFileHeaderMap(files)

	for key, values := range multipartForm.File {
		err = formatBindData(b.Name(), out, files, key, values, b.EnableSplitting, true)
		if err != nil {
			return err
		}
	}

	return parse(b.Name(), out, data, files)
}

// Reset resets the FormBinding binder.
func (b *FormBinding) Reset() {
	b.EnableSplitting = false
	b.MaxBodySize = 0
}

func acquireDataMap() map[string][]string {
	m, ok := dataMapPool.Get().(map[string][]string)
	if !ok {
		m = make(map[string][]string, 8)
	}
	return m
}

func releaseDataMap(m map[string][]string) {
	if len(m) > maxPoolableDataMapSize {
		return
	}

	clearDataMap(m)
	dataMapPool.Put(m)
}

func acquireFileHeaderMap() map[string][]*multipart.FileHeader {
	m, ok := formFileMapPool.Get().(map[string][]*multipart.FileHeader)
	if !ok {
		m = make(map[string][]*multipart.FileHeader)
	}
	return m
}

func releaseFileHeaderMap(m map[string][]*multipart.FileHeader) {
	if len(m) > maxPoolableDataMapSize {
		return
	}

	clearFileHeaderMap(m)
	formFileMapPool.Put(m)
}

func clearDataMap(m map[string][]string) {
	clear(m)
}

func clearFileHeaderMap(m map[string][]*multipart.FileHeader) {
	clear(m)
}
