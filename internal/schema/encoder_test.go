package schema

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

type E1 struct {
	F01 int     `schema:"f01"`
	F02 int     `schema:"-"`
	F03 string  `schema:"f03"`
	F04 string  `schema:"f04,omitempty"`
	F05 bool    `schema:"f05"`
	F06 bool    `schema:"f06"`
	F07 *string `schema:"f07"`
	F08 *int8   `schema:"f08"`
	F09 float64 `schema:"f09"`
	F10 func()  `schema:"f10"`
	F11 inner
}
type inner struct {
	F12 int
}

func TestFilled(t *testing.T) {
	f07 := "seven"
	var f08 int8 = 8
	s := &E1{
		F01: 1,
		F02: 2,
		F03: "three",
		F04: "four",
		F05: true,
		F06: false,
		F07: &f07,
		F08: &f08,
		F09: 1.618,
		F10: func() {},
		F11: inner{12},
	}

	vals := make(map[string][]string)
	errs := NewEncoder().Encode(s, vals)

	valExists(t, "f01", "1", vals)
	valNotExists(t, "f02", vals)
	valExists(t, "f03", "three", vals)
	valExists(t, "f05", "true", vals)
	valExists(t, "f06", "false", vals)
	valExists(t, "f07", "seven", vals)
	valExists(t, "f08", "8", vals)
	valExists(t, "f09", "1.618000", vals)
	valExists(t, "F12", "12", vals)

	emptyErr := MultiError{}
	if errs.Error() == emptyErr.Error() {
		t.Errorf("Expected error got %v", errs)
	}
}

type Aa int

type E3 struct {
	F01 bool    `schema:"f01"`
	F02 float32 `schema:"f02"`
	F03 float64 `schema:"f03"`
	F04 int     `schema:"f04"`
	F05 int8    `schema:"f05"`
	F06 int16   `schema:"f06"`
	F07 int32   `schema:"f07"`
	F08 int64   `schema:"f08"`
	F09 string  `schema:"f09"`
	F10 uint    `schema:"f10"`
	F11 uint8   `schema:"f11"`
	F12 uint16  `schema:"f12"`
	F13 uint32  `schema:"f13"`
	F14 uint64  `schema:"f14"`
	F15 Aa      `schema:"f15"`
}

// Test compatibility with default decoder types.
func TestCompat(t *testing.T) {
	src := &E3{
		F01: true,
		F02: 4.2,
		F03: 4.3,
		F04: -42,
		F05: -43,
		F06: -44,
		F07: -45,
		F08: -46,
		F09: "foo",
		F10: 42,
		F11: 43,
		F12: 44,
		F13: 45,
		F14: 46,
		F15: 1,
	}
	dst := &E3{}

	vals := make(map[string][]string)
	encoder := NewEncoder()
	decoder := NewDecoder()

	encoder.RegisterEncoder(src.F15, func(reflect.Value) string { return "1" })
	decoder.RegisterConverter(src.F15, func(string) reflect.Value { return reflect.ValueOf(1) })

	err := encoder.Encode(src, vals)
	if err != nil {
		t.Errorf("Encoder has non-nil error: %v", err)
	}
	err = decoder.Decode(dst, vals)
	if err != nil {
		t.Errorf("Decoder has non-nil error: %v", err)
	}

	if *src != *dst {
		t.Errorf("Decoder-Encoder compatibility: expected %v, got %v\n", src, dst)
	}
}

func TestEmpty(t *testing.T) {
	s := &E1{
		F01: 1,
		F02: 2,
		F03: "three",
	}

	estr := "schema: encoder not found for <nil>"
	vals := make(map[string][]string)
	err := NewEncoder().Encode(s, vals)
	if err.Error() != estr {
		t.Errorf("Expected: %s, got %v", estr, err)
	}

	valExists(t, "f03", "three", vals)
	valNotExists(t, "f04", vals)
}

func TestStruct(t *testing.T) {
	estr := "schema: interface must be a struct"
	vals := make(map[string][]string)
	err := NewEncoder().Encode("hello world", vals)

	if err.Error() != estr {
		t.Errorf("Expected: %s, got %v", estr, err)
	}
}

func TestSlices(t *testing.T) {
	type oneAsWord int
	ones := []oneAsWord{1, 2}
	s1 := &struct {
		ones     []oneAsWord `schema:"ones"`
		ints     []int       `schema:"ints"`
		nonempty []int       `schema:"nonempty"`
		empty    []int       `schema:"empty,omitempty"`
	}{ones, []int{1, 1}, []int{}, []int{}}
	vals := make(map[string][]string)

	encoder := NewEncoder()
	encoder.RegisterEncoder(ones[0], func(v reflect.Value) string { return "one" })
	err := encoder.Encode(s1, vals)
	if err != nil {
		t.Errorf("Encoder has non-nil error: %v", err)
	}

	valsExist(t, "ones", []string{"one", "one"}, vals)
	valsExist(t, "ints", []string{"1", "1"}, vals)
	valsExist(t, "nonempty", []string{}, vals)
	valNotExists(t, "empty", vals)
}

func TestCompatSlices(t *testing.T) {
	type oneAsWord int
	type s1 struct {
		Ones []oneAsWord `schema:"ones"`
		Ints []int       `schema:"ints"`
	}
	ones := []oneAsWord{1, 1}
	src := &s1{ones, []int{1, 1}}
	vals := make(map[string][]string)
	dst := &s1{}

	encoder := NewEncoder()
	encoder.RegisterEncoder(ones[0], func(v reflect.Value) string { return "one" })

	decoder := NewDecoder()
	decoder.RegisterConverter(ones[0], func(s string) reflect.Value {
		if s == "one" {
			return reflect.ValueOf(1)
		}
		return reflect.ValueOf(2)
	})

	err := encoder.Encode(src, vals)
	if err != nil {
		t.Errorf("Encoder has non-nil error: %v", err)
	}
	err = decoder.Decode(dst, vals)
	if err != nil {
		t.Errorf("Dncoder has non-nil error: %v", err)
	}

	if len(src.Ints) != len(dst.Ints) || len(src.Ones) != len(src.Ones) {
		t.Fatalf("Expected %v, got %v", src, dst)
	}

	for i, v := range src.Ones {
		if dst.Ones[i] != v {
			t.Fatalf("Expected %v, got %v", v, dst.Ones[i])
		}
	}

	for i, v := range src.Ints {
		if dst.Ints[i] != v {
			t.Fatalf("Expected %v, got %v", v, dst.Ints[i])
		}
	}
}

func TestRegisterEncoder(t *testing.T) {
	type oneAsWord int
	type twoAsWord int
	type oneSliceAsWord []int

	s1 := &struct {
		oneAsWord
		twoAsWord
		oneSliceAsWord
	}{1, 2, []int{1, 1}}
	v1 := make(map[string][]string)

	encoder := NewEncoder()
	encoder.RegisterEncoder(s1.oneAsWord, func(v reflect.Value) string { return "one" })
	encoder.RegisterEncoder(s1.twoAsWord, func(v reflect.Value) string { return "two" })
	encoder.RegisterEncoder(s1.oneSliceAsWord, func(v reflect.Value) string { return "one" })

	err := encoder.Encode(s1, v1)
	if err != nil {
		t.Errorf("Encoder has non-nil error: %v", err)
	}

	valExists(t, "oneAsWord", "one", v1)
	valExists(t, "twoAsWord", "two", v1)
	valExists(t, "oneSliceAsWord", "one", v1)
}

func TestEncoderOrder(t *testing.T) {
	type builtinEncoderSimple int
	type builtinEncoderSimpleOverridden int
	type builtinEncoderSlice []int
	type builtinEncoderSliceOverridden []int
	type builtinEncoderStruct struct{ nr int }
	type builtinEncoderStructOverridden struct{ nr int }

	s1 := &struct {
		builtinEncoderSimple           `schema:"simple"`
		builtinEncoderSimpleOverridden `schema:"simple_overridden"`
		builtinEncoderSlice            `schema:"slice"`
		builtinEncoderSliceOverridden  `schema:"slice_overridden"`
		builtinEncoderStruct           `schema:"struct"`
		builtinEncoderStructOverridden `schema:"struct_overridden"`
	}{
		1,
		1,
		[]int{2},
		[]int{2},
		builtinEncoderStruct{3},
		builtinEncoderStructOverridden{3},
	}
	v1 := make(map[string][]string)

	encoder := NewEncoder()
	encoder.RegisterEncoder(s1.builtinEncoderSimpleOverridden, func(v reflect.Value) string { return "one" })
	encoder.RegisterEncoder(s1.builtinEncoderSliceOverridden, func(v reflect.Value) string { return "two" })
	encoder.RegisterEncoder(s1.builtinEncoderStructOverridden, func(v reflect.Value) string { return "three" })

	err := encoder.Encode(s1, v1)
	if err != nil {
		t.Errorf("Encoder has non-nil error: %v", err)
	}

	valExists(t, "simple", "1", v1)
	valExists(t, "simple_overridden", "one", v1)
	valExists(t, "slice", "2", v1)
	valExists(t, "slice_overridden", "two", v1)
	valExists(t, "nr", "3", v1)
	valExists(t, "struct_overridden", "three", v1)
}

func valExists(t *testing.T, key string, expect string, result map[string][]string) {
	valsExist(t, key, []string{expect}, result)
}

func valsExist(t *testing.T, key string, expect []string, result map[string][]string) {
	vals, ok := result[key]
	if !ok {
		t.Fatalf("Key not found. Expected: %s", key)
	}

	if len(expect) != len(vals) {
		t.Fatalf("Expected: %v, got: %v", expect, vals)
	}

	for i, v := range expect {
		if vals[i] != v {
			t.Fatalf("Unexpected value. Expected: %v, got %v", v, vals[i])
		}
	}
}

func valNotExists(t *testing.T, key string, result map[string][]string) {
	if val, ok := result[key]; ok {
		t.Error("Key not ommited. Expected: empty; got: " + val[0] + ".")
	}
}

type E4 struct {
	ID string `json:"id"`
}

func TestEncoderSetAliasTag(t *testing.T) {
	data := map[string][]string{}

	s := E4{
		ID: "foo",
	}
	encoder := NewEncoder()
	encoder.SetAliasTag("json")
	encoder.Encode(&s, data)
	valExists(t, "id", "foo", data)
}

type E5 struct {
	F01 int      `schema:"f01,omitempty"`
	F02 string   `schema:"f02,omitempty"`
	F03 *string  `schema:"f03,omitempty"`
	F04 *int8    `schema:"f04,omitempty"`
	F05 float64  `schema:"f05,omitempty"`
	F06 E5F06    `schema:"f06,omitempty"`
	F07 E5F06    `schema:"f07,omitempty"`
	F08 []string `schema:"f08,omitempty"`
	F09 []string `schema:"f09,omitempty"`
}

type E5F06 struct {
	F0601 string `schema:"f0601,omitempty"`
}

func TestEncoderWithOmitempty(t *testing.T) {
	vals := map[string][]string{}

	s := E5{
		F02: "test",
		F07: E5F06{
			F0601: "test",
		},
		F09: []string{"test"},
	}

	encoder := NewEncoder()
	encoder.Encode(&s, vals)

	valNotExists(t, "f01", vals)
	valExists(t, "f02", "test", vals)
	valNotExists(t, "f03", vals)
	valNotExists(t, "f04", vals)
	valNotExists(t, "f05", vals)
	valNotExists(t, "f06", vals)
	valExists(t, "f0601", "test", vals)
	valNotExists(t, "f08", vals)
	valsExist(t, "f09", []string{"test"}, vals)
}

type E6 struct {
	F01 *inner
	F02 *inner
	F03 *inner `schema:",omitempty"`
}

func TestStructPointer(t *testing.T) {
	vals := map[string][]string{}
	s := E6{
		F01: &inner{2},
	}

	encoder := NewEncoder()
	encoder.Encode(&s, vals)
	valExists(t, "F12", "2", vals)
	valExists(t, "F02", "null", vals)
	valNotExists(t, "F03", vals)
}

func TestRegisterEncoderCustomArrayType(t *testing.T) {
	type CustomInt []int
	type S1 struct {
		SomeInts CustomInt `schema:",omitempty"`
	}

	ss := []S1{
		{},
		{CustomInt{}},
		{CustomInt{1, 2, 3}},
	}

	for s := range ss {
		vals := map[string][]string{}

		encoder := NewEncoder()
		encoder.RegisterEncoder(CustomInt{}, func(value reflect.Value) string {
			return fmt.Sprint(value.Interface())
		})

		encoder.Encode(s, vals)
	}
}

func TestRegisterEncoderStructIsZero(t *testing.T) {
	type S1 struct {
		SomeTime1 time.Time `schema:"tim1,omitempty"`
		SomeTime2 time.Time `schema:"tim2,omitempty"`
	}

	ss := []*S1{
		{
			SomeTime1: time.Date(2020, 8, 4, 13, 30, 1, 0, time.UTC),
		},
	}

	for s := range ss {
		vals := map[string][]string{}

		encoder := NewEncoder()
		encoder.RegisterEncoder(time.Time{}, func(value reflect.Value) string {
			return value.Interface().(time.Time).Format(time.RFC3339Nano)
		})

		err := encoder.Encode(ss[s], vals)
		if err != nil {
			t.Errorf("Encoder has non-nil error: %v", err)
		}

		ta, ok := vals["tim1"]
		if !ok {
			t.Error("expected tim1 to be present")
		}

		if len(ta) != 1 {
			t.Error("expected tim1 to be present")
		}

		if "2020-08-04T13:30:01Z" != ta[0] {
			t.Error("expected correct tim1 time")
		}

		_, ok = vals["tim2"]
		if ok {
			t.Error("expected tim1 not to be present")
		}
	}
}
