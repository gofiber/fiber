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
			tagValue: "apple,banana",
			expected: []string{"apple", "banana"},
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
		kind     reflect.Kind
		expected interface{}
	}{
		{
			name:     "Slice of strings with default value",
			field:    reflect.ValueOf(new([]string)).Elem(),
			tagValue: "apple,banana",
			kind:     reflect.String,
			expected: []string{"apple", "banana"},
		},
		{
			name:     "Slice of ints with default value",
			field:    reflect.ValueOf(new([]int)).Elem(),
			tagValue: "1,2,3",
			kind:     reflect.Int,
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaultForSlice(tt.field, tt.tagValue, tt.kind)
			if !reflect.DeepEqual(tt.field.Interface(), tt.expected) {
				t.Errorf("got %v, want %v", tt.field.Interface(), tt.expected)
			}
		})
	}
}
