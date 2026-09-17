package static

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
)

var ErrInvalidPath = errors.New("invalid path")

const invalidPathSentinel = "/__fiber_invalid__"

// ctxKey is the fasthttp user-value key under which the handler hands its
// fiber.Ctx to PathRewrite, which fasthttp calls with the RequestCtx alone.
type ctxKey struct{}

// hasParentDirSegment reports whether the routed path still carries a ".."
// segment. The router removes dot segments before matching, so this only
// catches a path set by hand through Ctx.Path.
func hasParentDirSegment(p string) bool {
	return hasDotDotSegment(utils.TrimLeft(p, '/'))
}

// hasDotDotSegment reports whether any "/"-separated segment of p is "..".
// The segments are only compared, so it walks them with utils.CutByte rather
// than materializing the []string strings.Split would allocate per request.
func hasDotDotSegment(p string) bool {
	for rest := p; rest != ""; {
		segment, more, found := utils.CutByte(rest, '/')
		if segment == ".." {
			return true
		}
		if !found {
			return false
		}
		rest = more
	}

	return false
}

// decodeFileName turns the routed path into a file name and reports whether
// it can be one: a backslash and control characters including NUL cannot,
// whether decoded or sent raw, and neither can a slash produced by decoding,
// which would move the name into another directory. With decodeEscapes set,
// the percent escapes the router left in p are decoded exactly once; without
// it, the router already decoded everything (UnescapePath), so a remaining "%"
// is a literal character. A malformed escape is kept as it is.
func decodeFileName(p []byte, decodeEscapes bool) (string, error) { //nolint:revive // the flag mirrors UnescapePath; see sanitizePath
	if !decodeEscapes || bytes.IndexByte(p, '%') < 0 {
		if slices.ContainsFunc(p, isUnsafeNameByte) {
			return "", ErrInvalidPath
		}
		return utils.UnsafeString(p), nil
	}
	out := make([]byte, 0, len(p))
	for i := 0; i < len(p); {
		c := p[i]
		if c == '%' && i+2 < len(p) {
			if hi, lo := unhex(p[i+1]), unhex(p[i+2]); hi >= 0 && lo >= 0 {
				v := byte(hi<<4 | lo) //nolint:gosec // G115: both nibbles are 0-15
				if v == '/' || isUnsafeNameByte(v) {
					return "", ErrInvalidPath
				}
				out = append(out, v)
				i += 3
				continue
			}
		}
		if isUnsafeNameByte(c) {
			return "", ErrInvalidPath
		}
		out = append(out, c)
		i++
	}
	return utils.UnsafeString(out), nil
}

// isUnsafeNameByte reports whether c can never be part of a served file name:
// a backslash, which Windows reads as a separator, or a control character.
func isUnsafeNameByte(c byte) bool {
	return c == '\\' || c < 0x20 || c == 0x7f
}

// unhex returns the value of a hexadecimal digit, or -1 for any other byte.
func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// sanitizePath turns the path the router matched, with the route's prefix
// stripped, into the path the file server opens, and returns ErrInvalidPath
// when it cannot name a file inside the root.
//
// The router normalized p as RFC 3986 Section 6.2.2 describes: ".", ".." and
// empty segments are gone and every escape that cannot change the path's
// structure is decoded. What is still encoded — reserved characters such as
// "%2F" or "%40", "%25" itself, "%5C" and control characters — is decoded here
// exactly once to obtain the file name, so "photo%402x.png" opens
// "photo@2x.png" and "100%25.txt" opens "100%.txt". A slash, backslash or
// control character produced by that decoding cannot be part of a file name
// and is rejected rather than treated as a separator: "private%2Fsecret.txt"
// stays one, non-existent name and never reaches "private/secret.txt", which
// the router did not match. A "%" left after this pass is data, never a second
// escape to decode; decoding again is what once let "/%2570rivate/secret.txt"
// past a guard mounted on "/static/private". Under UnescapePath the router
// decoded every escape already, so decodeEscapes is false and p is used as is.
func sanitizePath(p []byte, filesystem fs.FS, decodeEscapes bool) ([]byte, error) {
	hasTrailingSlash := len(p) > 0 && p[len(p)-1] == '/'

	s, err := decodeFileName(p, decodeEscapes)
	if err != nil {
		return nil, err
	}

	if filesystem == nil && strings.HasPrefix(s, "//") {
		return nil, ErrInvalidPath
	}

	s = pathpkg.Clean("/" + s)

	trimmed := utils.TrimLeft(s, '/')
	if hasDotDotSegment(trimmed) {
		return nil, ErrInvalidPath
	}

	if filesystem == nil {
		normalizedClean := filepath.ToSlash(trimmed)
		if strings.HasPrefix(normalizedClean, "//") {
			return nil, ErrInvalidPath
		}
		if volume := filepath.VolumeName(normalizedClean); volume != "" {
			return nil, ErrInvalidPath
		}
		if len(normalizedClean) >= 2 && normalizedClean[1] == ':' {
			drive := normalizedClean[0]
			if (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z') {
				return nil, ErrInvalidPath
			}
		}
		if strings.HasPrefix(filepath.ToSlash(s), "//") {
			return nil, ErrInvalidPath
		}
	}

	if filesystem != nil {
		s = trimmed
		if s == "" {
			return []byte("/"), nil
		}
		if !fs.ValidPath(s) {
			return nil, ErrInvalidPath
		}
		s = "/" + s
	}

	if hasTrailingSlash && len(s) > 1 && s[len(s)-1] != '/' {
		s += "/"
	}

	return utils.UnsafeBytes(s), nil
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
		// handlers holds one file server per route path, since the stripped prefix is the route's.
		handlers   sync.Map
		handlersMu sync.Mutex
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

	// newFileHandler builds the fasthttp file server for one route prefix.
	// decodeEscapes is false under UnescapePath, where the router decodes every
	// escape and a "%" in the routed path is a literal character.
	newFileHandler := func(prefix string, compressedFileSuffixes map[string]string, decodeEscapes bool) fasthttp.RequestHandler {
		// Is prefix a partial wildcard?
		if before, _, found := utils.CutByte(prefix, '*'); found {
			// /john* -> /john
			prefix = before
		}

		prefixLen := len(prefix)
		if prefixLen > 1 && prefix[prefixLen-1:] == "/" {
			// /john/ -> /john
			prefixLen--
		}

		// For io/fs.FS, Root must be empty so fasthttp's pathToFilePath
		// returns clean relative paths without prefixing the root.
		// PathRewrite already handles file-root and subdirectory cases.
		fsRoot := root
		if config.FS != nil {
			fsRoot = ""
		}

		var fsRootPrefix []byte
		if config.FS != nil && root != "" && root != "." && !rootIsFile {
			// fasthttp.FS.Root is forced to "" for io/fs.FS so pathToFilePath
			// returns clean relative paths. PathRewrite prepends the caller's
			// configured subdirectory after sanitizing the request path.
			fsRootPrefix = make([]byte, len(root)+1)
			fsRootPrefix[0] = '/'
			copy(fsRootPrefix[1:], root)
		}

		fileServer := &fasthttp.FS{
			Root:                   fsRoot,
			FS:                     config.FS,
			AllowEmptyRoot:         true,
			GenerateIndexPages:     config.Browse,
			AcceptByteRange:        config.ByteRange,
			Compress:               config.Compress,
			CompressBrotli:         config.Compress, // Brotli compression won't work without this
			CompressZstd:           config.Compress, // Zstd compression won't work without this
			CompressedFileSuffixes: compressedFileSuffixes,
			CacheDuration:          config.CacheDuration,
			SkipCache:              config.CacheDuration < 0,
			IndexNames:             config.IndexNames,
			PathNotFound: func(fctx *fasthttp.RequestCtx) {
				fctx.Response.SetStatusCode(fiber.StatusNotFound)
			},
		}

		fileServer.PathRewrite = func(fctx *fasthttp.RequestCtx) []byte {
			c, ok := fctx.UserValue(ctxKey{}).(fiber.Ctx)
			if !ok {
				return []byte(invalidPathSentinel)
			}
			// The path the router matched, not fasthttp's own decoding of the
			// request, so the file server resolves exactly what the route and
			// the middleware in front of it saw.
			path := c.Path()
			addTrailingSlash := false

			if len(path) >= prefixLen {
				if rootCheckErr != nil && fileServer.FS != nil {
					return []byte(invalidPathSentinel)
				}

				// If the root is a file, we need to reset the path to "/" always.
				switch {
				case rootIsFile && fileServer.FS == nil:
					path = "/"
				case rootIsFile && fileServer.FS != nil:
					path = root
				default:
					path = path[prefixLen:]
					if len(fsRootPrefix) > 0 && hasParentDirSegment(path) {
						return []byte(invalidPathSentinel)
					}
					// A trailing slash lets fasthttp serve a directory's index
					// without redirecting to the slash form first.
					addTrailingSlash = true
				}
			}

			rewritten := make([]byte, 0, len(path)+2)
			if path != "" && path[0] != '/' {
				rewritten = append(rewritten, '/')
			}
			rewritten = append(rewritten, path...)
			if addTrailingSlash && (len(rewritten) == 0 || rewritten[len(rewritten)-1] != '/') {
				rewritten = append(rewritten, '/')
			}

			sanitized, err := sanitizePath(rewritten, fileServer.FS, decodeEscapes)
			if err != nil {
				// return a guaranteed-missing path so fs responds with 404
				return []byte(invalidPathSentinel)
			}
			if len(fsRootPrefix) > 0 {
				rewrittenPath := make([]byte, len(fsRootPrefix)+len(sanitized))
				copy(rewrittenPath, fsRootPrefix)
				copy(rewrittenPath[len(fsRootPrefix):], sanitized)
				return rewrittenPath
			}
			return sanitized
		}

		return fileServer.NewRequestHandler()
	}

	// fileHandlerFor returns the file server for the route serving c, built on first use.
	fileHandlerFor := func(c fiber.Ctx) fasthttp.RequestHandler {
		routePath := c.Route().Path
		if h, ok := handlers.Load(routePath); ok {
			if handler, ok := h.(fasthttp.RequestHandler); ok {
				return handler
			}
		}

		handlersMu.Lock()
		defer handlersMu.Unlock()
		if h, ok := handlers.Load(routePath); ok {
			if handler, ok := h.(fasthttp.RequestHandler); ok {
				return handler
			}
		}
		rootCheck.Do(func() {
			rootIsFile, rootCheckErr = isFile(root, config.FS)
		})
		cfg := c.App().Config()
		handler := newFileHandler(routePath, cfg.CompressedFileSuffixes, !cfg.UnescapePath)
		handlers.Store(routePath, handler)
		return handler
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

		fileHandler := fileHandlerFor(c)

		// PathRewrite only receives the fasthttp context; hand it the fiber.Ctx
		// so the file server resolves the path the router matched.
		c.RequestCtx().SetUserValue(ctxKey{}, c)

		// Serve file
		fileHandler(c.RequestCtx())

		// Return request if found and not forbidden
		status := c.RequestCtx().Response.StatusCode()

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
				c.RequestCtx().Response.Header.Set(fiber.HeaderCacheControl, cacheControlValue)
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
		c.RequestCtx().SetContentType("") // Issue #420
		c.RequestCtx().Response.SetStatusCode(fiber.StatusOK)
		c.RequestCtx().Response.SetBodyString("")

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
