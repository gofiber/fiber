package static

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
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
	if config.FS != nil && root != "" {
		root = "."
	}

	if root != "." && !strings.HasPrefix(root, "/") {
		root = "./" + root
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
				Root:                 root,
				FS:                   config.FS,
				AllowEmptyRoot:       true,
				GenerateIndexPages:   config.Browse,
				AcceptByteRange:      config.ByteRange,
				Compress:             config.Compress,
				CompressedFileSuffix: c.App().Config().CompressedFileSuffix,
				CacheDuration:        config.CacheDuration,
				IndexNames:           []string{"index.html"},
				PathNotFound: func(fctx *fasthttp.RequestCtx) {
					fctx.Response.SetStatusCode(fiber.StatusNotFound)
				},
			}

			fs.PathRewrite = func(fctx *fasthttp.RequestCtx) []byte {
				path := fctx.Path()

				if len(path) >= prefixLen {
					checkFile, err := isFile(root)
					if err != nil {
						return path
					}

					// If the root is a file, we need to reset the path to "/" always.
					if checkFile {
						path = append(path[0:0], '/')
					} else {
						path = path[prefixLen:]
						if len(path) == 0 || path[len(path)-1] != '/' {
							path = append(path, '/')
						}
					}
				}

				if len(path) > 0 && path[0] != '/' {
					path = append([]byte("/"), path...)
				}

				return path
			}

			maxAge := config.MaxAge
			if maxAge > 0 {
				cacheControlValue = "public, max-age=" + strconv.Itoa(maxAge)
			}

			if config.Index != "" {
				fs.IndexNames = []string{config.Index}
			}

			fileHandler = fs.NewRequestHandler()
		})

		// Serve file
		fileHandler(c.Context())

		// Sets the response Content-Disposition header to attachment if the Download option is true
		if config.Download {
			c.Attachment()
		}

		// Return request if found and not forbidden
		status := c.Context().Response.StatusCode()
		if status != fiber.StatusNotFound && status != fiber.StatusForbidden {
			if len(cacheControlValue) > 0 {
				c.Context().Response.Header.Set(fiber.HeaderCacheControl, cacheControlValue)
			}

			if config.ModifyResponse != nil {
				return config.ModifyResponse(c)
			}

			return nil
		}

		// Reset response to default
		c.Context().SetContentType("") // Issue #420
		c.Context().Response.SetStatusCode(fiber.StatusOK)
		c.Context().Response.SetBodyString("")

		// Next middleware
		return c.Next()
	}
}

// isFile checks if the root is a file.
func isFile(root string) (bool, error) {
	file, err := os.Open(root)
	if err != nil {
		return false, err
	}

	stat, err := file.Stat()
	if err != nil {
		return false, err
	}

	return stat.Mode().IsRegular(), nil
}
