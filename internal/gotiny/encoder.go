package gotiny

import (
	"reflect"
	"unsafe"
)

type Encoder struct {
	buf     []byte //编码目的数组
	off     int
	boolPos int  //下一次要设置的bool在buf中的下标,即buf[boolPos]
	boolBit byte //下一次要设置的bool的buf[boolPos]中的bit位

	engines []encEng
	length  int
}

func Marshal(is ...interface{}) []byte {
	return NewEncoderWithPtr(is...).Encode(is...)
}

// 创建一个编码ps 指向类型的编码器
func NewEncoderWithPtr(ps ...interface{}) *Encoder {
	l := len(ps)
	engines := make([]encEng, l)
	for i := 0; i < l; i++ {
		rt := reflect.TypeOf(ps[i])
		if rt.Kind() != reflect.Ptr {
			panic("must a pointer type!")
		}
		engines[i] = getEncEngine(rt.Elem())
	}
	return &Encoder{
		length:  l,
		engines: engines,
	}
}

// 创建一个编码is 类型的编码器
func NewEncoder(is ...interface{}) *Encoder {
	l := len(is)
	engines := make([]encEng, l)
	for i := 0; i < l; i++ {
		engines[i] = getEncEngine(reflect.TypeOf(is[i]))
	}
	return &Encoder{
		length:  l,
		engines: engines,
	}
}

func NewEncoderWithType(ts ...reflect.Type) *Encoder {
	l := len(ts)
	engines := make([]encEng, l)
	for i := 0; i < l; i++ {
		engines[i] = getEncEngine(ts[i])
	}
	return &Encoder{
		length:  l,
		engines: engines,
	}
}

// 入参是要编码值的指针
func (e *Encoder) Encode(is ...interface{}) []byte {
	engines := e.engines
	for i := 0; i < len(engines) && i < len(is); i++ {
		engines[i](e, (*[2]unsafe.Pointer)(unsafe.Pointer(&is[i]))[1])
	}
	return e.reset()
}

// 入参是要编码的值得unsafe.Pointer 指针
func (e *Encoder) EncodePtr(ps ...unsafe.Pointer) []byte {
	engines := e.engines
	for i := 0; i < len(engines) && i < len(ps); i++ {
		engines[i](e, ps[i])
	}
	return e.reset()
}

// vs 是持有要编码的值
func (e *Encoder) EncodeValue(vs ...reflect.Value) []byte {
	engines := e.engines
	for i := 0; i < len(engines) && i < len(vs); i++ {
		engines[i](e, getUnsafePointer(&vs[i]))
	}
	return e.reset()
}

// 编码产生的数据将append到buf上
func (e *Encoder) AppendTo(buf []byte) {
	e.off = len(buf)
	e.buf = buf
}

func (e *Encoder) reset() []byte {
	buf := e.buf
	e.buf = buf[:e.off]
	e.boolBit = 0
	e.boolPos = 0
	return buf
}
