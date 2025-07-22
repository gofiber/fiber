package static

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
)

// sanitizePath validates and cleans the requested path.
// It returns an error if the path attempts to traverse directories.
func sanitizePath(p []byte, filesystem fs.FS) ([]byte, error) {
	var s string
	if bytes.IndexByte(p, '\\') >= 0 {
		b := make([]byte, len(p))
		copy(b, p)
		for i := range b {
			if b[i] == '\\' {
				b[i] = '/'
			}
		}
		s = utils.UnsafeString(b)
	} else {
		s = utils.UnsafeString(p)
	}

	// repeatedly unescape until it no longer changes, catching errors
	for strings.IndexByte(s, '%') >= 0 {
		us, err := url.PathUnescape(s)
		if err != nil {
			return nil, errors.New("invalid path")
		}
		if us == s {
			break
		}
		s = us
	}

	// reject any null bytes
	if strings.IndexByte(s, 0) >= 0 {
		return nil, errors.New("invalid path")
	}

	s = pathpkg.Clean("/" + s)

	if filesystem != nil {
		s = utils.TrimLeft(s, '/')
		if s == "" {
			return []byte("/"), nil
		}
		if !fs.ValidPath(s) {
			return nil, errors.New("invalid path")
		}
		s = "/" + s
	}

	return utils.UnsafeBytes(s), nil
}

// New creates a new middleware handler.
// The root argument specifies the root directory from which to serve static assets.
//
// Note: Root has to be string or fs.FS, otherwise it will panic.
func New(root string, cfg ...Config) fiber.Handler {
	config := configDefault(cfg...)

	var createFS sync.Once
	var fileHandler fasthttp.RequestHandler
	var cacheControlValue string
	var rootIsFile bool

	// adjustments for io/fs compatibility
	if config.FS != nil && root == "" {
		root = "."
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

		// Initialize FS
		createFS.Do(func() {
			prefix := c.Route().Path

			if check, err := isFile(root, config.FS); err == nil {
				rootIsFile = check
			}

			// Is prefix a partial wildcard?
			if strings.Contains(prefix, "*") {
				// /john* -> /john
				prefix = strings.Split(prefix, "*")[0]
			}

			prefixLen := len(prefix)
			if prefixLen > 1 && prefix[prefixLen-1:] == "/" {
				// /john/ -> /john
				prefixLen--
			}

			fs := &fasthttp.FS{
				Root:                   root,
				FS:                     config.FS,
				AllowEmptyRoot:         true,
				GenerateIndexPages:     config.Browse,
				AcceptByteRange:        config.ByteRange,
				Compress:               config.Compress,
				CompressBrotli:         config.Compress, // Brotli compression won't work without this
				CompressZstd:           config.Compress, // Zstd compression won't work without this
				CompressedFileSuffixes: c.App().Config().CompressedFileSuffixes,
				CacheDuration:          config.CacheDuration,
				SkipCache:              config.CacheDuration < 0,
				IndexNames:             config.IndexNames,
				PathNotFound: func(fctx *fasthttp.RequestCtx) {
					fctx.Response.SetStatusCode(fiber.StatusNotFound)
				},
			}

			fs.PathRewrite = func(fctx *fasthttp.RequestCtx) []byte {
				path := fctx.Path()

				if len(path) >= prefixLen {
					checkFile, err := isFile(root, fs.FS)
					if err != nil {
						return path
					}

					// If the root is a file, we need to reset the path to "/" always.
					switch {
					case checkFile && fs.FS == nil:
						path = []byte("/")
					case checkFile && fs.FS != nil:
						path = utils.UnsafeBytes(root)
					default:
						path = path[prefixLen:]
						if len(path) == 0 || path[len(path)-1] != '/' {
							path = append(path, '/')
						}
					}
				}

				if len(path) > 0 && path[0] != '/' {
					path = append([]byte("/"), path...)
				}

				sanitized, err := sanitizePath(path, fs.FS)
				if err != nil {
					// return a guaranteed-missing path so fs responds with 404
					return []byte("/__fiber_invalid__")
				}
				return sanitized
			}

			maxAge := config.MaxAge
			if maxAge > 0 {
				cacheControlValue = "public, max-age=" + strconv.Itoa(maxAge)
			}

			fileHandler = fs.NewRequestHandler()
		})

		// Serve file
		fileHandler(c.RequestCtx())

		// Sets the response Content-Disposition header to attachment if the Download option is true
		if config.Download {
			name := filepath.Base(c.Path())
			if rootIsFile {
				name = filepath.Base(root)
			}
			c.Attachment(name)
		}

		// Return request if found and not forbidden
		status := c.RequestCtx().Response.StatusCode()

		if status != fiber.StatusNotFound && status != fiber.StatusForbidden {
			if len(cacheControlValue) > 0 {
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
