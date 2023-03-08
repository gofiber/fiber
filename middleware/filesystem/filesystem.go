package filesystem

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Root is a FileSystem that provides access
	// to a collection of files and directories.
	//
	// Required. Default: nil
	Root fs.FS `json:"-"`

	// PathPrefix defines a prefix to be added to a filepath when
	// reading a file from the FileSystem.
	//
	// Optional. Default "."
	PathPrefix string `json:"path_prefix"`

	// Enable directory browsing.
	//
	// Optional. Default: false
	Browse bool `json:"browse"`

	// Index file for serving a directory.
	//
	// Optional. Default: "index.html"
	Index string `json:"index"`

	// When set to true, enables direct download for files.
	//
	// Optional. Default: false.
	Download bool `json:"download"`

	// The value for the Cache-Control HTTP-header
	// that is set on the file response. MaxAge is defined in seconds.
	//
	// Optional. Default value 0.
	MaxAge int `json:"max_age"`

	// File to return if path is not found. Useful for SPA's.
	//
	// Optional. Default: ""
	NotFoundFile string `json:"not_found_file"`
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:       nil,
	Root:       nil,
	PathPrefix: ".",
	Browse:     false,
	Index:      "/index.html",
	MaxAge:     0,
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
		if cfg.PathPrefix == "" {
			cfg.PathPrefix = ConfigDefault.PathPrefix
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

	// PathPrefix configurations for io/fs compatibility.
	if cfg.PathPrefix != "." && !strings.HasPrefix(cfg.PathPrefix, "/") {
		cfg.PathPrefix = "./" + cfg.PathPrefix
	}

	if cfg.NotFoundFile != "" {
		cfg.NotFoundFile = filepath.Join(cfg.PathPrefix, filepath.Clean("/"+cfg.NotFoundFile))
	}

	var once sync.Once
	var prefix string
	cacheControlStr := "public, max-age=" + strconv.Itoa(cfg.MaxAge)

	// Return new handler
	return func(c fiber.Ctx) error {
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
			file fs.File
			stat os.FileInfo
		)

		// Add PathPrefix
		if cfg.PathPrefix != "" {
			// PathPrefix already has a "/" prefix
			path = filepath.Join(cfg.PathPrefix, filepath.Clean("/"+path))
		}

		if len(path) > 1 {
			path = strings.TrimRight(path, "/")
		}

		file, err := openFile(cfg.Root, path)

		if err != nil && os.IsNotExist(err) && cfg.NotFoundFile != "" {
			file, err = openFile(cfg.Root, cfg.NotFoundFile)
		}
		if err != nil {
			if os.IsNotExist(err) {
				return c.Status(fiber.StatusNotFound).Next()
			}
			return fmt.Errorf("failed to open: %w", err)
		}

		stat, err = file.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat: %w", err)
		}

		// Serve index if path is directory
		if stat.IsDir() {
			indexPath := strings.TrimRight(path, "/") + cfg.Index
			indexPath = filepath.Join(cfg.PathPrefix, filepath.Clean("/"+indexPath))

			index, err := openFile(cfg.Root, indexPath)
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

		// Sets the response Content-Disposition header to attachment if the Download option is true and if it's a file
		if cfg.Download && !stat.IsDir() {
			c.Attachment()
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

// SendFile ...
func SendFile(c fiber.Ctx, filesystem fs.FS, path string) error {
	var (
		file fs.File
		stat os.FileInfo
	)

	path = filepath.Join(".", filepath.Clean("/"+path))

	file, err := openFile(filesystem, path)
	if err != nil {
		if os.IsNotExist(err) {
			return fiber.ErrNotFound
		}
		return fmt.Errorf("failed to open: %w", err)
	}

	stat, err = file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat: %w", err)
	}

	// Serve index if path is directory
	if stat.IsDir() {
		indexPath := strings.TrimRight(path, "/") + ConfigDefault.Index
		index, err := openFile(filesystem, indexPath)
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
