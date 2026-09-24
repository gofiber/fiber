package binder

import (
	"mime/multipart"
	"sync"

	"github.com/gofiber/fiber/v3/internal/mediatype"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

const MIMEMultipartForm string = "multipart/form-data"

var (
	bindDataPool = sync.Pool{
		New: func() any {
			return &bindData{values: make(map[string][]string, 8)}
		},
	}
	formFileMapPool = sync.Pool{
		New: func() any {
			return make(map[string][]*multipart.FileHeader)
		},
	}
)

// Keep oversized maps and arenas out of the pool so a rare large bind doesn't
// get retained and reused across subsequent requests.
const (
	maxPoolableDataMapSize = 256
	maxPoolableArenaSize   = 1024
)

// FormBinding is the form binder for form request body.
type FormBinding struct {
	EnableSplitting bool
	MaxBodySize     int
}

// Name returns the binding name.
func (*FormBinding) Name() string {
	return "form"
}

// Bind parses the request body and returns the result.
func (b *FormBinding) Bind(req *fasthttp.Request, out any) error {
	// Callers reaching the binder directly skip Fiber's normalization, and
	// comparing case-insensitively is not enough: PostArgs and MultipartForm match
	// the media type and "boundary=" case-sensitively. Guarded — the fold writes.
	if mediatype.IsForm(req.Header.ContentType()) {
		mediatype.NormalizeRequestContentType(&req.Header)
	}

	// Handle multipart form. Media types are case-insensitive
	// (RFC 9110 Section 8.3.1), so compare accordingly.
	if utils.EqualFold(FilterFlags(utils.UnsafeString(req.Header.ContentType())), MIMEMultipartForm) {
		return b.bindMultipart(req, out)
	}

	args := req.PostArgs()
	data := acquireBindData(out, args.Len())
	defer releaseBindData(data)

	for key, val := range args.All() {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		if err := data.bind(b.Name(), out, k, v, b.EnableSplitting, true); err != nil {
			return err
		}
	}

	return data.parse(b.Name(), out)
}

// bindMultipart parses the request body and returns the result.
func (b *FormBinding) bindMultipart(req *fasthttp.Request, out any) error {
	multipartForm, err := req.MultipartFormWithLimit(b.MaxBodySize)
	if err != nil {
		return err
	}

	// The files are merged into the decoder's view of the source map, so a
	// multipart form is always filed into one.
	mode := bindModeFor(out)
	if mode == bindPairs {
		mode = bindMap
	}
	data := acquireBindDataMode(mode)
	defer releaseBindData(data)

	for key, values := range multipartForm.Value {
		err = data.bindAll(b.Name(), out, key, values, b.EnableSplitting, true)
		if err != nil {
			return err
		}
	}

	files := acquireFileHeaderMap()
	defer releaseFileHeaderMap(files)

	for key, values := range multipartForm.File {
		err = formatBindData(b.Name(), out, files, key, values, b.EnableSplitting, true)
		if err != nil {
			return err
		}
	}

	if data.mode == bindLast {
		// A map[string]string keeps no files: parseToMap ignores them too.
		return data.parse(b.Name(), out)
	}
	return parse(b.Name(), out, data.values, files)
}

// Reset resets the FormBinding binder.
func (b *FormBinding) Reset() {
	b.EnableSplitting = false
	b.MaxBodySize = 0
}

// acquireBindData returns a pooled bindData for a bind into out of n pairs,
// 0 when the count is not known up front. A bind into a struct keeps its
// pairs in two slices, which get room for n here, so that a large bind
// allocates them once rather than growing them by appending: splitting can
// file more pairs than n, and append makes room for those.
func acquireBindData(out any, n int) *bindData {
	d := acquireBindDataMode(bindModeFor(out))
	if d.mode == bindPairs && n > cap(d.keys) {
		d.keys = make([]string, 0, n)
		d.pairValues = make([]string, 0, n)
	}
	return d
}

// acquireBindDataMode returns a pooled bindData that keeps its values as mode
// says.
func acquireBindDataMode(mode bindMode) *bindData {
	d, ok := bindDataPool.Get().(*bindData)
	if !ok {
		d = &bindData{values: make(map[string][]string, 8)}
	}
	if mode == bindLast && d.last == nil {
		d.last = make(map[string]string, 8)
	}
	d.mode = mode
	return d
}

func releaseBindData(d *bindData) {
	if len(d.values) > maxPoolableDataMapSize || len(d.last) > maxPoolableDataMapSize ||
		cap(d.arena.buf) > maxPoolableArenaSize || cap(d.keys) > maxPoolableArenaSize {
		return
	}

	d.reset()
	bindDataPool.Put(d)
}

// reset empties d for another bind, dropping the strings it held so they do
// not keep request memory alive while d sits in the pool.
func (d *bindData) reset() {
	clearDataMap(d.values)
	// A map in use for another mode is empty, and clear would still call
	// into the runtime for it.
	if len(d.last) > 0 {
		clear(d.last)
	}
	d.arena.reset()
	clear(d.keys)
	clear(d.pairValues)
	d.keys, d.pairValues = d.keys[:0], d.pairValues[:0]
}

func acquireFileHeaderMap() map[string][]*multipart.FileHeader {
	m, ok := formFileMapPool.Get().(map[string][]*multipart.FileHeader)
	if !ok {
		m = make(map[string][]*multipart.FileHeader)
	}
	return m
}

func releaseFileHeaderMap(m map[string][]*multipart.FileHeader) {
	if len(m) > maxPoolableDataMapSize {
		return
	}

	clearFileHeaderMap(m)
	formFileMapPool.Put(m)
}

func clearDataMap(m map[string][]string) {
	clear(m)
}

func clearFileHeaderMap(m map[string][]*multipart.FileHeader) {
	clear(m)
}
