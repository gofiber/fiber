// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unsafe"

	schema "github.com/gorilla/schema"
)

var schemaDecoder = schema.NewDecoder()

func groupPaths(prefix, path string) string {
	if path == "/" {
		path = ""
	}
	path = prefix + path
	path = strings.Replace(path, "//", "/", -1)
	return path
}

func getParams(path string) (params []string) {
	if len(path) < 1 {
		return
	}
	segments := strings.Split(path, "/")
	replacer := strings.NewReplacer(":", "", "?", "")
	for _, s := range segments {
		if s == "" {
			continue
		} else if s[0] == ':' {
			params = append(params, replacer.Replace(s))
		}
		if strings.Contains(s, "*") {
			params = append(params, "*")
		}
	}
	return
}

func getRegex(path string) (*regexp.Regexp, error) {
	pattern := "^"
	segments := strings.Split(path, "/")
	for _, s := range segments {
		if s == "" {
			continue
		}
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

func getFiles(root string) (files []string, dir bool, err error) {
	root = filepath.Clean(root)
	if _, err := os.Lstat(root); err != nil {
		return files, dir, fmt.Errorf("%s", err)
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		} else {
			dir = true
		}
		return err
	})
	return
}

func getMIME(extension string) (mime string) {
	if extension == "" {
		return mime
	}
	mime = extensionMIME[extension]
	if mime == "" {
		return MIMEOctetStream
	}
	return mime
}

// #nosec G103
// getString converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
var getString = func(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// #nosec G103
// getBytes converts string to a byte slice without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
var getBytes = func(s string) (b []byte) {
	return *(*[]byte)(unsafe.Pointer(&s))
}

// https://golang.org/src/net/net.go#L113
// Helper methods for application#test
type testConn struct {
	net.Conn
	r bytes.Buffer
	w bytes.Buffer
}

func (c *testConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP: net.IPv4(0, 0, 0, 0),
	}
}
func (c *testConn) LocalAddr() net.Addr                { return c.RemoteAddr() }
func (c *testConn) Read(b []byte) (int, error)         { return c.r.Read(b) }
func (c *testConn) Write(b []byte) (int, error)        { return c.w.Write(b) }
func (c *testConn) Close() error                       { return nil }
func (c *testConn) SetDeadline(t time.Time) error      { return nil }
func (c *testConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *testConn) SetWriteDeadline(t time.Time) error { return nil }
