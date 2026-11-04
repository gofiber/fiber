package session

import (
	"math"
	"reflect"
)

// sameData reports whether snapshot, the session data decoded again from the bytes it was loaded
// from, and data, what the request holds now, are the same data. It is reflect.DeepEqual except
// that a NaN is the same as a NaN: both carry the NaNs the bytes stored, which DeepEqual tells
// apart, so a session that held one read as edited and its request overwrote a change another
// request had stored.
//
// DeepEqual answers first, as it does for all data without a NaN, and the walk below only
// runs when it found a difference. It descends in step with snapshot, which is decoded from bytes
// and so has no cycles, so it ends even where data was made to refer to itself.
func sameData(snapshot, data map[any]any) bool {
	return reflect.DeepEqual(snapshot, data) || sameValue(reflect.ValueOf(snapshot), reflect.ValueOf(data))
}

// sameValue is reflect.DeepEqual for two values, except that a NaN is the same as a NaN. A
// map is the same as another when each of its keys is found in the other, which a NaN key never
// is, as with DeepEqual.
func sameValue(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	if a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Float32, reflect.Float64:
		return sameFloat(a.Float(), b.Float())
	case reflect.Complex64, reflect.Complex128:
		x, y := a.Complex(), b.Complex()
		return sameFloat(real(x), real(y)) && sameFloat(imag(x), imag(y))
	case reflect.Array:
		return sameElements(a, b)
	case reflect.Slice:
		return a.IsNil() == b.IsNil() && a.Len() == b.Len() && sameElements(a, b)
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return sameValue(a.Elem(), b.Elem())
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return a.UnsafePointer() == b.UnsafePointer() || sameValue(a.Elem(), b.Elem())
	case reflect.Struct:
		for i := range a.NumField() {
			if !sameValue(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		for iter := a.MapRange(); iter.Next(); {
			if other := b.MapIndex(iter.Key()); !other.IsValid() || !sameValue(iter.Value(), other) {
				return false
			}
		}
		return true
	case reflect.Func:
		return a.IsNil() && b.IsNil()
	default: // booleans, integers, strings and the like, which == tells
		return a.Equal(b)
	}
}

// sameElements reports whether the elements of the arrays or slices a and b, of one length, are the same.
func sameElements(a, b reflect.Value) bool {
	for i := range a.Len() {
		if !sameValue(a.Index(i), b.Index(i)) {
			return false
		}
	}
	return true
}

// sameFloat is == where a NaN is the same as a NaN.
func sameFloat(a, b float64) bool {
	return a == b || (math.IsNaN(a) && math.IsNaN(b))
}
