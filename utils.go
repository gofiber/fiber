// 🔌 Fiber is an Express.js inspired web framework build on 🚀 Fasthttp.
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"github.com/valyala/fasthttp"
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

// FakeRequest creates a readWriter and calls ServeConn on local servver
func (r *Fiber) FakeRequest(raw string) (string, error) {
	server := &fasthttp.Server{
		Handler:                            r.handler,
		Name:                               r.Server,
		Concurrency:                        r.Engine.Concurrency,
		DisableKeepalive:                   r.Engine.DisableKeepAlive,
		ReadBufferSize:                     r.Engine.ReadBufferSize,
		WriteBufferSize:                    r.Engine.WriteBufferSize,
		ReadTimeout:                        r.Engine.ReadTimeout,
		WriteTimeout:                       r.Engine.WriteTimeout,
		IdleTimeout:                        r.Engine.IdleTimeout,
		MaxConnsPerIP:                      r.Engine.MaxConnsPerIP,
		MaxRequestsPerConn:                 r.Engine.MaxRequestsPerConn,
		TCPKeepalive:                       r.Engine.TCPKeepalive,
		TCPKeepalivePeriod:                 r.Engine.TCPKeepalivePeriod,
		MaxRequestBodySize:                 r.Engine.MaxRequestBodySize,
		ReduceMemoryUsage:                  r.Engine.ReduceMemoryUsage,
		GetOnly:                            r.Engine.GetOnly,
		DisableHeaderNamesNormalizing:      r.Engine.DisableHeaderNamesNormalizing,
		SleepWhenConcurrencyLimitsExceeded: r.Engine.SleepWhenConcurrencyLimitsExceeded,
		NoDefaultServerHeader:              r.Server == "",
		NoDefaultContentType:               r.Engine.NoDefaultContentType,
		KeepHijackedConns:                  r.Engine.KeepHijackedConns,
	}
	rw := &readWriter{}
	rw.r.WriteString(raw)

	ch := make(chan error)
	go func() {
		ch <- server.ServeConn(rw)
	}()

	select {
	case err := <-ch:
		if err != nil {
			return "", err
		}
	case <-time.After(200 * time.Millisecond):
		return "", fmt.Errorf("Timeout")
	}

	err := server.ServeConn(rw)
	if err != nil {
		return "", err
	}
	resp, err := ioutil.ReadAll(&rw.w)
	if err != nil {
		return "", err
	}
	return getString(resp), nil
}

// Readwriter for test cases
type readWriter struct {
	net.Conn
	r bytes.Buffer
	w bytes.Buffer
}

func (rw *readWriter) Close() error {
	return nil
}

func (rw *readWriter) Read(b []byte) (int, error) {
	return rw.r.Read(b)
}

func (rw *readWriter) Write(b []byte) (int, error) {
	return rw.w.Write(b)
}

func (rw *readWriter) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP: net.IPv4zero,
	}
}

func (rw *readWriter) LocalAddr() net.Addr {
	return rw.RemoteAddr()
}

func (rw *readWriter) SetReadDeadline(t time.Time) error {
	return nil
}

func (rw *readWriter) SetWriteDeadline(t time.Time) error {
	return nil
}
