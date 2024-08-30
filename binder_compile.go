package fiber

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/gofiber/fiber/v3/internal/bind"
	"github.com/gofiber/utils/v2"
)

type Decoder func(c Ctx, rv reflect.Value) error

const bindTagRespHeader = "respHeader"
const bindTagHeader = "header"
const bindTagQuery = "query"
const bindTagParam = "param"
const bindTagCookie = "cookie"

const bindTagForm = "form"
const bindTagMultipart = "multipart"

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
var bindUnmarshalerType = reflect.TypeOf((*Binder)(nil)).Elem()

type bindCompileOption struct {
	bodyDecoder bool // to parse `form` or `multipart/form-data`
	reqDecoder  bool // to parse header/cookie/param/query/header/respHeader
}

func compileReqParser(rt reflect.Type, opt bindCompileOption) (Decoder, error) {
	var decoders []decoder

	el := rt.Elem()
	if el.Kind() != reflect.Struct {
		return nil, &UnsupportedBinderError{Type: rt}
	}

	for i := 0; i < el.NumField(); i++ {
		if !el.Field(i).IsExported() {
			// ignore unexported field
			continue
		}

		dec, err := compileFieldDecoder(el.Field(i), i, opt, nil)
		if err != nil {
			return nil, err
		}

		if dec != nil {
			decoders = append(decoders, dec)
		}
	}

	return func(c Ctx, rv reflect.Value) error {
		for _, decoder := range decoders {
			err := decoder.Decode(c, rv)
			if err != nil {
				return err
			}
		}

		return nil
	}, nil
}

type parentDecoder struct {
	tagScope   string
	tagContent string
}

func compileFieldDecoder(field reflect.StructField, index int, opt bindCompileOption, parent *parentDecoder) (decoder, error) {
	// Custom unmarshaler
	if reflect.PointerTo(field.Type).Implements(bindUnmarshalerType) {
		return &fieldCtxDecoder{index: index, fieldName: field.Name, fieldType: field.Type}, nil
	}

	// Validate tag scope
	var tags = []string{bindTagRespHeader, bindTagQuery, bindTagParam, bindTagHeader, bindTagCookie}
	if opt.bodyDecoder {
		tags = []string{bindTagForm, bindTagMultipart}
	}

	var tagScope = ""
	for _, loopTagScope := range tags {
		if _, ok := field.Tag.Lookup(loopTagScope); ok {
			tagScope = loopTagScope
			break
		}
	}

	if tagScope == "" {
		return nil, nil
	}

	// If parent tag scope is present, just override it and append the parent tag content
	var tagContent string
	if parent != nil {
		tagScope = parent.tagScope
		tagContent = parent.tagContent + "." + field.Tag.Get(tagScope)
	} else {
		tagContent = field.Tag.Get(tagScope)
	}

	if reflect.PointerTo(field.Type).Implements(textUnmarshalerType) {
		return compileTextBasedDecoder(field, index, tagScope, tagContent, opt)
	}

	if field.Type.Kind() == reflect.Slice {
		return compileSliceFieldTextBasedDecoder(field, index, tagScope, tagContent)
	}

	return compileTextBasedDecoder(field, index, tagScope, tagContent, opt)
}

func formGetter(ctx Ctx, key string, defaultValue ...string) string {
	return utils.UnsafeString(ctx.Request().PostArgs().Peek(key))
}

func multipartGetter(ctx Ctx, key string, defaultValue ...string) string {
	f, err := ctx.Request().MultipartForm()
	if err != nil {
		return ""
	}

	v, ok := f.Value[key]
	if !ok {
		return ""
	}

	return v[0]
}

func compileTextBasedDecoder(field reflect.StructField, index int, tagScope, tagContent string, opt bindCompileOption) (decoder, error) {
	var get func(ctx Ctx, key string, defaultValue ...string) string
	switch tagScope {
	case bindTagQuery:
		get = Ctx.Query
	case bindTagHeader:
		get = Ctx.Get
	case bindTagRespHeader:
		get = Ctx.GetRespHeader
	case bindTagParam:
		get = Ctx.Params
	case bindTagCookie:
		get = Ctx.Cookies
	case bindTagMultipart:
		get = multipartGetter
	case bindTagForm:
		get = formGetter
	default:
		return nil, errors.New("unexpected tag scope " + strconv.Quote(tagScope))
	}

	fieldDecoder := &fieldTextDecoder{
		index:     index,
		fieldName: field.Name,
		tag:       tagScope,
		reqField:  tagContent,
		get:       get,
	}

	// Support simple embeded structs
	if field.Type.Kind() == reflect.Struct {
		var decoders []decoder
		for i := 0; i < field.Type.NumField(); i++ {
			if !field.Type.Field(i).IsExported() {
				// ignore unexported field
				continue
			}

			dec, err := compileFieldDecoder(field.Type.Field(i), i, opt, &parentDecoder{tagScope: tagScope, tagContent: tagContent})
			if err != nil {
				return nil, err
			}

			decoders = append(decoders, dec)
		}

		fieldDecoder.subFieldDecoders = decoders

		return fieldDecoder, nil
	}

	textDecoder, err := bind.CompileTextDecoder(field.Type)
	if err != nil {
		return nil, err
	}

	fieldDecoder.dec = textDecoder

	return fieldDecoder, nil
}

func compileSliceFieldTextBasedDecoder(field reflect.StructField, index int, tagScope string, tagContent string) (decoder, error) {
	if field.Type.Kind() != reflect.Slice {
		panic("BUG: unexpected type, expecting slice " + field.Type.String())
	}

	et := field.Type.Elem()
	elementUnmarshaler, err := bind.CompileTextDecoder(et)
	if err != nil {
		return nil, fmt.Errorf("failed to build slice binder: %w", err)
	}

	var eqBytes = bytes.Equal
	var visitAll func(Ctx, func(key, value []byte))
	switch tagScope {
	case bindTagQuery:
		visitAll = visitQuery
	case bindTagHeader:
		visitAll = visitHeader
		eqBytes = utils.EqualFold[[]byte]
	case bindTagRespHeader:
		visitAll = visitResHeader
		eqBytes = utils.EqualFold[[]byte]
	case bindTagCookie:
		visitAll = visitCookie
	case bindTagForm:
		visitAll = visitForm
	case bindTagMultipart:
		visitAll = visitMultipart
	case bindTagParam:
		return nil, errors.New("using params with slice type is not supported")
	default:
		return nil, errors.New("unexpected tag scope " + strconv.Quote(tagScope))
	}

	return &fieldSliceDecoder{
		fieldIndex:     index,
		eqBytes:        eqBytes,
		fieldName:      field.Name,
		visitAll:       visitAll,
		reqKey:         []byte(tagContent),
		fieldType:      field.Type,
		elementType:    et,
		elementDecoder: elementUnmarshaler,
	}, nil
}
