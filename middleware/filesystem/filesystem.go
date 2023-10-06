package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
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
	Root http.FileSystem `json:"-"`

	// PathPrefix defines a prefix to be added to a filepath when
	// reading a file from the FileSystem.
	//
	// Use when using Go 1.16 embed.FS
	//
	// Optional. Default ""
	PathPrefix string `json:"path_prefix"`

	// Enable directory browsing.
	//
	// Optional. Default: false
	Browse bool `json:"browse"`

	// Index file for serving a directory.
	//
	// Optional. Default: "index.html"
	Index string `json:"index"`

	// The value for the Cache-Control HTTP-header
	// that is set on the file response. MaxAge is defined in seconds.
	//
	// Optional. Default value 0.
	MaxAge int `json:"max_age"`

	// File to return if path is not found. Useful for SPA's.
	//
	// Optional. Default: ""
	NotFoundFile string `json:"not_found_file"`

	// The value for the Content-Type HTTP-header
	// that is set on the file response
	//
	// Optional. Default: ""
	ContentTypeCharset string `json:"content_type_charset"`
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:               nil,
	Root:               nil,
	PathPrefix:         "",
	Browse:             false,
	Index:              "/index.html",
	MaxAge:             0,
	ContentTypeCharset: "",
}

// New creates a new middleware handler.
//
// filesystem does not handle url encoded values (for example spaces)
// on it's own. If you need that functionality, set "UnescapePath"
// in fiber.Config
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

	if cfg.PathPrefix != "" && !strings.HasPrefix(cfg.PathPrefix, "/") {
		cfg.PathPrefix = "/" + cfg.PathPrefix
	}

	var once sync.Once
	var prefix string
	cacheControlStr := "public, max-age=" + strconv.Itoa(cfg.MaxAge)

	// Return new handler
	return func(c *fiber.Ctx) error {
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
		// Add PathPrefix
		if cfg.PathPrefix != "" {
			// PathPrefix already has a "/" prefix
			path = cfg.PathPrefix + path
		}

		if len(path) > 1 {
			path = utils.TrimRight(path, '/')
		}
		file, err := cfg.Root.Open(path)
		if err != nil && errors.Is(err, fs.ErrNotExist) && cfg.NotFoundFile != "" {
			file, err = cfg.Root.Open(cfg.NotFoundFile)
		}
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return c.Status(fiber.StatusNotFound).Next()
			}
			return fmt.Errorf("failed to open: %w", err)
		}

		stat, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat: %w", err)
		}

		// Serve index if path is directory
		if stat.IsDir() {
			indexPath := utils.TrimRight(path, '/') + cfg.Index
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

		c.Status(fiber.StatusOK)

		modTime := stat.ModTime()
		contentLength := int(stat.Size())

		// Set Content Type header
		if cfg.ContentTypeCharset == "" {
			c.Type(getFileExtension(stat.Name()))
		} else {
			c.Type(getFileExtension(stat.Name()), cfg.ContentTypeCharset)
		}

		// Set Last Modified header
		if !modTime.IsZero() {
			c.Set(fiber.HeaderLastModified, modTime.UTC().Format(http.TimeFormat))
		}

		if method == fiber.MethodGet {
			if cfg.MaxAge > 0 {
				c.Set(fiber.HeaderCacheControl, cacheControlStr)
			}
			c.Response().SetBodyStream(file, contentLength)
			return nil
		}
		if method == fiber.MethodHead {
			c.Request().ResetBody()
			// Fasthttp should skipbody by default if HEAD?
			c.Response().SkipBody = true
			c.Response().Header.SetContentLength(contentLength)
			if err := file.Close(); err != nil {
				return fmt.Errorf("failed to close: %w", err)
			}
			return nil
		}

		return c.Next()
	}
}

// SendFile serves a file from an HTTP file system at the specified path.
// It handles content serving, sets appropriate headers, and returns errors when needed.
// Usage: err := SendFile(ctx, fs, "/path/to/file.txt")
func SendFile(c *fiber.Ctx, filesystem http.FileSystem, path string) error {
	file, err := filesystem.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fiber.ErrNotFound
		}
		return fmt.Errorf("failed to open: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat: %w", err)
	}

	// Serve index if path is directory
	if stat.IsDir() {
		indexPath := utils.TrimRight(path, '/') + ConfigDefault.Index
		index, err := filesystem.Open(indexPath)
		if err == nil {
			indexStat, err := index.Stat()
			if err == nil {
				file = index
				stat = indexStat
			}
		}
	}

	// Return forbidden if no index found
	if stat.IsDir() {
		return fiber.ErrForbidden
	}

	c.Status(fiber.StatusOK)

	modTime := stat.ModTime()
	contentLength := int(stat.Size())

	// Set Content Type header
	c.Type(getFileExtension(stat.Name()))

	// Set Last Modified header
	if !modTime.IsZero() {
		c.Set(fiber.HeaderLastModified, modTime.UTC().Format(http.TimeFormat))
	}

	method := c.Method()
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
			return fmt.Errorf("failed to close: %w", err)
		}
		return nil
	}

	return nil
}
