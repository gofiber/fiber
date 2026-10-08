package static

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
)

var ErrInvalidPath = errors.New("invalid path")

const invalidPathSentinel = "/__fiber_invalid__"

// windowsOS is runtime.GOOS on Windows, where a path component loses its
// trailing dots and spaces when a file is opened.
const windowsOS = "windows"

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
	prefixLen     int    // length of the route prefix stripped from the path
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
// path p, or invalidPathSentinel when p names nothing inside the root.
func (s *fileServer) requestPath(dst []byte, p string) []byte {
	addTrailingSlash := false
	if len(p) >= s.prefixLen {
		// a partial wildcard such as /static* also matches /staticx; the rest of
		// the path must start at a segment boundary
		if s.segmentBound && len(p) > s.prefixLen && p[s.prefixLen] != '/' {
			return append(dst, invalidPathSentinel...)
		}
		if s.invalid {
			return append(dst, invalidPathSentinel...)
		}
		// If the root is a file, we need to reset the path to "/" always.
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

	sanitized, err := sanitizePath(dst[start:], s.fsys, s.decodeEscapes)
	if err != nil {
		// return a guaranteed-missing path so fs responds with 404
		return append(dst[:0], invalidPathSentinel...)
	}
	return dst[:start+len(sanitized)]
}

// decodeFileName validates the escapes the router left in p and returns p
// unchanged. The router already produced the canonical resource name (it decoded
// every escape of an unreserved character and kept the rest encoded; see
// unescapeSafePath), so file lookup opens exactly the bytes the route and any
// path-scoped middleware matched. It refuses what cannot name such a file: a
// backslash or control character, a malformed or truncated escape, an escape of
// a separator or control character, and an escape of an unreserved character.
// That last one cannot survive the router's own decoding, so its only source is
// a stray '%' the router kept literally that lines up with the following bytes
// into a fresh escape ("%%370rivate" and "%7%30rivate" both normalize to
// "%70rivate"); decoding it a second time here would reach a name ("private")
// the router never matched and the middleware never guarded, so it is rejected.
// When decodeEscapes is off, UnescapePath already decoded everything and no
// escape remains to validate.
func decodeFileName(p []byte, decodeEscapes bool) ([]byte, error) { //nolint:revive // the flag mirrors UnescapePath; see sanitizePath
	if utils.IndexControl(p) >= 0 || bytes.IndexByte(p, '\\') >= 0 {
		return nil, ErrInvalidPath
	}
	i := bytes.IndexByte(p, '%')
	if !decodeEscapes || i < 0 {
		return p, nil
	}
	for ; i < len(p); i++ {
		if p[i] != '%' {
			continue
		}
		if i+2 >= len(p) {
			return nil, ErrInvalidPath
		}
		hi, lo := unhex(p[i+1]), unhex(p[i+2])
		if hi < 0 || lo < 0 {
			return nil, ErrInvalidPath
		}
		c := byte(hi<<4 | lo) //nolint:gosec // G115: both nibbles are 0-15
		if c == '/' || c == '\\' || c < 0x20 || c == 0x7f || isUnreserved(c) {
			return nil, ErrInvalidPath
		}
		i += 2
	}
	return p, nil
}

func isUnreserved(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '-' || c == '.' || c == '_' || c == '~'
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

// hasUnsafeSegment reports whether p holds a "." or ".." segment, or an empty
// one other than the root's leading and a directory's trailing slash. The
// router removes dot segments and keeps empty ones as sent, so none of them
// is part of a path it matched to a file.
func hasUnsafeSegment(p []byte) bool {
	start := 0
	for i := 0; i <= len(p); i++ {
		if i < len(p) && p[i] != '/' {
			continue
		}
		switch n := i - start; {
		case n == 0:
			if start > 0 && i < len(p) {
				return true
			}
		case n == 1 && p[start] == '.', n == 2 && p[start] == '.' && p[start+1] == '.':
			return true
		}
		start = i + 1
	}
	return false
}

// hasDriveLetter reports whether name starts with a Windows drive letter and a colon.
func hasDriveLetter(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	drive := name[0]
	return (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')
}

// hasTrailingDotOrSpaceSegment reports whether a segment of p ends in a dot or
// a space. Windows drops both from a path component when it opens a file, so
// "private./secret.txt" and "private /secret.txt" would open
// "private/secret.txt": a path the router kept apart from "private", and one
// that middleware guarding "private" never saw. Elsewhere such a name is an
// ordinary file, so sanitizePath applies this only on Windows, and there to
// every root: an fs.FS may be OS-backed (os.DirFS) and cannot be told apart
// from a virtual one such as embed.FS, so a virtual name ending in a dot or a
// space is refused on Windows rather than leave the alias open for the rest.
func hasTrailingDotOrSpaceSegment(p []byte) bool {
	for i, c := range p {
		if (c == '.' || c == ' ') && (i+1 == len(p) || p[i+1] == '/') {
			return true
		}
	}
	return false
}

// sanitizePath validates the routed path p, with the route's prefix stripped,
// and returns the path the file server opens, which is p unchanged. It fails
// with ErrInvalidPath when p cannot name a file inside the root: an escape of a
// slash ("private%2Fsecret.txt" never reaches the "private/secret.txt" the
// router did not match), an escape of an unreserved character (a stray '%' the
// router kept that would otherwise double-decode past the route and middleware),
// a malformed escape, a backslash, a control character, a ".", ".." or empty
// segment, a drive letter and, on Windows, a segment ending in a dot or a space
// that the OS would strip. Escapes the router kept remain encoded, keeping file
// lookup aligned with route and middleware matching.
func sanitizePath(p []byte, filesystem fs.FS, decodeEscapes bool) ([]byte, error) { //nolint:revive // the flag mirrors UnescapePath
	p, err := decodeFileName(p, decodeEscapes)
	if err != nil {
		return nil, err
	}
	if hasUnsafeSegment(p) || (runtime.GOOS == windowsOS && hasTrailingDotOrSpaceSegment(p)) {
		return nil, ErrInvalidPath
	}

	name := strings.TrimPrefix(utils.UnsafeString(p), "/")
	if filesystem != nil {
		if name = strings.TrimSuffix(name, "/"); name != "" && !fs.ValidPath(name) {
			return nil, ErrInvalidPath
		}
		return p, nil
	}
	if filepath.VolumeName(name) != "" || hasDriveLetter(name) {
		return nil, ErrInvalidPath
	}
	return p, nil
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
		if prefixLen > 1 && prefix[prefixLen-1:] == "/" {
			// /john/ -> /john
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

		// serve the path the handler built from the one the router matched,
		// not fasthttp's own decoding of the request
		files.PathRewrite = func(fctx *fasthttp.RequestCtx) []byte {
			if rw, ok := fctx.UserValue(rewriteKey{}).(*rewrite); ok {
				return rw.path
			}
			return []byte(invalidPathSentinel)
		}

		return &fileServer{
			handler:       files.NewRequestHandler(),
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
		rw.path = server.requestPath(rw.path[:0], c.Path())
		fctx := c.RequestCtx()
		fctx.SetUserValue(rewriteKey{}, rw)

		// Serve file
		server.handler(fctx)

		fctx.RemoveUserValue(rewriteKey{})
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
