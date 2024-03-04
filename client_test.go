//nolint:wrapcheck // We must not wrap errors in tests
package fiber

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/tlstest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func startServer(t *testing.T, app *App, ln *fasthttputil.InmemoryListener) {
	t.Helper()
	go func() {
		err := app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
		})
		if err != nil {
			panic(err)
		}
	}()

	// Server readiness check
	for i := 0; i < 10; i++ {
		conn, err := ln.Dial()
		if err == nil {
			conn.Close() //nolint:errcheck // We don't care about the error here
			break
		}
		// Wait a bit before retrying
		time.Sleep(100 * time.Millisecond)
		if i == 9 {
			t.Fatalf("Server did not become ready in time: %v", err)
		}
	}
}

func Test_Client_Invalid_URL(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString(c.Host())
	})

	// Wait for server to start
	startServer(t, app, ln)

	a := Get("http://example.com\r\n\r\nGET /\r\n\r\n")

	a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

	_, body, errs := a.String()

	require.Equal(t, "", body)
	require.Len(t, errs, 1)
	require.Error(t, errs[0],
		`Expected error "missing required Host header in request"`)
}

func Test_Client_Unsupported_Protocol(t *testing.T) {
	a := Get("ftp://example.com")

	_, body, errs := a.String()

	require.Equal(t, "", body)
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[0], `unsupported protocol "ftp". http and https are supported`)
}

func Test_Client_Get(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString(c.Host())
	})

	// Wait for server to start
	startServer(t, app, ln)

	for i := 0; i < 5; i++ {
		a := Get("http://example.com")

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, StatusOK, code)
		require.Equal(t, "example.com", body)
		require.Empty(t, errs)
	}
}

func Test_Client_Head(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Head("/", func(c Ctx) error {
		return c.SendStatus(StatusAccepted)
	})

	// Wait for server to start
	startServer(t, app, ln)

	for i := 0; i < 5; i++ {
		a := Head("http://example.com")

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, StatusAccepted, code)
		require.Equal(t, "", body)
		require.Empty(t, errs)
	}
}

func Test_Client_Post(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Post("/", func(c Ctx) error {
		return c.Status(StatusCreated).
			SendString(c.FormValue("foo"))
	})

	// Wait for server to start
	startServer(t, app, ln)

	for i := 0; i < 5; i++ {
		args := AcquireArgs()

		args.Set("foo", "bar")

		a := Post("http://example.com").
			Form(args)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, StatusCreated, code)
		require.Equal(t, "bar", body)
		require.Empty(t, errs)

		ReleaseArgs(args)
	}
}

func Test_Client_Put(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Put("/", func(c Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	// Wait for server to start
	startServer(t, app, ln)

	for i := 0; i < 5; i++ {
		args := AcquireArgs()

		args.Set("foo", "bar")

		a := Put("http://example.com").
			Form(args)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, StatusOK, code)
		require.Equal(t, "bar", body)
		require.Empty(t, errs)

		ReleaseArgs(args)
	}
}

func Test_Client_Patch(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Patch("/", func(c Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	// Wait for server to start
	startServer(t, app, ln)

	for i := 0; i < 5; i++ {
		args := AcquireArgs()

		args.Set("foo", "bar")

		a := Patch("http://example.com").
			Form(args)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, StatusOK, code)
		require.Equal(t, "bar", body)
		require.Empty(t, errs)

		ReleaseArgs(args)
	}
}

func Test_Client_Delete(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Delete("/", func(c Ctx) error {
		return c.Status(StatusNoContent).
			SendString("deleted")
	})

	// Wait for server to start
	startServer(t, app, ln)

	for i := 0; i < 5; i++ {
		args := AcquireArgs()

		a := Delete("http://example.com")

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, StatusNoContent, code)
		require.Equal(t, "", body)
		require.Empty(t, errs)

		ReleaseArgs(args)
	}
}

func Test_Client_UserAgent(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.Send(c.Request().Header.UserAgent())
	})

	// Wait for server to start
	startServer(t, app, ln)

	t.Run("default", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			a := Get("http://example.com")

			a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

			code, body, errs := a.String()

			require.Equal(t, StatusOK, code)
			require.Equal(t, defaultUserAgent, body)
			require.Empty(t, errs)
		}
	})

	t.Run("custom", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			c := AcquireClient()
			c.UserAgent = "ua"

			a := c.Get("http://example.com")

			a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

			code, body, errs := a.String()

			require.Equal(t, StatusOK, code)
			require.Equal(t, "ua", body)
			require.Empty(t, errs)
			ReleaseClient(c)
		}
	})
}

func Test_Client_Agent_Set_Or_Add_Headers(t *testing.T) {
	handler := func(c Ctx) error {
		c.Request().Header.VisitAll(func(key, value []byte) {
			if k := string(key); k == "K1" || k == "K2" {
				_, err := c.Write(key)
				require.NoError(t, err)
				_, err = c.Write(value)
				require.NoError(t, err)
			}
		})
		return nil
	}

	wrapAgent := func(a *Agent) {
		a.Set("k1", "v1").
			SetBytesK([]byte("k1"), "v1").
			SetBytesV("k1", []byte("v1")).
			AddBytesK([]byte("k1"), "v11").
			AddBytesV("k1", []byte("v22")).
			AddBytesKV([]byte("k1"), []byte("v33")).
			SetBytesKV([]byte("k2"), []byte("v2")).
			Add("k2", "v22")
	}

	testAgent(t, handler, wrapAgent, "K1v1K1v11K1v22K1v33K2v2K2v22")
}

func Test_Client_Agent_Connection_Close(t *testing.T) {
	handler := func(c Ctx) error {
		if c.Request().Header.ConnectionClose() {
			return c.SendString("close")
		}
		return c.SendString("not close")
	}

	wrapAgent := func(a *Agent) {
		a.ConnectionClose()
	}

	testAgent(t, handler, wrapAgent, "close")
}

func Test_Client_Agent_UserAgent(t *testing.T) {
	handler := func(c Ctx) error {
		return c.Send(c.Request().Header.UserAgent())
	}

	wrapAgent := func(a *Agent) {
		a.UserAgent("ua").
			UserAgentBytes([]byte("ua"))
	}

	testAgent(t, handler, wrapAgent, "ua")
}

func Test_Client_Agent_Cookie(t *testing.T) {
	handler := func(c Ctx) error {
		return c.SendString(
			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3") + c.Cookies("k4"))
	}

	wrapAgent := func(a *Agent) {
		a.Cookie("k1", "v1").
			CookieBytesK([]byte("k2"), "v2").
			CookieBytesKV([]byte("k2"), []byte("v2")).
			Cookies("k3", "v3", "k4", "v4").
			CookiesBytesKV([]byte("k3"), []byte("v3"), []byte("k4"), []byte("v4"))
	}

	testAgent(t, handler, wrapAgent, "v1v2v3v4")
}

func Test_Client_Agent_Referer(t *testing.T) {
	handler := func(c Ctx) error {
		return c.Send(c.Request().Header.Referer())
	}

	wrapAgent := func(a *Agent) {
		a.Referer("http://referer.com").
			RefererBytes([]byte("http://referer.com"))
	}

	testAgent(t, handler, wrapAgent, "http://referer.com")
}

func Test_Client_Agent_ContentType(t *testing.T) {
	handler := func(c Ctx) error {
		return c.Send(c.Request().Header.ContentType())
	}

	wrapAgent := func(a *Agent) {
		a.ContentType("custom-type").
			ContentTypeBytes([]byte("custom-type"))
	}

	testAgent(t, handler, wrapAgent, "custom-type")
}

func Test_Client_Agent_Host(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString(c.Host())
	})

	// Wait for server to start
	startServer(t, app, ln)

	a := Get("http://1.1.1.1:8080").
		Host("example.com").
		HostBytes([]byte("example.com"))

	require.Equal(t, "1.1.1.1:8080", a.HostClient.Addr)

	a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

	code, body, errs := a.String()

	require.Equal(t, StatusOK, code)
	require.Equal(t, "example.com", body)
	require.Empty(t, errs)
}

func Test_Client_Agent_QueryString(t *testing.T) {
	handler := func(c Ctx) error {
		return c.Send(c.Request().URI().QueryString())
	}

	wrapAgent := func(a *Agent) {
		a.QueryString("foo=bar&bar=baz").
			QueryStringBytes([]byte("foo=bar&bar=baz"))
	}

	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
}

func Test_Client_Agent_BasicAuth(t *testing.T) {
	handler := func(c Ctx) error {
		// Get authorization header
		auth := c.Get(HeaderAuthorization)
		// Decode the header contents
		raw, err := base64.StdEncoding.DecodeString(auth[6:])
		require.NoError(t, err)

		return c.Send(raw)
	}

	wrapAgent := func(a *Agent) {
		a.BasicAuth("foo", "bar").
			BasicAuthBytes([]byte("foo"), []byte("bar"))
	}

	testAgent(t, handler, wrapAgent, "foo:bar")
}

func Test_Client_Agent_BodyString(t *testing.T) {
	handler := func(c Ctx) error {
		return c.Send(c.Request().Body())
	}

	wrapAgent := func(a *Agent) {
		a.BodyString("foo=bar&bar=baz")
	}

	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
}

func Test_Client_Agent_Body(t *testing.T) {
	handler := func(c Ctx) error {
		return c.Send(c.Request().Body())
	}

	wrapAgent := func(a *Agent) {
		a.Body([]byte("foo=bar&bar=baz"))
	}

	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
}

func Test_Client_Agent_BodyStream(t *testing.T) {
	handler := func(c Ctx) error {
		return c.Send(c.Request().Body())
	}

	wrapAgent := func(a *Agent) {
		a.BodyStream(strings.NewReader("body stream"), -1)
	}

	testAgent(t, handler, wrapAgent, "body stream")
}

func Test_Client_Agent_Custom_Response(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString("custom")
	})

	// Wait for server to start
	startServer(t, app, ln)

	for i := 0; i < 5; i++ {
		a := AcquireAgent()
		resp := AcquireResponse()

		req := a.Request()
		req.Header.SetMethod(MethodGet)
		req.SetRequestURI("http://example.com")

		require.NoError(t, a.Parse())

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.SetResponse(resp).
			String()

		require.Equal(t, StatusOK, code)
		require.Equal(t, "custom", body)
		require.Equal(t, "custom", string(resp.Body()))
		require.Empty(t, errs)

		ReleaseResponse(resp)
	}
}

func Test_Client_Agent_Dest(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString("dest")
	})

	// Wait for server to start
	startServer(t, app, ln)

	t.Run("small dest", func(t *testing.T) {
		dest := []byte("de")

		c := AcquireClient()
		a := c.Get("http://example.com")
		defer ReleaseClient(c)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.Dest(dest[:0]).String()

		require.Equal(t, StatusOK, code)
		require.Equal(t, "dest", body)
		require.Equal(t, "de", string(dest))
		require.Empty(t, errs)
	})

	t.Run("enough dest", func(t *testing.T) {
		dest := []byte("foobar")

		c := AcquireClient()
		a := c.Get("http://example.com")
		defer ReleaseClient(c)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.Dest(dest[:0]).String()

		require.Equal(t, StatusOK, code)
		require.Equal(t, "dest", body)
		require.Equal(t, "destar", string(dest))
		require.Empty(t, errs)
	})
}

// readErrorConn is a struct for testing retryIf
type readErrorConn struct {
	net.Conn
}

func (*readErrorConn) Read(_ []byte) (int, error) {
	return 0, errors.New("error")
}

func (*readErrorConn) Write(p []byte) (int, error) {
	return len(p), nil
}

func (*readErrorConn) Close() error {
	return nil
}

func (*readErrorConn) LocalAddr() net.Addr {
	return nil
}

func (*readErrorConn) RemoteAddr() net.Addr {
	return nil
}

func (*readErrorConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (*readErrorConn) SetWriteDeadline(_ time.Time) error {
	return nil
}

func Test_Client_Agent_RetryIf(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	// Wait for server to start
	startServer(t, app, ln)

	a := Post("http://example.com").
		RetryIf(func(_ *Request) bool {
			return true
		})
	dialsCount := 0
	a.HostClient.Dial = func(_ string) (net.Conn, error) {
		dialsCount++
		switch dialsCount {
		case 1:
			return &readErrorConn{}, nil
		case 2:
			return &readErrorConn{}, nil
		case 3:
			return &readErrorConn{}, nil
		case 4:
			return ln.Dial()
		default:
			t.Fatalf("unexpected number of dials: %d", dialsCount)
		}
		panic("unreachable")
	}

	_, _, errs := a.String()
	require.Equal(t, 4, dialsCount)
	require.Empty(t, errs)
}

func Test_Client_Agent_Json(t *testing.T) {
	// Test without ctype parameter
	handler := func(c Ctx) error {
		require.Equal(t, MIMEApplicationJSON, string(c.Request().Header.ContentType()))

		return c.Send(c.Request().Body())
	}

	wrapAgent := func(a *Agent) {
		a.JSON(data{Success: true})
	}

	testAgent(t, handler, wrapAgent, `{"success":true}`)

	// Test with ctype parameter
	handler = func(c Ctx) error {
		require.Equal(t, "application/problem+json", string(c.Request().Header.ContentType()))

		return c.Send(c.Request().Body())
	}

	wrapAgent = func(a *Agent) {
		a.JSON(data{Success: true}, "application/problem+json")
	}

	testAgent(t, handler, wrapAgent, `{"success":true}`)
}

func Test_Client_Agent_Json_Error(t *testing.T) {
	a := Get("http://example.com").
		JSONEncoder(json.Marshal).
		JSON(complex(1, 1))

	_, body, errs := a.String()

	require.Equal(t, "", body)
	require.Len(t, errs, 1)
	wantErr := new(json.UnsupportedTypeError)
	require.ErrorAs(t, errs[0], &wantErr)
}

func Test_Client_Agent_XML(t *testing.T) {
	handler := func(c Ctx) error {
		require.Equal(t, MIMEApplicationXML, string(c.Request().Header.ContentType()))

		return c.Send(c.Request().Body())
	}

	wrapAgent := func(a *Agent) {
		a.XML(data{Success: true})
	}

	testAgent(t, handler, wrapAgent, "<data><success>true</success></data>")
}

func Test_Client_Agent_XML_Error(t *testing.T) {
	a := Get("http://example.com").
		XML(complex(1, 1))

	_, body, errs := a.String()
	require.Equal(t, "", body)
	require.Len(t, errs, 1)
	wantErr := new(xml.UnsupportedTypeError)
	require.ErrorAs(t, errs[0], &wantErr)
}

func Test_Client_Agent_Form(t *testing.T) {
	handler := func(c Ctx) error {
		require.Equal(t, MIMEApplicationForm, string(c.Request().Header.ContentType()))

		return c.Send(c.Request().Body())
	}

	args := AcquireArgs()

	args.Set("foo", "bar")

	wrapAgent := func(a *Agent) {
		a.Form(args)
	}

	testAgent(t, handler, wrapAgent, "foo=bar")

	ReleaseArgs(args)
}

func Test_Client_Agent_MultipartForm(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Post("/", func(c Ctx) error {
		require.Equal(t, "multipart/form-data; boundary=myBoundary", c.Get(HeaderContentType))

		mf, err := c.MultipartForm()
		require.NoError(t, err)
		require.Equal(t, "bar", mf.Value["foo"][0])

		return c.Send(c.Request().Body())
	})

	// Wait for server to start
	startServer(t, app, ln)

	args := AcquireArgs()

	args.Set("foo", "bar")

	a := Post("http://example.com").
		Boundary("myBoundary").
		MultipartForm(args)

	a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

	code, body, errs := a.String()

	require.Equal(t, StatusOK, code)
	require.Equal(t, "--myBoundary\r\nContent-Disposition: form-data; name=\"foo\"\r\n\r\nbar\r\n--myBoundary--\r\n", body)
	require.Empty(t, errs)
	ReleaseArgs(args)
}

func Test_Client_Agent_MultipartForm_Errors(t *testing.T) {
	a := AcquireAgent()
	a.mw = &errorMultipartWriter{}

	args := AcquireArgs()
	args.Set("foo", "bar")

	ff1 := &FormFile{"", "name1", []byte("content"), false}
	ff2 := &FormFile{"", "name2", []byte("content"), false}
	a.FileData(ff1, ff2).
		MultipartForm(args)

	require.Len(t, a.errs, 4)
	ReleaseArgs(args)
}

func Test_Client_Agent_MultipartForm_SendFiles(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Post("/", func(c Ctx) error {
		require.Equal(t, "multipart/form-data; boundary=myBoundary", c.Get(HeaderContentType))

		fh1, err := c.FormFile("field1")
		require.NoError(t, err)
		require.Equal(t, "name", fh1.Filename)
		buf := make([]byte, fh1.Size)
		f, err := fh1.Open()
		require.NoError(t, err)
		defer func() {
			err := f.Close()
			require.NoError(t, err)
		}()
		_, err = f.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "form file", string(buf))

		fh2, err := c.FormFile("index")
		require.NoError(t, err)
		checkFormFile(t, fh2, ".github/testdata/index.html")

		fh3, err := c.FormFile("file3")
		require.NoError(t, err)
		checkFormFile(t, fh3, ".github/testdata/index.tmpl")

		return c.SendString("multipart form files")
	})

	// Wait for server to start
	startServer(t, app, ln)

	for i := 0; i < 5; i++ {
		ff := AcquireFormFile()
		ff.Fieldname = "field1"
		ff.Name = "name"
		ff.Content = []byte("form file")

		a := Post("http://example.com").
			Boundary("myBoundary").
			FileData(ff).
			SendFiles(".github/testdata/index.html", "index", ".github/testdata/index.tmpl").
			MultipartForm(nil)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, StatusOK, code)
		require.Equal(t, "multipart form files", body)
		require.Empty(t, errs)

		ReleaseFormFile(ff)
	}
}

func checkFormFile(t *testing.T, fh *multipart.FileHeader, filename string) {
	t.Helper()

	basename := filepath.Base(filename)
	require.Equal(t, fh.Filename, basename)

	b1, err := os.ReadFile(filename) //nolint:gosec // We're in a test so reading user-provided files by name is fine
	require.NoError(t, err)

	b2 := make([]byte, fh.Size)
	f, err := fh.Open()
	require.NoError(t, err)
	defer func() {
		err := f.Close()
		require.NoError(t, err)
	}()
	_, err = f.Read(b2)
	require.NoError(t, err)
	require.Equal(t, b1, b2)
}

func Test_Client_Agent_Multipart_Random_Boundary(t *testing.T) {
	a := Post("http://example.com").
		MultipartForm(nil)

	reg := regexp.MustCompile(`multipart/form-data; boundary=\w{30}`)

	require.True(t, reg.Match(a.req.Header.Peek(HeaderContentType)))
}

func Test_Client_Agent_Multipart_Invalid_Boundary(t *testing.T) {
	a := Post("http://example.com").
		Boundary("*").
		MultipartForm(nil)

	require.Len(t, a.errs, 1)
	require.ErrorContains(t, a.errs[0], "mime: invalid boundary character")
}

func Test_Client_Agent_SendFile_Error(t *testing.T) {
	a := Post("http://example.com").
		SendFile("non-exist-file!", "")

	require.Len(t, a.errs, 1)
	require.ErrorIs(t, a.errs[0], os.ErrNotExist)
}

func Test_Client_Debug(t *testing.T) {
	handler := func(c Ctx) error {
		return c.SendString("debug")
	}

	var output bytes.Buffer

	wrapAgent := func(a *Agent) {
		a.Debug(&output)
	}

	testAgent(t, handler, wrapAgent, "debug", 1)

	str := output.String()

	require.Contains(t, str, "Connected to example.com(InmemoryListener)")
	require.Contains(t, str, "GET / HTTP/1.1")
	require.Contains(t, str, "User-Agent: fiber")
	require.Contains(t, str, "Host: example.com\r\n\r\n")
	require.Contains(t, str, "HTTP/1.1 200 OK")
	require.Contains(t, str, "Content-Type: text/plain; charset=utf-8\r\nContent-Length: 5\r\n\r\ndebug")
}

func Test_Client_Agent_Timeout(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		time.Sleep(time.Millisecond * 200)
		return c.SendString("timeout")
	})

	// Wait for server to start
	startServer(t, app, ln)

	a := Get("http://example.com").
		Timeout(time.Millisecond * 50)

	a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

	_, body, errs := a.String()

	require.Equal(t, "", body)
	require.Len(t, errs, 1)
	require.ErrorIs(t, errs[0], fasthttp.ErrTimeout)
}

func Test_Client_Agent_Reuse(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString("reuse")
	})

	// Wait for server to start
	startServer(t, app, ln)

	a := Get("http://example.com").
		Reuse()

	a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

	code, body, errs := a.String()

	require.Equal(t, StatusOK, code)
	require.Equal(t, "reuse", body)
	require.Empty(t, errs)

	code, body, errs = a.String()

	require.Equal(t, StatusOK, code)
	require.Equal(t, "reuse", body)
	require.Empty(t, errs)
}

func Test_Client_Agent_InsecureSkipVerify(t *testing.T) {
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	require.NoError(t, err)

	//nolint:gosec // We're in a test so using old ciphers is fine
	serverTLSConf := &tls.Config{
		Certificates: []tls.Certificate{cer},
	}

	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString("ignore tls")
	})

	// We can't use startServer here because the listener is a different type
	go func() {
		//nolint:errcheck // We don't care about the error here
		app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
		})
	}()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	code, body, errs := Get("https://" + ln.Addr().String()).
		InsecureSkipVerify().
		InsecureSkipVerify().
		String()

	require.Empty(t, errs)
	require.Equal(t, StatusOK, code)
	require.Equal(t, "ignore tls", body)
}

func Test_Client_Agent_TLS(t *testing.T) {
	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString("tls")
	})

	// We can't use startServer here because the listener is a different type
	go func() {
		//nolint:errcheck // We don't care about the error here
		app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
		})
	}()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	code, body, errs := Get("https://" + ln.Addr().String()).
		TLSConfig(clientTLSConf).
		String()

	require.Empty(t, errs)
	require.Equal(t, StatusOK, code)
	require.Equal(t, "tls", body)
}

func Test_Client_Agent_MaxRedirectsCount(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		if c.Request().URI().QueryArgs().Has("foo") {
			return c.Redirect().To("/foo")
		}
		return c.Redirect().To("/")
	})
	app.Get("/foo", func(c Ctx) error {
		return c.SendString("redirect")
	})

	// Wait for server to start
	startServer(t, app, ln)

	t.Run("success", func(t *testing.T) {
		a := Get("http://example.com?foo").
			MaxRedirectsCount(1)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, 200, code)
		require.Equal(t, "redirect", body)
		require.Empty(t, errs)
	})

	t.Run("error", func(t *testing.T) {
		a := Get("http://example.com").
			MaxRedirectsCount(1)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		_, body, errs := a.String()

		require.Equal(t, "", body)
		require.Len(t, errs, 1)
		require.ErrorIs(t, errs[0], fasthttp.ErrTooManyRedirects)
	})
}

func Test_Client_Agent_Struct(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.JSON(data{true})
	})

	app.Get("/error", func(c Ctx) error {
		return c.SendString(`{"success"`)
	})

	// Wait for server to start
	startServer(t, app, ln)

	t.Run("success", func(t *testing.T) {
		a := Get("http://example.com")

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		var d data

		code, body, errs := a.Struct(&d)

		require.Equal(t, StatusOK, code)
		require.Equal(t, `{"success":true}`, string(body))
		require.Empty(t, errs)
		require.True(t, d.Success)
	})

	t.Run("pre error", func(t *testing.T) {
		a := Get("http://example.com")

		errPre := errors.New("pre errors")

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }
		a.errs = append(a.errs, errPre)

		var d data
		_, body, errs := a.Struct(&d)

		require.Equal(t, "", string(body))
		require.Len(t, errs, 1)
		require.ErrorIs(t, errs[0], errPre)
		require.False(t, d.Success)
	})

	t.Run("error", func(t *testing.T) {
		a := Get("http://example.com/error")

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		var d data

		code, body, errs := a.JSONDecoder(json.Unmarshal).Struct(&d)

		require.Equal(t, StatusOK, code)
		require.Equal(t, `{"success"`, string(body))
		require.Len(t, errs, 1)
		wantErr := new(json.SyntaxError)
		require.ErrorAs(t, errs[0], &wantErr)
		require.EqualValues(t, 10, wantErr.Offset)
	})

	t.Run("nil jsonDecoder", func(t *testing.T) {
		a := AcquireAgent()
		defer ReleaseAgent(a)
		defer a.ConnectionClose()
		request := a.Request()
		request.Header.SetMethod(MethodGet)
		request.SetRequestURI("http://example.com")
		err := a.Parse()
		require.NoError(t, err)
		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }
		var d data
		code, body, errs := a.Struct(&d)
		require.Equal(t, StatusOK, code)
		require.Equal(t, `{"success":true}`, string(body))
		require.Empty(t, errs)
		require.True(t, d.Success)
	})
}

func Test_Client_Agent_Parse(t *testing.T) {
	a := Get("https://example.com:10443")

	require.NoError(t, a.Parse())
}

func testAgent(t *testing.T, handler Handler, wrapAgent func(agent *Agent), excepted string, count ...int) {
	t.Helper()

	ln := fasthttputil.NewInmemoryListener()

	app := New()

	app.Get("/", handler)

	// Wait for server to start
	startServer(t, app, ln)

	c := 1
	if len(count) > 0 {
		c = count[0]
	}

	for i := 0; i < c; i++ {
		a := Get("http://example.com")

		wrapAgent(a)

		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		require.Equal(t, StatusOK, code)
		require.Equal(t, excepted, body)
		require.Empty(t, errs)
	}
}

type data struct {
	Success bool `json:"success" xml:"success"`
}

type errorMultipartWriter struct {
	count int
}

func (*errorMultipartWriter) Boundary() string           { return "myBoundary" }
func (*errorMultipartWriter) SetBoundary(_ string) error { return nil }
func (e *errorMultipartWriter) CreateFormFile(_, _ string) (io.Writer, error) {
	if e.count == 0 {
		e.count++
		return nil, errors.New("CreateFormFile error")
	}
	return errorWriter{}, nil
}
func (*errorMultipartWriter) WriteField(_, _ string) error { return errors.New("WriteField error") }
func (*errorMultipartWriter) Close() error                 { return errors.New("Close error") }

type errorWriter struct{}

func (errorWriter) Write(_ []byte) (int, error) { return 0, errors.New("Write error") }
