// 🚀 Fiber is an Express.js inspired web framework written in Go with 💖
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 Links: https://github.com/gofiber/fiber, https://fiber.wiki

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @erikdubbelboer, @savsgio, @julienschmidt, @koddr

package fiber

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
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
	if ext == "" {
		return mime
	}
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
	sh := *(*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bh.Data, bh.Len, bh.Cap = sh.Data, sh.Len, sh.Len
	return b
}

// Test takes a http.Request and execute a fake connection to the application
// It returns a http.Response when the connection was successfull
func (r *Fiber) Test(req *http.Request) (*http.Response, error) {
	// Get raw http request
	reqRaw, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, err
	}
	// Setup a fiber server struct
	r.httpServer = r.setupServer()
	// Create fake connection
	conn := &conn{}
	// Pass HTTP request to conn
	_, err = conn.r.Write(reqRaw)
	if err != nil {
		return nil, err
	}
	// Serve conn to server
	channel := make(chan error)
	go func() {
		channel <- r.httpServer.ServeConn(conn)
	}()
	// Wait for callback
	select {
	case err := <-channel:
		if err != nil {
			return nil, err
		}
		// Throw timeout error after 200ms
	case <-time.After(500 * time.Millisecond):
		return nil, fmt.Errorf("Timeout")
	}
	// Get raw HTTP response
	respRaw, err := ioutil.ReadAll(&conn.w)
	if err != nil {
		return nil, err
	}
	// Create buffer
	reader := strings.NewReader(getString(respRaw))
	buffer := bufio.NewReader(reader)
	// Convert raw HTTP response to http.Response
	resp, err := http.ReadResponse(buffer, req)
	if err != nil {
		return nil, err
	}
	// Return *http.Response
	return resp, nil
}

// https://golang.org/src/net/net.go#L113
type conn struct {
	net.Conn
	r bytes.Buffer
	w bytes.Buffer
}

func (c *conn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP: net.IPv4(0, 0, 0, 0),
	}
}
func (c *conn) LocalAddr() net.Addr                { return c.LocalAddr() }
func (c *conn) Read(b []byte) (int, error)         { return c.r.Read(b) }
func (c *conn) Write(b []byte) (int, error)        { return c.w.Write(b) }
func (c *conn) Close() error                       { return nil }
func (c *conn) SetDeadline(t time.Time) error      { return nil }
func (c *conn) SetReadDeadline(t time.Time) error  { return nil }
func (c *conn) SetWriteDeadline(t time.Time) error { return nil }
