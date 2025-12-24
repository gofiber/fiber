package binder

import (
	"mime/multipart"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

const MIMEMultipartForm string = "multipart/form-data"

var (
	formMapPool = sync.Pool{
		New: func() any {
			return make(map[string][]string)
		},
	}
	formFileMapPool = sync.Pool{
		New: func() any {
			return make(map[string][]*multipart.FileHeader)
		},
	}
)

// FormBinding is the form binder for form request body.
type FormBinding struct {
	EnableSplitting bool
}

// Name returns the binding name.
func (*FormBinding) Name() string {
	return "form"
}

// Bind parses the request body and returns the result.
func (b *FormBinding) Bind(req *fasthttp.Request, out any) error {
	// Handle multipart form
	if FilterFlags(utils.UnsafeString(req.Header.ContentType())) == MIMEMultipartForm {
		return b.bindMultipart(req, out)
	}

	data := acquireFormMap()
	defer releaseFormMap(data)

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
	multipartForm, err := req.MultipartForm()
	if err != nil {
		return err
	}

	data := acquireFormMap()
	defer releaseFormMap(data)

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
}

func acquireFormMap() map[string][]string {
	m, ok := formMapPool.Get().(map[string][]string)
	if !ok {
		m = make(map[string][]string)
	}
	clearFormMap(m)
	return m
}

func releaseFormMap(m map[string][]string) {
	clearFormMap(m)
	formMapPool.Put(m)
}

func acquireFileHeaderMap() map[string][]*multipart.FileHeader {
	m, ok := formFileMapPool.Get().(map[string][]*multipart.FileHeader)
	if !ok {
		m = make(map[string][]*multipart.FileHeader)
	}
	clearFileHeaderMap(m)
	return m
}

func releaseFileHeaderMap(m map[string][]*multipart.FileHeader) {
	clearFileHeaderMap(m)
	formFileMapPool.Put(m)
}

func clearFormMap(m map[string][]string) {
	for k := range m {
		delete(m, k)
	}
}

func clearFileHeaderMap(m map[string][]*multipart.FileHeader) {
	for k := range m {
		delete(m, k)
	}
}
