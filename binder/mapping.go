package binder

import (
	"errors"
	"fmt"
	"maps"
	"mime/multipart"
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
	tags = []string{"header", "respHeader", "cookie", "query", "form", "uri", "msgpack"}
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
func parse(aliasTag string, out any, data map[string][]string, files ...map[string][]*multipart.FileHeader) error {
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
	return parseToStruct(aliasTag, out, data, files...)
}

// Parse data into the struct with gofiber/schema
func parseToStruct(aliasTag string, out any, data map[string][]string, files ...map[string][]*multipart.FileHeader) error {
	// Get decoder from pool
	schemaDecoder := decoderPoolMap[aliasTag].Get().(*schema.Decoder) //nolint:errcheck,forcetypeassert // not needed
	defer decoderPoolMap[aliasTag].Put(schemaDecoder)

	// Set alias tag
	schemaDecoder.SetAliasTag(aliasTag)

	if err := schemaDecoder.Decode(out, data, files...); err != nil {
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
			return ErrMapNotConvertible
		}

		maps.Copy(newMap, data)
	case reflect.String, reflect.Interface:
		newMap, ok := ptr.(map[string]string)
		if !ok {
			return ErrMapNotConvertible
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

func isStringKeyMap(t reflect.Type) bool {
	return t.Kind() == reflect.Map && t.Key().Kind() == reflect.String
}

func isExported(f reflect.StructField) bool {
	return f.PkgPath == ""
}

func fieldName(f reflect.StructField, aliasTag string) string {
	name := f.Tag.Get(aliasTag)
	if name == "" {
		name = f.Name
	} else {
		name = strings.Split(name, ",")[0]
	}

	return utils.ToLower(name)
}

type fieldInfo struct {
	names       map[string]reflect.Kind
	nestedKinds map[reflect.Kind]struct{}
}

var (
	headerFieldCache     sync.Map
	respHeaderFieldCache sync.Map
	cookieFieldCache     sync.Map
	queryFieldCache      sync.Map
	formFieldCache       sync.Map
	uriFieldCache        sync.Map
)

func getFieldCache(aliasTag string) *sync.Map {
	switch aliasTag {
	case "header":
		return &headerFieldCache
	case "respHeader":
		return &respHeaderFieldCache
	case "cookie":
		return &cookieFieldCache
	case "form":
		return &formFieldCache
	case "uri":
		return &uriFieldCache
	case "query":
		return &queryFieldCache
	}

	panic("unknown alias tag: " + aliasTag)
}

func buildFieldInfo(t reflect.Type, aliasTag string) fieldInfo {
	info := fieldInfo{
		names:       make(map[string]reflect.Kind),
		nestedKinds: make(map[reflect.Kind]struct{}),
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !isExported(f) {
			continue
		}
		info.names[fieldName(f, aliasTag)] = f.Type.Kind()

		if f.Type.Kind() == reflect.Struct {
			for j := 0; j < f.Type.NumField(); j++ {
				sf := f.Type.Field(j)
				if !isExported(sf) {
					continue
				}
				info.nestedKinds[sf.Type.Kind()] = struct{}{}
			}
		}
	}

	return info
}

func equalFieldType(out any, kind reflect.Kind, key, aliasTag string) bool {
	typ := reflect.TypeOf(out).Elem()
	key = utils.ToLower(key)

	if isStringKeyMap(typ) {
		return true
	}

	if typ.Kind() != reflect.Struct {
		return false
	}

	cache := getFieldCache(aliasTag)
	val, ok := cache.Load(typ)
	if !ok {
		info := buildFieldInfo(typ, aliasTag)
		val, _ = cache.LoadOrStore(typ, info)
	}

	info, ok := val.(fieldInfo)
	if !ok {
		return false
	}

	if k, ok := info.names[key]; ok && k == kind {
		return true
	}
	if _, ok := info.nestedKinds[kind]; ok {
		return true
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

func formatBindData[T, K any](aliasTag string, out any, data map[string][]T, key string, value K, enableSplitting, supportBracketNotation bool) error { //nolint:revive // it's okay
	var err error
	if supportBracketNotation && strings.Contains(key, "[") {
		key, err = parseParamSquareBrackets(key)
		if err != nil {
			return err
		}
	}

	switch v := any(value).(type) {
	case string:
		dataMap, ok := any(data).(map[string][]string)
		if !ok {
			return fmt.Errorf("unsupported value type: %T", value)
		}

		assignBindData(aliasTag, out, dataMap, key, v, enableSplitting)
	case []string:
		dataMap, ok := any(data).(map[string][]string)
		if !ok {
			return fmt.Errorf("unsupported value type: %T", value)
		}

		for _, val := range v {
			assignBindData(aliasTag, out, dataMap, key, val, enableSplitting)
		}
	case []*multipart.FileHeader:
		for _, val := range v {
			valT, ok := any(val).(T)
			if !ok {
				return fmt.Errorf("unsupported value type: %T", value)
			}
			data[key] = append(data[key], valT)
		}
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}

	return err
}

func assignBindData(aliasTag string, out any, data map[string][]string, key, value string, enableSplitting bool) { //nolint:revive // it's okay
	if enableSplitting && strings.Contains(value, ",") && equalFieldType(out, reflect.Slice, key, aliasTag) {
		values := strings.Split(value, ",")
		for i := 0; i < len(values); i++ {
			data[key] = append(data[key], values[i])
		}
	} else {
		data[key] = append(data[key], value)
	}
}
