package binder

import (
	"maps"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"reflect"
	"slices"
	"sync"
)

var (
	fileHeaderType    = reflect.TypeFor[multipart.FileHeader]()
	fileHeaderPtrType = reflect.PointerTo(fileHeaderType)
)

// fileHeaderTypes caches, per destination type, whether it holds a *multipart.FileHeader anywhere.
var fileHeaderTypes sync.Map // map[reflect.Type]bool

// holdsFileHeader reports whether a value of type t can hold a *multipart.FileHeader.
func holdsFileHeader(t reflect.Type) bool {
	if cached, ok := fileHeaderTypes.Load(t); ok {
		if held, isBool := cached.(bool); isBool {
			return held
		}
	}
	held := containsFileHeader(t, map[reflect.Type]struct{}{})
	fileHeaderTypes.Store(t, held)
	return held
}

func containsFileHeader(t reflect.Type, seen map[reflect.Type]struct{}) bool {
	if t == fileHeaderType {
		return true
	}
	if _, ok := seen[t]; ok {
		return false
	}
	seen[t] = struct{}{}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return containsFileHeader(t.Elem(), seen)
	case reflect.Struct:
		for i := range t.NumField() {
			if f := t.Field(i); reachable(&f) && containsFileHeader(f.Type, seen) {
				return true
			}
		}
	default:
	}
	return false
}

// reachable reports whether schema can fill something through the field f: an exported field, or an embedded
// struct, whose exported fields are promoted even when the name of its type is not exported.
func reachable(f *reflect.StructField) bool {
	return f.IsExported() || f.Anonymous
}

// fileHeaderFields caches, per struct type, the indexes of the fields a walk has to visit.
var fileHeaderFields sync.Map // map[reflect.Type][]int

// headerFields returns the indexes of the fields of the struct type t that schema can fill and through which a
// *multipart.FileHeader can be reached. A walk visits those and skips the rest, which is most of the fields.
func headerFields(t reflect.Type) []int {
	if cached, ok := fileHeaderFields.Load(t); ok {
		if fields, isInts := cached.([]int); isInts {
			return fields
		}
	}
	var fields []int
	for i := range t.NumField() {
		if f := t.Field(i); reachable(&f) && holdsFileHeader(f.Type) {
			fields = append(fields, i)
		}
	}
	fileHeaderFields.Store(t, fields)
	return fields
}

// fileHeaders holds the *multipart.FileHeader values of a bind as the request carried them. A header comes
// from a file part and from nothing else: schema decodes a form, query, header or cookie value whose key
// continues the name of a file field, such as "avatar.filename" or "avatar[size]", into the exported fields of the header
// the file part was bound to, or into one it creates for the value alone. The root cause is in
// github.com/gofiber/schema, which treats the field as a pointer to a struct as well as a file slot.
// The JSON, XML, CBOR and MsgPack decoders are not guarded: they fill the exported fields of a header from a
// body like those of any struct, so bind a *multipart.FileHeader field from a form only.
type fileHeaders struct {
	saved map[*multipart.FileHeader]multipart.FileHeader // nil until a header is saved
}

// guardFileHeaders records the headers out already holds and those of files, the file parts about to be bound.
// It returns nil when out cannot hold one, which is nearly always.
func guardFileHeaders(out any, files []map[string][]*multipart.FileHeader) *fileHeaders {
	v := reflect.ValueOf(out)
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() || !holdsFileHeader(v.Type()) {
		return nil
	}
	g := &fileHeaders{}
	n := 0
	for _, byKey := range files {
		for _, headers := range byKey {
			n += len(headers)
		}
	}
	if n > 0 {
		g.saved = make(map[*multipart.FileHeader]multipart.FileHeader, n)
	}
	for _, byKey := range files {
		for _, headers := range byKey {
			for _, fh := range headers {
				g.save(fh)
			}
		}
	}
	g.walk(v, recording)
	return g
}

func (g *fileHeaders) save(fh *multipart.FileHeader) {
	if fh == nil {
		return
	}
	if _, ok := g.saved[fh]; ok {
		return
	}
	if g.saved == nil {
		g.saved = make(map[*multipart.FileHeader]multipart.FileHeader)
	}
	saved := *fh
	saved.Header = textproto.MIMEHeader(http.Header(fh.Header).Clone())
	g.saved[fh] = saved
}

// restore puts the recorded headers back as they were and clears the ones no file part or earlier bind produced.
func (g *fileHeaders) restore(out any) {
	g.walk(reflect.ValueOf(out), restoring)
}

type guardPhase uint8

const (
	recording guardPhase = iota
	restoring
)

// walk visits every *multipart.FileHeader reachable from v. Recording, it saves the ones it finds;
// restoring, it resets the recorded ones that were changed and sets the others to nil where it can.
func (g *fileHeaders) walk(v reflect.Value, phase guardPhase) {
	g.visit(v, phase, map[seenPointer]struct{}{})
}

// seenPointer identifies a pointer, slice or map already walked. The address alone would not: a pointer to a struct
// and a pointer to its first field share one, and two slices of one array differ in their length.
type seenPointer struct {
	typ reflect.Type
	ptr uintptr
	n   int // the length of a slice, zero for the rest
}

// first reports whether the walk meets the value identified by key for the first time, and remembers it.
func first(seen map[seenPointer]struct{}, key seenPointer) bool {
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}
	return true
}

func (g *fileHeaders) visit(v reflect.Value, phase guardPhase, seen map[seenPointer]struct{}) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		if v.Type() == fileHeaderPtrType {
			// An element of an unexported embedded slice or map cannot be read as an interface, and schema cannot set it either.
			if v.CanInterface() {
				if fh, ok := reflect.TypeAssert[*multipart.FileHeader](v); ok {
					g.take(fh, phase)
				}
			}
			return
		}
		if first(seen, seenPointer{typ: v.Type(), ptr: v.Pointer()}) {
			g.visit(v.Elem(), phase, seen)
		}
	case reflect.Struct:
		for _, i := range headerFields(v.Type()) {
			field := v.Field(i)
			g.visit(field, phase, seen)
			if phase == restoring && field.Type() == fileHeaderPtrType && field.CanSet() && !field.IsNil() {
				if fh, ok := reflect.TypeAssert[*multipart.FileHeader](field); ok && !g.known(fh) {
					field.SetZero()
				}
			}
		}
	case reflect.Slice, reflect.Array:
		// A slice of headers cannot hold itself, and a slice of anything else may, through its element type.
		if v.Kind() == reflect.Slice && v.Type().Elem() != fileHeaderPtrType &&
			!first(seen, seenPointer{typ: v.Type(), ptr: v.Pointer(), n: v.Len()}) {
			return
		}
		for i := range v.Len() {
			elem := v.Index(i)
			g.visit(elem, phase, seen)
			if phase == restoring && elem.Type() == fileHeaderPtrType && elem.CanSet() && !elem.IsNil() {
				if fh, ok := reflect.TypeAssert[*multipart.FileHeader](elem); ok && !g.known(fh) {
					elem.SetZero()
				}
			}
		}
	case reflect.Map:
		if v.Type().Elem() != fileHeaderPtrType && !first(seen, seenPointer{typ: v.Type(), ptr: v.Pointer()}) {
			return
		}
		for _, key := range v.MapKeys() {
			g.visit(v.MapIndex(key), phase, seen)
		}
	default:
	}
}

func (g *fileHeaders) known(fh *multipart.FileHeader) bool {
	_, ok := g.saved[fh]
	return ok
}

// take saves fh while recording and resets it to its record while restoring, if schema changed it.
func (g *fileHeaders) take(fh *multipart.FileHeader, phase guardPhase) {
	if phase == recording {
		g.save(fh)
		return
	}
	// fh is the request's own header, which the application may be reading from another goroutine (it is
	// what c.FormFile and c.MultipartForm return): write it only when schema changed it.
	if saved, ok := g.saved[fh]; ok && !sameFileHeader(fh, &saved) {
		*fh = saved // the record is private to g, which is dropped once restored: its header map is not shared
	}
}

// sameFileHeader reports whether fh still holds what saved recorded. Schema sets exported fields only, so those
// are all there is to compare.
func sameFileHeader(fh, saved *multipart.FileHeader) bool {
	return fh.Filename == saved.Filename && fh.Size == saved.Size &&
		(fh.Header == nil) == (saved.Header == nil) &&
		maps.EqualFunc(fh.Header, saved.Header, slices.Equal)
}
