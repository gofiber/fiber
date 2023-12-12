package utils

import (
	"testing"
)

func TestIsZeroValue(t *testing.T) {
	var zeroInt int
	var zeroInt8 int8
	var zeroInt16 int16
	var zeroInt32 int32
	var zeroInt64 int64
	var zeroUint uint
	var zeroUint8 uint8
	var zeroUint16 uint16
	var zeroUint32 uint32
	var zeroUint64 uint64
	var zeroUintptr uintptr
	var zeroString string
	var zeroBool bool
	var zeroFloat32 float32
	var zeroFloat64 float64
	var zeroComplex64 complex64
	var zeroComplex128 complex128
	var zeroSliceInt []int
	var zeroSliceInt8 []int8
	var zeroSliceInt16 []int16
	var zeroSliceInt32 []int32
	var zeroSliceInt64 []int64
	var zeroSliceUint []uint
	var zeroSliceUint8 []uint8
	var zeroSliceUint16 []uint16
	var zeroSliceUint32 []uint32
	var zeroSliceUint64 []uint64
	var zeroSliceUintptr []uintptr
	var zeroSliceString []string
	var zeroSliceBool []bool
	var zeroSliceFloat32 []float32
	var zeroSliceFloat64 []float64
	var zeroSliceComplex64 []complex64
	var zeroSliceComplex128 []complex128
	var zeroStruct struct{}
	var zeroPtr *int
	type customInt int
	var zeroCustomInt customInt
	var zeroMap map[string]int
	type customStruct struct {
		A int
	}
	var zeroCustomStruct customStruct

	AssertEqual(t, true, IsZeroValue(nil))
	AssertEqual(t, true, IsZeroValue(zeroInt))
	AssertEqual(t, true, IsZeroValue(zeroInt8))
	AssertEqual(t, true, IsZeroValue(zeroInt16))
	AssertEqual(t, true, IsZeroValue(zeroInt32))
	AssertEqual(t, true, IsZeroValue(zeroInt64))
	AssertEqual(t, true, IsZeroValue(zeroUint))
	AssertEqual(t, true, IsZeroValue(zeroUint8))
	AssertEqual(t, true, IsZeroValue(zeroUint16))
	AssertEqual(t, true, IsZeroValue(zeroUint32))
	AssertEqual(t, true, IsZeroValue(zeroUint64))
	AssertEqual(t, true, IsZeroValue(zeroUintptr))
	AssertEqual(t, true, IsZeroValue(zeroString))
	AssertEqual(t, true, IsZeroValue(zeroBool))
	AssertEqual(t, true, IsZeroValue(zeroFloat32))
	AssertEqual(t, true, IsZeroValue(zeroFloat64))
	AssertEqual(t, true, IsZeroValue(zeroComplex64))
	AssertEqual(t, true, IsZeroValue(zeroComplex128))
	AssertEqual(t, true, IsZeroValue(zeroSliceInt))
	AssertEqual(t, true, IsZeroValue(zeroSliceInt8))
	AssertEqual(t, true, IsZeroValue(zeroSliceInt16))
	AssertEqual(t, true, IsZeroValue(zeroSliceInt32))
	AssertEqual(t, true, IsZeroValue(zeroSliceInt64))
	AssertEqual(t, true, IsZeroValue(zeroSliceUint))
	AssertEqual(t, true, IsZeroValue(zeroSliceUint8))
	AssertEqual(t, true, IsZeroValue(zeroSliceUint16))
	AssertEqual(t, true, IsZeroValue(zeroSliceUint32))
	AssertEqual(t, true, IsZeroValue(zeroSliceUint64))
	AssertEqual(t, true, IsZeroValue(zeroSliceUintptr))
	AssertEqual(t, true, IsZeroValue(zeroSliceString))
	AssertEqual(t, true, IsZeroValue(zeroSliceBool))
	AssertEqual(t, true, IsZeroValue(zeroSliceFloat32))
	AssertEqual(t, true, IsZeroValue(zeroSliceFloat64))
	AssertEqual(t, true, IsZeroValue(zeroSliceComplex64))
	AssertEqual(t, true, IsZeroValue(zeroSliceComplex128))
	AssertEqual(t, true, IsZeroValue(zeroStruct))
	AssertEqual(t, true, IsZeroValue(zeroPtr))
	AssertEqual(t, true, IsZeroValue(zeroCustomInt))
	AssertEqual(t, true, IsZeroValue(zeroMap))
	AssertEqual(t, true, IsZeroValue(zeroCustomStruct))
}

// go test -v -run=^$ -bench=Benchmark_Utils_IsVeroValue -benchmem -count=4
func Benchmark_Utils_IsVeroValue(b *testing.B) {
	var zeroInterface interface{}
	var zeroInt int
	var zeroInt8 int8
	var zeroInt16 int16
	var zeroInt32 int32
	var zeroInt64 int64
	var zeroUint uint
	var zeroUint8 uint8
	var zeroUint16 uint16
	var zeroUint32 uint32
	var zeroUint64 uint64
	var zeroUintptr uintptr
	var zeroString string
	var zeroBool bool
	var zeroFloat32 float32
	var zeroFloat64 float64
	var zeroComplex64 complex64
	var zeroComplex128 complex128
	var zeroSliceInt []int
	var zeroSliceInt8 []int8
	var zeroSliceInt16 []int16
	var zeroSliceInt32 []int32
	var zeroSliceInt64 []int64
	var zeroSliceUint []uint
	var zeroSliceUint8 []uint8
	var zeroSliceUint16 []uint16
	var zeroSliceUint32 []uint32
	var zeroSliceUint64 []uint64
	var zeroSliceUintptr []uintptr
	var zeroSliceFloat32 []float32
	var zeroSliceFloat64 []float64
	var zeroSliceComplex64 []complex64
	var zeroSliceComplex128 []complex128
	var zeroSliceString []string
	var zeroSliceBool []bool
	var zeroStruct struct{}
	var zeroPtr *int
	type customInt int
	var zeroCustomInt customInt
	var zeroMap map[string]int
	type customStruct struct {
		A int
	}
	var zeroCustomStruct customStruct

	testCases := []struct {
		description string
		in          interface{}
		want        bool
	}{
		{"interface", zeroInterface, true},
		{"int", zeroInt, true},
		{"int8", zeroInt8, true},
		{"int16", zeroInt16, true},
		{"int32", zeroInt32, true},
		{"int64", zeroInt64, true},
		{"uint", zeroUint, true},
		{"uint8", zeroUint8, true},
		{"uint16", zeroUint16, true},
		{"uint32", zeroUint32, true},
		{"uint64", zeroUint64, true},
		{"uintptr", zeroUintptr, true},
		{"string", zeroString, true},
		{"bool", zeroBool, true},
		{"float32", zeroFloat32, true},
		{"float64", zeroFloat64, true},
		{"complex64", zeroComplex64, true},
		{"complex128", zeroComplex128, true},
		{"sliceInt", zeroSliceInt, true},
		{"sliceInt8", zeroSliceInt8, true},
		{"sliceInt16", zeroSliceInt16, true},
		{"sliceInt32", zeroSliceInt32, true},
		{"sliceInt64", zeroSliceInt64, true},
		{"sliceUint", zeroSliceUint, true},
		{"sliceUint8", zeroSliceUint8, true},
		{"sliceUint16", zeroSliceUint16, true},
		{"sliceUint32", zeroSliceUint32, true},
		{"sliceUint64", zeroSliceUint64, true},
		{"sliceUintptr", zeroSliceUintptr, true},
		{"sliceFloat32", zeroSliceFloat32, true},
		{"sliceFloat64", zeroSliceFloat64, true},
		{"sliceComplex64", zeroSliceComplex64, true},
		{"sliceComplex128", zeroSliceComplex128, true},
		{"sliceString", zeroSliceString, true},
		{"sliceBool", zeroSliceBool, true},
		{"struct", zeroStruct, true},
		{"ptr", zeroPtr, true},
		{"customInt", zeroCustomInt, true},
		{"map", zeroMap, true},
		{"customStruct", zeroCustomStruct, true},
	}

	for _, tc := range testCases {
		b.Run(tc.description, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				IsZeroValue(tc.in)
			}
		})
	}
}
