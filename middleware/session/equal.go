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
// DeepEqual answers first, as it does for all data without a NaN, and the walk below only runs
// when it found a difference. The walk notes the pairs it has compared, as DeepEqual does, so it
// ends on data that refers to itself: one an edit in place made so, or one a type's decoder
// rebuilt in the snapshot too.
func sameData(snapshot, data map[any]any) bool {
	return reflect.DeepEqual(snapshot, data) || sameValue(reflect.ValueOf(snapshot), reflect.ValueOf(data), make(map[visit]struct{}))
}

// visit is a pair of references the walk has reached, with what they refer to: a cycle can only
// pass through a pointer, a map or a slice, so those are all it notes.
type visit struct {
	typ  reflect.Type
	a, b uintptr
	n    int // the length of a slice
}

// sameValue is reflect.DeepEqual for two values, except that a NaN is the same as a NaN. A
// map is the same as another when each of its keys is found in the other, which a NaN key never
// is, as with DeepEqual. A pair already reached counts as the same, which is what ends a cycle: a
// difference anywhere in it is found by the first visit.
func sameValue(a, b reflect.Value, seen map[visit]struct{}) bool {
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
		return sameElements(a, b, seen)
	case reflect.Slice:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		if a.Len() == 0 || a.UnsafePointer() == b.UnsafePointer() || reached(seen, a, b, a.Len()) {
			return true
		}
		return sameElements(a, b, seen)
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return sameValue(a.Elem(), b.Elem(), seen)
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		if a.UnsafePointer() == b.UnsafePointer() || reached(seen, a, b, 0) {
			return true
		}
		return sameValue(a.Elem(), b.Elem(), seen)
	case reflect.Struct:
		for i := range a.NumField() {
			if !sameValue(a.Field(i), b.Field(i), seen) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		if a.UnsafePointer() == b.UnsafePointer() || reached(seen, a, b, 0) {
			return true
		}
		for iter := a.MapRange(); iter.Next(); {
			if other := b.MapIndex(iter.Key()); !other.IsValid() || !sameValue(iter.Value(), other, seen) {
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

// reached reports whether the references a and b, of one type, were reached before, and notes
// them if not. n is the length of a slice, whose pair is told by it as well as by where it starts.
func reached(seen map[visit]struct{}, a, b reflect.Value, n int) bool {
	key := visit{typ: a.Type(), a: uintptr(a.UnsafePointer()), b: uintptr(b.UnsafePointer()), n: n}
	if _, ok := seen[key]; ok {
		return true
	}
	seen[key] = struct{}{}
	return false
}

// sameElements reports whether the elements of the arrays or slices a and b, of one length, are the same.
func sameElements(a, b reflect.Value, seen map[visit]struct{}) bool {
	for i := range a.Len() {
		if !sameValue(a.Index(i), b.Index(i), seen) {
			return false
		}
	}
	return true
}

// sameFloat is == where a NaN is the same as a NaN.
func sameFloat(a, b float64) bool {
	return a == b || (math.IsNaN(a) && math.IsNaN(b))
}
