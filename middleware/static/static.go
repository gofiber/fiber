package static

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// New creates a new middleware handler.
// The root argument specifies the root directory from which to serve static assets.
//
// Note: Root has to be string or fs.FS, otherwise it will panic.
func New(root string, cfg ...Config) fiber.Handler {
	config := configDefault(cfg...)

	var createFS sync.Once
	var fileHandler fasthttp.RequestHandler
	var cacheControlValue string

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

				// Add a leading slash if missing
				if len(path) > 0 && path[0] != '/' {
					path = append([]byte("/"), path...)
				}

				// Perform explicit path validation
				absRoot, err := filepath.Abs(root)
				if err != nil {
					fctx.Response.SetStatusCode(fiber.StatusInternalServerError)
					return nil
				}

				// Clean the path and resolve it against the root
				cleanPath := filepath.Clean(utils.UnsafeString(path))
				absPath := filepath.Join(absRoot, cleanPath)
				relPath, err := filepath.Rel(absRoot, absPath)

				// Check if the resolved path is within the root
				if err != nil || strings.HasPrefix(relPath, "..") {
					fctx.Response.SetStatusCode(fiber.StatusForbidden)
					return nil
				}

				return []byte(cleanPath)
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
			c.Attachment()
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
	} else {
		file, err = os.Open(filepath.Clean(root))
		if err != nil {
			return false, fmt.Errorf("static: %w", err)
		}
	}

	stat, err := file.Stat()
	if err != nil {
		return false, fmt.Errorf("static: %w", err)
	}

	return stat.Mode().IsRegular(), nil
}
