// 🔌 Fiber is an Expressjs inspired web framework build on 🚀 Fasthttp.
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unsafe"
)

var (
	applicationjson = []byte("application/json")
)

var replacer = strings.NewReplacer(":", "", "?", "")

func getParams(path string) (params []string) {
	segments := strings.Split(path, "/")
	for _, s := range segments {
		if s == "" {
			continue
		}
		if strings.Contains(s, ":") {
			s = replacer.Replace(s)
			params = append(params, s)
			continue
		}
		if strings.Contains(s, "*") {
			params = append(params, "*")
		}
	}
	return params
}

func getRegex(path string) (*regexp.Regexp, error) {
	pattern := "^"
	segments := strings.Split(path, "/")
	for _, s := range segments {
		if s == "" {
			continue
		}
		if strings.Contains(s, ":") {
			if strings.Contains(s, "?") {
				pattern += "(?:/([^/]+?))?"
			} else {
				pattern += "/(?:([^/]+?))"
			}
		} else if strings.Contains(s, "*") {
			pattern += "/(.*)"
		} else {
			pattern += "/" + s
		}
	}
	pattern += "/?$"
	regex, err := regexp.Compile(pattern)
	return regex, err
}

// walkDir loops trough directory and store file paths in array
func walkDir(root string) (files []string, isDir bool, err error) {
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

func B2S(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
func S2B(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}

// NoCopy embed this type into a struct, which mustn't be copied,
// so `go vet` gives a warning if this struct is copied.
//
// See https://github.com/golang/go/issues/8005#issuecomment-190753527 for details.
// and also: https://stackoverflow.com/questions/52494458/nocopy-minimal-example
type noCopy struct{}

// Lock ...
func (*noCopy) Lock() {}

// Unlock ...
func (*noCopy) Unlock() {}

// StringSliceIndexOf returns index position in slice from given string
// If value is -1, the string does not found
func stringSliceIndexOf(vs []string, s string) int {
	for i, v := range vs {
		if v == s {
			return i
		}
	}
	return -1
}

// StringSliceInclude returns true or false if given string is in slice
func stringSliceInclude(vs []string, t string) bool {
	return stringSliceIndexOf(vs, t) >= 0
}
