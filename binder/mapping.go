package binder

import (
	"reflect"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3/internal/schema"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
)

// ParserConfig form decoder config for SetParserDecoder
type ParserConfig struct {
	IgnoreUnknownKeys bool
	SetAliasTag       string
	ParserType        []ParserType
	ZeroEmpty         bool
}

// ParserType require two element, type and converter for register.
// Use ParserType with BodyParser for parsing custom type in form data.
type ParserType struct {
	Customtype any
	Converter  func(string) reflect.Value
}

var (
	// decoderPoolMap helps to improve binders
	decoderPoolMap = map[string]*sync.Pool{}
	// tags is used to classify parser's pool
	tags = []string{HeaderBinder.Name(), RespHeaderBinder.Name(), CookieBinder.Name(), QueryBinder.Name(), FormBinder.Name(), URIBinder.Name()}
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
		decoder.RegisterConverter(reflect.ValueOf(v.Customtype).Interface(), v.Converter)
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

	return schemaDecoder.Decode(out, data)
}

// Parse data into the map
// thanks to https://github.com/gin-gonic/gin/blob/master/binding/binding.go
func parseToMap(ptr any, data map[string][]string) error {
	elem := reflect.TypeOf(ptr).Elem()

	// map[string][]string
	if elem.Kind() == reflect.Slice {
		newMap, ok := ptr.(map[string][]string)
		if !ok {
			return ErrMapNotConvertable
		}

		for k, v := range data {
			newMap[k] = v
		}

		return nil
	}

	// map[string]string
	newMap, ok := ptr.(map[string]string)
	if !ok {
		return ErrMapNotConvertable
	}

	for k, v := range data {
		newMap[k] = v[len(v)-1]
	}

	return nil
}

func parseParamSquareBrackets(k string) (string, error) {
	bb := bytebufferpool.Get()
	defer bytebufferpool.Put(bb)

	kbytes := []byte(k)

	for i, b := range kbytes {
		if b == '[' && kbytes[i+1] != ']' {
			if err := bb.WriteByte('.'); err != nil {
				return "", err //nolint:wrapchec,wrapcheck // unnecessary to wrap it
			}
		}

		if b == '[' || b == ']' {
			continue
		}

		if err := bb.WriteByte(b); err != nil {
			return "", err //nolint:wrapchec,wrapcheck // unnecessary to wrap it
		}
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
			continue
		}
		// Get tag from field if exist
		inputFieldName := typeField.Tag.Get(QueryBinder.Name())
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
