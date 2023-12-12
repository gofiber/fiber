package utils

import (
	"errors"
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
	var zeroError error
	var zeroCustomSlice []customInt
	var zeroCustomMap map[customInt]customInt

	nonZeroInt := 1
	nonZeroInt8 := int8(1)
	nonZeroInt16 := int16(1)
	nonZeroInt32 := int32(1)
	nonZeroInt64 := int64(1)
	nonZeroUint := uint(1)
	nonZeroUint8 := uint8(1)
	nonZeroUint16 := uint16(1)
	nonZeroUint32 := uint32(1)
	nonZeroUint64 := uint64(1)
	nonZeroUintptr := uintptr(1)
	nonZeroString := "1"
	nonZeroBool := true
	nonZeroFloat32 := float32(1)
	nonZeroFloat64 := float64(1)
	nonZeroComplex64 := complex64(1 + 0i)
	nonZeroComplex128 := complex128(1 + 0i)
	nonZeroSliceInt := []int{1}
	nonZeroSliceInt8 := []int8{1}
	nonZeroSliceInt16 := []int16{1}
	nonZeroSliceInt32 := []int32{1}
	nonZeroSliceInt64 := []int64{1}
	nonZeroSliceUint := []uint{1}
	nonZeroSliceUint8 := []uint8{1}
	nonZeroSliceUint16 := []uint16{1}
	nonZeroSliceUint32 := []uint32{1}
	nonZeroSliceUint64 := []uint64{1}
	nonZeroSliceUintptr := []uintptr{1}
	nonZeroSliceString := []string{"1"}
	nonZeroSliceBool := []bool{true}
	nonZeroSliceFloat32 := []float32{1}
	nonZeroSliceFloat64 := []float64{1}
	nonZeroSliceComplex64 := []complex64{1 + 0i}
	nonZeroSliceComplex128 := []complex128{1 + 0i}
	nonZeroStruct := struct{ A int }{A: 1}
	nonZeroPtr := &nonZeroInt
	nonZeroCustomInt := customInt(1)
	nonZeroMap := map[string]int{"1": 1}
	nonZeroCustomStruct := customStruct{A: 1}
	nonZeroError := errors.New("1")
	nonZeroCustomSlice := []customInt{1}
	nonZeroCustomMap := map[customInt]customInt{1: 1}

	testCases := []struct {
		description string
		in          interface{}
		want        bool
	}{
		// Zero values, var not initialized
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
		{"error", zeroError, true},
		{"customSlice", zeroCustomSlice, true},
		{"customMap", zeroCustomMap, true},
		// Zero values, var initialized
		{"nil", nil, true},
		{"zeroInt", 0, true},
		{"zeroInt8", int8(0), true},
		{"zeroInt16", int16(0), true},
		{"zeroInt32", int32(0), true},
		{"zeroInt64", int64(0), true},
		{"zeroUint", uint(0), true},
		{"zeroUint8", uint8(0), true},
		{"zeroUint16", uint16(0), true},
		{"zeroUint32", uint32(0), true},
		{"zeroUint64", uint64(0), true},
		{"zeroUintptr", uintptr(0), true},
		{"zeroString", "", true},
		{"zeroBool", false, true},
		{"zeroFloat32", float32(0), true},
		{"zeroFloat64", float64(0), true},
		{"zeroComplex64", complex64(0 + 0i), true},
		{"zeroComplex128", complex128(0 + 0i), true},
		{"zeroSliceInt", []int{}, true},
		{"zeroSliceInt8", []int8{}, true},
		{"zeroSliceInt16", []int16{}, true},
		{"zeroSliceInt32", []int32{}, true},
		{"zeroSliceInt64", []int64{}, true},
		{"zeroSliceUint", []uint{}, true},
		{"zeroSliceUint8", []uint8{}, true},
		{"zeroSliceUint16", []uint16{}, true},
		{"zeroSliceUint32", []uint32{}, true},
		{"zeroSliceUint64", []uint64{}, true},
		{"zeroSliceUintptr", []uintptr{}, true},
		{"zeroSliceFloat32", []float32{}, true},
		{"zeroSliceFloat64", []float64{}, true},
		{"zeroSliceComplex64", []complex64{}, true},
		{"zeroSliceComplex128", []complex128{}, true},
		{"zeroSliceString", []string{}, true},
		{"zeroSliceBool", []bool{}, true},
		{"zeroStruct", struct{}{}, true},
		{"zeroPtr", (*int)(nil), true},
		{"zeroCustomInt", customInt(0), true},
		{"zeroMap", map[string]int{}, true},
		{"zeroCustomStruct", customStruct{}, true},
		{"zeroError", error(nil), true},
		{"zeroCustomSlice", []customInt{}, true},
		{"zeroCustomMap", map[customInt]customInt{}, true},
		// Not zero values
		{"nonZeroInt", nonZeroInt, false},
		{"nonZeroInt8", nonZeroInt8, false},
		{"nonZeroInt16", nonZeroInt16, false},
		{"nonZeroInt32", nonZeroInt32, false},
		{"nonZeroInt64", nonZeroInt64, false},
		{"nonZeroUint", nonZeroUint, false},
		{"nonZeroUint8", nonZeroUint8, false},
		{"nonZeroUint16", nonZeroUint16, false},
		{"nonZeroUint32", nonZeroUint32, false},
		{"nonZeroUint64", nonZeroUint64, false},
		{"nonZeroUintptr", nonZeroUintptr, false},
		{"nonZeroString", nonZeroString, false},
		{"nonZeroBool", nonZeroBool, false},
		{"nonZeroFloat32", nonZeroFloat32, false},
		{"nonZeroFloat64", nonZeroFloat64, false},
		{"nonZeroComplex64", nonZeroComplex64, false},
		{"nonZeroComplex128", nonZeroComplex128, false},
		{"nonZeroSliceInt", nonZeroSliceInt, false},
		{"nonZeroSliceInt8", nonZeroSliceInt8, false},
		{"nonZeroSliceInt16", nonZeroSliceInt16, false},
		{"nonZeroSliceInt32", nonZeroSliceInt32, false},
		{"nonZeroSliceInt64", nonZeroSliceInt64, false},
		{"nonZeroSliceUint", nonZeroSliceUint, false},
		{"nonZeroSliceUint8", nonZeroSliceUint8, false},
		{"nonZeroSliceUint16", nonZeroSliceUint16, false},
		{"nonZeroSliceUint32", nonZeroSliceUint32, false},
		{"nonZeroSliceUint64", nonZeroSliceUint64, false},
		{"nonZeroSliceUintptr", nonZeroSliceUintptr, false},
		{"nonZeroSliceString", nonZeroSliceString, false},
		{"nonZeroSliceBool", nonZeroSliceBool, false},
		{"nonZeroSliceFloat32", nonZeroSliceFloat32, false},
		{"nonZeroSliceFloat64", nonZeroSliceFloat64, false},
		{"nonZeroSliceComplex64", nonZeroSliceComplex64, false},
		{"nonZeroSliceComplex128", nonZeroSliceComplex128, false},
		{"nonZeroStruct", nonZeroStruct, false},
		{"nonZeroPtr", nonZeroPtr, false},
		{"nonZeroCustomInt", nonZeroCustomInt, false},
		{"nonZeroMap", nonZeroMap, false},
		{"nonZeroCustomStruct", nonZeroCustomStruct, false},
		{"nonZeroError", nonZeroError, false},
		{"nonZeroCustomSlice", nonZeroCustomSlice, false},
		{"nonZeroCustomMap", nonZeroCustomMap, false},
		// Initialized Zero value custom slice and map
		{"zeroCustomSlice", []customInt{}, true},
		{"zeroCustomMap", map[customInt]customInt{}, true},
		// Initialized Not Zero value custom slice and map
		{"notZeroCustomSlice", []customInt{1}, false},
		{"notZeroCustomMap", map[customInt]customInt{1: 1}, false},
		// Initialized Zero value custom slice and map pointers
		{"zeroPtrCustomSlice", &[]customInt{}, true},
		{"zeroPtrCustomMap", &map[customInt]customInt{}, true},
		// Initialized Not Zero value custom slice and map pointers
		{"notZeroPtrCustomSlice", &[]customInt{1}, false},
		{"notZeroPtrCustomMap", &map[customInt]customInt{1: 1}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			AssertEqual(t, IsZeroValue(tc.in), tc.want, tc.description)
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Utils_IsVeroValue -benchmem -count=4
func Benchmark_Utils_IsVeroValue(b *testing.B) {
	type customInt int
	type customStruct struct {
		A int
	}
	type customSlice []customInt
	type customMap map[customInt]customInt
	testCases := []struct {
		description string
		in          interface{}
		want        bool
	}{
		{"nil", nil, true},
		{"basic(int)", 0, true},
		{"slice", []int{}, true},
		{"map", map[string]int{}, true},
		{"struct", struct{}{}, true},
		{"ptr", (*int)(nil), true},
		{"custom type", customInt(0), true},
		{"custom struct", customStruct{}, true},
		{"custom slice", customSlice{}, true},
		{"custom map", customMap{}, true},
		{"custom ptr slice", &customSlice{}, true},
		{"custom ptr map", &customMap{}, true},
		{"custom ptr slice with value", &customSlice{1}, false},
		{"custom ptr map with value", &customMap{1: 1}, false},
	}

	for _, tc := range testCases {
		b.Run(tc.description, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				IsZeroValue(tc.in)
			}
		})
	}
}
