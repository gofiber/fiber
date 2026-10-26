package static

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/byterange"
	"github.com/gofiber/fiber/v3/internal/fspath"
)

var ErrInvalidPath = errors.New("invalid path")

// invalidPathSentinel is the path PathRewrite falls back to when the handler
// gave it none. The handler never opens a refused path, so it only matters if
// the file server runs outside it.
const invalidPathSentinel = "/__fiber_invalid__"

// rewrite carries the path the file server opens for one request. The handler
// builds it before it calls fasthttp, so PathRewrite only has to read it.
type rewrite struct {
	path []byte
}

// rewriteKey is the fasthttp user-value key under which PathRewrite finds the rewrite.
type rewriteKey struct{}

var rewritePool = sync.Pool{
	New: func() any { return &rewrite{path: make([]byte, 0, 128)} },
}

// fileServer is the fasthttp file server of one route of one app, with what
// it needs to turn a routed path into the path it opens.
type fileServer struct {
	fsys          fs.FS
	handler       fasthttp.RequestHandler
	root          string
	fsRootPrefix  []byte // the fs.FS subdirectory served, put before every path
	prefixLen     int    // length of the route prefix stripped from the path, without its trailing slash
	decodeEscapes bool   // off under UnescapePath, which already decoded everything
	rootIsFile    bool
	segmentBound  bool // the route is a /prefix* mount, so the rest of the path starts at a /
	invalid       bool // the fs.FS root cannot be opened, so no path names a file
}

// fileServerKey identifies a file server: the decoding mode and the compressed
// file suffixes are the app's, and the prefix to strip is the route's.
type fileServerKey struct {
	app   *fiber.App
	route string
}

// requestPath appends to dst the path the file server opens for the routed
// path p. It reports false when p names nothing inside the root, and then there
// is nothing to open: a file of any name could stand in for that answer.
func (s *fileServer) requestPath(dst []byte, p string) ([]byte, bool) {
	addTrailingSlash := false
	if len(p) >= s.prefixLen {
		// a partial wildcard such as /static* also matches /staticx; the rest of
		// the path must start at a segment boundary
		if s.segmentBound && len(p) > s.prefixLen && p[s.prefixLen] != '/' {
			return dst, false
		}
		if s.invalid {
			return dst, false
		}
		// If the root is a file, we need to reset the path to "/" always. What the
		// request path says below the prefix is dropped, so no spelling of it can
		// select another file, and an empty segment inside it, as in
		// /web/https://example.com, is part of an opaque name. A root mount refuses
		// a path that starts with "//" whatever it serves, as a directory root does
		// in fspath.Valid; on a prefix, even one that starts with "//" itself, that
		// segment is the route's own.
		if s.rootIsFile && s.prefixLen == 0 && len(p) > 1 && p[0] == '/' && p[1] == '/' {
			return dst, false
		}
		switch {
		case s.rootIsFile && s.fsys == nil:
			p = "/"
		case s.rootIsFile && s.fsys != nil:
			p = s.root
		default:
			p = p[s.prefixLen:]
			// a trailing slash lets fasthttp serve a directory index without a redirect
			addTrailingSlash = true
		}
	}

	dst = append(dst, s.fsRootPrefix...)
	start := len(dst)
	if p == "" || p[0] != '/' {
		dst = append(dst, '/')
	}
	dst = append(dst, p...)
	if addTrailingSlash && dst[len(dst)-1] != '/' {
		dst = append(dst, '/')
	}

	// PathRewrite skips fasthttp's Path() normalization; fspath lists what
	// each check covers instead.
	if !fspath.Valid(dst[start:], s.fsys, s.decodeEscapes) {
		return dst[:0], false
	}
	return dst, true
}

// New creates a new middleware handler.
// The root argument specifies the root directory from which to serve static assets.
//
// Note: Root has to be string or fs.FS; otherwise, it will panic.
func New(root string, cfg ...Config) fiber.Handler {
	config := configDefault(cfg...)

	var (
		rootCheck    sync.Once
		rootCheckErr error
		rootIsFile   bool
		// servers holds one file server per app and route, since the stripped prefix is the route's.
		servers   sync.Map
		serversMu sync.Mutex
	)
	cacheControlValue := ""
	if config.MaxAge > 0 {
		cacheControlValue = "public, max-age=" + strconv.Itoa(config.MaxAge)
	}

	// adjustments for io/fs compatibility: io/fs paths are always relative and
	// slash-separated, so a leading slash (e.g. "/" or "/dist") is never a valid
	// fs path and makes isFile's fs.FS.Open fail, which sends every request to
	// the PathNotFound handler. Strip it and treat an empty result as the fs root ".".
	if config.FS != nil {
		root = strings.TrimLeft(root, "/")
		if root == "" {
			root = "."
		}
	}

	// newFileServer builds the fasthttp file server for one route prefix;
	// decodeEscapes is off under UnescapePath, which already decoded everything.
	newFileServer := func(prefix string, compressedFileSuffixes map[string]string, decodeEscapes bool) *fileServer {
		// Is prefix a partial wildcard?
		segmentBound := false
		if before, _, found := utils.CutByte(prefix, '*'); found {
			// /john* -> /john
			prefix = before
			// only /john* must not match /johnx; /john/* and /* end in a slash
			segmentBound = before != "" && before[len(before)-1] != '/'
		}

		prefixLen := len(prefix)
		if prefixLen > 0 && prefix[prefixLen-1] == '/' {
			// /john/ -> /john and / -> "", so "//john" keeps the empty segment
			// that fspath.Valid refuses
			prefixLen--
		}

		// For io/fs.FS, Root must be empty so fasthttp's pathToFilePath
		// returns clean relative paths without prefixing the root.
		// requestPath already handles file-root and subdirectory cases.
		fsRoot := root
		if config.FS != nil {
			fsRoot = ""
		}

		var fsRootPrefix []byte
		if config.FS != nil && root != "" && root != "." && !rootIsFile {
			// fasthttp.FS.Root is forced to "" for io/fs.FS so pathToFilePath
			// returns clean relative paths. requestPath puts the caller's
			// configured subdirectory before the sanitized request path.
			fsRootPrefix = append([]byte{'/'}, root...)
		}

		files := &fasthttp.FS{
			Root:           fsRoot,
			FS:             config.FS,
			AllowEmptyRoot: true,
			// A file root serves only itself, and its compressed copy lies beside it, outside the root.
			AllowSymlinksOutsideRoot: config.AllowSymlinksOutsideRoot || (rootIsFile && config.FS == nil),
			GenerateIndexPages:       config.Browse,
			AcceptByteRange:          config.ByteRange,
			Compress:                 config.Compress,
			CompressBrotli:           config.Compress, // Brotli compression won't work without this
			CompressZstd:             config.Compress, // Zstd compression won't work without this
			CompressedFileSuffixes:   compressedFileSuffixes,
			CacheDuration:            config.CacheDuration,
			SkipCache:                config.CacheDuration < 0,
			IndexNames:               config.IndexNames,
			PathNotFound: func(fctx *fasthttp.RequestCtx) {
				fctx.Response.SetStatusCode(fiber.StatusNotFound)
			},
		}

		// serve the path the handler built from the one the router matched,
		// not fasthttp's own decoding of the request
		files.PathRewrite = func(fctx *fasthttp.RequestCtx) []byte {
			if rw, ok := fctx.UserValue(rewriteKey{}).(*rewrite); ok {
				return rw.path
			}
			return []byte(invalidPathSentinel)
		}

		handler := files.NewRequestHandler()
		if config.ByteRange {
			handler = byterange.Wrap(handler)
		}

		return &fileServer{
			handler:       handler,
			fsys:          config.FS,
			root:          root,
			fsRootPrefix:  fsRootPrefix,
			prefixLen:     prefixLen,
			segmentBound:  segmentBound,
			decodeEscapes: decodeEscapes,
			rootIsFile:    rootIsFile,
			invalid:       rootCheckErr != nil && config.FS != nil,
		}
	}

	// serverFor returns the file server for the app and route serving c, built on first use.
	serverFor := func(c fiber.Ctx) *fileServer {
		key := fileServerKey{app: c.App(), route: c.Route().Path}
		if s, ok := servers.Load(key); ok {
			if server, ok := s.(*fileServer); ok {
				return server
			}
		}

		serversMu.Lock()
		defer serversMu.Unlock()
		if s, ok := servers.Load(key); ok {
			if server, ok := s.(*fileServer); ok {
				return server
			}
		}
		rootCheck.Do(func() {
			rootIsFile, rootCheckErr = isFile(root, config.FS)
		})
		cfg := c.App().Config()
		server := newFileServer(key.route, cfg.CompressedFileSuffixes, !cfg.UnescapePath)
		servers.Store(key, server)
		return server
	}

	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if config.Next != nil && config.Next(c) {
			return c.Next()
		}

		// We only serve static assets on GET or HEAD methods
		method := c.Method()
		if method != fiber.MethodGet && method != fiber.MethodHead {
			return c.Next()
		}

		server := serverFor(c)

		// PathRewrite only receives the fasthttp context; hand it the path to open.
		rw, ok := rewritePool.Get().(*rewrite)
		if !ok {
			rw = &rewrite{}
		}
		var valid bool
		rw.path, valid = server.requestPath(rw.path[:0], c.Path())
		fctx := c.RequestCtx()

		if valid {
			fctx.SetUserValue(rewriteKey{}, rw)

			// Serve file
			server.handler(fctx)

			fctx.RemoveUserValue(rewriteKey{})
		} else {
			// A refused path is a miss as fasthttp reports one, without a lookup
			fctx.Response.SetStatusCode(fiber.StatusNotFound)
		}
		rewritePool.Put(rw)

		// Return request if found and not forbidden
		status := fctx.Response.StatusCode()

		if status != fiber.StatusNotFound && status != fiber.StatusForbidden {
			// Only a served file is an attachment; the header must not leak onto a
			// miss, a redirect, or an error such as 416.
			if config.Download && status >= fiber.StatusOK && status < fiber.StatusMultipleChoices {
				name := filepath.Base(c.Path())
				if rootIsFile {
					name = filepath.Base(root)
				}
				// Attachment derives a Content-Type from the extension; keep the detected one.
				contentType := utils.CopyString(c.GetRespHeader(fiber.HeaderContentType))
				c.Attachment(name)
				if contentType != "" {
					c.Set(fiber.HeaderContentType, contentType)
				}
			}

			// An error response such as 416 must not be cached under the file's MaxAge.
			if cacheControlValue != "" && status >= fiber.StatusOK && status < fiber.StatusBadRequest {
				fctx.Response.Header.Set(fiber.HeaderCacheControl, cacheControlValue)
			}

			if config.ModifyResponse != nil {
				return config.ModifyResponse(c)
			}

			return nil
		}

		// Return custom 404 handler if provided.
		if config.NotFoundHandler != nil {
			return config.NotFoundHandler(c)
		}

		// Reset response to default
		fctx.SetContentType("") // Issue #420
		fctx.Response.SetStatusCode(fiber.StatusOK)
		fctx.Response.SetBodyString("")

		// Next middleware
		return c.Next()
	}
}

// isFile checks if the root is a file.
func isFile(root string, filesystem fs.FS) (bool, error) {
	var file fs.File
	var err error

	if filesystem != nil {
		file, err = filesystem.Open(root)
		if err != nil {
			return false, fmt.Errorf("static: %w", err)
		}
		defer func() {
			_ = file.Close() //nolint:errcheck // not needed
		}()
	} else {
		file, err = os.Open(filepath.Clean(root))
		if err != nil {
			return false, fmt.Errorf("static: %w", err)
		}
		defer func() {
			_ = file.Close() //nolint:errcheck // not needed
		}()
	}

	stat, err := file.Stat()
	if err != nil {
		return false, fmt.Errorf("static: %w", err)
	}

	return stat.Mode().IsRegular(), nil
}
