package binder

import (
	"mime/multipart"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

const MIMEMultipartForm string = "multipart/form-data"

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
	data := make(map[string][]string)

	// Handle multipart form
	if FilterFlags(utils.UnsafeString(req.Header.ContentType())) == MIMEMultipartForm {
		return b.bindMultipart(req, out)
	}

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

	data := make(map[string][]string)
	for key, values := range multipartForm.Value {
		err = formatBindData(b.Name(), out, data, key, values, b.EnableSplitting, true)
		if err != nil {
			return err
		}
	}

	files := make(map[string][]*multipart.FileHeader)
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
