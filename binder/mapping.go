package binder

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"

	"github.com/gofiber/schema"
)

// ParserConfig form decoder config for SetParserDecoder
type ParserConfig struct {
	SetAliasTag       string
	ParserType        []ParserType
	IgnoreUnknownKeys bool
	ZeroEmpty         bool
}

// ParserType require two element, type and converter for register.
// Use ParserType with BodyParser for parsing custom type in form data.
type ParserType struct {
	CustomType any
	Converter  func(string) reflect.Value
}

var (
	// decoderPoolMap helps to improve binders
	decoderPoolMap = map[string]*sync.Pool{}
	// tags is used to classify parser's pool
	tags = []string{"header", "respHeader", "cookie", "query", "form", "uri"}
)

// SetParserDecoder allow globally change the option of form decoder, update decoderPool
func SetParserDecoder(parserConfig ParserConfig) {
	for _, tag := range tags {
		decoderPoolMap[tag] = &sync.Pool{New: func() any {
			return decoderBuilder(parserConfig)
		}}
	}
}

func decoderBuilder(parserConfig ParserConfig) any {
	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(parserConfig.IgnoreUnknownKeys)
	if parserConfig.SetAliasTag != "" {
		decoder.SetAliasTag(parserConfig.SetAliasTag)
	}
	for _, v := range parserConfig.ParserType {
		decoder.RegisterConverter(reflect.ValueOf(v.CustomType).Interface(), v.Converter)
	}
	decoder.ZeroEmpty(parserConfig.ZeroEmpty)
	return decoder
}

func init() {
	for _, tag := range tags {
		decoderPoolMap[tag] = &sync.Pool{New: func() any {
			return decoderBuilder(ParserConfig{
				IgnoreUnknownKeys: true,
				ZeroEmpty:         true,
			})
		}}
	}
}

// parse data into the map or struct
func parse(aliasTag string, out any, data map[string][]string) error {
	ptrVal := reflect.ValueOf(out)

	// Get pointer value
	if ptrVal.Kind() == reflect.Ptr {
		ptrVal = ptrVal.Elem()
	}

	// Parse into the map
	if ptrVal.Kind() == reflect.Map && ptrVal.Type().Key().Kind() == reflect.String {
		return parseToMap(ptrVal.Interface(), data)
	}

	// Parse into the struct
	return parseToStruct(aliasTag, out, data)
}

// Parse data into the struct with gorilla/schema
func parseToStruct(aliasTag string, out any, data map[string][]string) error {
	// Get decoder from pool
	schemaDecoder := decoderPoolMap[aliasTag].Get().(*schema.Decoder) //nolint:errcheck,forcetypeassert // not needed
	defer decoderPoolMap[aliasTag].Put(schemaDecoder)

	// Set alias tag
	schemaDecoder.SetAliasTag(aliasTag)

	if err := schemaDecoder.Decode(out, data); err != nil {
		return fmt.Errorf("bind: %w", err)
	}

	return nil
}

// Parse data into the map
// thanks to https://github.com/gin-gonic/gin/blob/master/binding/binding.go
func parseToMap(ptr any, data map[string][]string) error {
	elem := reflect.TypeOf(ptr).Elem()

	switch elem.Kind() {
	case reflect.Slice:
		newMap, ok := ptr.(map[string][]string)
		if !ok {
			return ErrMapNotConvertable
		}

		for k, v := range data {
			newMap[k] = v
		}
	case reflect.String, reflect.Interface:
		newMap, ok := ptr.(map[string]string)
		if !ok {
			return ErrMapNotConvertable
		}

		for k, v := range data {
			if len(v) == 0 {
				newMap[k] = ""
				continue
			}
			newMap[k] = v[len(v)-1]
		}
	default:
		return nil // it's not necessary to check all types
	}

	return nil
}

func parseParamSquareBrackets(k string) (string, error) {
	bb := bytebufferpool.Get()
	defer bytebufferpool.Put(bb)

	kbytes := []byte(k)
	openBracketsCount := 0

	for i, b := range kbytes {
		if b == '[' {
			openBracketsCount++
			if i+1 < len(kbytes) && kbytes[i+1] != ']' {
				if err := bb.WriteByte('.'); err != nil {
					return "", err //nolint:wrapcheck // unnecessary to wrap it
				}
			}
			continue
		}

		if b == ']' {
			openBracketsCount--
			if openBracketsCount < 0 {
				return "", errors.New("unmatched brackets")
			}
			continue
		}

		if err := bb.WriteByte(b); err != nil {
			return "", err //nolint:wrapcheck // unnecessary to wrap it
		}
	}

	if openBracketsCount > 0 {
		return "", errors.New("unmatched brackets")
	}

	return bb.String(), nil
}

func equalFieldType(out any, kind reflect.Kind, key string) bool {
	// Get type of interface
	outTyp := reflect.TypeOf(out).Elem()
	key = utils.ToLower(key)

	// Support maps
	if outTyp.Kind() == reflect.Map && outTyp.Key().Kind() == reflect.String {
		return true
	}

	// Must be a struct to match a field
	if outTyp.Kind() != reflect.Struct {
		return false
	}
	// Copy interface to an value to be used
	outVal := reflect.ValueOf(out).Elem()
	// Loop over each field
	for i := 0; i < outTyp.NumField(); i++ {
		// Get field value data
		structField := outVal.Field(i)
		// Can this field be changed?
		if !structField.CanSet() {
			continue
		}
		// Get field key data
		typeField := outTyp.Field(i)
		// Get type of field key
		structFieldKind := structField.Kind()
		// Does the field type equals input?
		if structFieldKind != kind {
			// Is the field an embedded struct?
			if structFieldKind == reflect.Struct {
				// Loop over embedded struct fields
				for j := 0; j < structField.NumField(); j++ {
					structFieldField := structField.Field(j)

					// Can this embedded field be changed?
					if !structFieldField.CanSet() {
						continue
					}

					// Is the embedded struct field type equal to the input?
					if structFieldField.Kind() == kind {
						return true
					}
				}
			}

			continue
		}
		// Get tag from field if exist
		inputFieldName := typeField.Tag.Get("query") // Name of query binder
		if inputFieldName == "" {
			inputFieldName = typeField.Name
		} else {
			inputFieldName = strings.Split(inputFieldName, ",")[0]
		}
		// Compare field/tag with provided key
		if utils.ToLower(inputFieldName) == key {
			return true
		}
	}
	return false
}

// Get content type from content type header
func FilterFlags(content string) string {
	for i, char := range content {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}
	return content
}

func formatBindData[T any](out any, data map[string][]string, key string, value T, enableSplitting, supportBracketNotation bool) error { //nolint:revive // it's okay
	var err error
	if supportBracketNotation && strings.Contains(key, "[") {
		key, err = parseParamSquareBrackets(key)
		if err != nil {
			return err
		}
	}

	switch v := any(value).(type) {
	case string:
		assignBindData(out, data, key, v, enableSplitting)
	case []string:
		for _, val := range v {
			assignBindData(out, data, key, val, enableSplitting)
		}
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}

	return err
}

func assignBindData(out any, data map[string][]string, key, value string, enableSplitting bool) { //nolint:revive // it's okay
	if enableSplitting && strings.Contains(value, ",") && equalFieldType(out, reflect.Slice, key) {
		values := strings.Split(value, ",")
		for i := 0; i < len(values); i++ {
			data[key] = append(data[key], values[i])
		}
	} else {
		data[key] = append(data[key], value)
	}
}
