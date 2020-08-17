package filesystem

import (
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	Next func(c *fiber.Ctx) bool

	// Root is a FileSystem that provides access
	// to a collection of files and directories.
	// Required. Default: nil
	Root http.FileSystem

	// Index file for serving a directory.
	// Optional. Default: "index.html"
	Index string

	// Enable directory browsing.
	// Optional. Default: false
	Browse bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:   nil,
	Root:   nil,
	Index:  "/index.html",
	Browse: false,
}

// New creates a new middleware handler
func New(config Config) fiber.Handler {
	// Set config
	cfg := config

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Index == "" {
		cfg.Index = ConfigDefault.Index
	}
	if !strings.HasPrefix(cfg.Index, "/") {
		cfg.Index = "/" + cfg.Index
	}
	if cfg.Root == nil {
		log.Fatal("filesystem: Root cannot be nil")
	}

	var prefix string

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Set prefix
		if len(prefix) == 0 {
			prefix = c.Route().Path
		}

		// Strip prefix
		path := strings.TrimPrefix(c.Path(), prefix)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		file, err := cfg.Root.Open(path)
		if err != nil {
			return err
		}

		stat, err := file.Stat()
		if err != nil {
			return err
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
				if err := dirList(c, file); err != nil {
					return err
				}
			}
			return c.SendStatus(fiber.StatusForbidden)
		}

		modTime := stat.ModTime()
		contentLength := int(stat.Size())

		// Set Content Type header
		c.Type(getFileExtension(stat.Name()))

		// Set Last Modified header
		if !modTime.IsZero() {
			c.Set(fiber.HeaderLastModified, modTime.UTC().Format(http.TimeFormat))
		}

		if c.Method() == fiber.MethodGet {
			c.Fasthttp().SetBodyStream(file, contentLength)
			return nil
		}
		if c.Method() == fiber.MethodHead {
			c.Fasthttp().ResetBody()
			c.Fasthttp().Response.SkipBody = true
			c.Fasthttp().Response.Header.SetContentLength(contentLength)
			if err := file.Close(); err != nil {
				return err
			}
			return nil
		}

		return c.Next()
	}
}
