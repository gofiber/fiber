package gotiny

import (
	"fmt"
	"reflect"
	"sync"
	"time"
	"unsafe"
)

type decEng func(*Decoder, unsafe.Pointer) // 解码器

var (
	rt2decEng = map[reflect.Type]decEng{
		reflect.TypeOf((*bool)(nil)).Elem():           decBool,
		reflect.TypeOf((*int)(nil)).Elem():            decInt,
		reflect.TypeOf((*int8)(nil)).Elem():           decInt8,
		reflect.TypeOf((*int16)(nil)).Elem():          decInt16,
		reflect.TypeOf((*int32)(nil)).Elem():          decInt32,
		reflect.TypeOf((*int64)(nil)).Elem():          decInt64,
		reflect.TypeOf((*uint)(nil)).Elem():           decUint,
		reflect.TypeOf((*uint8)(nil)).Elem():          decUint8,
		reflect.TypeOf((*uint16)(nil)).Elem():         decUint16,
		reflect.TypeOf((*uint32)(nil)).Elem():         decUint32,
		reflect.TypeOf((*uint64)(nil)).Elem():         decUint64,
		reflect.TypeOf((*uintptr)(nil)).Elem():        decUintptr,
		reflect.TypeOf((*unsafe.Pointer)(nil)).Elem(): decPointer,
		reflect.TypeOf((*float32)(nil)).Elem():        decFloat32,
		reflect.TypeOf((*float64)(nil)).Elem():        decFloat64,
		reflect.TypeOf((*complex64)(nil)).Elem():      decComplex64,
		reflect.TypeOf((*complex128)(nil)).Elem():     decComplex128,
		reflect.TypeOf((*[]byte)(nil)).Elem():         decBytes,
		reflect.TypeOf((*string)(nil)).Elem():         decString,
		reflect.TypeOf((*time.Time)(nil)).Elem():      decTime,
		reflect.TypeOf((*struct{})(nil)).Elem():       decIgnore,
		reflect.TypeOf(nil):                           decIgnore,
	}

	baseDecEngines = []decEng{
		reflect.Invalid:       decIgnore,
		reflect.Bool:          decBool,
		reflect.Int:           decInt,
		reflect.Int8:          decInt8,
		reflect.Int16:         decInt16,
		reflect.Int32:         decInt32,
		reflect.Int64:         decInt64,
		reflect.Uint:          decUint,
		reflect.Uint8:         decUint8,
		reflect.Uint16:        decUint16,
		reflect.Uint32:        decUint32,
		reflect.Uint64:        decUint64,
		reflect.Uintptr:       decUintptr,
		reflect.UnsafePointer: decPointer,
		reflect.Float32:       decFloat32,
		reflect.Float64:       decFloat64,
		reflect.Complex64:     decComplex64,
		reflect.Complex128:    decComplex128,
		reflect.String:        decString,
	}
	decLock sync.RWMutex
)

func getDecEngine(rt reflect.Type) decEng {
	decLock.RLock()
	engine := rt2decEng[rt]
	decLock.RUnlock()
	if engine != nil {
		return engine
	}
	decLock.Lock()
	buildDecEngine(rt, &engine)
	decLock.Unlock()
	return engine
}

func buildDecEngine(rt reflect.Type, engPtr *decEng) {
	engine, has := rt2decEng[rt]
	if has {
		*engPtr = engine
		return
	}

	if _, engine = implementOtherSerializer(rt); engine != nil {
		rt2decEng[rt] = engine
		*engPtr = engine
		return
	}

	kind := rt.Kind()
	var eEng decEng
	switch kind {
	case reflect.Ptr:
		et := rt.Elem()
		defer buildDecEngine(et, &eEng)
		engine = func(d *Decoder, p unsafe.Pointer) {
			if d.decIsNotNil() {
				if isNil(p) {
					*(*unsafe.Pointer)(p) = unsafe.Pointer(reflect.New(et).Elem().UnsafeAddr())
				}
				eEng(d, *(*unsafe.Pointer)(p))
			} else if !isNil(p) {
				*(*unsafe.Pointer)(p) = nil
			}
		}
	case reflect.Array:
		l, et := rt.Len(), rt.Elem()
		size := et.Size()
		defer buildDecEngine(et, &eEng)
		engine = func(d *Decoder, p unsafe.Pointer) {
			for i := 0; i < l; i++ {
				eEng(d, unsafe.Pointer(uintptr(p)+uintptr(i)*size))
			}
		}
	case reflect.Slice:
		et := rt.Elem()
		size := et.Size()
		defer buildDecEngine(et, &eEng)
		engine = func(d *Decoder, p unsafe.Pointer) {
			header := (*reflect.SliceHeader)(p)
			if d.decIsNotNil() {
				l := d.decLength()
				if isNil(p) || header.Cap < l {
					*header = reflect.SliceHeader{Data: reflect.MakeSlice(rt, l, l).Pointer(), Len: l, Cap: l}
				} else {
					header.Len = l
				}
				for i := 0; i < l; i++ {
					eEng(d, unsafe.Pointer(header.Data+uintptr(i)*size))
				}
			} else if !isNil(p) {
				*header = reflect.SliceHeader{}
			}
		}
	case reflect.Map:
		kt, vt := rt.Key(), rt.Elem()
		skt, svt := reflect.SliceOf(kt), reflect.SliceOf(vt)
		var kEng, vEng decEng
		defer buildDecEngine(kt, &kEng)
		defer buildDecEngine(vt, &vEng)
		engine = func(d *Decoder, p unsafe.Pointer) {
			if d.decIsNotNil() {
				l := d.decLength()
				var v reflect.Value
				if isNil(p) {
					v = reflect.MakeMapWithSize(rt, l)
					*(*unsafe.Pointer)(p) = unsafe.Pointer(v.Pointer())
				} else {
					v = reflect.NewAt(rt, p).Elem()
				}
				keys, vals := reflect.MakeSlice(skt, l, l), reflect.MakeSlice(svt, l, l)
				for i := 0; i < l; i++ {
					key, val := keys.Index(i), vals.Index(i)
					kEng(d, unsafe.Pointer(key.UnsafeAddr()))
					vEng(d, unsafe.Pointer(val.UnsafeAddr()))
					v.SetMapIndex(key, val)
				}
			} else if !isNil(p) {
				*(*unsafe.Pointer)(p) = nil
			}
		}
	case reflect.Struct:
		fields, offs := getFieldType(rt, 0)
		nf := len(fields)
		fEngines := make([]decEng, nf)
		defer func() {
			for i := 0; i < nf; i++ {
				buildDecEngine(fields[i], &fEngines[i])
			}
		}()
		engine = func(d *Decoder, p unsafe.Pointer) {
			for i := 0; i < len(fEngines) && i < len(offs); i++ {
				fEngines[i](d, unsafe.Pointer(uintptr(p)+offs[i]))
			}
		}
	case reflect.Interface:
		engine = func(d *Decoder, p unsafe.Pointer) {
			if d.decIsNotNil() {
				name := ""
				decString(d, unsafe.Pointer(&name))
				et, has := name2type[name]
				if !has {
					//panic("unknown typ:" + name)
					fmt.Println("[session] Register this type first with the `RegisterType` method.")
				}
				v := reflect.NewAt(rt, p).Elem()
				var ev reflect.Value
				if v.IsNil() || v.Elem().Type() != et {
					ev = reflect.New(et).Elem()
				} else {
					ev = v.Elem()
				}
				getDecEngine(et)(d, getUnsafePointer(&ev))
				v.Set(ev)
			} else if !isNil(p) {
				*(*unsafe.Pointer)(p) = nil
			}
		}
	case reflect.Chan, reflect.Func:
		//panic("not support " + rt.String() + " type")
	default:
		engine = baseDecEngines[kind]
	}
	rt2decEng[rt] = engine
	*engPtr = engine
}
