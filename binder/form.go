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

		if strings.Contains(k, "[") {
			k, err = parseParamSquareBrackets(k)
		}

		if b.EnableSplitting && strings.Contains(v, ",") && equalFieldType(out, reflect.Slice, k) {
			values := strings.Split(v, ",")
			for i := 0; i < len(values); i++ {
				data[k] = append(data[k], values[i])
			}
		} else {
			data[k] = append(data[k], v)
		}
	})

	if err != nil {
		return err
	}

	return parse(b.Name(), out, data)
}

// bindMultipart parses the request body and returns the result.
func (b *FormBinding) bindMultipart(req *fasthttp.Request, out any) error {
	data, err := req.MultipartForm()
	if err != nil {
		return err
	}

	temp := make(map[string][]string)
	for key, values := range data.Value {
		if strings.Contains(key, "[") {
			k, err := parseParamSquareBrackets(key)
			if err != nil {
				return err
			}

			key = k // We have to update key in case bracket notation and slice type are used at the same time
		}

		for _, v := range values {
			if strings.Contains(v, ",") && equalFieldType(out, reflect.Slice, key) {
				temp[key] = strings.Split(v, ",")
			} else {
				temp[key] = append(temp[key], v)
			}
		}
	}

	return parse(b.Name(), out, temp)
}

// Reset resets the FormBinding binder.
func (b *FormBinding) Reset() {
	b.EnableSplitting = false
}
