package binder

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"mime/multipart"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/gofiber/fiber/v3/internal/headerlist"
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

// ParserType configures a converter for the type of CustomType.
// ConverterE takes precedence over Converter when both are set.
type ParserType struct {
	CustomType any
	Converter  func(string) reflect.Value
	ConverterE func(string) (reflect.Value, error)
}

// tags is used to classify parser's pool; a tag's position is its index in
// decoderPoolSet
var tags = [...]string{bindingHeader, bindingRespHeader, bindingCookie, bindingQuery, bindingForm, bindingURI}

// decoderPoolSet holds the decoder pool of each tag, at the tag's index in
// tags. A set is never written once published: SetParserDecoder publishes a
// new one, so a bind finds its pool with one atomic load instead of the
// read lock every bind used to take, whose shared reader count every core
// contended on.
type decoderPoolSet [len(tags)]*sync.Pool

var (
	// decoderPools is the published decoderPoolSet
	decoderPools atomic.Pointer[decoderPoolSet]
	// decoderPoolsMu serializes SetParserDecoder
	decoderPoolsMu sync.Mutex
)

// tagIndex returns the index of tag in tags, or -1.
func tagIndex(tag string) int {
	switch tag {
	case bindingHeader:
		return 0
	case bindingRespHeader:
		return 1
	case bindingCookie:
		return 2
	case bindingQuery:
		return 3
	case bindingForm:
		return 4
	case bindingURI:
		return 5
	}
	return -1
}

func getDecoderPool(tag string) *sync.Pool {
	if i := tagIndex(tag); i >= 0 {
		return decoderPools.Load()[i]
	}
	panic(fmt.Sprintf("decoder pool not initialized for tag %q", tag))
}

// newDecoderPools returns a decoderPoolSet whose decoders are built with
// parserConfig.
func newDecoderPools(parserConfig ParserConfig) *decoderPoolSet {
	var pools decoderPoolSet
	for i, tag := range tags {
		pools[i] = &sync.Pool{New: func() any {
			return decoderBuilder(tag, parserConfig)
		}}
	}
	return &pools
}

// SetParserDecoder allow globally change the option of form decoder, update decoderPool
func SetParserDecoder(parserConfig ParserConfig) {
	decoderPoolsMu.Lock()
	defer decoderPoolsMu.Unlock()

	decoderPools.Store(newDecoderPools(parserConfig))
}

func decoderBuilder(aliasTag string, parserConfig ParserConfig) any {
	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(parserConfig.IgnoreUnknownKeys)
	// Bake the per-source tag once (pools are keyed per tag); doing it per
	// request reset the decoder's type cache. ParserConfig.SetAliasTag is not
	// honored here: one global tag is incompatible with per-source binding.
	decoder.SetAliasTag(aliasTag)
	for _, v := range parserConfig.ParserType {
		customType := reflect.ValueOf(v.CustomType).Interface()
		if v.ConverterE != nil {
			decoder.RegisterConverterE(customType, v.ConverterE)
		} else {
			decoder.RegisterConverter(customType, v.Converter)
		}
	}
	decoder.ZeroEmpty(parserConfig.ZeroEmpty)
	return &pooledDecoder{Decoder: decoder, types: make(map[reflect.Type]*boundType)}
}

// pooledDecoder is a pooled schema.Decoder and the nested keys it was given: schema
// caches a parsed path per destination type and key spelling for the decoder's lifetime.
type pooledDecoder struct {
	*schema.Decoder
	types    map[reflect.Type]*boundType
	bound    *boundType // the current bind's destination, set by its first nested key
	keyBytes int
	elements int // counted elements summed over the current bind
}

// boundType is what a pooledDecoder keeps per destination type.
type boundType struct {
	keys  map[string]int // nested key -> its counted elements
	shape bindShape
}

// A decoder whose cached keys, charged with what schema and the decoder keep per
// key, pass maxDecoderKeyBytes is left to the GC instead of being pooled again.
const (
	maxDecoderKeyBytes = 4 << 20 // about 5,000 form keys; a form past it rebuilds schema's path cache every bind
	cachedKeyOverhead  = 320
	cachedSegmentBytes = 160
)

// cachedKeyBytes is what caching key costs; schema keeps a path hop per segment.
func cachedKeyBytes(key string) int {
	return 2*len(key) + cachedKeyOverhead + cachedSegmentBytes*(strings.Count(key, ".")+1)
}

// admit checks key before d decodes it into out and adds its counted elements to d.elements.
func (d *pooledDecoder) admit(out any, key string) error {
	if err := checkKeyDepth(key); err != nil {
		return err
	}
	// schema never caches a key without a dot.
	if strings.IndexByte(key, '.') < 0 {
		return nil
	}
	if d.bound == nil {
		d.bound = d.boundType(reflect.TypeOf(out))
	}
	n, ok := d.bound.keys[key]
	if !ok {
		n = d.bound.shape.elements(key)
		d.bound.keys[strings.Clone(key)] = n
		d.keyBytes += cachedKeyBytes(key)
	}
	d.elements += n
	return nil
}

func (d *pooledDecoder) boundType(t reflect.Type) *boundType {
	b, ok := d.types[t]
	if !ok {
		b = &boundType{keys: make(map[string]int), shape: shapeOf(t)}
		d.types[t] = b
	}
	return b
}

// checkElements refuses the admitted keys, which keys yields again, when their
// counted indexes can make schema allocate more than maxNestedBytes.
func (d *pooledDecoder) checkElements(keys iter.Seq[string]) error {
	if d.elements == 0 {
		return nil
	}
	limit := maxNestedBytes / int(d.bound.shape.elemSize)
	if d.elements > limit && d.distinctElements(keys, limit) > limit {
		return ErrTooManyElements
	}
	return nil
}

// distinctElements is d.elements with each distinct prefix up to an index counted once,
// since a slice grows at most once per index; it stops once the sum passes limit.
func (d *pooledDecoder) distinctElements(keys iter.Seq[string], limit int) int {
	seen := make(map[string]struct{})
	n := 0
	for key := range keys {
		if d.bound.keys[key] == 0 {
			continue
		}
		for end, elems := range d.bound.shape.indexes(key) {
			if _, dup := seen[key[:end]]; dup {
				continue
			}
			seen[key[:end]] = struct{}{}
			if n += elems; n > limit {
				return n
			}
		}
	}
	return n
}

func putDecoder(pool *sync.Pool, d *pooledDecoder) {
	d.bound, d.elements = nil, 0
	if d.keyBytes <= maxDecoderKeyBytes {
		pool.Put(d)
	}
}

// maxNestedBytes bounds the slice elements the counted indexes of a bind's keys may allocate.
const maxNestedBytes = 8 << 20

// maxSliceIndex is schema's largest slice index; it refuses a key with a larger one.
const maxSliceIndex = 1000

// bindShape is what the element budget needs to know about a destination type.
type bindShape struct {
	// elemSize is the largest element of a slice a counted index can grow, 0 if none.
	elemSize uintptr
	// cyclic is set when a field leads back to a type on its path without passing
	// a slice, so the index-free paths to a slice have no bound.
	cyclic bool
}

// elements is how many slice elements the counted indexes of key can make schema allocate.
func (s bindShape) elements(key string) int {
	if s.elemSize == 0 {
		return 0
	}
	n := 0
	for _, elems := range s.indexes(key) {
		n += elems
	}
	return n
}

// indexes yields where each counted index of key ends and the length it grows its slice to.
// A first index is free after a root field or in an acyclic type: the type bounds its slices.
func (s bindShape) indexes(key string) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		end, pos, first := -1, 0, true
		for seg := range strings.SplitSeq(key, ".") {
			end += len(seg) + 1
			pos++
			// Checked first since ParseInt allocates the error for a field name.
			if pos == 1 || seg == "" || (seg[0] < '0' || seg[0] > '9') && seg[0] != '+' && seg[0] != '-' {
				continue
			}
			i, err := utils.ParseInt(seg)
			if err != nil || i < 0 {
				continue
			}
			if first {
				first = false
				if pos == 2 || !s.cyclic {
					continue
				}
			}
			if !yield(end, int(min(i, maxSliceIndex))+1) {
				return
			}
		}
	}
}

func shapeOf(t reflect.Type) bindShape {
	if t == nil {
		return bindShape{}
	}
	t = unwrapType(t)
	if t.Kind() != reflect.Struct {
		return bindShape{}
	}
	w := shapeWalk{path: make(map[reflect.Type]bool), inElem: make(map[reflect.Type]bool)}
	w.indexFree(t)
	s := bindShape{elemSize: w.nested, cyclic: w.cyclic}
	if s.cyclic {
		s.elemSize = max(s.elemSize, w.direct)
	}
	return s
}

// shapeWalk collects a bindShape from the struct types below a root.
type shapeWalk struct {
	path   map[reflect.Type]bool // reached without an index: true while on the path
	inElem map[reflect.Type]bool // reached through a slice element
	direct uintptr               // largest element of a slice reached without an index
	nested uintptr               // largest element of a slice inside a slice element
	cyclic bool
}

func (w *shapeWalk) indexFree(t reflect.Type) {
	w.path[t] = true
	for i := range t.NumField() {
		elem, size, slice := structElem(t.Field(i).Type)
		if elem == nil {
			continue
		}
		onPath, seen := w.path[elem]
		switch {
		case slice:
			w.direct = max(w.direct, size)
			w.inElement(elem)
		case onPath:
			w.cyclic = true
		case !seen:
			w.indexFree(elem)
		}
	}
	w.path[t] = false
}

func (w *shapeWalk) inElement(t reflect.Type) {
	if w.inElem[t] {
		return
	}
	w.inElem[t] = true
	for i := range t.NumField() {
		elem, size, slice := structElem(t.Field(i).Type)
		if elem == nil {
			continue
		}
		if slice {
			w.nested = max(w.nested, size)
		}
		w.inElement(elem)
	}
}

// structElem returns the struct type schema walks into through a field of type t and,
// for a slice, the bytes each element it grows can take. Maps end a path.
func structElem(t reflect.Type) (reflect.Type, uintptr, bool) {
	t = unwrapType(t)
	var size uintptr
	slice := t.Kind() == reflect.Slice
	if slice {
		e := t.Elem()
		size = e.Size()
		t = unwrapType(e)
		if e != t {
			size += t.Size() // a pointer element is allocated once a key reaches it
		}
	}
	if t.Kind() == reflect.Array {
		t = unwrapType(t.Elem())
	}
	if t.Kind() != reflect.Struct {
		return nil, 0, false
	}
	return t, size, slice
}

func init() {
	decoderPools.Store(newDecoderPools(ParserConfig{
		IgnoreUnknownKeys: true,
		ZeroEmpty:         true,
	}))
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

// maxKeySegments is the most segments a key decoded into a struct may have
// ("a.0.b" has three). The decoder recurses and walks prefixes per segment, and
// a type that contains itself puts no other bound on the depth.
const maxKeySegments = 64

// checkKeyDepth refuses a key of more than maxKeySegments segments. Map
// destinations never reach the decoder, so their keys are not limited.
func checkKeyDepth(key string) error {
	// A key shorter than the limit cannot hold that many dots.
	if len(key) >= maxKeySegments && strings.Count(key, ".") >= maxKeySegments {
		return ErrKeyTooDeep
	}
	return nil
}

// Parse data into the struct with gofiber/schema
func parseToStruct(aliasTag string, out any, data map[string][]string, files ...map[string][]*multipart.FileHeader) error {
	// Only a file part fills a *multipart.FileHeader, see fileHeaders.
	if g := guardFileHeaders(out, files); g != nil {
		defer g.restore(out)
	}

	// Get decoder from pool
	pool := getDecoderPool(aliasTag)
	schemaDecoder := pool.Get().(*pooledDecoder) //nolint:errcheck,forcetypeassert // not needed
	defer putDecoder(pool, schemaDecoder)

	for key := range formKeys(data, files) {
		if err := schemaDecoder.admit(out, key); err != nil {
			return err
		}
	}
	if err := schemaDecoder.checkElements(formKeys(data, files)); err != nil {
		return err
	}

	// Alias tag is baked in at build time (see decoderBuilder); setting it here
	// would reset the decoder's type cache on every request.
	if err := schemaDecoder.Decode(out, data, files...); err != nil {
		return err //nolint:wrapcheck // a wrap adds no context and costs a formatted message per failed bind
	}

	return nil
}

// formKeys yields the keys of data and of each of files.
func formKeys(data map[string][]string, files []map[string][]*multipart.FileHeader) iter.Seq[string] {
	return func(yield func(string) bool) {
		for key := range data {
			if !yield(key) {
				return
			}
		}
		for _, f := range files {
			for key := range f {
				if !yield(key) {
					return
				}
			}
		}
	}
}

// parseValuesToStruct is parseToStruct for key/value pairs: values[i] is a
// value of keys[i]. The keys of files are among them, with an empty value.
func parseValuesToStruct(aliasTag string, out any, keys, values []string, files ...map[string][]*multipart.FileHeader) error {
	g := guardFileHeaders(out, files)
	if g != nil {
		defer g.restore(out)
	}

	pool := getDecoderPool(aliasTag)
	schemaDecoder := pool.Get().(*pooledDecoder) //nolint:errcheck,forcetypeassert // not needed
	defer putDecoder(pool, schemaDecoder)

	for _, key := range keys {
		if err := schemaDecoder.admit(out, key); err != nil {
			return err
		}
	}
	if err := schemaDecoder.checkElements(slices.Values(keys)); err != nil {
		return err
	}

	err := schemaDecoder.DecodeValues(out, keys, values)
	// Only a destination the guard walks has a field that takes a file.
	if g != nil && len(files) > 0 && len(files[0]) > 0 {
		err = unfilled(err, schemaDecoder.decodeFiles(g, out, keys, files[0]))
	}
	return err
}

// unfilled returns err, what DecodeValues reported for the pairs, without the empty required fields a file fills:
// those the decode of the files alone, filesErr, does not report. DecodeValues takes no files, so it reports them all.
func unfilled(err, filesErr error) error {
	var missing schema.MultiError
	if !errors.As(err, &missing) {
		return err
	}
	var stillMissing schema.MultiError
	if filesErr != nil && !errors.As(filesErr, &stillMissing) {
		return err
	}
	for key, e := range missing {
		var empty schema.EmptyFieldError
		if errors.As(e, &empty) && stillMissing[key] == nil {
			delete(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return missing
}

// decodeFiles decodes files into a scratch value, the keys of one field in the order of keys, and sets the file fields
// of out from it: decoded into out, the files would rerun the defaults and required checks the pairs have done. It
// returns what the decode of all files reports.
func (d *pooledDecoder) decodeFiles(g *fileHeaders, out any, keys []string, files map[string][]*multipart.FileHeader) error {
	dst := reflect.ValueOf(out).Elem()
	if dst.Kind() != reflect.Struct {
		return nil
	}
	scratch := reflect.New(dst.Type())
	if behindUnexportedPointer(dst.Type()) {
		allocEmbedded(scratch.Elem(), dst, nil)
	}
	// All at once first, which grows each slice the keys index to its length in one step.
	err := d.Decode(scratch.Interface(), nil, files)
	if len(files) > 1 {
		one := acquireFileHeaderMap()
		defer releaseFileHeaderMap(one)
		for _, key := range keys {
			if headers, ok := files[key]; ok {
				one[key] = headers
				d.Decode(scratch.Interface(), nil, one) //nolint:errcheck // reported by the decode of the pairs
				delete(one, key)
			}
		}
	}
	g.setFiles(dst, scratch.Elem())
	return err //nolint:wrapcheck // judged by unfilled, not returned as is
}

// unexportedPointers caches behindUnexportedPointer per struct type.
var unexportedPointers sync.Map // map[reflect.Type]bool

// behindUnexportedPointer reports whether the struct type t holds a file field under an unexported embedded pointer,
// through struct values and pointers: schema follows one that is set but cannot allocate it, so a scratch value
// needs one set for it.
func behindUnexportedPointer(t reflect.Type) bool {
	if cached, ok := unexportedPointers.Load(t); ok {
		if found, isBool := cached.(bool); isBool {
			return found
		}
	}
	found := reachesUnexportedPointer(t, nil)
	unexportedPointers.Store(t, found)
	return found
}

// reachesUnexportedPointer is behindUnexportedPointer for t, skipping the types in seen, which a recursive type
// leads back to.
func reachesUnexportedPointer(t reflect.Type, seen []reflect.Type) bool {
	if slices.Contains(seen, t) {
		return false
	}
	seen = append(seen, t)
	return slices.ContainsFunc(headerFields(t), func(i int) bool {
		f := t.Field(i)
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			if !f.IsExported() {
				return true
			}
			ft = ft.Elem()
		}
		return ft.Kind() == reflect.Struct && reachesUnexportedPointer(ft, seen)
	})
}

// allocEmbedded gives scratch a zero pointee of its own for each pointer to a struct dst has set, so that an unexported
// embedded one is set in scratch too. The files decoded into them reach dst through setFiles, and the defaults the
// decode sets there go nowhere. seen holds the pointers followed so far, which a cycle in dst leads back to.
func allocEmbedded(scratch, dst reflect.Value, seen []uintptr) {
	for _, i := range headerFields(dst.Type()) {
		to, from := scratch.Field(i), dst.Field(i)
		switch {
		case from.Kind() == reflect.Struct:
			allocEmbedded(to, from, seen)
		case from.Kind() == reflect.Pointer && from.Type() != fileHeaderPtrType && !from.IsNil() &&
			from.Elem().Kind() == reflect.Struct && !slices.Contains(seen, from.Pointer()):
			p := reflect.New(from.Type().Elem())
			if to.CanSet() {
				to.Set(p)
			} else {
				reflect.NewAt(to.Type(), unsafe.Pointer(to.UnsafeAddr())).Elem().Set(p) //nolint:gosec // reflect cannot set the unexported field; scratch is our own value
			}
			allocEmbedded(p.Elem(), from.Elem(), append(seen, from.Pointer()))
		default:
		}
	}
}

var fileHeaderSliceType = reflect.SliceOf(fileHeaderPtrType)

// setFiles sets each field of to that takes files to what it holds in from, a value of the same type, where that is
// the header of a file part. What to does not hold, a nil pointer or an element past the end of a slice, is skipped.
func (g *fileHeaders) setFiles(to, from reflect.Value) {
	switch t := from.Type(); {
	case t == fileHeaderPtrType || t == fileHeaderSliceType || t.Kind() == reflect.Pointer && t.Elem() == fileHeaderSliceType:
		if g.holdsFile(from) && to.CanSet() {
			to.Set(from)
		}
	case t.Kind() == reflect.Pointer:
		if !from.IsNil() && !to.IsNil() {
			g.setFiles(to.Elem(), from.Elem())
		}
	case t.Kind() == reflect.Struct:
		for _, i := range headerFields(t) {
			g.setFiles(to.Field(i), from.Field(i))
		}
	case t.Kind() == reflect.Slice || t.Kind() == reflect.Array:
		for i := range min(to.Len(), from.Len()) {
			g.setFiles(to.Index(i), from.Index(i))
		}
	default:
	}
}

// holdsFile reports whether v, a *multipart.FileHeader or a slice of them or a pointer to one, holds the header of
// a file part. Schema sets all the files of a key or none, so the first header of a slice answers for it.
func (g *fileHeaders) holdsFile(v reflect.Value) bool {
	if v.Type() != fileHeaderPtrType {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return false
			}
			v = v.Elem()
		}
		if v.Len() == 0 {
			return false
		}
		v = v.Index(0)
	}
	if !v.CanInterface() {
		return false
	}
	fh, ok := reflect.TypeAssert[*multipart.FileHeader](v)
	return ok && g.known(fh)
}

// parseLast is parse for a map[string]string destination, from the last value
// filed under each key, which is all parseToMap keeps of a key's values. It
// handles the two types bindModeFor gives bindLast as parseToMap would, and a
// nil pointer to one as parse does, by handing it to the struct decoder.
func parseLast(aliasTag string, out any, last map[string]string) error {
	switch m := out.(type) {
	case *map[string]string:
		if m != nil {
			if *m == nil {
				*m = make(map[string]string, len(last))
			}
			maps.Copy(*m, last)
			return nil
		}
	case map[string]string:
		if m == nil {
			return ErrMapNilDestination
		}
		maps.Copy(m, last)
		return nil
	}
	return parse(aliasTag, out, nil)
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
		newMap, ok := reflect.TypeAssert[map[string][]string](target)
		if !ok {
			return ErrMapNotConvertible
		}

		// The value slices are the caller's to keep: the binders append them
		// as any slice is appended, and drop them from their pooled map on
		// release (see bindData).
		maps.Copy(newMap, data)
	case reflect.String:
		newMap, ok := reflect.TypeAssert[map[string]string](target)
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

// visible reports whether the keys of aliasTag reach through f: an exported field, and for the cookie name checks an
// embedded struct of an unexported type too, whose exported fields schema promotes.
func visible(f *reflect.StructField, aliasTag string) bool {
	return isExported(f) || aliasTag == bindingCookie && f.Anonymous && unwrapType(f.Type).Kind() == reflect.Struct
}

// fieldName returns the lowercased name a field is matched by: the alias from
// its tag, or the Go field name when the tag carries none.
func fieldName(f *reflect.StructField, aliasTag string) string {
	if f == nil {
		return ""
	}

	name := tagName(f, aliasTag)
	if name == "" {
		name = f.Name
	}

	return utilsstrings.ToLower(name)
}

// tagName returns the alias in f's tag as written, or "" when the tag carries none.
func tagName(f *reflect.StructField, aliasTag string) string {
	name := f.Tag.Get(aliasTag)
	if first, _, found := utils.CutByte(name, ','); found {
		name = first
	}
	return name
}

// fieldInfo describes how the keys of one struct type resolve to fields, so a
// slice field can be told from a scalar sharing a struct with one. Cached per type.
type fieldInfo struct {
	fields      map[string]reflect.Type
	embedded    map[string]reflect.Type
	promoted    map[string]promotedField
	kinds       map[string]reflect.Kind
	nestedKinds map[reflect.Kind]struct{}
	cookie      atomic.Pointer[cookieNames] // built on the first cookie bind into the type
	exact       []string                    // the aliases as written of the tagged fields a one-segment key resolves to
	embeds      []string                    // the lowercased names of the embedded structs at any depth
	containers  []namedField                // the struct, slice of struct and map fields a one-segment key resolves to
	id          uintptr                     // typeID of the type it describes, which the slot of the cache is checked against
}

type namedField struct {
	typ     reflect.Type // the struct, or the element struct of a slice or array; nil for a map
	name    string       // the alias as written, or the lowercased name of an untagged field
	indexed bool
}

type promotedField struct {
	typ   reflect.Type
	alias string // as written, "" for an untagged field
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
// just slower, and a slot is filled with the field information the map holds,
// so it costs no allocation.
type fieldCache struct {
	slots [fieldCacheSlots]atomic.Pointer[fieldInfo]
	types sync.Map // reflect.Type -> *fieldInfo
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

func buildFieldInfo(t reflect.Type, aliasTag string) *fieldInfo {
	info := &fieldInfo{
		fields:      make(map[string]reflect.Type),
		embedded:    make(map[string]reflect.Type),
		promoted:    make(map[string]promotedField),
		nestedKinds: make(map[reflect.Kind]struct{}),
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !visible(&f, aliasTag) {
			continue
		}
		fieldType := unwrapType(f.Type)
		name := fieldName(&f, aliasTag)

		if f.Anonymous && fieldType.Kind() == reflect.Struct {
			info.embedded[name] = fieldType
			info.embeds = append(info.embeds, name)
			info.collectPromoted(fieldType, aliasTag, 1, map[reflect.Type]struct{}{t: {}})
			continue
		}

		info.fields[name] = fieldType
		info.addNamed(name, tagName(&f, aliasTag), fieldType)
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

	for name, p := range info.promoted {
		if p.typ == nil {
			continue
		}
		info.nestedKinds[p.typ.Kind()] = struct{}{}
		if _, shadowed := info.fields[name]; !shadowed {
			info.addNamed(name, p.alias, p.typ)
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
		if !visible(&f, aliasTag) {
			continue
		}
		fieldType := unwrapType(f.Type)
		name := fieldName(&f, aliasTag)

		if f.Anonymous && fieldType.Kind() == reflect.Struct {
			info.embeds = append(info.embeds, name)
			info.collectPromoted(fieldType, aliasTag, depth+1, visited)
			continue
		}

		switch existing, ok := info.promoted[name]; {
		case !ok || existing.depth > depth:
			info.promoted[name] = promotedField{typ: fieldType, alias: tagName(&f, aliasTag), depth: depth}
		case existing.depth == depth:
			// Declared at the same depth by two embedded structs: ambiguous.
			info.promoted[name] = promotedField{depth: depth}
		}
	}
}

// addNamed records a field a one-segment key resolves to for the cookie name checks.
func (info *fieldInfo) addNamed(name, alias string, t reflect.Type) {
	if alias != "" {
		info.exact = append(info.exact, alias)
		name = alias
	}
	indexed := t.Kind() == reflect.Slice || t.Kind() == reflect.Array
	if indexed {
		t = unwrapType(t.Elem())
	}
	switch t.Kind() { //nolint:exhaustive // the kinds a dotted key walks into
	case reflect.Struct:
		info.containers = append(info.containers, namedField{typ: t, name: name, indexed: indexed})
	case reflect.Map:
		info.containers = append(info.containers, namedField{name: name})
	}
}

// spelling reports whether a key segment is the alias of a tagged field as written, or one only ignoring case.
func (info *fieldInfo) spelling(segment string) (exact, folded bool) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the two results
	for _, alias := range info.exact {
		if len(alias) != len(segment) {
			continue
		}
		if alias == segment {
			return true, false
		}
		folded = folded || utils.EqualFold(alias, segment)
	}
	return false, folded
}

// takesCookie reports whether a cookie name reaches tagged fields only by their aliases as written, as Ctx.Cookies
// reads it: schema folds every segment of a dotted name and reaches a promoted field through its embedded struct.
func (c *fieldCache) takesCookie(info *fieldInfo, name string) bool {
	if exact, folded := info.spelling(name); exact || folded {
		return exact
	}
	if len(info.containers) == 0 && len(info.embeds) == 0 || strings.IndexByte(name, '.') < 0 {
		return true
	}
	for {
		segment, rest, _ := strings.Cut(name, ".")
		if _, folded := info.spelling(segment); folded {
			return false
		}
		i := slices.IndexFunc(info.containers, func(f namedField) bool { return utils.EqualFold(f.name, segment) })
		if i < 0 {
			// A path through an embedded struct's name is skipped, unless a field of that name shadows the struct.
			return !slices.ContainsFunc(info.embeds, func(embed string) bool { return utils.EqualFold(embed, segment) })
		}
		if info.containers[i].typ == nil || rest == "" {
			// Past a map field the rest is its key, read as written.
			return true
		}
		next := info.containers[i]
		if next.indexed {
			// The decoder reads 00 or +0 as index 0 too.
			index, after, _ := strings.Cut(rest, ".")
			if !isDigits(index) || len(index) > 1 && index[0] == '0' {
				return false
			}
			rest = after
		}
		var ok bool
		if info, ok = c.get(next.typ, bindingCookie); !ok || rest == "" {
			return true
		}
		name = rest
	}
}

// cookieNames are the cookie names a bind into one struct type takes without a walk through takesCookie: the
// aliases as written and the paths to them through nested structs.
type cookieNames struct {
	info  *fieldInfo
	exact []string
	bits  uint64 // the nameBit of each exact name: a name whose bit is clear is none of them in any case
	flat  bool   // no nested or embedded struct, so a dotted name reaches no field through one
	// closed is flat with every field tagged: a name that is none of the exact names binds no field.
	closed bool
}

// nameBit is a bit of the length and the first byte of s that every spelling of s in another case shares, as folding
// changes only the 0x20 bit of a letter.
func nameBit(s string) uint64 {
	if s == "" {
		return 1
	}
	return 1 << ((uint(len(s)) + uint(s[0]|0x20)) & 63)
}

// cookieNames returns the cookie names of the struct type t, or nil when t is no struct or has no tagged, nested or
// embedded field, so that every name binds.
func (c *fieldCache) cookieNames(t reflect.Type) *cookieNames {
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	info, ok := c.get(t, bindingCookie)
	if !ok || len(info.exact) == 0 && len(info.containers) == 0 && len(info.embeds) == 0 {
		return nil
	}
	if n := info.cookie.Load(); n != nil {
		return n
	}
	n := &cookieNames{info: info, flat: len(info.containers) == 0 && len(info.embeds) == 0}
	n.closed = n.flat
	for i := range t.NumField() {
		if f := t.Field(i); f.Anonymous || f.IsExported() && tagName(&f, bindingCookie) == "" {
			n.closed = false
		}
	}
	n.collect(c, info, "", &[]*fieldInfo{})
	info.cookie.Store(n)
	return n
}

// collect adds the aliases of at, reached through prefix, that takesCookie takes, and those of the structs nested in
// at. A struct reached through a second path is not expanded again; its names are walked.
func (n *cookieNames) collect(c *fieldCache, at *fieldInfo, prefix string, expanded *[]*fieldInfo) {
	if slices.Contains(*expanded, at) {
		return
	}
	*expanded = append(*expanded, at)
	for _, alias := range at.exact {
		if name := prefix + alias; c.takesCookie(n.info, name) {
			n.exact = append(n.exact, name)
			n.bits |= nameBit(name)
		}
	}
	for _, f := range at.containers {
		if f.typ != nil && !f.indexed {
			if next, ok := c.get(f.typ, bindingCookie); ok {
				n.collect(c, next, prefix+f.name+".", expanded)
			}
		}
	}
}

// index returns the position of name among the exact names, or -1. n may be nil.
func (n *cookieNames) index(name string) int {
	if n == nil || n.bits&nameBit(name) == 0 {
		return -1
	}
	return slices.Index(n.exact, name)
}

// unknown reports whether name, which is none of the exact names, binds no field of a closed type in any spelling.
func (n *cookieNames) unknown(name string) bool {
	return n.closed && n.bits&nameBit(name) == 0
}

// takes reports whether a bind takes the cookie name as Ctx.Cookies reads it, and records it in d to take only the
// first of a repeated name. i is its index among the exact names, or -1; CookieBinding.Bind marks the first 64 itself.
func (n *cookieNames) takes(name string, i int, d *bindData) bool {
	switch {
	case i >= 0:
		return d.firstOf(name)
	case !n.check(name):
		return false
	case n.closed:
		// Binding no field, it need not be the first.
		return true
	default:
		return d.firstOf(name)
	}
}

// check reports whether takesCookie takes a name that is none of the exact names.
func (n *cookieNames) check(name string) bool {
	if n.bits&nameBit(name) != 0 {
		for _, exact := range n.exact {
			if len(exact) == len(name) && utils.EqualFold(exact, name) {
				return cookieFieldCache.takesCookie(n.info, name)
			}
		}
	}
	// No alias is name in another case, which leaves only a dotted name to the walk.
	return n.flat || strings.IndexByte(name, '.') < 0 || cookieFieldCache.takesCookie(n.info, name)
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
func (c *fieldCache) slot(t reflect.Type) *atomic.Pointer[fieldInfo] {
	return &c.slots[(typeID(t)>>4)&(fieldCacheSlots-1)]
}

// get returns the field information of a struct type, building it on first use.
func (c *fieldCache) get(t reflect.Type, aliasTag string) (*fieldInfo, bool) {
	id := typeID(t)
	slot := &c.slots[(id>>4)&(fieldCacheSlots-1)]
	if info := slot.Load(); info != nil && info.id == id {
		return info, true
	}

	val, ok := c.types.Load(t)
	if !ok {
		info := buildFieldInfo(t, aliasTag)
		info.id = id
		val, _ = c.types.LoadOrStore(t, info)
	}

	info, ok := val.(*fieldInfo)
	if ok {
		slot.Store(info)
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
			// The decoder reads the rest of the key as one map key.
			return unwrapType(cur.Elem()).Kind() == kind
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
// The media type ends at the ';' or the space or tab before it, as fasthttp's form readers end it.
func FilterFlags(content string) string {
	if i := utils.IndexAny3(content, ' ', '\t', ';'); i >= 0 {
		return content[:i]
	}
	return content
}

// bindKey returns key as the binders file it: rewritten from bracket notation
// to dots when the source supports that notation and key uses it.
func bindKey(key string, supportBracketNotation bool) (string, error) { //nolint:revive // the flag is the source's setting
	if supportBracketNotation && strings.IndexByte(key, '[') >= 0 {
		return parseParamSquareBrackets(key)
	}
	return key, nil
}

// bindData is what a string binder files the values it reads under, pooled
// so that filing a value costs no allocation of its own. What it keeps
// depends on the destination, see bindMode: a struct is decoded straight from
// the key/value pairs in the order read, a map[string]string gets the last
// value of each key, and any other map gets the source map parseToMap copies.
type bindData struct {
	// values is the source map, for a map destination other than a
	// map[string]string, and for a multipart form
	values map[string][]string
	// last is the last value filed under each key, for a map[string]string
	// destination
	last map[string]string
	// seen is the names a source whose first value wins has filed past the
	// first len(few), see firstOf
	seen map[string]struct{}
	// few is the first names firstOf saw, nFew of them
	few [8]string
	// keys and pairValues are the pairs, for a struct destination
	keys, pairValues []string
	// names is the sorted keys of a multipart form's files or values
	names []string
	// bound is the first 64 exact cookie names bound so far, a bit per index, see cookieNames
	bound uint64
	nFew  int
	mode  bindMode
}

// bindMode is how a bindData keeps what is filed in it.
type bindMode uint8

const (
	// bindPairs keeps key/value pairs for schema.Decoder.DecodeValues, which
	// groups them itself: nothing has to build or hash a map.
	bindPairs bindMode = iota
	// bindMap keeps a map of the values filed under each key, appended as
	// any slice is: a map of slices keeps those slices (see parseToMap).
	bindMap
	// bindLast keeps the last value filed under each key, which is all
	// parseToMap puts in a map[string]string: its other values need no
	// slice to be kept in.
	bindLast
)

// bindModeFor returns how a bind into out keeps its values. It dispatches as
// parse does: a map with string keys, or a pointer to one, gets a map, and
// everything else is decoded as a struct.
func bindModeFor(out any) bindMode {
	mode, _ := bindTarget(out)
	return mode
}

// bindTarget returns bindModeFor's mode and, for one it dispatched through
// reflection, the type out points to.
func bindTarget(out any) (bindMode, reflect.Type) {
	switch out.(type) {
	case *map[string][]string, map[string][]string:
		return bindMap, nil
	case *map[string]string, map[string]string:
		return bindLast, nil
	}
	t := reflect.TypeOf(out)
	if t == nil {
		return bindPairs, nil
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Map || t.Key().Kind() != reflect.String {
		return bindPairs, t
	}
	return bindMap, t
}

// parse decodes what was filed into out.
func (d *bindData) parse(aliasTag string, out any) error {
	switch d.mode {
	case bindPairs:
		return parseValuesToStruct(aliasTag, out, d.keys, d.pairValues)
	case bindLast:
		return parseLast(aliasTag, out, d.last)
	default:
		return parse(aliasTag, out, d.values)
	}
}

// bind files value under key: a key in bracket notation is rewritten to dots
// first when supportBracketNotation is set, and a comma-separated value is
// split when splitting is on and the key names a slice.
func (d *bindData) bind(aliasTag string, out any, key, value string, enableSplitting, supportBracketNotation bool) error { //nolint:revive // the flags are the binder's settings
	key, err := bindKey(key, supportBracketNotation)
	if err != nil {
		return err
	}
	d.split(aliasTag, out, key, value, enableSplitting)
	return nil
}

// bindFiles files a multipart form's headers under key, rewritten from bracket notation as value keys are. A struct
// also gets the key as a pair with an empty value, as Decode reads the key of a file.
func (d *bindData) bindFiles(files map[string][]*multipart.FileHeader, key string, headers []*multipart.FileHeader) error {
	key, err := bindKey(key, true)
	if err != nil {
		return err
	}
	if _, filed := files[key]; !filed && d.mode == bindPairs {
		d.add(key, "")
	}
	// Of the names that spell one key, the one that sorts last wins, as among the values.
	files[key] = headers
	return nil
}

// maxSplitKeyBytes is the longest key whose value is split: every piece is filed
// under the whole key, so splitting costs the key's length times its commas.
const maxSplitKeyBytes = 1 << 10

// split files value under key, split at its commas when splitting is on and
// the key, at most maxSplitKeyBytes long, names a slice of out's.
func (d *bindData) split(aliasTag string, out any, key, value string, enableSplitting bool) { //nolint:revive // the flag is the binder's setting
	if enableSplitting && len(key) <= maxSplitKeyBytes && strings.IndexByte(value, ',') >= 0 &&
		equalFieldType(out, reflect.Slice, key, aliasTag) {
		// A header value is a comma-separated list (RFC 9110 Section 5.6.1): its
		// elements carry no surrounding OWS, and empty ones are ignored. Quoted
		// strings are not parsed, so a comma inside one still splits.
		list := aliasTag == bindingHeader || aliasTag == bindingRespHeader
		added := false
		for v := range strings.SplitSeq(value, ",") {
			if list {
				if v = headerlist.TrimOWS(v); v == "" {
					continue
				}
			}
			d.add(key, v)
			added = true
		}
		if !added {
			// Only empty elements: bind it as an empty value, so that the header
			// still replaces what was bound.
			d.add(key, "")
		}
		return
	}
	d.add(key, value)
}

// firstOf reports whether name is new to this bind, and records it. The first names are scanned in few, which costs
// less than hashing them; past len(few) the map holds them all, so a long list stays linear.
func (d *bindData) firstOf(name string) bool {
	if len(d.seen) == 0 {
		if slices.Contains(d.few[:d.nFew], name) {
			return false
		}
		if d.nFew < len(d.few) {
			d.few[d.nFew] = name
			d.nFew++
			return true
		}
		if d.seen == nil {
			d.seen = make(map[string]struct{}, 2*len(d.few))
		}
		for _, f := range d.few {
			d.seen[f] = struct{}{}
		}
	}
	n := len(d.seen)
	d.seen[name] = struct{}{}
	return len(d.seen) > n
}

// add appends value to the values of key.
func (d *bindData) add(key, value string) {
	switch d.mode {
	case bindPairs:
		d.keys = append(d.keys, key)
		d.pairValues = append(d.pairValues, value)
	case bindLast:
		d.last[key] = value
	default:
		d.values[key] = append(d.values[key], value)
	}
}
