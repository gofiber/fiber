package middleware

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/gofiber/fiber"
)

// Middleware types
type (
	// FileSystemConfig defines the config for FileSystem middleware.
	FileSystemConfig struct {
		// Next defines a function to skip this middleware if returned true.
		Next func(ctx *fiber.Ctx) bool

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
)

// FileSystemConfigDefault is the default config
var FileSystemConfigDefault = FileSystemConfig{
	Next:   nil,
	Root:   nil,
	Index:  "/index.html",
	Browse: false,
}

// FileSystem is the default initiator allowing to pass a http.FileSystem
func FileSystem(root http.FileSystem) fiber.Handler {
	// Create default config
	var config = FileSystemConfigDefault
	// Set root
	config.Root = root
	// Return FileSystemWithConfig
	return FileSystemWithConfig(config)
}

// FileSystemWithConfig allows you to pass an FileSystemConfig
func FileSystemWithConfig(config FileSystemConfig) fiber.Handler {
	// Set config default values
	if config.Index == "" {
		config.Index = FileSystemConfigDefault.Index
	}
	if !strings.HasPrefix(config.Index, "/") {
		config.Index = "/" + config.Index
	}
	if config.Root == nil {
		log.Fatal("FileSystem: Root value is missing!")
	}

	// Middleware settings
	var prefix string

	// Return handler
	return func(c *fiber.Ctx) {
		// Set prefix
		if len(prefix) == 0 {
			prefix = c.Route().Path
		}

		// Strip prefix
		path := strings.TrimPrefix(c.Path(), prefix)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		file, err := config.Root.Open(path)
		if err != nil {
			c.Next(err)
			return
		}

		stat, err := file.Stat()
		if err != nil {
			c.Next(err)
			return
		}

		// Serve index if path is directory
		if stat.IsDir() {
			indexPath := strings.TrimSuffix(path, "/") + config.Index
			index, err := config.Root.Open(indexPath)
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
			if config.Browse {
				if err := dirList(c, file); err != nil {
					c.Next(err)
				}
				return
			}
			c.SendStatus(fiber.StatusForbidden)
			return
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
			c.Fasthttp.SetBodyStream(file, contentLength)
			return
		} else if c.Method() == fiber.MethodHead {
			c.Fasthttp.ResetBody()
			c.Fasthttp.Response.SkipBody = true
			c.Fasthttp.Response.Header.SetContentLength(contentLength)
			if err := file.Close(); err != nil {
				c.Next(err)
			}
			return
		}

		c.Next()
	}
}

func getFileExtension(path string) string {
	n := strings.LastIndexByte(path, '.')
	if n < 0 {
		return ""
	}
	return path[n:]
}

func dirList(c *fiber.Ctx, f http.File) error {
	fileinfos, err := f.Readdir(-1)
	if err != nil {
		return err
	}

	fm := make(map[string]os.FileInfo, len(fileinfos))
	filenames := make([]string, 0, len(fileinfos))
	for _, fi := range fileinfos {
		name := fi.Name()
		fm[name] = fi
		filenames = append(filenames, name)
	}

	basePathEscaped := html.EscapeString(c.Path())
	c.Write(fmt.Sprintf("<html><head><title>%s</title><style>.dir { font-weight: bold }</style></head><body>", basePathEscaped))
	c.Write(fmt.Sprintf("<h1>%s</h1>", basePathEscaped))
	c.Write("<ul>")

	if len(basePathEscaped) > 1 {
		parentPathEscaped := html.EscapeString(c.Path() + "/..")
		c.Write(fmt.Sprintf(`<li><a href="%s" class="dir">..</a></li>`, parentPathEscaped))
	}

	sort.Strings(filenames)
	for _, name := range filenames {
		pathEscaped := html.EscapeString(path.Join(c.Path() + "/" + name))
		fi := fm[name]
		auxStr := "dir"
		className := "dir"
		if !fi.IsDir() {
			auxStr = fmt.Sprintf("file, %d bytes", fi.Size())
			className = "file"
		}
		c.Write(fmt.Sprintf(`<li><a href="%s" class="%s">%s</a>, %s, last modified %s</li>`,
			pathEscaped, className, html.EscapeString(name), auxStr, fi.ModTime()))
	}
	c.Write("</ul></body></html>")

	c.Type("html")

	return nil
}
