package gotiny

import (
	"encoding"
	"encoding/gob"
	"reflect"
	"strings"
	"unsafe"
)

const (
	ptr1Size = 4 << (^uintptr(0) >> 63) // unsafe.Sizeof(uintptr(0)) but an ideal const
)

func float64ToUint64(v unsafe.Pointer) uint64 {
	return reverse64Byte(*(*uint64)(v))
}

func uint64ToFloat64(u uint64) float64 {
	u = reverse64Byte(u)
	return *((*float64)(unsafe.Pointer(&u)))
}

func reverse64Byte(u uint64) uint64 {
	u = (u << 32) | (u >> 32)
	u = ((u << 16) & 0xFFFF0000FFFF0000) | ((u >> 16) & 0xFFFF0000FFFF)
	u = ((u << 8) & 0xFF00FF00FF00FF00) | ((u >> 8) & 0xFF00FF00FF00FF)
	return u
}

func float32ToUint32(v unsafe.Pointer) uint32 {
	return reverse32Byte(*(*uint32)(v))
}

func uint32ToFloat32(u uint32) float32 {
	u = reverse32Byte(u)
	return *((*float32)(unsafe.Pointer(&u)))
}

func reverse32Byte(u uint32) uint32 {
	u = (u << 16) | (u >> 16)
	return ((u << 8) & 0xFF00FF00) | ((u >> 8) & 0xFF00FF)
}

// int -5 -4 -3 -2 -1 0 1 2 3 4 5  6
// uint 9  7  5  3  1 0 2 4 6 8 10 12
func int64ToUint64(v int64) uint64 {
	return uint64((v << 1) ^ (v >> 63))
}

// uint 9  7  5  3  1 0 2 4 6 8 10 12
// int -5 -4 -3 -2 -1 0 1 2 3 4 5  6
func uint64ToInt64(u uint64) int64 {
	v := int64(u)
	return (-(v & 1)) ^ (v>>1)&0x7FFFFFFFFFFFFFFF
}

// int -5 -4 -3 -2 -1 0 1 2 3 4 5  6
// uint 9  7  5  3  1 0 2 4 6 8 10 12
func int32ToUint32(v int32) uint32 {
	return uint32((v << 1) ^ (v >> 31))
}

// uint 9  7  5  3  1 0 2 4 6 8 10 12
// int -5 -4 -3 -2 -1 0 1 2 3 4 5  6
func uint32ToInt32(u uint32) int32 {
	v := int32(u)
	return (-(v & 1)) ^ (v>>1)&0x7FFFFFFF
}

// int -5 -4 -3 -2 -1 0 1 2 3 4 5  6
// uint 9  7  5  3  1 0 2 4 6 8 10 12
func int16ToUint16(v int16) uint16 {
	return uint16((v << 1) ^ (v >> 15))
}

// uint 9  7  5  3  1 0 2 4 6 8 10 12
// int -5 -4 -3 -2 -1 0 1 2 3 4 5  6
func uint16ToInt16(u uint16) int16 {
	v := int16(u)
	return (-(v & 1)) ^ (v>>1)&0x7FFF
}

func isNil(p unsafe.Pointer) bool {
	return *(*unsafe.Pointer)(p) == nil
}

type gobInter interface {
	gob.GobEncoder
	gob.GobDecoder
}

type binInter interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// 只应该由指针来实现该接口
type GoTinySerializer interface {
	// 编码方法，将对象的序列化结果append到入参数并返回，方法不应该修改入参数值原有的值
	GotinyEncode([]byte) []byte
	// 解码方法，将入参解码到对象里并返回使用的长度。方法从入参的第0个字节开始使用，并且不应该修改入参中的任何数据
	GotinyDecode([]byte) int
}

func implementOtherSerializer(rt reflect.Type) (encEng encEng, decEng decEng) {
	rtNil := reflect.Zero(reflect.PtrTo(rt)).Interface()
	if _, ok := rtNil.(GoTinySerializer); ok {
		encEng = func(e *Encoder, p unsafe.Pointer) {
			e.buf = reflect.NewAt(rt, p).Interface().(GoTinySerializer).GotinyEncode(e.buf)
		}
		decEng = func(d *Decoder, p unsafe.Pointer) {
			d.index += reflect.NewAt(rt, p).Interface().(GoTinySerializer).GotinyDecode(d.buf[d.index:])
		}
		return
	}

	if _, ok := rtNil.(binInter); ok {
		encEng = func(e *Encoder, p unsafe.Pointer) {
			buf, err := reflect.NewAt(rt, p).Interface().(encoding.BinaryMarshaler).MarshalBinary()
			if err != nil {
				panic(err)
			}
			e.encLength(len(buf))
			e.buf = append(e.buf, buf...)
		}

		decEng = func(d *Decoder, p unsafe.Pointer) {
			length := d.decLength()
			start := d.index
			d.index += length
			if err := reflect.NewAt(rt, p).Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(d.buf[start:d.index]); err != nil {
				panic(err)
			}
		}
		return
	}

	if _, ok := rtNil.(gobInter); ok {
		encEng = func(e *Encoder, p unsafe.Pointer) {
			buf, err := reflect.NewAt(rt, p).Interface().(gob.GobEncoder).GobEncode()
			if err != nil {
				panic(err)
			}
			e.encLength(len(buf))
			e.buf = append(e.buf, buf...)
		}
		decEng = func(d *Decoder, p unsafe.Pointer) {
			length := d.decLength()
			start := d.index
			d.index += length
			if err := reflect.NewAt(rt, p).Interface().(gob.GobDecoder).GobDecode(d.buf[start:d.index]); err != nil {
				panic(err)
			}
		}
	}
	return
}

// rt.kind is reflect.struct
func getFieldType(rt reflect.Type, baseOff uintptr) (fields []reflect.Type, offs []uintptr) {
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if ignoreField(field) {
			continue
		}
		ft := field.Type
		if ft.Kind() == reflect.Struct {
			if _, engine := implementOtherSerializer(ft); engine == nil {
				fFields, fOffs := getFieldType(ft, field.Offset+baseOff)
				fields = append(fields, fFields...)
				offs = append(offs, fOffs...)
				continue
			}
		}
		fields = append(fields, ft)
		offs = append(offs, field.Offset+baseOff)
	}
	return
}

func ignoreField(field reflect.StructField) bool {
	tinyTag, ok := field.Tag.Lookup("gotiny")
	return ok && strings.TrimSpace(tinyTag) == "-"
}
