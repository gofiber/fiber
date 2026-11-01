package client

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/paramdelim"
	"github.com/gofiber/fiber/v3/log"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Rand_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args int
	}{
		{name: "test generate", args: 16},
		{name: "test generate smaller string", args: 8},
		{name: "test generate larger string", args: 32},
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

// Test_PathParamEndChars_MatchesSharedSet pins the client's terminator set to
// the shared route grammar plus '#', which ends the path only client-side, so a
// router delimiter cannot be added without the client following (#4635).
func Test_PathParamEndChars_MatchesSharedSet(t *testing.T) {
	t.Parallel()
	want := paramdelim.PathEndChars()
	want['#'] = true

	require.Equal(t, want, pathParamEndChars)
}

// clientTestCJSONBody and clientTestCXMLBody are the payloads the parser tests
// send as JSON and as XML.
type clientTestCJSONBody struct {
	Name string `json:"name"`
}

type clientTestCXMLBody struct {
	XMLName xml.Name `xml:"body"`
	Name    string   `xml:"name"`
}

func Test_Parser_Request_URL(t *testing.T) {
	t.Parallel()
	// Every case builds its own client and request, so the cases run in parallel.
	for _, tc := range []struct {
		client  *Client
		req     *Request
		wantErr error // the error the request must fail with; nil for none
		name    string
		want    string
	}{
		{
			name:   "client baseurl should be set",
			client: New().SetBaseURL("http://example.com/api"),
			req:    AcquireRequest().SetURL(""),
			want:   "http://example.com/api",
		},
		{
			name:   "request url should be set",
			client: New(),
			req:    AcquireRequest().SetURL("http://example.com/api"),
			want:   "http://example.com/api",
		},
		{
			name:   "the request url will override baseurl with protocol",
			client: New().SetBaseURL("http://example.com/api"),
			req:    AcquireRequest().SetURL("http://example.com/api/v1"),
			want:   "http://example.com/api/v1",
		},
		{
			name:   "the request url should be append after baseurl without protocol",
			client: New().SetBaseURL("http://example.com/api"),
			req:    AcquireRequest().SetURL("/v1"),
			want:   "http://example.com/api/v1",
		},
		{
			name:   "the path param from client",
			client: New().SetBaseURL("http://example.com/api/:id").SetPathParam("id", "5"),
			req:    AcquireRequest(),
			want:   "http://example.com/api/5",
		},
		{
			name:   "the path param from request",
			client: New().SetBaseURL("http://example.com/api/:id/:name").SetPathParam("id", "5"),
			req: AcquireRequest().SetURL("/{key}").SetPathParams(map[string]string{"name": "fiber", "key": "val"}).
				DelPathParams("key"),
			want: "http://example.com/api/5/fiber/%7Bkey%7D",
		},
		{
			name:   "the path param from request and client",
			client: New().SetBaseURL("http://example.com/api/:id/:name").SetPathParam("id", "5"),
			req: AcquireRequest().SetURL("/:key").
				SetPathParams(map[string]string{"name": "fiber", "key": "val", "id": "12"}),
			want: "http://example.com/api/12/fiber/val",
		},
		{
			name:   "path param does not match a longer placeholder",
			client: New().SetBaseURL("http://example.com/api/:idx").SetPathParam("id", "5"),
			req:    AcquireRequest(),
			want:   "http://example.com/api/:idx",
		},
		{
			name:   "path params sharing a prefix are resolved independently",
			client: New().SetBaseURL("http://example.com/api/:idx/:id"),
			req:    AcquireRequest().SetPathParams(map[string]string{"id": "5", "idx": "9"}),
			want:   "http://example.com/api/9/5",
		},
		{
			name:   "path param value cannot open a query",
			client: New(),
			req:    AcquireRequest().SetURL("http://example.com/api/:id").SetPathParam("id", "a?b"),
			want:   "http://example.com/api/a%3Fb",
		},
		{
			name:   "path param value keeps its escaped separator when the client does not normalize",
			client: New().SetDisablePathNormalizing(true),
			req:    AcquireRequest().SetURL("http://example.com/api/:id").SetPathParam("id", "a/b"),
			want:   "http://example.com/api/a%2Fb",
		},
		{
			name:   "a request can ask for its escaped separator from a client that does not normalize",
			client: NewWithClient(&fasthttp.Client{DisablePathNormalizing: true}),
			req: AcquireRequest().SetURL("http://example.com/api/:id").SetPathParam("id", "a/b").
				SetDisablePathNormalizing(true),
			want: "http://example.com/api/a%2Fb",
		},
		{
			name:   "a substituted value is not substituted again",
			client: New(),
			req: AcquireRequest().SetURL("http://example.com/api/:id").
				SetPathParams(map[string]string{"id": ":name", "name": "fiber"}),
			want: "http://example.com/api/:name",
		},
		{
			name:   "request disable path normalizing should be respected",
			client: New(),
			req: AcquireRequest().SetURL("https://example.my.host/other.host%2Fpath%2Fto%2Fdata%23123").
				SetDisablePathNormalizing(true),
			want: "https://example.my.host/other.host%2Fpath%2Fto%2Fdata%23123",
		},
		{
			name:   "client disable path normalizing should be respected",
			client: New().SetDisablePathNormalizing(true),
			req:    AcquireRequest().SetURL("https://example.my.host/other.host%2Fpath%2Fto%2Fdata%23123"),
			want:   "https://example.my.host/other.host%2Fpath%2Fto%2Fdata%23123",
		},
		{
			// The wrapped client rewrites the path whatever the request says, so the
			// request alone cannot make a separator in a value safe.
			name:   "a separator in a value is rejected when only the request disables normalizing",
			client: New(),
			req: AcquireRequest().SetURL("http://example.com/api/:id").SetPathParam("id", "a/b").
				SetDisablePathNormalizing(true),
			wantErr: ErrPathParamInPath,
		},
		{
			name:    "a separator in a value is rejected when only the fasthttp client does not normalize",
			client:  NewWithClient(&fasthttp.Client{DisablePathNormalizing: true}),
			req:     AcquireRequest().SetURL("http://example.com/api/:id").SetPathParam("id", "a/b"),
			wantErr: ErrPathParamInPath,
		},
		{
			// The value would be escaped to "a%2Fb", but fasthttp percent-decodes
			// the path while normalizing it, so an encoded "/" is indistinguishable
			// from a real separator on the wire. Escaping still contains "?" and
			// "#", which normalizing does not decode.
			name:    "a separator in a value is rejected when normalizing will decode it",
			client:  New(),
			req:     AcquireRequest().SetURL("http://example.com/api/:id").SetPathParam("id", "a/b"),
			wantErr: ErrPathParamInPath,
		},
		{
			// "@" ends a userinfo section, so this used to resolve to the host
			// "evil.com" and the request left for another server.
			name:    "path param value cannot rewrite the host",
			client:  New(),
			req:     AcquireRequest().SetURL("http://:host/api").SetPathParam("host", "x@evil.com"),
			wantErr: ErrPathParamInHost,
		},
		{
			name:   "query params from client should be set",
			client: New().SetParam("foo", "bar"),
			req:    AcquireRequest().SetURL("http://example.com/api/v1"),
			want:   "http://example.com/api/v1?foo=bar",
		},
		{
			name:   "query params from request should be set",
			client: New(),
			req:    AcquireRequest().SetURL("http://example.com/api/v1").SetParam("bar", "foo"),
			want:   "http://example.com/api/v1?bar=foo",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t.Cleanup(func() { ReleaseRequest(tc.req) })

			err := parserRequestURL(tc.client, tc.req)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, tc.req.RawRequest.URI().String())
		})
	}

	t.Run("the url is error", func(t *testing.T) {
		t.Parallel()
		client := New().SetBaseURL("example.com/api")
		req := AcquireRequest().SetURL("/v1")

		require.Equal(t, ErrURLFormat, parserRequestURL(client, req))
	})

	t.Run("a host path param keeps the rest of the URL intact", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().SetURL("http://:host/api/v1?a=b").SetPathParam("host", "\u00fcn\u00efcode.example.com")

		require.NoError(t, parserRequestURL(client, req))
		require.Equal(t, "\u00fcn\u00efcode.example.com", string(req.RawRequest.URI().Host()))
		// A percent-escape in the host makes fasthttp abandon the rest of the
		// URI, so this pins that a host value never gets escaped.
		require.Equal(t, "/api/v1", string(req.RawRequest.URI().Path()))
		require.Equal(t, "a=b", string(req.RawRequest.URI().QueryString()))
	})

	t.Run("query params should be merged", func(t *testing.T) {
		t.Parallel()
		client := New().SetParam("bar", "foo1")
		req := AcquireRequest().SetURL("http://example.com/api/v1?bar=foo2").SetParam("bar", "foo")

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
}

func Test_Parser_Request_Header(t *testing.T) {
	t.Parallel()
	type cookies struct {
		Foo string `cookie:"foo"`
		Bar int    `cookie:"bar"`
	}

	// Every case builds its own client and request, so the cases run in parallel.
	for _, tc := range []struct {
		client    *Client
		req       *Request
		get       func(h *fasthttp.RequestHeader) []byte // reads the header under test, which must be want
		cookies   map[string]string                      // cookie name -> the value the request header has, "" for none
		name      string
		want      string
		noReferer bool // the request must carry no Referer header at all
	}{
		{
			name:   "client header should be set",
			client: New().SetHeaders(map[string]string{fiber.HeaderContentType: "application/json"}),
			req:    AcquireRequest(),
			get:    (*fasthttp.RequestHeader).ContentType,
			want:   "application/json",
		},
		{
			name:   "request header should be set",
			client: New(),
			req:    AcquireRequest().SetHeaders(map[string]string{fiber.HeaderContentType: "application/json, utf-8"}),
			get:    (*fasthttp.RequestHeader).ContentType,
			want:   "application/json, utf-8",
		},
		{
			name:   "request header should override client header",
			client: New().SetHeader(fiber.HeaderContentType, "application/xml"),
			req:    AcquireRequest().SetHeader(fiber.HeaderContentType, "application/json, utf-8"),
			get:    (*fasthttp.RequestHeader).ContentType,
			want:   "application/json, utf-8",
		},
		{
			name:   "auto set json header",
			client: New(),
			req:    AcquireRequest().SetJSON(clientTestCJSONBody{Name: "foo"}),
			get:    (*fasthttp.RequestHeader).ContentType,
			want:   applicationJSON,
		},
		{
			name:   "auto set xml header",
			client: New(),
			req:    AcquireRequest().SetXML(clientTestCXMLBody{Name: "foo"}),
			get:    (*fasthttp.RequestHeader).ContentType,
			want:   applicationXML,
		},
		{
			name:   "auto set form data header",
			client: New(),
			req:    AcquireRequest().SetFormDataWithMap(map[string]string{"foo": "bar", "ball": "circle and square"}),
			get:    (*fasthttp.RequestHeader).ContentType,
			want:   applicationForm,
		},
		{
			name:   "ua should have default value",
			client: New(),
			req:    AcquireRequest(),
			get:    (*fasthttp.RequestHeader).UserAgent,
			want:   "fiber",
		},
		{
			name:   "ua in client should be set",
			client: New().SetUserAgent("foo"),
			req:    AcquireRequest(),
			get:    (*fasthttp.RequestHeader).UserAgent,
			want:   "foo",
		},
		{
			name:   "ua in request should have higher level",
			client: New().SetUserAgent("foo"),
			req:    AcquireRequest().SetUserAgent("bar"),
			get:    (*fasthttp.RequestHeader).UserAgent,
			want:   "bar",
		},
		{
			name:   "referer in client should be set",
			client: New().SetReferer("https://example.com"),
			req:    AcquireRequest(),
			get:    (*fasthttp.RequestHeader).Referer,
			want:   "https://example.com",
		},
		{
			name:   "referer in request should have higher level",
			client: New().SetReferer("http://example.com"),
			req:    AcquireRequest().SetReferer("https://example.com"),
			get:    (*fasthttp.RequestHeader).Referer,
			want:   "https://example.com",
		},
		// An empty "Referer:" says nothing that a missing header does not, and it
		// went out on every request that never asked for one.
		{name: "no referer sends no Referer header", client: New(), req: AcquireRequest(), noReferer: true},
		{
			name:      "a Referer set through the headers is replaced, not kept empty",
			client:    New(),
			req:       AcquireRequest().SetHeader(fiber.HeaderReferer, "https://header.example"),
			noReferer: true,
		},
		{
			name: "client cookie should be set",
			client: New().SetCookie("foo", "bar").SetCookies(map[string]string{"bar": "foo", "bar1": "foo1"}).
				DelCookies("bar1"),
			req:     AcquireRequest(),
			cookies: map[string]string{"foo": "bar", "bar": "foo", "bar1": ""},
		},
		{
			name:    "request cookie should be set",
			client:  New(),
			req:     AcquireRequest().SetCookiesWithStruct(&cookies{Foo: "bar", Bar: 67}),
			cookies: map[string]string{"foo": "bar", "bar": "67", "bar1": ""},
		},
		{
			name:    "request cookie will override client cookie",
			client:  New().SetCookie("foo", "bar").SetCookies(map[string]string{"bar": "foo", "bar1": "foo1"}),
			req:     AcquireRequest().SetCookiesWithStruct(&cookies{Foo: "bar", Bar: 67}),
			cookies: map[string]string{"foo": "bar", "bar": "67", "bar1": "foo1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t.Cleanup(func() { ReleaseRequest(tc.req) })

			require.NoError(t, parserRequestHeader(tc.client, tc.req))
			if tc.get != nil {
				require.Equal(t, tc.want, string(tc.get(&tc.req.RawRequest.Header)))
			}
			if tc.noReferer {
				require.False(t, hasRefererHeader(&tc.req.RawRequest.Header))
			}
			for name, want := range tc.cookies {
				require.Equal(t, want, string(tc.req.RawRequest.Header.Cookie(name)), "cookie %q", name)
			}
		})
	}

	t.Run("auto set file header", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().AddFileWithReader("hello", io.NopCloser(strings.NewReader("world"))).
			SetFormData("foo", "bar")

		require.NoError(t, parserRequestHeader(client, req))
		require.Contains(t, string(req.RawRequest.Header.MultipartFormBoundary()), "FiberFormBoundary")
		require.Contains(t, string(req.RawRequest.Header.ContentType()), multipartFormData)
	})

	t.Run("a referer that is no longer set does not linger on a resend", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().SetReferer("https://example.com")

		require.NoError(t, parserRequestHeader(client, req))
		require.Equal(t, []byte("https://example.com"), req.RawRequest.Header.Referer())

		// Clearing the request value leaves the client's, which is unset.
		req.SetReferer("")
		require.NoError(t, parserRequestHeader(client, req))
		require.False(t, hasRefererHeader(&req.RawRequest.Header))
	})

	t.Run("a cookie that is no longer set does not linger on a resend", func(t *testing.T) {
		t.Parallel()
		client := New().SetCookie("foo", "bar")
		req := AcquireRequest().SetCookie("baz", "qux")

		require.NoError(t, parserRequestHeader(client, req))
		require.Equal(t, "bar", string(req.RawRequest.Header.Cookie("foo")))
		require.Equal(t, "qux", string(req.RawRequest.Header.Cookie("baz")))

		client.DelCookies("foo")
		req.DelCookies("baz")
		require.NoError(t, parserRequestHeader(client, req))
		require.Empty(t, req.RawRequest.Header.Cookie("foo"))
		require.Empty(t, req.RawRequest.Header.Cookie("baz"))
	})
}

func Test_Parser_Request_Body(t *testing.T) {
	t.Parallel()
	type cborData struct {
		Name string `cbor:"name"`
		Age  int    `cbor:"age"`
	}

	data := cborData{Name: "foo", Age: 12}
	encoded, err := cbor.Marshal(data)
	require.NoError(t, err)

	// Every case builds its own request, so the cases run in parallel.
	for _, tc := range []struct {
		req      *Request
		name     string
		want     string
		contains []string // the parts a multipart body, whose boundary varies, must contain instead of want
	}{
		{
			name: "json body",
			req:  AcquireRequest().SetJSON(clientTestCJSONBody{Name: "foo"}),
			want: "{\"name\":\"foo\"}",
		},
		{
			name: "xml body",
			req:  AcquireRequest().SetXML(clientTestCXMLBody{Name: "foo"}),
			want: "<body><name>foo</name></body>",
		},
		{name: "CBOR body", req: AcquireRequest().SetCBOR(data), want: string(encoded)},
		{
			name: "form data body",
			req:  AcquireRequest().SetFormDataWithMap(map[string]string{"ball": "circle and square"}),
			want: "ball=circle+and+square",
		},
		{name: "raw body", req: AcquireRequest().SetRawBody([]byte("hello world")), want: "hello world"},
		{
			name:     "file body",
			req:      AcquireRequest().AddFileWithReader("hello", io.NopCloser(strings.NewReader("world"))),
			contains: []string{"--FiberFormBoundary", "world"},
		},
		{
			name: "file and form data",
			req: AcquireRequest().AddFileWithReader("hello", io.NopCloser(strings.NewReader("world"))).
				SetFormData("foo", "bar"),
			contains: []string{"--FiberFormBoundary", "world", "bar"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t.Cleanup(func() { ReleaseRequest(tc.req) })

			require.NoError(t, parserRequestBody(New(), tc.req))
			if tc.contains == nil {
				require.Equal(t, tc.want, string(tc.req.RawRequest.Body()))
			}
			for _, part := range tc.contains {
				require.Contains(t, string(tc.req.RawRequest.Body()), part)
			}
		})
	}

	t.Run("form data body error", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().SetFormDataWithMap(map[string]string{"": ""})

		require.NoError(t, parserRequestBody(client, req))
	})

	t.Run("file body open error", func(t *testing.T) {
		t.Parallel()
		client := New()
		missingPath := filepath.Join(t.TempDir(), "missing.txt")

		req := AcquireRequest().AddFile(missingPath)

		require.ErrorContains(t, parserRequestBody(client, req), "open file error")
	})

	t.Run("file body missing path and name", func(t *testing.T) {
		t.Parallel()
		client := New()
		file := AcquireFile(SetFileReader(io.NopCloser(strings.NewReader("world"))))

		req := AcquireRequest().AddFiles(file)

		require.ErrorIs(t, parserRequestBody(client, req), ErrFileNoName)
	})

	t.Run("raw body error", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest().SetRawBody([]byte("hello world"))

		req.body = nil

		require.ErrorIs(t, parserRequestBody(client, req), ErrBodyType)
	})

	t.Run("unsupported body type", func(t *testing.T) {
		t.Parallel()
		client := New()
		req := AcquireRequest()

		req.bodyType = 999 // some invalid type

		require.ErrorIs(t, parserRequestBody(client, req), ErrBodyTypeNotSupported)
	})
}

// dummyLogger records what the client logs through Debugf, the only method its
// debug logging calls. Every other method is the nil embedded logger's and
// panics, so a call the tests do not expect cannot pass unnoticed.
type dummyLogger struct {
	log.CommonLogger
	buf *bytes.Buffer
}

func (l *dummyLogger) Debugf(format string, v ...any) {
	fmt.Fprintf(l.buf, format, v...)
}

// clientTestCLoggedGet sends one GET to a loopback server through a client that
// logs into the returned buffer once debug switched debug logging on or off. It
// also returns the address of the server.
func clientTestCLoggedGet(t *testing.T, debug func(*Client) *Client) (logs *bytes.Buffer, addr string) { //nolint:nonamedreturns // gocritic unnamedResult requires explicit result names for clarity when returning the log and the address
	t.Helper()

	app, addr := startTestServerWithPort(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error { return c.SendString("response") })
	})
	t.Cleanup(func() { require.NoError(t, app.Shutdown()) })

	logs = &bytes.Buffer{}
	client := New()
	debug(client).SetLogger(&dummyLogger{buf: logs})

	resp, err := client.Get("http://" + addr)
	require.NoError(t, err)
	resp.Close()

	return logs, addr
}

func Test_Client_Logger_Debug(t *testing.T) {
	t.Parallel()
	buf, addr := clientTestCLoggedGet(t, (*Client).Debug)

	require.Contains(t, buf.String(), "Host: "+addr)
	require.Contains(t, buf.String(), "Content-Length: 8")
}

func Test_Client_Logger_DisableDebug(t *testing.T) {
	t.Parallel()
	buf, _ := clientTestCLoggedGet(t, (*Client).DisableDebug)

	require.Empty(t, buf.String())
}

func Benchmark_Parser_Request_Body_File(b *testing.B) {
	b.Helper()

	const (
		fileCount = 3
		fileSize  = 32 << 10 // 32KB payload per file
	)

	formValues := map[string]string{"username": "fiber", "api_key": "d5942ef5"}

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
		req := AcquireRequest().SetURL("/users/:id/posts/:postID").
			SetPathParams(map[string]string{"id": "12345", "postID": "67890"})
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

	req := clientTestBRequest(t)
	req.RawRequest.SetRequestURI("http://example.com/")
	req.RawRequest.Header.SetMethod(fiber.MethodPost)
	req.RawRequest.SetBodyStream(bytes.NewBufferString("request body"), len("request body"))

	resp := clientTestBResponse(t)
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

func Test_ParseCookieIgnoringBadAttrs(t *testing.T) {
	t.Parallel()
	manyAttrs := "a=1; Expires=bogus" + strings.Repeat("; x", 4_000) + "; Secure"
	tests := []struct {
		name     string
		value    string
		wantVal  string
		wantPath string
		secure   bool
		httpOnly bool
	}{
		{name: "trailing bad attr", value: "a=1; Secure; Expires=bogus", wantVal: "1", secure: true},
		{name: "leading bad attr", value: "a=1; Expires=bogus; Secure", wantVal: "1", secure: true},
		{name: "bad attr between", value: "a=1; Secure; Max-Age=abc; HttpOnly", wantVal: "1", secure: true, httpOnly: true},
		{name: "two bad attrs", value: "a=1; Expires=bogus; Path=/p; Max-Age=abc", wantVal: "1", wantPath: "/p"},
		{name: "no attrs", value: "a=1", wantVal: "1"},
		{name: "value only", value: "1", wantVal: "1"},
		{name: "many attributes", value: manyAttrs, wantVal: "1", secure: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cookie := clientTestPooled(t, fasthttp.AcquireCookie, fasthttp.ReleaseCookie)

			require.NoError(t, parseCookieIgnoringBadAttrs(cookie, []byte(tc.value)))
			require.Equal(t, tc.wantVal, string(cookie.Value()))
			require.Equal(t, tc.wantPath, string(cookie.Path()))
			require.Equal(t, tc.secure, cookie.Secure())
			require.Equal(t, tc.httpOnly, cookie.HTTPOnly())
		})
	}
}

func Benchmark_ParseCookieIgnoringBadAttrs(b *testing.B) {
	value := []byte("a=1; Max-Age=bogus" + strings.Repeat("; x", 4_000))

	b.ReportAllocs()
	b.SetBytes(int64(len(value)))
	b.ResetTimer()
	for range b.N {
		cookie := fasthttp.AcquireCookie()
		if err := parseCookieIgnoringBadAttrs(cookie, value); err != nil {
			b.Fatal(err)
		}
		fasthttp.ReleaseCookie(cookie)
	}
}

func Test_Client_ResponseCookie_UnparsableAttribute(t *testing.T) {
	t.Parallel()
	client := New()

	req := clientTestBRequest(t)
	req.RawRequest.SetRequestURI("http://example.com/")

	resp := clientTestBResponse(t)
	resp.RawResponse.Header.Add("Set-Cookie", "sid=abc; Secure; HttpOnly; Expires=notadate; Path=/api")

	require.NoError(t, parserResponseCookie(client, resp, req))
	require.Len(t, resp.cookie, 1)

	// Only the malformed attribute is ignored (RFC 6265 §5.2); the ones before
	// and after it survive, as does an expiry that never parsed.
	got := resp.cookie[0]
	require.Equal(t, "sid", string(got.Key()))
	require.Equal(t, "abc", string(got.Value()))
	require.True(t, got.Secure())
	require.True(t, got.HTTPOnly())
	require.Equal(t, "/api", string(got.Path()))
	require.True(t, got.Expire().IsZero() || got.Expire().Equal(fasthttp.CookieExpireUnlimited))
}

// hasRefererHeader reports whether h carries a Referer header, an empty one
// included: Referer answers with an empty value for a missing header and for an
// empty one alike.
func hasRefererHeader(h *fasthttp.RequestHeader) bool {
	for name := range h.All() {
		if strings.EqualFold(string(name), fiber.HeaderReferer) {
			return true
		}
	}
	return false
}

// Test_Client_NoRefererByDefault checks the request a server receives: a client
// with no referer configured must not send the header at all.
func Test_Client_NoRefererByDefault(t *testing.T) {
	t.Parallel()
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString(strconv.FormatBool(hasRefererHeader(&c.Request().Header)))
		})
	})

	resp, err := client.Get("http://example.com/")
	require.NoError(t, err)
	require.Equal(t, "false", resp.String())
	resp.Close()

	resp, err = client.Get("http://example.com/", Config{Referer: "https://example.org/"})
	require.NoError(t, err)
	require.Equal(t, "true", resp.String())
	resp.Close()
}
