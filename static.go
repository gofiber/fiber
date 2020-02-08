// 🚀 Fiber is an Express.js inspired web framework written in Go with 💖
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 Links: https://github.com/gofiber/fiber, https://fiber.wiki

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @erikdubbelboer, @savsgio, @julienschmidt, @koddr

package fiber

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Static https://fiber.wiki/application#static
func (r *Fiber) Static(args ...string) {
	prefix := "/"
	root := "./"
	wildcard := false
	// enable / disable gzipping somewhere?
	gzip := true

	if len(args) == 1 {
		root = args[0]
	} else if len(args) == 2 {
		prefix = args[0]
		root = args[1]
		if prefix[0] != '/' {
			prefix = "/" + prefix
		}
	}

	// Check if wildcard for single files
	if prefix == "*" || prefix == "/*" {
		wildcard = true
	}

	// Check if root exists
	if _, err := os.Lstat(root); err != nil {
		log.Fatal("Static: ", err)
	}

	// Lets get all files from root
	files, _, err := getFiles(root)
	if err != nil {
		log.Fatal("Static: ", err)
	}

	// ./static/compiled => static/compiled
	mount := filepath.Clean(root)

	// Loop over all files
	for _, file := range files {
		// Ignore the .gzipped files by fasthttp
		if strings.Contains(file, ".fasthttp.gz") {
			continue
		}

		// Time to create a fake path for the route match
		// static/index.html => /index.html
		path := filepath.Join(prefix, strings.Replace(file, mount, "", 1))

		// Store original file path to use in ctx handler
		filePath := file

		// If the file is an index.html, bind the prefix to index.html directly
		if filepath.Base(filePath) == "index.html" || filepath.Base(filePath) == "index.htm" {
			r.routes = append(r.routes, &Route{"GET", prefix, wildcard, false, nil, nil, func(c *Ctx) {
				c.SendFile(filePath, gzip)
			}})
		}

		// Add the route + SendFile(filepath) to routes
		r.routes = append(r.routes, &Route{"GET", path, wildcard, false, nil, nil, func(c *Ctx) {
			c.SendFile(filePath, gzip)
		}})
	}
}
