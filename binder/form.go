package binder

import (
	"reflect"
	"strings"

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
	var err error

	// Handle multipart form
	if FilterFlags(utils.UnsafeString(req.Header.ContentType())) == MIMEMultipartForm {
		return b.bindMultipart(req, out)
	}

	req.PostArgs().VisitAll(func(key, val []byte) {
		if err != nil {
			return
		}

		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		err = formatBindData(out, data, k, v, b.EnableSplitting, true)
	})

	if err != nil {
		return err
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

	// Bind form values
	for key, values := range multipartForm.Value {
		err = formatBindData(out, data, key, values, b.EnableSplitting, true)
		if err != nil {
			return err
		}
	}

	// Check struct type
	outValue := reflect.ValueOf(out)
	if outValue.Kind() == reflect.Ptr {
		outValue = outValue.Elem()
	}

	// If it's a struct, process files
	if outValue.Kind() == reflect.Struct {
		// Bind files
		for key, fileHeaders := range multipartForm.File {
			if len(fileHeaders) > 0 {
				field := outValue.FieldByNameFunc(func(s string) bool {
					// Check form tag
					field, ok := outValue.Type().FieldByName(s)
					if !ok {
						return false
					}
					formTag := field.Tag.Get("form")
					if formTag == "" {
						return strings.EqualFold(s, key)
					}
					return strings.EqualFold(strings.Split(formTag, ",")[0], key)
				})

				if field.IsValid() && field.Type().AssignableTo(reflect.TypeOf(fileHeaders[0])) {
					field.Set(reflect.ValueOf(fileHeaders[0]))
				}
			}
		}
	}

	return parse(b.Name(), out, data)
}

// Reset resets the FormBinding binder.
func (b *FormBinding) Reset() {
	b.EnableSplitting = false
}
