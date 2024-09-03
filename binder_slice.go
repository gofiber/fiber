package fiber

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3/internal/bind"
	"github.com/gofiber/utils/v2"
)

var _ decoder = (*fieldSliceDecoder)(nil)

type fieldSliceDecoder struct {
	fieldIndex int
	fieldName  string
	fieldType  reflect.Type
	reqKey     []byte
	// [utils.EqualFold] for headers and [bytes.Equal] for query/params.
	eqBytes          func([]byte, []byte) bool
	elementType      reflect.Type
	elementDecoder   bind.TextDecoder
	visitAll         func(Ctx, func(key []byte, value []byte))
	subFieldDecoders []decoder
	fragments        []requestKeyFragment
}

func (d *fieldSliceDecoder) Decode(ctx Ctx, reqValue reflect.Value) error {
	if len(d.subFieldDecoders) > 0 {
		rv, err := d.decodeSubFields(ctx, reqValue)
		if err != nil {
			return err
		}

		reqValue.Field(d.fieldIndex).Set(rv)
		return nil
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

func (d *fieldSliceDecoder) decodeSubFields(ctx Ctx, reqValue reflect.Value) (reflect.Value, error) {
	rv := reflect.New(d.fieldType).Elem()

	// reqValue => ana struct
	for _, subFieldDecoder := range d.subFieldDecoders {
		if subFieldDecoder.Kind() == "text" {
			textDec, ok := subFieldDecoder.(*fieldTextDecoder)
			if !ok {
				continue
			}

			test := make(map[string]int)

			count := 0
			maxIndex := 0
			d.visitAll(ctx, func(key, value []byte) {
				var num int
				if !bytes.Contains(key, []byte(".")) {
					return
				}

				frag := prepareFragments(utils.UnsafeString(key))

				if textDec.subFieldDecoders == nil && len(frag) != len(textDec.fragments) {
					return
				}

				if textDec.subFieldDecoders != nil && len(frag) > len(textDec.fragments) {

				}

				for i, f := range frag {
					if textDec.fragments[i].isNum && f.isNum {
						if f.num > maxIndex {
							maxIndex = f.num
						}
						num = f.num
					} else if textDec.fragments[i].key != f.key {
						return
					}
				}
				count++
				test[utils.UnsafeString(key)] = num
			})

			if count == 0 {
				reqValue.Field(d.fieldIndex).Set(reflect.MakeSlice(d.fieldType, 0, 0))
				continue
			}

			if rv.Len() < maxIndex+1 {
				rv = reflect.MakeSlice(d.fieldType, maxIndex+1, maxIndex+1)
			}

			d.visitAll(ctx, func(key, value []byte) {
				if index, ok := test[utils.UnsafeString(key)]; ok {
					textDec.dec.UnmarshalString(utils.UnsafeString(value), rv.Index(index).Field(textDec.fieldIndex))
				}
			})
		} else {
			sliceDec, ok := subFieldDecoder.(*fieldSliceDecoder)
			if !ok {
				continue
			}

			var count int
			var maxIndex int

			d.visitAll(ctx, func(key, value []byte) {
				if !bytes.Contains(key, []byte(".")) {
					return
				}

				frag := prepareFragments(utils.UnsafeString(key))

				if len(frag) < len(sliceDec.fragments)+1 {
					return
				}
				for i := 0; i < len(sliceDec.fragments)+1; i++ {
					if i == len(sliceDec.fragments) && frag[i].isNum {
						count++
						if frag[i].num > maxIndex {
							maxIndex = frag[i].num
						}
						continue
					}

					if frag[i].key != sliceDec.fragments[i].key && !frag[i].isNum {
						return
					}
				}
			})
			//sliceDec.decodeSubFields(ctx, rv)
		}
	}

	return rv, nil
}

func prepareFragments(key string) []requestKeyFragment {
	split := strings.Split(key, ".")
	fragments := make([]requestKeyFragment, 0, len(split))
	for _, fragment := range split {
		num, err := strconv.Atoi(fragment)
		fragments = append(fragments, requestKeyFragment{
			key:   fragment,
			num:   num,
			isNum: err == nil,
		})
	}

	return fragments
}

func (d *fieldSliceDecoder) Kind() string {
	return "slice"
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
