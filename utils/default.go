package utils

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var (
	mu          sync.RWMutex
	structCache = make(map[reflect.Type][]reflect.StructField)
)

const (
	tagName = "default"
)

func tagHandlers(field reflect.Value, tagValue string) {
	mu.Lock()
	defer mu.Unlock()

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
		setDefaultForSlice(field, tagValue, field.Type().Elem())
	}
}

func setDefaultForSlice(field reflect.Value, tagValue string, elemType reflect.Type) {
	mu.Lock()
	defer mu.Unlock()

	items := strings.Split(tagValue, ",")
	slice := reflect.MakeSlice(reflect.SliceOf(elemType), 0, len(items))
	for _, item := range items {
		var val reflect.Value
		//nolint:exhaustive // We don't need to handle all types
		switch elemType.Kind() {
		case reflect.Ptr:
			elemKind := elemType.Elem().Kind()
			//nolint:exhaustive // We don't need to handle all types
			switch elemKind {
			case reflect.String:
				strVal := item
				val = reflect.ValueOf(&strVal)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if intVal, err := strconv.ParseInt(item, 10, 64); err == nil {
					intPtr := reflect.New(elemType.Elem())
					intPtr.Elem().SetInt(intVal)
					val = intPtr
				}
			}
		case reflect.String:
			val = reflect.ValueOf(item)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if intVal, err := strconv.ParseInt(item, 10, 64); err == nil {
				switch elemType.Kind() {
				case reflect.Int:
					if strconv.IntSize == 64 && (intVal >= int64(math.MinInt32) && intVal <= int64(math.MaxInt32)) {
						val = reflect.ValueOf(int(intVal))
					}
				case reflect.Int8:
					if intVal >= int64(math.MinInt8) && intVal <= int64(math.MaxInt8) {
						val = reflect.ValueOf(int8(intVal))
					}
				case reflect.Int16:
					if intVal >= int64(math.MinInt16) && intVal <= int64(math.MaxInt16) {
						val = reflect.ValueOf(int16(intVal))
					}
				case reflect.Int32:
					if intVal >= int64(math.MinInt32) && intVal <= int64(math.MaxInt32) {
						val = reflect.ValueOf(int32(intVal))
					}
				case reflect.Int64:
					val = reflect.ValueOf(intVal)
				}
			}
		}
		if val.IsValid() {
			slice = reflect.Append(slice, val)
		}
	}

	field.Set(slice)
}

func getFieldsWithDefaultTag(t reflect.Type) []reflect.StructField {
	mu.RLock()
	fields, ok := structCache[t]
	mu.RUnlock()
	if ok {
		return fields
	}

	var newFields []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if _, ok := field.Tag.Lookup(tagName); ok {
			newFields = append(newFields, field)
		}
	}

	mu.Lock()
	structCache[t] = newFields
	mu.Unlock()

	return newFields
}

func SetDefaultValues(out interface{}) {
	elem := reflect.ValueOf(out).Elem()
	typ := elem.Type()

	fields := getFieldsWithDefaultTag(typ)
	for _, fieldInfo := range fields {
		field := elem.FieldByName(fieldInfo.Name)
		tagValue := fieldInfo.Tag.Get(tagName)
		tagHandlers(field, tagValue)
	}
}
