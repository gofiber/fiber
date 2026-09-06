package binder

import (
	"fmt"
	"maps"
	"mime/multipart"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/bytebufferpool"

	"github.com/gofiber/schema"
)

// ParserConfig form decoder config for SetParserDecoder
type ParserConfig struct {
	// SetAliasTag is ignored: every binder decodes with the tag of its own source.
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
	decoderPoolMu sync.RWMutex
	// decoderPoolMap helps to improve binders
	decoderPoolMap = map[string]*sync.Pool{}
	// tags is used to classify parser's pool
	tags = []string{bindingHeader, bindingRespHeader, bindingCookie, bindingQuery, bindingForm, bindingURI}
)

func getDecoderPool(tag string) *sync.Pool {
	decoderPoolMu.RLock()
	pool := decoderPoolMap[tag]
	if pool == nil {
		decoderPoolMu.RUnlock()
		panic(fmt.Sprintf("decoder pool not initialized for tag %q", tag))
	}
	decoderPoolMu.RUnlock()

	return pool
}

// SetParserDecoder allow globally change the option of form decoder, update decoderPool
func SetParserDecoder(parserConfig ParserConfig) {
	decoderPoolMu.Lock()
	defer decoderPoolMu.Unlock()

	for _, tag := range tags {
		decoderPoolMap[tag] = &sync.Pool{New: func() any {
			return decoderBuilder(tag, parserConfig)
		}}
	}
}

func decoderBuilder(aliasTag string, parserConfig ParserConfig) any {
	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(parserConfig.IgnoreUnknownKeys)
	// Bake the per-source tag once (pools are keyed per tag); doing it per
	// request reset the decoder's type cache. ParserConfig.SetAliasTag is not
	// honored here: one global tag is incompatible with per-source binding.
	decoder.SetAliasTag(aliasTag)
	for _, v := range parserConfig.ParserType {
		decoder.RegisterConverter(reflect.ValueOf(v.CustomType).Interface(), v.Converter)
	}
	decoder.ZeroEmpty(parserConfig.ZeroEmpty)
	return decoder
}

func init() {
	decoderPoolMu.Lock()
	defer decoderPoolMu.Unlock()

	for _, tag := range tags {
		decoderPoolMap[tag] = &sync.Pool{New: func() any {
			return decoderBuilder(tag, ParserConfig{
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
	if ptrVal.Kind() == reflect.Pointer {
		ptrVal = ptrVal.Elem()
	}

	// Parse into the map
	if ptrVal.Kind() == reflect.Map && ptrVal.Type().Key().Kind() == reflect.String {
		return parseToMap(ptrVal, data)
	}

	// Parse into the struct
	return parseToStruct(aliasTag, out, data, files...)
}

// Parse data into the struct with gofiber/schema
func parseToStruct(aliasTag string, out any, data map[string][]string, files ...map[string][]*multipart.FileHeader) error {
	// Get decoder from pool
	pool := getDecoderPool(aliasTag)
	schemaDecoder := pool.Get().(*schema.Decoder) //nolint:errcheck,forcetypeassert // not needed
	defer pool.Put(schemaDecoder)

	// Alias tag is baked in at build time (see decoderBuilder); setting it here
	// would reset the decoder's type cache on every request.
	if err := schemaDecoder.Decode(out, data, files...); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// Parse data into the map
// thanks to https://github.com/gin-gonic/gin/blob/master/binding/binding.go
func parseToMap(target reflect.Value, data map[string][]string) error {
	if !target.IsValid() {
		return ErrInvalidDestinationValue
	}

	if target.Kind() == reflect.Interface && !target.IsNil() {
		target = target.Elem()
	}

	if target.Kind() != reflect.Map || target.Type().Key().Kind() != reflect.String {
		return nil // nothing to do for non-map destinations
	}

	if target.IsNil() {
		if !target.CanSet() {
			return ErrMapNilDestination
		}
		target.Set(reflect.MakeMap(target.Type()))
	}

	switch target.Type().Elem().Kind() {
	case reflect.Slice:
		newMap, ok := target.Interface().(map[string][]string)
		if !ok {
			return ErrMapNotConvertible
		}

		maps.Copy(newMap, data)
	case reflect.String:
		newMap, ok := target.Interface().(map[string]string)
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
		// Interface element maps (e.g. map[string]any) are left untouched because
		// the binder cannot safely infer element conversions without mutating
		// caller-provided values. These destinations therefore see a successful
		// no-op parse.
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
				return "", ErrUnmatchedBrackets
			}
			continue
		}

		if err := bb.WriteByte(b); err != nil {
			return "", err //nolint:wrapcheck // unnecessary to wrap it
		}
	}

	if openBracketsCount > 0 {
		return "", ErrUnmatchedBrackets
	}

	return bb.String(), nil
}

func isExported(f *reflect.StructField) bool {
	if f == nil {
		return false
	}
	return f.PkgPath == ""
}

// fieldName returns the lowercased name a field is matched by: the alias from
// its tag, or the Go field name when the tag carries none.
func fieldName(f *reflect.StructField, aliasTag string) string {
	if f == nil {
		return ""
	}

	name := f.Tag.Get(aliasTag)
	if first, _, found := strings.Cut(name, ","); found {
		name = first
	}
	if name == "" {
		name = f.Name
	}

	return utilsstrings.ToLower(name)
}

// fieldInfo describes how the keys of one struct type resolve to fields, so a
// slice field can be told from a scalar sharing a struct with one. Cached per type.
type fieldInfo struct {
	fields      map[string]reflect.Type
	embedded    map[string]reflect.Type
	promoted    map[string]promotedField
	kinds       map[string]reflect.Kind
	nestedKinds map[reflect.Kind]struct{}
}

type promotedField struct {
	typ   reflect.Type
	depth int
}

func unwrapType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}

// fieldCache holds the field information of the struct types bound through
// one alias tag. It is consulted for every key of every bound request, and the
// interface-keyed hash of a sync.Map lookup was most of what that cost, so a
// small direct-mapped cache in front answers a type seen before in a few
// loads. A collision only falls through to the map; nothing is ever wrong,
// just slower.
type fieldCache struct {
	slots [fieldCacheSlots]atomic.Pointer[fieldCacheEntry]
	types sync.Map // reflect.Type -> *fieldInfo
}

type fieldCacheEntry struct {
	info *fieldInfo
	id   uintptr
}

// fieldCacheSlots is a power of two, so a slot is a mask rather than a division.
const fieldCacheSlots = 64

var (
	headerFieldCache     fieldCache
	respHeaderFieldCache fieldCache
	cookieFieldCache     fieldCache
	queryFieldCache      fieldCache
	formFieldCache       fieldCache
	uriFieldCache        fieldCache
)

func getFieldCache(aliasTag string) *fieldCache {
	switch aliasTag {
	case bindingHeader:
		return &headerFieldCache
	case bindingRespHeader:
		return &respHeaderFieldCache
	case bindingCookie:
		return &cookieFieldCache
	case bindingForm:
		return &formFieldCache
	case bindingURI:
		return &uriFieldCache
	case bindingQuery:
		return &queryFieldCache
	}

	panic("unknown alias tag: " + aliasTag)
}

func buildFieldInfo(t reflect.Type, aliasTag string) fieldInfo {
	info := fieldInfo{
		fields:      make(map[string]reflect.Type),
		embedded:    make(map[string]reflect.Type),
		promoted:    make(map[string]promotedField),
		nestedKinds: make(map[reflect.Kind]struct{}),
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !isExported(&f) {
			continue
		}
		fieldType := unwrapType(f.Type)
		name := fieldName(&f, aliasTag)

		if f.Anonymous && fieldType.Kind() == reflect.Struct {
			info.embedded[name] = fieldType
			info.collectPromoted(fieldType, aliasTag, 1, map[reflect.Type]struct{}{t: {}})
			continue
		}

		info.fields[name] = fieldType
		if fieldType.Kind() == reflect.Struct {
			for j := 0; j < fieldType.NumField(); j++ {
				sf := fieldType.Field(j)
				if !isExported(&sf) {
					continue
				}
				info.nestedKinds[unwrapType(sf.Type).Kind()] = struct{}{}
			}
		}
	}

	for _, p := range info.promoted {
		if p.typ != nil {
			info.nestedKinds[p.typ.Kind()] = struct{}{}
		}
	}
	info.resolveKinds()

	return info
}

// resolveKinds records what resolve answers for a key naming a single field, so
// that common lookup costs one map hit rather than a walk down the type. It asks
// resolve rather than reading the maps, keeping the precedence in one place; an
// unresolvable name is kept as reflect.Invalid, which a missing one is not.
func (info *fieldInfo) resolveKinds() {
	info.kinds = make(map[string]reflect.Kind, len(info.fields)+len(info.embedded)+len(info.promoted))
	record := func(name string) {
		if _, done := info.kinds[name]; done {
			return
		}
		kind := reflect.Invalid
		if t, ok := info.resolve(name, false); ok {
			kind = t.Kind()
		}
		info.kinds[name] = kind
	}
	for name := range info.fields {
		record(name)
	}
	for name := range info.embedded {
		record(name)
	}
	for name := range info.promoted {
		record(name)
	}
}

// collectPromoted records the fields of an embedded struct, and of the structs
// it embeds, as promoted fields of the outer type. visited stops cycles, and is
// scoped to the path being walked: a type two sibling branches both embed has
// to be seen through each of them for the alias to come out ambiguous.
func (info *fieldInfo) collectPromoted(t reflect.Type, aliasTag string, depth int, visited map[reflect.Type]struct{}) {
	if _, seen := visited[t]; seen {
		return
	}
	visited[t] = struct{}{}
	defer delete(visited, t)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !isExported(&f) {
			continue
		}
		fieldType := unwrapType(f.Type)
		name := fieldName(&f, aliasTag)

		if f.Anonymous && fieldType.Kind() == reflect.Struct {
			info.collectPromoted(fieldType, aliasTag, depth+1, visited)
			continue
		}

		switch existing, ok := info.promoted[name]; {
		case !ok || existing.depth > depth:
			info.promoted[name] = promotedField{typ: fieldType, depth: depth}
		case existing.depth == depth:
			// Declared at the same depth by two embedded structs: ambiguous.
			info.promoted[name] = promotedField{depth: depth}
		}
	}
}

// typeID identifies a type by the address of its descriptor, which is stable
// and unique for the life of the program. It is the data word of the
// interface, read directly: reflect.ValueOf(t).Pointer() yields the same
// word at several times the cost, and this runs for every key of every
// bound request.
func typeID(t reflect.Type) uintptr {
	return (*[2]uintptr)(unsafe.Pointer(&t))[1] //nolint:gosec // G103: reading the interface's data word, see above
}

// slot returns the direct-mapped slot a type hashes to.
func (c *fieldCache) slot(t reflect.Type) *atomic.Pointer[fieldCacheEntry] {
	return &c.slots[(typeID(t)>>4)&(fieldCacheSlots-1)]
}

// get returns the field information of a struct type, building it on first use.
func (c *fieldCache) get(t reflect.Type, aliasTag string) (*fieldInfo, bool) {
	id := typeID(t)
	slot := &c.slots[(id>>4)&(fieldCacheSlots-1)]
	if e := slot.Load(); e != nil && e.id == id {
		return e.info, true
	}

	val, ok := c.types.Load(t)
	if !ok {
		info := buildFieldInfo(t, aliasTag)
		val, _ = c.types.LoadOrStore(t, &info)
	}

	info, ok := val.(*fieldInfo)
	if ok {
		slot.Store(&fieldCacheEntry{id: id, info: info})
	}
	return info, ok
}

// nextKeySegment returns the next segment of a key in dot, bracket or mixed
// notation ("a.b", "a[b]", "a[b].c") and the remainder; empty segments are skipped.
func nextKeySegment(key string) (segment, rest string, ok bool) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the three results for clarity
	for key != "" {
		i := 0
		for i < len(key) && key[i] != '.' && key[i] != '[' && key[i] != ']' {
			i++
		}
		segment, rest := key[:i], key[i:]
		if rest != "" {
			rest = rest[1:]
		}
		if segment != "" {
			return segment, rest, true
		}
		key = rest
	}

	return "", "", false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// structKeyKind reports whether the field a key names in a struct type has the
// given kind. An unknown first segment falls back to the coarse nested-kind
// check. t is a struct type; equalFieldType has established that.
func structKeyKind(cache *fieldCache, t reflect.Type, kind reflect.Kind, key, aliasTag string) bool {
	segment, rest, ok := nextKeySegment(key)
	if !ok {
		return kind == reflect.Struct
	}
	// A key naming a field of this struct directly is the common case, and
	// resolveKinds already answered it: no walk down the type, and no second
	// pass over the key to find out there is nothing after this segment.
	if rest == "" {
		info, found := cache.get(t, aliasTag)
		if !found {
			return false
		}
		if k, resolved := info.kinds[segment]; resolved && k != reflect.Invalid {
			return k == kind
		}
		_, ok := info.nestedKinds[kind]
		return ok
	}
	// One segment of lookahead: whether a segment has more after it decides how
	// an embedded struct's own alias resolves, and it ends the walk. The kind
	// travels with the type, so each level asks it once.
	cur, curKind := t, reflect.Struct
	first := true
	for {
		nextSegment, nextRest, hasMore := nextKeySegment(rest)

		switch curKind {
		case reflect.Struct:
			info, found := cache.get(cur, aliasTag)
			if !found {
				return false
			}
			next, resolved := info.resolve(segment, hasMore)
			if !resolved {
				if first {
					_, ok := info.nestedKinds[kind]
					return ok
				}
				return false
			}
			cur = next
		case reflect.Slice, reflect.Array:
			// Elements are addressed by index before their fields.
			if !isDigits(segment) {
				return false
			}
			cur = unwrapType(cur.Elem())
		case reflect.Map:
			cur = unwrapType(cur.Elem())
		default:
			// A scalar has nothing below it.
			return false
		}
		curKind = cur.Kind()

		if !hasMore {
			break
		}
		segment, rest = nextSegment, nextRest
		first = false
	}

	return curKind == kind
}

// resolve returns the type a key segment names within this struct: its own
// fields first, then an embedded struct's alias or a promoted field.
func (info *fieldInfo) resolve(segment string, hasMore bool) (reflect.Type, bool) { //nolint:revive // flag-parameter: hasMore is the position of the segment in the key, not a mode switch
	if t, ok := info.fields[segment]; ok {
		return t, true
	}
	if hasMore {
		if t, ok := info.embedded[segment]; ok {
			return t, true
		}
	}
	if p, ok := info.promoted[segment]; ok {
		if p.typ == nil {
			// Ambiguous alias: treated as unresolved.
			return nil, false
		}
		return p.typ, true
	}
	if t, ok := info.embedded[segment]; ok {
		return t, true
	}

	return nil, false
}

// equalFieldType reports whether the value bound under key can hold the pieces
// of a split value: a slice field of a struct, or a slice-valued map.
func equalFieldType(out any, kind reflect.Kind, key, aliasTag string) bool {
	if out == nil {
		return false
	}

	// The type answers everything here; a reflect.Value is only taken to tell
	// a nil map from one the decoder can fill.
	typ := reflect.TypeOf(out)
	typKind := typ.Kind()
	isPointer := typKind == reflect.Pointer
	for typKind == reflect.Pointer {
		typ = typ.Elem()
		typKind = typ.Kind()
	}

	switch typKind {
	case reflect.Map:
		// Only a slice-valued map can hold more than one value per key. A nil
		// map counts when a pointer to it was passed, since the decoder
		// allocates it before filling it; a nil one by value stays unfillable.
		if !isPointer && reflect.ValueOf(out).IsNil() {
			return false
		}
		return typ.Key().Kind() == reflect.String &&
			typ.Elem().Kind() == reflect.Slice && kind == reflect.Slice
	case reflect.Struct:
		// A struct is decoded through a pointer; a plain value cannot be set.
		if !isPointer {
			return false
		}
	default:
		return false
	}

	// Lower the key only once a struct lookup is needed.
	return structKeyKind(getFieldCache(aliasTag), typ, kind, utilsstrings.ToLower(key), aliasTag)
}

// FilterFlags returns the media type value by trimming any parameters from a Content-Type header.
func FilterFlags(content string) string {
	if i := utils.IndexAny2(content, ' ', ';'); i >= 0 {
		return content[:i]
	}
	return content
}

func formatBindData[T, K any](aliasTag string, out any, data map[string][]T, key string, value K, enableSplitting, supportBracketNotation bool) error { //nolint:revive // it's okay
	var err error
	if supportBracketNotation && strings.IndexByte(key, '[') >= 0 {
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
	if enableSplitting && strings.IndexByte(value, ',') >= 0 && equalFieldType(out, reflect.Slice, key, aliasTag) {
		for v := range strings.SplitSeq(value, ",") {
			data[key] = append(data[key], v)
		}
	} else {
		data[key] = append(data[key], value)
	}
}
