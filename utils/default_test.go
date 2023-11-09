package utils

import (
	"reflect"
	"testing"
)

type TestStruct struct {
	Name     string   `default:"John"`
	Age      int      `default:"25"`
	Height   float64  `default:"5.9"`
	IsActive bool     `default:"true"`
	Friends  []string `default:"Alice,Bob"`
}

func TestSetDefaultValues(t *testing.T) {
	tests := []struct {
		name     string
		input    *TestStruct
		expected *TestStruct
	}{
		{
			name:     "All fields empty",
			input:    &TestStruct{},
			expected: &TestStruct{Name: "John", Age: 25, Height: 5.9, IsActive: true, Friends: []string{"Alice", "Bob"}},
		},
		{
			name:     "Some fields set",
			input:    &TestStruct{Name: "Doe", Age: 0, Height: 0.0, IsActive: false, Friends: nil},
			expected: &TestStruct{Name: "Doe", Age: 25, Height: 5.9, IsActive: true, Friends: []string{"Alice", "Bob"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaultValues(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				t.Errorf("got %v, want %v", tt.input, tt.expected)
			}
		})
	}
}

func TestTagHandlers(t *testing.T) {
	tests := []struct {
		name     string
		field    reflect.Value
		tagValue string
		expected interface{}
	}{
		{
			name:     "String field with default value",
			field:    reflect.ValueOf(new(string)).Elem(),
			tagValue: "test",
			expected: "test",
		},
		{
			name:     "Int field with default value",
			field:    reflect.ValueOf(new(int)).Elem(),
			tagValue: "42",
			expected: 42,
		},
		{
			name:     "Float64 field with default value",
			field:    reflect.ValueOf(new(float64)).Elem(),
			tagValue: "3.14",
			expected: 3.14,
		},
		{
			name:     "Bool field with default value",
			field:    reflect.ValueOf(new(bool)).Elem(),
			tagValue: "true",
			expected: true,
		},
		{
			name:     "Slice of strings with default value",
			field:    reflect.ValueOf(new([]string)).Elem(),
			tagValue: "apple,1",
			expected: []string{"apple", "1"},
		},
		{
			name:     "Slice of ints with default value",
			field:    reflect.ValueOf(new([]int)).Elem(),
			tagValue: "1,2",
			expected: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tagHandlers(tt.field, tt.tagValue)
			if !reflect.DeepEqual(tt.field.Interface(), tt.expected) {
				t.Errorf("got %v, want %v", tt.field.Interface(), tt.expected)
			}
		})
	}
}
func TestSetDefaultForSlice(t *testing.T) {
	tests := []struct {
		name     string
		field    reflect.Value
		tagValue string
		elemType reflect.Type
		expected interface{}
	}{
		{
			name:     "Slice of strings with default value",
			field:    reflect.ValueOf(new([]string)).Elem(),
			tagValue: "apple,banana",
			elemType: reflect.TypeOf(""),
			expected: []string{"apple", "banana"},
		},
		{
			name:     "Slice of ints with default value",
			field:    reflect.ValueOf(new([]int)).Elem(),
			tagValue: "1,2,3",
			elemType: reflect.TypeOf(0),
			expected: []int{1, 2, 3},
		},
		{
			name:     "Slice of string pointers with default value",
			field:    reflect.ValueOf(new([]*string)).Elem(),
			tagValue: "apple,banana",
			elemType: reflect.TypeOf(new(*string)).Elem(),
			expected: []*string{str("apple"), str("banana")},
		},
		{
			name:     "Slice of int pointers with default value",
			field:    reflect.ValueOf(new([]*int)).Elem(),
			tagValue: "1,2,3",
			elemType: reflect.TypeOf(new(*int)).Elem(),
			expected: []*int{ptr(1), ptr(2), ptr(3)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaultForSlice(tt.field, tt.tagValue, tt.elemType)
			if !reflect.DeepEqual(tt.field.Interface(), tt.expected) {
				t.Errorf("got %v, want %v", tt.field.Interface(), tt.expected)
			}
		})
	}
}

// ptr is a helper function to take the address of a string.
func ptr(s int) *int {
	return &s
}

func str(s string) *string {
	return &s
}
