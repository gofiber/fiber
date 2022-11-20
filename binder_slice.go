package fiber

import (
	"bytes"
	"reflect"
	"strconv"

	"github.com/gofiber/fiber/v3/internal/bind"
	"github.com/gofiber/utils/v2"
)

var _ decoder = (*fieldSliceDecoder)(nil)

type fieldSliceDecoder struct {
	fieldIndex int
	elems      []subElem
	fieldName  string
	fieldType  reflect.Type
	reqKey     []byte
	// [utils.EqualFold] for headers and [bytes.Equal] for query/params.
	eqBytes        func([]byte, []byte) bool
	elementType    reflect.Type
	elementDecoder bind.TextDecoder
	visitAll       func(Ctx, func(key []byte, value []byte))
}

func (d *fieldSliceDecoder) Decode(ctx Ctx, reqValue reflect.Value) error {
	if d.elementType.Kind() == reflect.Struct {
		return d.decodeStruct(ctx, reqValue)
	}

	count := 0
	d.visitAll(ctx, func(key, value []byte) {
		if d.eqBytes(key, d.reqKey) {
			count++
		}
	})

	rv := reflect.MakeSlice(d.fieldType, 0, count)

	if count == 0 {
		reqValue.Field(d.fieldIndex).Set(rv)
		return nil
	}

	var err error
	d.visitAll(ctx, func(key, value []byte) {
		if err != nil {
			return
		}
		if d.eqBytes(key, d.reqKey) {
			ev := reflect.New(d.elementType)
			if ee := d.elementDecoder.UnmarshalString(utils.UnsafeString(value), ev.Elem()); ee != nil {
				err = ee
			}

			rv = reflect.Append(rv, ev.Elem())
		}
	})

	if err != nil {
		return err
	}

	reqValue.Field(d.fieldIndex).Set(rv)
	return nil
}

func (d *fieldSliceDecoder) decodeStruct(ctx Ctx, reqValue reflect.Value) error {
	var maxNum int
	d.visitAll(ctx, func(key, value []byte) {
		start := bytes.IndexByte(key, byte('['))
		end := bytes.IndexByte(key, byte(']'))

		if start != -1 || end != -1 {
			num := utils.UnsafeString(key[start+1 : end])

			if len(num) > 0 {
				maxNum, _ = strconv.Atoi(num)
			}
		}
	})

	if maxNum != 0 {
		maxNum += 1
	}

	rv := reflect.MakeSlice(d.fieldType, maxNum, maxNum)
	if maxNum == 0 {
		reqValue.Field(d.fieldIndex).Set(rv)
		return nil
	}

	var err error
	d.visitAll(ctx, func(key, value []byte) {
		if err != nil {
			return
		}

		if bytes.IndexByte(key, byte('[')) == -1 {
			return
		}

		// TODO: support queries like data[0][users][0][name]
		ints := make([]int, 0)
		elems := make([]string, 0)

		// nested
		lookupKey := key
		for {
			start := bytes.IndexByte(lookupKey, byte('['))
			end := bytes.IndexByte(lookupKey, byte(']'))

			if start == -1 || end == -1 {
				break
			}

			content := utils.UnsafeString(lookupKey[start+1 : end])
			num, errElse := strconv.Atoi(content)

			if errElse == nil {
				ints = append(ints, num)
			} else {
				elems = append(elems, content)
			}

			lookupKey = lookupKey[end+1:]
		}

		for _, elem := range d.elems {
			if elems[0] == elem.tag {
				ev := reflect.New(elem.et)
				if ee := elem.elementDecoder.UnmarshalString(utils.UnsafeString(value), ev.Elem()); ee != nil {
					err = ee
				}

				i := rv.Index(ints[0])
				i.Field(elem.index).Set(ev.Elem())
			}
		}
	})

	if err != nil {
		return err
	}

	reqValue.Field(d.fieldIndex).Set(rv)
	return nil
}

func visitQuery(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Context().QueryArgs().VisitAll(f)
}

func visitHeader(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Request().Header.VisitAll(f)
}

func visitResHeader(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Response().Header.VisitAll(f)
}

func visitCookie(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Request().Header.VisitAllCookie(f)
}

func visitForm(ctx Ctx, f func(key []byte, value []byte)) {
	ctx.Request().PostArgs().VisitAll(f)
}

func visitMultipart(ctx Ctx, f func(key []byte, value []byte)) {
	mp, err := ctx.Request().MultipartForm()
	if err != nil {
		return
	}

	for key, values := range mp.Value {
		for _, value := range values {
			f(utils.UnsafeBytes(key), utils.UnsafeBytes(value))
		}
	}
}
