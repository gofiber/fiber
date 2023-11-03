package utils

import (
	"reflect"
	"strconv"
	"strings"
)

func tagHandlers(field reflect.Value, tagValue string) {
	//nolint:exhaustive // We don't need to handle all types
	switch field.Kind() {
	case reflect.String:
		if field.String() == "" {
			field.SetString(tagValue)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Int() == 0 {
			if i, err := strconv.ParseInt(tagValue, 10, 64); err == nil {
				field.SetInt(i)
			}
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() == 0.0 {
			if f, err := strconv.ParseFloat(tagValue, 64); err == nil {
				field.SetFloat(f)
			}
		}
	case reflect.Bool:
		if !field.Bool() {
			if b, err := strconv.ParseBool(tagValue); err == nil {
				field.SetBool(b)
			}
		}
	case reflect.Slice:
		setDefaultForSlice(field, tagValue, field.Type().Elem().Kind())
	}
}

func setDefaultForSlice(field reflect.Value, tagValue string, kind reflect.Kind) {
	items := strings.Split(tagValue, ",")
	for _, item := range items {
		//nolint:exhaustive // We don't need to handle all types
		switch kind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if i, err := strconv.ParseInt(item, 10, 64); err == nil {
				field.Set(reflect.Append(field, reflect.ValueOf(int(i))))
			}
		case reflect.String:
			field.Set(reflect.Append(field, reflect.ValueOf(item)))
		}
	}
}

var structCache = make(map[reflect.Type][]reflect.StructField)

func getFieldsWithDefaultTag(t reflect.Type) []reflect.StructField {
	if fields, ok := structCache[t]; ok {
		return fields
	}

	var fields []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if _, ok := field.Tag.Lookup("default"); ok {
			fields = append(fields, field)
		}
	}
	structCache[t] = fields
	return fields
}

func SetDefaultValues(out interface{}) {
	val := reflect.ValueOf(out).Elem()
	typ := val.Type()

	fields := getFieldsWithDefaultTag(typ)
	for _, fieldInfo := range fields {
		field := val.FieldByName(fieldInfo.Name)
		tagValue := fieldInfo.Tag.Get("default")
		tagHandlers(field, tagValue)
	}
}
