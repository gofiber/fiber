package fiber

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unsafe"
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

// b2s converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func b2s(b []byte) string {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&b))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*string)(unsafe.Pointer(&bh))
}

// s2b converts string to a byte slice without memory allocation.
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func s2b(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
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
