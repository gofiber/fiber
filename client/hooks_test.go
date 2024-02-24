package client

import (
	"bytes"
	"encoding/xml"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/log"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := randString(tt.args)
			require.Len(t, got, 16)
		})
	}
}

func Test_Parser_Request_URL(t *testing.T) {
	t.Parallel()

	t.Run("client baseurl should be set", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api", req.RawRequest.URI().String())
	})

	t.Run("request url should be set", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().SetURL("http://example.com/api")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api", req.RawRequest.URI().String())
	})

	t.Run("the request url will override baseurl with protocol", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("http://example.com/api/v1")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/v1", req.RawRequest.URI().String())
	})

	t.Run("the request url should be append after baseurl without protocol", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("/v1")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/v1", req.RawRequest.URI().String())
	})

	t.Run("the url is error", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().SetBaseURL("example.com/api")
		req := AcquireRequest().SetURL("/v1")

		err := parserRequestURL(client, req)
		require.Equal(t, ErrURLFormat, err)
	})

	t.Run("the path param from client", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().
			SetBaseURL("http://example.com/api/:id").
			SetPathParam("id", "5")
		req := AcquireRequest()

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, "http://example.com/api/5", req.RawRequest.URI().String())
	})

	t.Run("the path param from request", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().
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
		client := AcquireClient().
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

	t.Run("query params from client should be set", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().
			SetParam("foo", "bar")
		req := AcquireRequest().SetURL("http://example.com/api/v1")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("foo=bar"), req.RawRequest.URI().QueryString())
	})

	t.Run("query params from request should be set", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().
			SetURL("http://example.com/api/v1").
			SetParam("bar", "foo")

		err := parserRequestURL(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("bar=foo"), req.RawRequest.URI().QueryString())
	})

	t.Run("query params should be merged", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().
			SetParam("bar", "foo1")
		req := AcquireRequest().
			SetURL("http://example.com/api/v1?bar=foo2").
			SetParam("bar", "foo")

		err := parserRequestURL(client, req)
		require.NoError(t, err)

		values, _ := url.ParseQuery(string(req.RawRequest.URI().QueryString()))

		flag1, flag2, flag3 := false, false, false
		for _, v := range values["bar"] {
			if v == "foo1" {
				flag1 = true
			} else if v == "foo2" {
				flag2 = true
			} else if v == "foo" {
				flag3 = true
			}
		}
		require.True(t, flag1)
		require.True(t, flag2)
		require.True(t, flag3)
	})
}

func Test_Parser_Request_Header(t *testing.T) {
	t.Parallel()

	t.Run("client header should be set", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().
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
		client := AcquireClient()

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
		client := AcquireClient().
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
		client := AcquireClient()
		req := AcquireRequest().
			SetJSON(jsonData{
				Name: "foo",
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte(applicationJSON), req.RawRequest.Header.ContentType())
	})

	t.Run("auto set xml header", func(t *testing.T) {
		t.Parallel()
		type xmlData struct {
			XMLName xml.Name `xml:"body"`
			Name    string   `xml:"name"`
		}
		client := AcquireClient()
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
		client := AcquireClient()
		req := AcquireRequest().
			SetFormDatas(map[string]string{
				"foo":  "bar",
				"ball": "cricle and square",
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, applicationForm, string(req.RawRequest.Header.ContentType()))
	})

	t.Run("auto set file header", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().
			AddFileWithReader("hello", io.NopCloser(strings.NewReader("world"))).
			SetFormData("foo", "bar")

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.True(t, strings.Contains(string(req.RawRequest.Header.MultipartFormBoundary()), "--FiberFormBoundary"))
		require.True(t, strings.Contains(string(req.RawRequest.Header.ContentType()), multipartFormData))
	})

	t.Run("ua should have default value", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest()

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("fiber"), req.RawRequest.Header.UserAgent())
	})

	t.Run("ua in client should be set", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().SetUserAgent("foo")
		req := AcquireRequest()

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("foo"), req.RawRequest.Header.UserAgent())
	})

	t.Run("ua in request should have higher level", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().SetUserAgent("foo")
		req := AcquireRequest().SetUserAgent("bar")

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("bar"), req.RawRequest.Header.UserAgent())
	})

	t.Run("referer in client should be set", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().SetReferer("https://example.com")
		req := AcquireRequest()

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("https://example.com"), req.RawRequest.Header.Referer())
	})

	t.Run("referer in request should have higher level", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().SetReferer("http://example.com")
		req := AcquireRequest().SetReferer("https://example.com")

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("https://example.com"), req.RawRequest.Header.Referer())
	})

	t.Run("client cookie should be set", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient().
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
		require.Equal(t, "", string(req.RawRequest.Header.Cookie("bar1")))
	})

	t.Run("request cookie should be set", func(t *testing.T) {
		t.Parallel()
		type cookies struct {
			Foo string `cookie:"foo"`
			Bar int    `cookie:"bar"`
		}

		client := AcquireClient()

		req := AcquireRequest().
			SetCookiesWithStruct(&cookies{
				Foo: "bar",
				Bar: 67,
			})

		err := parserRequestHeader(client, req)
		require.NoError(t, err)
		require.Equal(t, "bar", string(req.RawRequest.Header.Cookie("foo")))
		require.Equal(t, "67", string(req.RawRequest.Header.Cookie("bar")))
		require.Equal(t, "", string(req.RawRequest.Header.Cookie("bar1")))
	})

	t.Run("request cookie will override client cookie", func(t *testing.T) {
		t.Parallel()
		type cookies struct {
			Foo string `cookie:"foo"`
			Bar int    `cookie:"bar"`
		}

		client := AcquireClient().
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
		client := AcquireClient()
		req := AcquireRequest().
			SetJSON(jsonData{
				Name: "foo",
			})

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("{\"name\":\"foo\"}"), req.RawRequest.Body())
	})

	t.Run("xml body", func(t *testing.T) {
		t.Parallel()
		type xmlData struct {
			XMLName xml.Name `xml:"body"`
			Name    string   `xml:"name"`
		}
		client := AcquireClient()
		req := AcquireRequest().
			SetXML(xmlData{
				Name: "foo",
			})

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("<body><name>foo</name></body>"), req.RawRequest.Body())
	})

	t.Run("form data body", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().
			SetFormDatas(map[string]string{
				"ball": "cricle and square",
			})

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Equal(t, "ball=cricle+and+square", string(req.RawRequest.Body()))
	})

	t.Run("form data body error", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().
			SetFormDatas(map[string]string{
				"": "",
			})

		err := parserRequestBody(client, req)
		require.NoError(t, err)
	})

	t.Run("file body", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().
			AddFileWithReader("hello", io.NopCloser(strings.NewReader("world")))

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.True(t, strings.Contains(string(req.RawRequest.Body()), "----FiberFormBoundary"))
		require.True(t, strings.Contains(string(req.RawRequest.Body()), "world"))
	})

	t.Run("file and form data", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().
			AddFileWithReader("hello", io.NopCloser(strings.NewReader("world"))).
			SetFormData("foo", "bar")

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.True(t, strings.Contains(string(req.RawRequest.Body()), "----FiberFormBoundary"))
		require.True(t, strings.Contains(string(req.RawRequest.Body()), "world"))
		require.True(t, strings.Contains(string(req.RawRequest.Body()), "bar"))
	})

	t.Run("raw body", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().
			SetRawBody([]byte("hello world"))

		err := parserRequestBody(client, req)
		require.NoError(t, err)
		require.Equal(t, []byte("hello world"), req.RawRequest.Body())
	})

	t.Run("raw body error", func(t *testing.T) {
		t.Parallel()
		client := AcquireClient()
		req := AcquireRequest().
			SetRawBody([]byte("hello world"))

		req.body = nil

		err := parserRequestBody(client, req)
		require.ErrorIs(t, err, ErrBodyType)
	})
}

func Test_Client_Logger_Debug(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("response")
	})

	var url string

	go func() {
		require.NoError(t, app.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				url = addr.String()
			},
		}))
	}()

	defer func(app *fiber.App) {
		_ = app.Shutdown()
	}(app)

	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	log.SetOutput(&buf)

	client := AcquireClient()
	client.Debug()

	resp, err := client.Get("http://" + url)
	defer resp.Close()

	require.NoError(t, err)
	require.Contains(t, buf.String(), "Host: "+url)
	require.Contains(t, buf.String(), "Content-Length: 8")
}

func Test_Client_Logger_DisableDebug(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("response")
	})

	var url string

	go func() {
		require.NoError(t, app.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				url = addr.String()
			},
		}))
	}()

	defer func(app *fiber.App) {
		_ = app.Shutdown()
	}(app)

	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	log.SetOutput(&buf)

	client := AcquireClient()
	client.DisableDebug()

	resp, err := client.Get("http://" + url)
	defer resp.Close()

	require.NoError(t, err)
	require.Empty(t, buf.String())
}
