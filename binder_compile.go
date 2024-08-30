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

		dec, err := compileFieldDecoder(el.Field(i), i, opt, parentStruct{})
		if err != nil {
			return nil, err
		}

		if dec != nil {
			decoders = append(decoders, dec...)
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

type parentStruct struct {
	tag   string
	index []int
}

func lookupTagScope(field reflect.StructField, opt bindCompileOption) (tagScope string) {
	var tags = []string{bindTagRespHeader, bindTagQuery, bindTagParam, bindTagHeader, bindTagCookie}
	if opt.bodyDecoder {
		tags = []string{bindTagForm, bindTagMultipart}
	}

	for _, loopTagScope := range tags {
		if _, ok := field.Tag.Lookup(loopTagScope); ok {
			tagScope = loopTagScope
			break
		}
	}

	return
}

func compileFieldDecoder(field reflect.StructField, index int, opt bindCompileOption, parent parentStruct) ([]decoder, error) {
	if reflect.PtrTo(field.Type).Implements(bindUnmarshalerType) {
		return []decoder{&fieldCtxDecoder{index: index, fieldName: field.Name, fieldType: field.Type}}, nil
	}

	tagScope := lookupTagScope(field, opt)
	if tagScope == "" {
		return nil, nil
	}

	tagContent := field.Tag.Get(tagScope)

	if parent.tag != "" {
		tagContent = parent.tag + "." + tagContent
	}

	if reflect.PtrTo(field.Type).Implements(textUnmarshalerType) {
		return compileTextBasedDecoder(field, index, tagScope, tagContent)
	}

	if field.Type.Kind() == reflect.Slice {
		return compileSliceFieldTextBasedDecoder(field, index, tagScope, tagContent)
	}

	// Nested binding support
	if field.Type.Kind() == reflect.Struct {
		var decoders []decoder
		el := field.Type

		for i := 0; i < el.NumField(); i++ {
			if !el.Field(i).IsExported() {
				// ignore unexported field
				continue
			}
			var indexes []int
			if len(parent.index) > 0 {
				indexes = append(indexes, parent.index...)
			}
			indexes = append(indexes, index)
			dec, err := compileFieldDecoder(el.Field(i), i, opt, parentStruct{
				tag:   tagContent,
				index: indexes,
			})
			if err != nil {
				return nil, err
			}

			if dec != nil {
				decoders = append(decoders, dec...)
			}
		}

		return decoders, nil
	}

	return compileTextBasedDecoder(field, index, tagScope, tagContent, parent.index)
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

func compileTextBasedDecoder(field reflect.StructField, index int, tagScope, tagContent string, parentIndex ...[]int) ([]decoder, error) {
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

	textDecoder, err := bind.CompileTextDecoder(field.Type)
	if err != nil {
		return nil, err
	}

	fieldDecoder := &fieldTextDecoder{
		index:     index,
		fieldName: field.Name,
		tag:       tagScope,
		reqField:  tagContent,
		dec:       textDecoder,
		get:       get,
	}

	if len(parentIndex) > 0 {
		fieldDecoder.parentIndex = parentIndex[0]
	}

	return []decoder{fieldDecoder}, nil
}

type subElem struct {
	et             reflect.Type
	tag            string
	index          int
	elementDecoder bind.TextDecoder
}

func compileSliceFieldTextBasedDecoder(field reflect.StructField, index int, tagScope string, tagContent string) ([]decoder, error) {
	if field.Type.Kind() != reflect.Slice {
		panic("BUG: unexpected type, expecting slice " + field.Type.String())
	}

	var elems []subElem
	var elementUnmarshaler bind.TextDecoder
	var err error

	et := field.Type.Elem()
	if et.Kind() == reflect.Struct {
		elems = make([]subElem, et.NumField())
		for i := 0; i < et.NumField(); i++ {
			if !et.Field(i).IsExported() {
				// ignore unexported field
				continue
			}

			// Skip different tag scopes (main -> sub)
			subScope := lookupTagScope(et.Field(i), bindCompileOption{})
			if subScope != tagScope {
				continue
			}

			elementUnmarshaler, err := bind.CompileTextDecoder(et.Field(i).Type)
			if err != nil {
				return nil, fmt.Errorf("failed to build slice binder: %w", err)
			}

			elem := subElem{
				index:          i,
				tag:            et.Field(i).Tag.Get(subScope),
				et:             et.Field(i).Type,
				elementDecoder: elementUnmarshaler,
			}

			elems = append(elems, elem)
		}
	} else {
		elementUnmarshaler, err = bind.CompileTextDecoder(et)
		if err != nil {
			return nil, fmt.Errorf("failed to build slice binder: %w", err)
		}
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

	fieldSliceDecoder := &fieldSliceDecoder{
		elems:          elems,
		fieldIndex:     index,
		eqBytes:        eqBytes,
		fieldName:      field.Name,
		visitAll:       visitAll,
		reqKey:         []byte(tagContent),
		fieldType:      field.Type,
		elementType:    et,
		elementDecoder: elementUnmarshaler,
	}

	return []decoder{fieldSliceDecoder}, nil
}
