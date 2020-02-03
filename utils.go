// 🔌 Fiber is an Express.js inspired web framework build on 🚀 Fasthttp.
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unsafe"
)

func getParams(path string) (params []string) {
	segments := strings.Split(path, "/")
	replacer := strings.NewReplacer(":", "", "?", "")
	for _, s := range segments {
		if s == "" {
			continue
		} else if s[0] == ':' {
			params = append(params, replacer.Replace(s))
		} else if s[0] == '*' {
			params = append(params, "*")
		}
	}
	return params
}

func getRegex(path string) (*regexp.Regexp, error) {
	pattern := "^"
	segments := strings.Split(path, "/")
	for _, s := range segments {
		if s[0] == ':' {
			if strings.Contains(s, "?") {
				pattern += "(?:/([^/]+?))?"
			} else {
				pattern += "/(?:([^/]+?))"
			}
		} else if s[0] == '*' {
			pattern += "/(.*)"
		} else {
			pattern += "/" + s
		}
	}
	pattern += "/?$"
	regex, err := regexp.Compile(pattern)
	return regex, err
}

func getFiles(root string) (files []string, isDir bool, err error) {
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		} else {
			isDir = true
		}
		return err
	})
	return files, isDir, err
}

func getType(ext string) (mime string) {
	if ext[0] == '.' {
		ext = ext[1:]
	}
	mime = contentTypes[ext]
	if mime == "" {
		return contentTypeOctetStream
	}
	return mime
}

func getStatus(status int) (msg string) {
	return statusMessages[status]
}

// #nosec G103
// getString converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
func getString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// #nosec G103
// getBytes converts string to a byte slice without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
func getBytes(s string) (b []byte) {
	// return *(*[]byte)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := *(*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return b
}
