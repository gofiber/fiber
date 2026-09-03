package client

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Rand_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args int
	}{
		{
			name: "test generate",
			args: 16,
		},
		{
			name: "test generate smaller string",
			args: 8,
		},
		{
			name: "test generate larger string",
			args: 32,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := unsafeRandString(tt.args)
			require.NoError(t, err)
			require.Len(t, got, tt.args)
		})
	}

	t.Run("valid characters", func(t *testing.T) {
		t.Parallel()
		got, err := unsafeRandString(32)
		require.NoError(t, err)
		for i := 0; i < len(got); i++ {
			require.Contains(t, letterBytes, string(got[i]))
		}
	})
}

func Test_Parser_Request_URL(t *testing.T) {
	t.Parallel()

	t.Run("client baseurl should be set", func(t *testing.T) {
		t.Parallel()
		client := New().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api", req.RawRequest.URI().String())
	})

	t.Run("request url should be set", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().SetURL("http://example.com/api")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api", req.RawRequest.URI().String())
	})

	t.Run("the request url will override baseurl with protocol", func(t *testing.T) {
		t.Parallel()
		client := New().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("http://example.com/api/v1")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/v1", req.RawRequest.URI().String())
	})

	t.Run("the request url should be append after baseurl without protocol", func(t *testing.T) {
		t.Parallel()
		client := New().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("/v1")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/v1", req.RawRequest.URI().String())
	})

	t.Run("the url is error", func(t *testing.T) {
		t.Parallel()
		client := New().SetBaseURL("example.com/api")
		req := AcquireRequest().SetURL("/v1")

		err := parserRequestURL(client, req)
		require.Equal(t, ErrURLFormat, err)
	})

	t.Run("the path param from client", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetBaseURL("http://example.com/api/:id").
			SetPathParam("id", "5")
		req := AcquireRequest()

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/5", req.RawRequest.URI().String())
	})

	t.Run("the path param from request", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetBaseURL("http://example.com/api/:id/:name").
			SetPathParam("id", "5")
		req := AcquireRequest().
			SetURL("/{key}").
			SetPathParams(map[string]string{
				"name": "fiber",
				"key":  "val",
			}).
			DelPathParams("key")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/5/fiber/%7Bkey%7D", req.RawRequest.URI().String())
	})

	t.Run("the path param from request and client", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetBaseURL("http://example.com/api/:id/:name").
			SetPathParam("id", "5")
		req := AcquireRequest().
			SetURL("/:key").
			SetPathParams(map[string]string{
				"name": "fiber",
				"key":  "val",
				"id":   "12",
			})

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/12/fiber/val", req.RawRequest.URI().String())
	})

	t.Run("path param does not match a longer placeholder", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetBaseURL("http://example.com/api/:idx").
			SetPathParam("id", "5")
		req := AcquireRequest()

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/:idx", req.RawRequest.URI().String())
	})

	t.Run("path params sharing a prefix are resolved independently", func(t *testing.T) {
		t.Parallel()
		client := New().SetBaseURL("http://example.com/api/:idx/:id")
		req := AcquireRequest().
			SetPathParams(map[string]string{
				"id":  "5",
				"idx": "9",
			})

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/9/5", req.RawRequest.URI().String())
	})

	t.Run("path param value cannot open a query", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetURL("http://example.com/api/:id").
			SetPathParam("id", "a?b")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/a%3Fb", req.RawRequest.URI().String())
	})

	t.Run("path param value keeps its escaped separator when normalizing is off", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetURL("http://example.com/api/:id").
			SetPathParam("id", "a/b").
			SetDisablePathNormalizing(true)

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/a%2Fb", req.RawRequest.URI().String())
	})

	t.Run("a separator in a value is rejected when normalizing will decode it", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetURL("http://example.com/api/:id").
			SetPathParam("id", "a/b")

		// The value would be escaped to "a%2Fb", but fasthttp percent-decodes
		// the path while normalizing it, so an encoded "/" is indistinguishable
		// from a real separator on the wire. Escaping still contains "?" and
		// "#", which normalizing does not decode.
		err := parserRequestURL(client, req)
		require.ErrorIs(t, err, ErrPathParamInPath)
	})

	t.Run("path param value cannot rewrite the host", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetURL("http://:host/api").
			SetPathParam("host", "x@evil.com")

		// "@" ends a userinfo section, so this used to resolve to the host
		// "evil.com" and the request left for another server.
		err := parserRequestURL(client, req)
		require.ErrorIs(t, err, ErrPathParamInHost)
	})

	t.Run("a host path param keeps the rest of the URL intact", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetURL("http://:host/api/v1?a=b").
			SetPathParam("host", "\u00fcn\u00efcode.example.com")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "\u00fcn\u00efcode.example.com", string(req.RawRequest.URI().Host()))
		// A percent-escape in the host makes fasthttp abandon the rest of the
		// URI, so this pins that a host value never gets escaped.
		require.Equal(t, "/api/v1", string(req.RawRequest.URI().Path()))
		require.Equal(t, "a=b", string(req.RawRequest.URI().QueryString()))
	})

	t.Run("a substituted value is not substituted again", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetURL("http://example.com/api/:id").
			SetPathParams(map[string]string{
				"id":   ":name",
				"name": "fiber",
			})

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/:name", req.RawRequest.URI().String())
	})

	t.Run("query params from client should be set", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetParam("foo", "bar")
		req := AcquireRequest().SetURL("http://example.com/api/v1")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("foo=bar"), req.RawRequest.URI().QueryString())
	})

	t.Run("query params from request should be set", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetURL("http://example.com/api/v1").
			SetParam("bar", "foo")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("bar=foo"), req.RawRequest.URI().QueryString())
	})

	t.Run("query params should be merged", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetParam("bar", "foo1")
		req := AcquireRequest().
			SetURL("http://example.com/api/v1?bar=foo2").
			SetParam("bar", "foo")

		err := parserRequestURL(client, req)
		require.NoError(t, err)

		values, err := url.ParseQuery(string(req.RawRequest.URI().QueryString()))
		require.NoError(t, err)

		flag1, flag2, flag3 := false, false, false
		for _, v := range values["bar"] {
			switch v {
			case "foo1":
				flag1 = true
			case "foo2":
				flag2 = true
			case "foo":
				flag3 = true
			default:
				t.Fatalf("unexpected query param value: %s", v)
			}
		}
		require.True(t, flag1)
		require.True(t, flag2)
		require.True(t, flag3)
	})

	t.Run("request disable path normalizing should be respected", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetURL("https://example.my.host/other.host%2Fpath%2Fto%2Fdata%23123").
			SetDisablePathNormalizing(true)

		t.Cleanup(func() {
			ReleaseRequest(req)
		})

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "https://example.my.host/other.host%2Fpath%2Fto%2Fdata%23123", req.RawRequest.URI().String())
	})

	t.Run("client disable path normalizing should be respected", func(t *testing.T) {
		t.Parallel()
		client := New().SetDisablePathNormalizing(true)
		req := AcquireRequest().
			SetURL("https://example.my.host/other.host%2Fpath%2Fto%2Fdata%23123")

		t.Cleanup(func() {
			ReleaseRequest(req)
		})

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "https://example.my.host/other.host%2Fpath%2Fto%2Fdata%23123", req.RawRequest.URI().String())
	})
}

func Test_Parser_Request_Header(t *testing.T) {
	t.Parallel()

	t.Run("client header should be set", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetHeaders(map[string]string{
				fiber.HeaderContentType: "application/json",
			})

		req := AcquireRequest()

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("application/json"), req.RawRequest.Header.ContentType())
	})

	t.Run("request header should be set", func(t *testing.T) {
		t.Parallel()
		client := New()

		req := AcquireRequest().
			SetHeaders(map[string]string{
				fiber.HeaderContentType: "application/json, utf-8",
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("application/json, utf-8"), req.RawRequest.Header.ContentType())
	})

	t.Run("request header should override client header", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetHeader(fiber.HeaderContentType, "application/xml")

		req := AcquireRequest().
			SetHeader(fiber.HeaderContentType, "application/json, utf-8")

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("application/json, utf-8"), req.RawRequest.Header.ContentType())
	})

	t.Run("auto set json header", func(t *testing.T) {
		t.Parallel()
		type jsonData struct {
			Name string `json:"name"`
		}
		client := New()
		req := AcquireRequest().
			SetJSON(jsonData{
				Name: "foo",
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte(applicationJSON), req.RawRequest.Header.ContentType()) //nolint:testifylint // test
	})

	t.Run("auto set xml header", func(t *testing.T) {
		t.Parallel()
		type xmlData struct {
			XMLName xml.Name `xml:"body"`
			Name    string   `xml:"name"`
		}
		client := New()
		req := AcquireRequest().
			SetXML(xmlData{
				Name: "foo",
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte(applicationXML), req.RawRequest.Header.ContentType())
	})

	t.Run("auto set form data header", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetFormDataWithMap(map[string]string{
				"foo":  "bar",
				"ball": "circle and square",
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, applicationForm, string(req.RawRequest.Header.ContentType()))
	})

	t.Run("auto set file header", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			AddFileWithReader("hello", io.NopCloser(strings.NewReader("world"))).
			SetFormData("foo", "bar")

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Contains(t, string(req.RawRequest.Header.MultipartFormBoundary()), "FiberFormBoundary")
		require.Contains(t, string(req.RawRequest.Header.ContentType()), multipartFormData)
	})

	t.Run("ua should have default value", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest()

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("fiber"), req.RawRequest.Header.UserAgent())
	})

	t.Run("ua in client should be set", func(t *testing.T) {
		t.Parallel()
		client := New().SetUserAgent("foo")
		req := AcquireRequest()

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("foo"), req.RawRequest.Header.UserAgent())
	})

	t.Run("ua in request should have higher level", func(t *testing.T) {
		t.Parallel()
		client := New().SetUserAgent("foo")
		req := AcquireRequest().SetUserAgent("bar")

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("bar"), req.RawRequest.Header.UserAgent())
	})

	t.Run("referer in client should be set", func(t *testing.T) {
		t.Parallel()
		client := New().SetReferer("https://example.com")
		req := AcquireRequest()

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("https://example.com"), req.RawRequest.Header.Referer())
	})

	t.Run("referer in request should have higher level", func(t *testing.T) {
		t.Parallel()
		client := New().SetReferer("http://example.com")
		req := AcquireRequest().SetReferer("https://example.com")

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("https://example.com"), req.RawRequest.Header.Referer())
	})

	t.Run("client cookie should be set", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetCookie("foo", "bar").
			SetCookies(map[string]string{
				"bar":  "foo",
				"bar1": "foo1",
			}).
			DelCookies("bar1")

		req := AcquireRequest()

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, "bar", string(req.RawRequest.Header.Cookie("foo")))
		require.Equal(t, "foo", string(req.RawRequest.Header.Cookie("bar")))
		require.Empty(t, string(req.RawRequest.Header.Cookie("bar1")))
	})

	t.Run("request cookie should be set", func(t *testing.T) {
		t.Parallel()
		type cookies struct {
			Foo string `cookie:"foo"`
			Bar int    `cookie:"bar"`
		}

		client := New()

		req := AcquireRequest().
			SetCookiesWithStruct(&cookies{
				Foo: "bar",
				Bar: 67,
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, "bar", string(req.RawRequest.Header.Cookie("foo")))
		require.Equal(t, "67", string(req.RawRequest.Header.Cookie("bar")))
		require.Empty(t, string(req.RawRequest.Header.Cookie("bar1")))
	})

	t.Run("request cookie will override client cookie", func(t *testing.T) {
		t.Parallel()
		type cookies struct {
			Foo string `cookie:"foo"`
			Bar int    `cookie:"bar"`
		}

		client := New().
			SetCookie("foo", "bar").
			SetCookies(map[string]string{
				"bar":  "foo",
				"bar1": "foo1",
			})

		req := AcquireRequest().
			SetCookiesWithStruct(&cookies{
				Foo: "bar",
				Bar: 67,
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, "bar", string(req.RawRequest.Header.Cookie("foo")))
		require.Equal(t, "67", string(req.RawRequest.Header.Cookie("bar")))
		require.Equal(t, "foo1", string(req.RawRequest.Header.Cookie("bar1")))
	})
}

func Test_Parser_Request_Body(t *testing.T) {
	t.Parallel()

	t.Run("json body", func(t *testing.T) {
		t.Parallel()
		type jsonData struct {
			Name string `json:"name"`
		}
		client := New()
		req := AcquireRequest().
			SetJSON(jsonData{
				Name: "foo",
			})

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("{\"name\":\"foo\"}"), req.RawRequest.Body()) //nolint:testifylint // test
	})

	t.Run("xml body", func(t *testing.T) {
		t.Parallel()
		type xmlData struct {
			XMLName xml.Name `xml:"body"`
			Name    string   `xml:"name"`
		}
		client := New()
		req := AcquireRequest().
			SetXML(xmlData{
				Name: "foo",
			})

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("<body><name>foo</name></body>"), req.RawRequest.Body())
	})

	t.Run("CBOR body", func(t *testing.T) {
		t.Parallel()
		type cborData struct {
			Name string `cbor:"name"`
			Age  int    `cbor:"age"`
		}

		data := cborData{
			Name: "foo",
			Age:  12,
		}

		client := New()
		req := AcquireRequest().
			SetCBOR(data)

		err := parserRequestBody(client, req)
		require.NoError(t, err)

		encoded, err := cbor.Marshal(data)
		require.NoError(t, err)
		require.Equal(t, encoded, req.RawRequest.Body())
	})

	t.Run("form data body", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetFormDataWithMap(map[string]string{
				"ball": "circle and square",
			})

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Equal(t, "ball=circle+and+square", string(req.RawRequest.Body()))
	})

	t.Run("form data body error", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetFormDataWithMap(map[string]string{
				"": "",
			})

		err := parserRequestBody(client, req)
		require.NoError(t, err)
	})

	t.Run("file body", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			AddFileWithReader("hello", io.NopCloser(strings.NewReader("world")))

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Contains(t, string(req.RawRequest.Body()), "--FiberFormBoundary")
		require.Contains(t, string(req.RawRequest.Body()), "world")
	})

	t.Run("file body open error", func(t *testing.T) {
		t.Parallel()
		client := New()
		missingPath := filepath.Join(t.TempDir(), "missing.txt")

		req := AcquireRequest().AddFile(missingPath)

		err := parserRequestBody(client, req)
		require.ErrorContains(t, err, "open file error")
	})

	t.Run("file body missing path and name", func(t *testing.T) {
		t.Parallel()
		client := New()
		file := AcquireFile(SetFileReader(io.NopCloser(strings.NewReader("world"))))

		req := AcquireRequest().AddFiles(file)

		err := parserRequestBody(client, req)
		require.ErrorIs(t, err, ErrFileNoName)
	})

	t.Run("file and form data", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			AddFileWithReader("hello", io.NopCloser(strings.NewReader("world"))).
			SetFormData("foo", "bar")

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Contains(t, string(req.RawRequest.Body()), "--FiberFormBoundary")
		require.Contains(t, string(req.RawRequest.Body()), "world")
		require.Contains(t, string(req.RawRequest.Body()), "bar")
	})

	t.Run("raw body", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetRawBody([]byte("hello world"))

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("hello world"), req.RawRequest.Body())
	})

	t.Run("raw body error", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().
			SetRawBody([]byte("hello world"))

		req.body = nil

		err := parserRequestBody(client, req)
		require.ErrorIs(t, err, ErrBodyType)
	})

	t.Run("unsupported body type", func(t *testing.T) {
		t.Parallel()

		client := New()
		req := AcquireRequest()

		req.bodyType = 999 // some invalid type

		err := parserRequestBody(client, req)
		require.ErrorIs(t, err, ErrBodyTypeNotSupported)
	})
}

type dummyLogger struct {
	buf *bytes.Buffer
}

func (*dummyLogger) Trace(_ ...any) {}

func (*dummyLogger) Debug(_ ...any) {}

func (*dummyLogger) Info(_ ...any) {}

func (*dummyLogger) Warn(_ ...any) {}

func (*dummyLogger) Error(_ ...any) {}

func (*dummyLogger) Fatal(_ ...any) {}

func (*dummyLogger) Panic(_ ...any) {}

func (*dummyLogger) Tracef(_ string, _ ...any) {}

func (l *dummyLogger) Debugf(format string, v ...any) {
	fmt.Fprintf(l.buf, format, v...)
}

func (*dummyLogger) Infof(_ string, _ ...any) {}

func (*dummyLogger) Warnf(_ string, _ ...any) {}

func (*dummyLogger) Errorf(_ string, _ ...any) {}

func (*dummyLogger) Fatalf(_ string, _ ...any) {}

func (*dummyLogger) Panicf(_ string, _ ...any) {}

func (*dummyLogger) Tracew(_ string, _ ...any) {}

func (*dummyLogger) Debugw(_ string, _ ...any) {}

func (*dummyLogger) Infow(_ string, _ ...any) {}

func (*dummyLogger) Warnw(_ string, _ ...any) {}

func (*dummyLogger) Errorw(_ string, _ ...any) {}

func (*dummyLogger) Fatalw(_ string, _ ...any) {}

func (*dummyLogger) Panicw(_ string, _ ...any) {}

func Test_Client_Logger_Debug(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("response")
	})

	addrChan := make(chan string)
	go func() {
		assert.NoError(t, app.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				addrChan <- addr.String()
			},
		}))
	}()

	defer func(app *fiber.App) {
		require.NoError(t, app.Shutdown())
	}(app)

	var buf bytes.Buffer
	logger := &dummyLogger{buf: &buf}

	client := New()
	client.Debug().SetLogger(logger)

	addr := <-addrChan
	resp, err := client.Get("http://" + addr)
	require.NoError(t, err)
	defer resp.Close()

	require.NoError(t, err)
	require.Contains(t, buf.String(), "Host: "+addr)
	require.Contains(t, buf.String(), "Content-Length: 8")
}

func Test_Client_Logger_DisableDebug(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("response")
	})

	addrChan := make(chan string)
	go func() {
		assert.NoError(t, app.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				addrChan <- addr.String()
			},
		}))
	}()

	defer func(app *fiber.App) {
		require.NoError(t, app.Shutdown())
	}(app)

	var buf bytes.Buffer
	logger := &dummyLogger{buf: &buf}

	client := New()
	client.DisableDebug().SetLogger(logger)

	addr := <-addrChan
	resp, err := client.Get("http://" + addr)
	require.NoError(t, err)
	defer resp.Close()

	require.NoError(t, err)
	require.Empty(t, buf.String())
}

func Benchmark_Parser_Request_Body_File(b *testing.B) {
	b.Helper()

	const (
		fileCount = 3
		fileSize  = 32 << 10 // 32KB payload per file
	)

	formValues := map[string]string{
		"username": "fiber",
		"api_key":  "d5942ef5",
	}

	fileContents := make([][]byte, fileCount)
	for i := range fileContents {
		fileContents[i] = bytes.Repeat([]byte{byte('a' + i)}, fileSize)
	}

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var totalBytes int64
		for _, c := range fileContents {
			totalBytes += int64(len(c))
		}
		b.SetBytes(totalBytes)
		req := newBenchmarkRequest(formValues, fileContents)
		if err := parserRequestBodyFile(req); err != nil {
			b.Fatalf("parserRequestBodyFile: %v", err)
		}
		releaseBenchmarkRequest(req)
	}
}

func newBenchmarkRequest(formValues map[string]string, fileContents [][]byte) *Request {
	req := &Request{
		boundary:   "FiberBenchmarkBoundary",
		formData:   FormData{Args: fasthttp.AcquireArgs()},
		RawRequest: fasthttp.AcquireRequest(),
		files:      make([]*File, len(fileContents)),
	}

	req.RawRequest.Header.SetContentType("multipart/form-data; boundary=" + req.boundary)

	for key, value := range formValues {
		req.formData.Set(key, value)
	}

	for i, content := range fileContents {
		req.files[i] = AcquireFile(
			SetFileName(fmt.Sprintf("file-%d.bin", i)),
			SetFileFieldName(fmt.Sprintf("file%d", i)),
			SetFileReader(io.NopCloser(bytes.NewReader(content))),
		)
	}

	return req
}

func releaseBenchmarkRequest(req *Request) {
	fasthttp.ReleaseRequest(req.RawRequest)
	fasthttp.ReleaseArgs(req.formData.Args)
	for _, f := range req.files {
		ReleaseFile(f)
	}
}

func Benchmark_Parser_Request_URL_PathParams(b *testing.B) {
	client := New().SetBaseURL("http://example.com/api/:version")
	client.SetPathParam("version", "v1")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req := AcquireRequest().
			SetURL("/users/:id/posts/:postID").
			SetPathParams(map[string]string{
				"id":     "12345",
				"postID": "67890",
			})
		if err := parserRequestURL(client, req); err != nil {
			b.Fatal(err)
		}
		ReleaseRequest(req)
	}
}

func Test_Client_Logger_StreamedBodiesLogHeadersOnly(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	client := New()
	client.Debug().SetLogger(&dummyLogger{buf: &buf})

	req := AcquireRequest()
	t.Cleanup(func() { ReleaseRequest(req) })
	req.RawRequest.SetRequestURI("http://example.com/")
	req.RawRequest.Header.SetMethod(fiber.MethodPost)
	req.RawRequest.SetBodyStream(bytes.NewBufferString("request body"), len("request body"))

	resp := AcquireResponse()
	t.Cleanup(func() { ReleaseResponse(resp) })
	resp.RawResponse.SetBodyStream(bytes.NewBufferString("response body"), len("response body"))

	require.NoError(t, logger(client, resp, req))

	out := buf.String()
	require.Contains(t, out, "POST http://example.com/ HTTP/1.1")
	require.Contains(t, out, "HTTP/1.1 200 OK")
	require.NotContains(t, out, "request body", "a streamed body is consumed by its first reader")
	require.NotContains(t, out, "response body")
	require.True(t, req.RawRequest.IsBodyStream())
	require.True(t, resp.RawResponse.IsBodyStream())
}

func Test_Client_ResponseCookie_UnparsableAttribute(t *testing.T) {
	t.Parallel()

	client := New()

	req := AcquireRequest()
	t.Cleanup(func() { ReleaseRequest(req) })
	req.RawRequest.SetRequestURI("http://example.com/")

	resp := AcquireResponse()
	t.Cleanup(func() { ReleaseResponse(resp) })
	resp.RawResponse.Header.Add("Set-Cookie", "sid=abc; Expires=notadate; Path=/")

	require.NoError(t, parserResponseCookie(client, resp, req))
	require.Len(t, resp.cookie, 1)
	require.Equal(t, "sid", string(resp.cookie[0].Key()))
	require.Equal(t, "abc", string(resp.cookie[0].Value()))
}
