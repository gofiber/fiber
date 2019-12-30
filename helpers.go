package fiber

import (
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

// Credits to @savsgio
// https://github.com/savsgio/gotils/blob/master/conv.go

// b2s converts byte slice to a string without memory allocation.
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
func s2b(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}
