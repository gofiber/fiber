package static

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

func New(cfg ...Config) fiber.Handler {
	config := configDefault(cfg...)

	var createFS sync.Once
	var fileHandler fasthttp.RequestHandler
	var cacheControlValue string
	var modifyResponse fiber.Handler

	return func(c fiber.Ctx) error {
		createFS.Do(func() {
			prefix := c.Route().Path
			isStar := prefix == "/*"

			// Is prefix a partial wildcard?
			if strings.Contains(prefix, "*") {
				// /john* -> /john
				isStar = true
				prefix = strings.Split(prefix, "*")[0]
				// Fix this later
			}

			prefixLen := len(prefix)
			if prefixLen > 1 && prefix[prefixLen-1:] == "/" {
				// /john/ -> /john
				prefixLen--
				prefix = prefix[:prefixLen]
			}

			fmt.Printf("prefix: %s, prefixlen: %d, isStar: %t\n", prefix, prefixLen, isStar)

			fs := &fasthttp.FS{
				Root:                 config.Root,
				AllowEmptyRoot:       true,
				GenerateIndexPages:   false,
				AcceptByteRange:      false,
				Compress:             false,
				CompressedFileSuffix: c.App().Config().CompressedFileSuffix,
				CacheDuration:        config.CacheDuration,
				IndexNames:           []string{"index.html"},
				PathRewrite: func(fctx *fasthttp.RequestCtx) []byte {
					path := fctx.Path()
					fmt.Println(string(path))
					if len(path) >= prefixLen {
						// TODO: All routes have to contain star we don't need this mechanishm anymore i think
						if config.IsFile {
							path = append(path[0:0], '/')
							fmt.Printf("istar %s", path)
						} else {
							path = path[prefixLen:]
							fmt.Printf("path2 %s\n", path)
							if len(path) == 0 || path[len(path)-1] != '/' {
								path = append(path, '/')
							}
						}
					}
					if len(path) > 0 && path[0] != '/' {
						path = append([]byte("/"), path...)
					}
					fmt.Printf("path %s\n", path)
					return path
				},
				PathNotFound: func(fctx *fasthttp.RequestCtx) {
					fctx.Response.SetStatusCode(fiber.StatusNotFound)
				},
			}

			maxAge := config.MaxAge
			if maxAge > 0 {
				cacheControlValue = "public, max-age=" + strconv.Itoa(maxAge)
			}

			fs.CacheDuration = config.CacheDuration
			fs.Compress = config.Compress
			fs.AcceptByteRange = config.ByteRange
			fs.GenerateIndexPages = config.Browse
			if config.Index != "" {
				fs.IndexNames = []string{config.Index}
			}
			modifyResponse = config.ModifyResponse

			fileHandler = fs.NewRequestHandler()
		})

		// Don't execute middleware if Next returns true
		if config.Next != nil && config.Next(c) {
			return c.Next()
		}

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
			if modifyResponse != nil {
				return modifyResponse(c)
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
