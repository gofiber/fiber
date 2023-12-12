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
	var zeroError error
	var zeroCustomSlice []customInt
	var zeroCustomMap map[customInt]customInt

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
		{"zeroIntInited", 0, true},
		{"zeroInt8Inited", int8(0), true},
		{"zeroInt16Inited", int16(0), true},
		{"zeroInt32Inited", int32(0), true},
		{"zeroInt64Inited", int64(0), true},
		{"zeroUintInited", uint(0), true},
		{"zeroUint8Inited", uint8(0), true},
		{"zeroUint16Inited", uint16(0), true},
		{"zeroUint32Inited", uint32(0), true},
		{"zeroUint64Inited", uint64(0), true},
		{"zeroUintptrInited", uintptr(0), true},
		{"zeroStringInited", "", true},
		{"zeroBoolInited", false, true},
		{"zeroFloat32Inited", float32(0), true},
		{"zeroFloat64Inited", float64(0), true},
		{"zeroComplex64Inited", complex64(0 + 0i), true},
		{"zeroComplex128Inited", complex128(0 + 0i), true},
		{"zeroStructInited", struct{}{}, true},
		{"zeroPtrInited", (*int)(nil), true},
		{"zeroCustomIntInited", customInt(0), true},
		{"emptyMap", map[string]int{}, false},
		{"emptyCustomStruct", customStruct{}, true},
		{"zeroErrorInited", error(nil), true},
		// Not zero values
		{"notZeroInt", 1, false},
		{"notZeroInt8", int8(1), false},
		{"notZeroInt16", int16(1), false},
		{"notZeroInt32", int32(1), false},
		{"notZeroInt64", int64(1), false},
		{"notZeroUint", uint(1), false},
		{"notZeroUint8", uint8(1), false},
		{"notZeroUint16", uint16(1), false},
		{"notZeroUint32", uint32(1), false},
		{"notZeroUint64", uint64(1), false},
		{"notZeroUintptr", uintptr(1), false},
		{"notZeroString", "1", false},
		{"notZeroBool", true, false},
		{"notZeroFloat32", float32(1), false},
		{"notZeroFloat64", float64(1), false},
		{"notZeroComplex64", complex64(1 + 0i), false},
		{"notZeroComplex128", complex128(1 + 0i), false},
		{"notZeroSliceBool", []bool{true}, false},
		{"notZeroSliceInt", []int{1}, false},
		{"notZeroSliceInt8", []int8{1}, false},
		{"notZeroSliceInt16", []int16{1}, false},
		{"notZeroSliceInt32", []int32{1}, false},
		{"notZeroSliceInt64", []int64{1}, false},
		{"notZeroSliceUint", []uint{1}, false},
		{"notZeroSliceUint8", []uint8{1}, false},
		{"notZeroSliceUint16", []uint16{1}, false},
		{"notZeroSliceUint32", []uint32{1}, false},
		{"notZeroSliceUint64", []uint64{1}, false},
		{"notZeroSliceUintptr", []uintptr{1}, false},
		{"notZeroSliceFloat32", []float32{1}, false},
		{"notZeroSliceFloat64", []float64{1}, false},
		{"notZeroSliceComplex64", []complex64{1 + 0i}, false},
		{"notZeroSliceComplex128", []complex128{1 + 0i}, false},
		{"notZeroSliceString", []string{"1"}, false},
		{"notZeroSliceStruct", []struct{}{{}}, false},
		{"notZeroSlicePtr", []*int{&zeroInt}, false},
		{"notZeroSliceCustomInt", []customInt{1}, false},
		{"notZeroSliceMap", []map[string]int{{"1": 1}}, false},
		{"notZeroSliceCustomStruct", []customStruct{{1}}, false},
		{"notZeroSliceError", []error{nil}, false},
		{"notZeroSliceCustomSlice", [][]customInt{{1}}, false},
		{"notZeroSliceCustomMap", []map[customInt]customInt{{1: 1}}, false},
		// Empty slices
		{"emptySliceBool", []bool{}, false},
		{"emptySliceInt", []int{}, false},
		{"emptySliceInt8", []int8{}, false},
		{"emptySliceInt16", []int16{}, false},
		{"emptySliceInt32", []int32{}, false},
		{"emptySliceInt64", []int64{}, false},
		{"emptySliceUint", []uint{}, false},
		{"emptySliceUint8", []uint8{}, false},
		{"emptySliceUint16", []uint16{}, false},
		{"emptySliceUint32", []uint32{}, false},
		{"emptySliceUint64", []uint64{}, false},
		{"emptySliceUintptr", []uintptr{}, false},
		{"emptySliceFloat32", []float32{}, false},
		{"emptySliceFloat64", []float64{}, false},
		{"emptySliceComplex64", []complex64{}, false},
		{"emptySliceComplex128", []complex128{}, false},
		{"emptySliceString", []string{}, false},
		{"emptySliceStruct", []struct{}{}, false},
		{"emptySlicePtr", []*int{}, false},
		{"emptySliceCustomInt", []customInt{}, false},
		{"emptySliceMap", []map[string]int{}, false},
		{"emptySliceCustomStruct", []customStruct{}, false},
		{"emptySliceError", []error{}, false},
		{"emptySliceCustomSlice", [][]customInt{}, false},
		{"emptySliceCustomMap", []map[customInt]customInt{}, false},
		// Empty maps
		{"emptyMap", map[string]int{}, false},
		{"emptyCustomMap", map[customInt]customInt{}, false},
		// Not empty maps
		{"notEmptyMap", map[string]int{"1": 1}, false},
		{"notEmptyCustomMap", map[customInt]customInt{1: 1}, false},
		// Empty structs
		{"emptyStruct", struct{}{}, true},
		{"emptyCustomStruct", customStruct{}, true},
		// Not empty structs
		{"notEmptyStruct", struct{ A int }{1}, false},
		{"notEmptyCustomStruct", customStruct{1}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			AssertEqual(t, IsZeroValue(tc.in), tc.want, tc.description)
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Utils_IsVeroValue -benchmem -count=4
func Benchmark_Utils_IsVeroValue(b *testing.B) {
	var zeroSlice []int
	var zeroMap map[string]int
	var zeroStruct struct{}

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
		{"zeroSlice", zeroSlice, true},
		{"emptySlice", []int{}, true},
		{"zeroMap", zeroMap, true},
		{"emptyMap", map[string]int{}, true},
		{"emptyMap", map[string]int{}, true},
		{"zeroStruct", zeroStruct, true},
		{"emptyStruct", struct{}{}, true},
		{"ptr", (*int)(nil), true},
		{"customType", customInt(0), true},
		{"customStruct", customStruct{}, true},
		{"customSlice", customSlice{}, true},
		{"customMap", customMap{}, true},
		{"customPtrSlice", &customSlice{}, true},
		{"customPtrMap", &customMap{}, true},
		{"customPtrSliceWithValue", &customSlice{1}, false},
		{"customPtrMapWithValue", &customMap{1: 1}, false},
	}

	for _, tc := range testCases {
		b.Run(tc.description, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				IsZeroValue(tc.in)
			}
		})
	}
}
