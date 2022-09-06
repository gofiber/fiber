package fiber

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

type Static struct {
	Root   string
	Config StaticConfig
}

// Static defines configuration options when defining static assets.
type StaticConfig struct {
	// When set to true, the server tries minimizing CPU usage by caching compressed files.
	// This works differently than the github.com/gofiber/compression middleware.
	// Optional. Default value false
	Compress bool `json:"compress"`

	// When set to true, enables byte range requests.
	// Optional. Default value false
	ByteRange bool `json:"byte_range"`

	// When set to true, enables directory browsing.
	// Optional. Default value false.
	Browse bool `json:"browse"`

	// When set to true, enables direct download.
	// Optional. Default value false.
	Download bool `json:"download"`

	// The name of the index file for serving a directory.
	// Optional. Default value "index.html".
	Index string `json:"index"`

	// Expiration duration for inactive file handlers.
	// Use a negative time.Duration to disable it.
	//
	// Optional. Default value 10 * time.Second.
	CacheDuration time.Duration `json:"cache_duration"`

	// The value for the Cache-Control HTTP-header
	// that is set on the file response. MaxAge is defined in seconds.
	//
	// Optional. Default value 0.
	MaxAge int `json:"max_age"`

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *Ctx) bool
}

func NewStatic(root string, config ...StaticConfig) *Static {
	st := Static{
		Root: root,
	}

	if len(config) > 0 {
		st.Config = config[0]
	}

	return &st
}

func (app *App) registerStatic(prefix, root string, config ...StaticConfig) IRouter {
	// For security we want to restrict to the current work directory.
	if root == "" {
		root = "."
	}
	// Cannot have an empty prefix
	if prefix == "" {
		prefix = "/"
	}
	// Prefix always start with a '/' or '*'
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	// in case sensitive routing, all to lowercase
	if !app.config.CaseSensitive {
		prefix = utils.ToLower(prefix)
	}
	// Strip trailing slashes from the root path
	if len(root) > 0 && root[len(root)-1] == '/' {
		root = root[:len(root)-1]
	}
	// Is prefix a direct wildcard?
	isStar := prefix == "/*"
	// Is prefix a root slash?
	isRoot := prefix == "/"
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
	// Fileserver settings
	fs := &fasthttp.FS{
		Root:                 root,
		AllowEmptyRoot:       true,
		GenerateIndexPages:   false,
		AcceptByteRange:      false,
		Compress:             false,
		CompressedFileSuffix: app.config.CompressedFileSuffix,
		CacheDuration:        10 * time.Second,
		IndexNames:           []string{"index.html"},
		PathRewrite: func(fctx *fasthttp.RequestCtx) []byte {
			path := fctx.Path()
			if len(path) >= prefixLen {
				if isStar && app.getString(path[0:prefixLen]) == prefix {
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
		},
		PathNotFound: func(fctx *fasthttp.RequestCtx) {
			fctx.Response.SetStatusCode(StatusNotFound)
		},
	}

	// Set config if provided
	var cacheControlValue string
	if len(config) > 0 {
		maxAge := config[0].MaxAge
		if maxAge > 0 {
			cacheControlValue = "public, max-age=" + strconv.Itoa(maxAge)
		}
		fs.CacheDuration = config[0].CacheDuration
		fs.Compress = config[0].Compress
		fs.AcceptByteRange = config[0].ByteRange
		fs.GenerateIndexPages = config[0].Browse
		if config[0].Index != "" {
			fs.IndexNames = []string{config[0].Index}
		}
	}
	fileHandler := fs.NewRequestHandler()
	handler := func(c *Ctx) error {
		// Don't execute middleware if Next returns true
		if len(config) != 0 && config[0].Next != nil && config[0].Next(c) {
			return c.Next()
		}
		// Serve file
		fileHandler(c.fasthttp)
		// Sets the response Content-Disposition header to attachment if the Download option is true
		if len(config) > 0 && config[0].Download {
			c.Attachment()
		}
		// Return request if found and not forbidden
		status := c.fasthttp.Response.StatusCode()
		if status != StatusNotFound && status != StatusForbidden {
			if len(cacheControlValue) > 0 {
				c.fasthttp.Response.Header.Set(HeaderCacheControl, cacheControlValue)
			}
			return nil
		}
		// Reset response to default
		c.fasthttp.SetContentType("") // Issue #420
		c.fasthttp.Response.SetStatusCode(StatusOK)
		c.fasthttp.Response.SetBodyString("")
		// Next middleware
		return c.Next()
	}

	// Create route metadata without pointer
	route := Route{
		// Router booleans
		use:  true,
		root: isRoot,
		path: prefix,
		// Public data
		Method:   MethodGet,
		Path:     prefix,
		Handlers: []Handler{handler},
	}
	// Increment global handler count
	atomic.AddUint32(&app.handlersCount, 1)
	// Add route to stack
	app.addRoute(MethodGet, &route)
	// Add HEAD route
	app.addRoute(MethodHead, &route)
	return app
}
