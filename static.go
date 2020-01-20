// 🚀 Fiber, Express on Steriods
// 📌 Don't use in production until version 1.0.0
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"path/filepath"
	"strings"
)

// Static https://gofiber.github.io/fiber/#/application?id=static
func (r *Fiber) Static(args ...string) {
	prefix := "/"
	root := "./"
	wildcard := false
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
	// Lets get all files from root
	files, _, err := walkDir(root)
	if err != nil {
		panic(err)
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
		if filepath.Base(filePath) == "index.html" {
			r.routes = append(r.routes, &route{"GET", prefix, wildcard, false, nil, nil, func(c *Ctx) {
				c.SendFile(filePath, gzip)
			}})
		}
		// Add the route + SendFile(filepath) to routes
		r.routes = append(r.routes, &route{"GET", path, wildcard, false, nil, nil, func(c *Ctx) {
			c.SendFile(filePath, gzip)
		}})
	}
}
