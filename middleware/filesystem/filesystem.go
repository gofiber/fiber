package filesystem

import (
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Root is a FileSystem that provides access
	// to a collection of files and directories.
	//
	// Required. Default: nil
	Root http.FileSystem

	// Index file for serving a directory.
	//
	// Optional. Default: "index.html"
	Index string

	// Enable directory browsing.
	//
	// Optional. Default: false
	Browse bool

	// File to return if path is not found. Useful for SPA's.
	//
	// Optional. Default: ""
	NotFoundFile string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:   nil,
	Root:   nil,
	Index:  "/index.html",
	Browse: false,
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.Index == "" {
			cfg.Index = ConfigDefault.Index
		}
		if !strings.HasPrefix(cfg.Index, "/") {
			cfg.Index = "/" + cfg.Index
		}
		if cfg.NotFoundFile != "" && !strings.HasPrefix(cfg.NotFoundFile, "/") {
			cfg.NotFoundFile = "/" + cfg.NotFoundFile
		}
	}

	if cfg.Root == nil {
		panic("filesystem: Root cannot be nil")
	}

	var once sync.Once
	var prefix string

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		method := c.Method()

		// We only serve static assets on GET or HEAD methods
		if method != fiber.MethodGet && method != fiber.MethodHead {
			return c.Next()
		}

		// Set prefix once
		once.Do(func() {
			prefix = c.Route().Path
		})

		// Strip prefix
		path := strings.TrimPrefix(c.Path(), prefix)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		var (
			file http.File
			stat os.FileInfo
		)

		file, err = cfg.Root.Open(path)
		if err != nil && os.IsNotExist(err) && cfg.NotFoundFile != "" {
			file, err = cfg.Root.Open(cfg.NotFoundFile)
		}

		if err != nil {
			if os.IsNotExist(err) {
				return c.Status(fiber.StatusNotFound).Next()
			}
			return
		}

		if stat, err = file.Stat(); err != nil {
			return
		}

		// Serve index if path is directory
		if stat.IsDir() {
			indexPath := strings.TrimSuffix(path, "/") + cfg.Index
			index, err := cfg.Root.Open(indexPath)
			if err == nil {
				indexStat, err := index.Stat()
				if err == nil {
					file = index
					stat = indexStat
				}
			}
		}

		// Browse directory if no index found and browsing is enabled
		if stat.IsDir() {
			if cfg.Browse {
				return dirList(c, file)
			}
			return fiber.ErrForbidden
		}

		modTime := stat.ModTime()
		contentLength := int(stat.Size())

		// Set Content Type header
		c.Type(getFileExtension(stat.Name()))

		// Set Last Modified header
		if !modTime.IsZero() {
			c.Set(fiber.HeaderLastModified, modTime.UTC().Format(http.TimeFormat))
		}

		if method == fiber.MethodGet {
			c.Response().SetBodyStream(file, contentLength)
			return nil
		}
		if method == fiber.MethodHead {
			c.Request().ResetBody()
			// Fasthttp should skipbody by default if HEAD?
			c.Response().SkipBody = true
			c.Response().Header.SetContentLength(contentLength)
			if err := file.Close(); err != nil {
				return err
			}
			return nil
		}

		return c.Next()
	}
}
