package byterange

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_classify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		field     string
		canonical string
		verdict   verdict
	}{
		// Handed on as they are.
		{name: "single range", field: "bytes=0-4", verdict: pass},
		{name: "open ended", field: "bytes=5-", verdict: pass},
		{name: "suffix", field: "bytes=-5", verdict: pass},

		// Spelled the way the file server parses them.
		{name: "unit in upper case", field: "BYTES=0-4", canonical: "bytes=0-4", verdict: rewrite},
		{name: "unit in mixed case", field: "Bytes=0-4", canonical: "bytes=0-4", verdict: rewrite},
		{name: "space after the equals sign", field: "bytes= 0-4", canonical: "bytes=0-4", verdict: rewrite},
		{name: "space before the equals sign", field: "bytes =0-4", canonical: "bytes=0-4", verdict: rewrite},
		{name: "trailing comma", field: "bytes=0-4,", canonical: "bytes=0-4", verdict: rewrite},
		{name: "leading comma", field: "bytes=,0-4", canonical: "bytes=0-4", verdict: rewrite},
		{name: "empty elements around", field: "bytes=, ,0-4, ,", canonical: "bytes=0-4", verdict: rewrite},
		{name: "space around the spec", field: "bytes=  0-4  ", canonical: "bytes=0-4", verdict: rewrite},

		// Unknown units are ignored.
		{name: "unknown unit", field: "items=0-4", verdict: ignore},
		{name: "unknown unit with several ranges", field: "items=0-4,6-9", verdict: ignore},
		{name: "unit that only starts like bytes", field: "bytes2=0-4", verdict: ignore},

		// Several ranges are ignored, not refused.
		{name: "two ranges", field: "bytes=0-1,3-4", verdict: ignore},
		{name: "two ranges with empty elements between", field: "bytes=0-1,,3-4", verdict: ignore},
		{name: "three ranges", field: "bytes=0-1,3-4,6-", verdict: ignore},
		{name: "overlapping ranges", field: "bytes=0-5,3-8", verdict: ignore},

		// Not a ranges-specifier: left for the file server to refuse.
		{name: "no equals sign", field: "bytes", verdict: pass},
		{name: "no unit", field: "=0-4", verdict: pass},
		{name: "no range set", field: "bytes=", verdict: pass},
		{name: "only empty elements", field: "bytes=,,", verdict: pass},
		{name: "garbage", field: "garbage", verdict: pass},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf [64]byte
			canonical, got := classify(buf[:0], []byte(tc.field))
			require.Equal(t, tc.verdict, got)
			if tc.verdict == rewrite {
				require.Equal(t, tc.canonical, string(canonical))
			} else {
				require.Nil(t, canonical)
			}
		})
	}
}

func Test_ifRangeMatches(t *testing.T) {
	t.Parallel()

	const modified = "Thu, 02 Jan 2020 03:04:05 GMT"
	modifiedAt := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name         string
		ifRange      string
		lastModified string
		etag         string
		want         bool
	}{
		{name: "exact date", ifRange: modified, lastModified: modified, want: true},
		{name: "exact date with spaces around", ifRange: "  " + modified + " ", lastModified: modified, want: true},
		{name: "other date", ifRange: "Fri, 03 Jan 2020 03:04:05 GMT", lastModified: modified, want: false},
		{name: "same instant, another spelling", ifRange: "Thursday, 02-Jan-20 03:04:05 GMT", lastModified: modified, want: false},
		{name: "date, no Last-Modified", ifRange: modified, want: false},
		{name: "not a date", ifRange: "yesterday", lastModified: modified, want: false},
		{name: "empty", ifRange: "", lastModified: modified, want: false},
		{name: "strong tag", ifRange: `"v1"`, etag: `"v1"`, want: true},
		{name: "other strong tag", ifRange: `"v1"`, etag: `"v2"`, want: false},
		{name: "strong tag, no ETag", ifRange: `"v1"`, want: false},
		{name: "strong tag against a weak ETag", ifRange: `"v1"`, etag: `W/"v1"`, want: false},
		{name: "weak tag is never strong", ifRange: `W/"v1"`, etag: `W/"v1"`, want: false},
		{name: "weak tag against a strong ETag", ifRange: `W/"v1"`, etag: `"v1"`, want: false},
		{name: "tag is not compared with the date", ifRange: `"v1"`, lastModified: modified, want: false},
		{name: "date is not compared with the ETag", ifRange: modified, etag: modified, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var resp fasthttp.Response
			if tc.lastModified != "" {
				resp.Header.Set(fasthttp.HeaderLastModified, tc.lastModified)
			}
			if tc.etag != "" {
				resp.Header.Set(fasthttp.HeaderETag, tc.etag)
			}
			require.Equal(t, tc.want, ifRangeMatches([]byte(tc.ifRange), &resp, modifiedAt.Add(time.Hour)))
		})
	}

	// A date is a strong validator only when it is old enough (§8.8.2.2).
	t.Run("a date is strong once strongAfter old", func(t *testing.T) {
		t.Parallel()

		var resp fasthttp.Response
		resp.Header.Set(fasthttp.HeaderLastModified, modified)
		for age, want := range map[time.Duration]bool{
			-time.Hour:                    false, // a date in the future cannot be strong
			0:                             false,
			time.Second:                   false,
			strongAfter - time.Nanosecond: false,
			strongAfter:                   true,
			time.Hour:                     true,
		} {
			require.Equal(t, want, ifRangeMatches([]byte(modified), &resp, modifiedAt.Add(age)), age.String())
		}
	})
}

const (
	fileContent  = "0123456789abcdefghij"
	fileModified = "Thu, 02 Jan 2020 03:04:05 GMT"
)

// newFileServer serves a 20-byte file, old enough for a strong Last-Modified,
// through fasthttp's file server with ranges enabled.
func newFileServer(t *testing.T) fasthttp.RequestHandler {
	t.Helper()

	dir := t.TempDir()
	name := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(name, []byte(fileContent), 0o600))
	require.NoError(t, os.Chtimes(name, time.Time{}, time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)))

	server := &fasthttp.FS{
		Root:            dir,
		AcceptByteRange: true,
		SkipCache:       true,
	}
	return server.NewRequestHandler()
}

// discardLogger is the logger of the contexts the tests build.
type discardLogger struct{}

func (discardLogger) Printf(string, ...any) {}

// reply is a response read back out of a RequestCtx.
type reply struct {
	ctx  *fasthttp.RequestCtx
	body string
}

func (r reply) status() int            { return r.ctx.Response.StatusCode() }
func (r reply) header(k string) string { return string(r.ctx.Response.Header.Peek(k)) }

// request runs Serve for a request to the file with the given method and fields.
func request(t *testing.T, handler fasthttp.RequestHandler, method string, fields map[string]string, prepare ...func(*fasthttp.RequestCtx)) reply {
	t.Helper()

	return requestAt(t, time.Now, handler, method, fields, prepare...)
}

// requestAt is request with the clock that ages Last-Modified given.
func requestAt(t *testing.T, now func() time.Time, handler fasthttp.RequestHandler, method string, fields map[string]string, prepare ...func(*fasthttp.RequestCtx)) reply {
	t.Helper()

	var req fasthttp.Request
	req.Header.SetMethod(method)
	req.SetRequestURI("/file.txt")
	for k, v := range fields {
		req.Header.Set(k, v)
	}

	// The file server logs the ranges it refuses, which needs a logger.
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&req, nil, discardLogger{})
	for _, p := range prepare {
		p(ctx)
	}

	serveAt(ctx, handler, now)

	// Reading the body of a stream drains and closes it.
	return reply{ctx: ctx, body: string(ctx.Response.Body())}
}

func Test_Wrap_RangeRules(t *testing.T) {
	t.Parallel()

	handler := newFileServer(t)

	tests := []struct {
		name         string
		method       string
		rangeField   string
		wantRange    string // Content-Range of the response
		wantBody     string
		wantLength   string
		wantStatus   int
		wantAccepted bool // Accept-Ranges is advertised
	}{
		{name: "no Range", method: fasthttp.MethodGet, wantStatus: 200, wantBody: fileContent, wantLength: "20", wantAccepted: true},
		{name: "a range", method: fasthttp.MethodGet, rangeField: "bytes=0-4", wantStatus: 206, wantBody: "01234", wantRange: "bytes 0-4/20", wantLength: "5", wantAccepted: true},
		{name: "suffix range", method: fasthttp.MethodGet, rangeField: "bytes=-5", wantStatus: 206, wantBody: "fghij", wantRange: "bytes 15-19/20", wantLength: "5", wantAccepted: true},
		{name: "open ended range", method: fasthttp.MethodGet, rangeField: "bytes=15-", wantStatus: 206, wantBody: "fghij", wantRange: "bytes 15-19/20", wantLength: "5", wantAccepted: true},

		// Units are case-insensitive (§14.1).
		{name: "unit in upper case", method: fasthttp.MethodGet, rangeField: "BYTES=0-4", wantStatus: 206, wantBody: "01234", wantRange: "bytes 0-4/20", wantLength: "5", wantAccepted: true},
		{name: "unit in mixed case", method: fasthttp.MethodGet, rangeField: "Bytes=-5", wantStatus: 206, wantBody: "fghij", wantRange: "bytes 15-19/20", wantLength: "5", wantAccepted: true},
		{name: "space after the equals sign", method: fasthttp.MethodGet, rangeField: "bytes= 0-4", wantStatus: 206, wantBody: "01234", wantRange: "bytes 0-4/20", wantLength: "5", wantAccepted: true},
		{name: "empty list elements", method: fasthttp.MethodGet, rangeField: "bytes=,0-4,", wantStatus: 206, wantBody: "01234", wantRange: "bytes 0-4/20", wantLength: "5", wantAccepted: true},

		// The whole representation: Range is ignored.
		{name: "several ranges", method: fasthttp.MethodGet, rangeField: "bytes=0-1,3-4", wantStatus: 200, wantBody: fileContent, wantLength: "20", wantAccepted: true},
		{name: "several ranges, one of them unsatisfiable", method: fasthttp.MethodGet, rangeField: "bytes=0-1,100-200", wantStatus: 200, wantBody: fileContent, wantLength: "20", wantAccepted: true},
		{name: "unknown unit", method: fasthttp.MethodGet, rangeField: "items=0-4", wantStatus: 200, wantBody: fileContent, wantLength: "20", wantAccepted: true},
		{name: "only GET has ranges: HEAD", method: fasthttp.MethodHead, rangeField: "bytes=0-4", wantStatus: 200, wantLength: "20", wantAccepted: true},

		// Refused by the file server, as before.
		{name: "start past the end", method: fasthttp.MethodGet, rangeField: "bytes=100-200", wantStatus: 416, wantBody: "Range Not Satisfiable"},
		{name: "reversed range", method: fasthttp.MethodGet, rangeField: "bytes=5-1", wantStatus: 416, wantBody: "Range Not Satisfiable"},
		{name: "no range set", method: fasthttp.MethodGet, rangeField: "bytes=", wantStatus: 416, wantBody: "Range Not Satisfiable"},
		{name: "garbage", method: fasthttp.MethodGet, rangeField: "garbage", wantStatus: 416, wantBody: "Range Not Satisfiable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fields := map[string]string{}
			if tc.rangeField != "" {
				fields[fasthttp.HeaderRange] = tc.rangeField
			}
			got := request(t, handler, tc.method, fields)

			require.Equal(t, tc.wantStatus, got.status())
			require.Equal(t, tc.wantBody, got.body)
			require.Equal(t, tc.wantRange, got.header(fasthttp.HeaderContentRange))
			if tc.wantLength != "" {
				require.Equal(t, tc.wantLength, got.header("Content-Length"))
			}
			if tc.wantAccepted {
				require.Equal(t, "bytes", got.header("Accept-Ranges"))
			}

			// The rest of the chain sees the request as it came.
			require.Equal(t, tc.rangeField, string(got.ctx.Request.Header.Peek(fasthttp.HeaderRange)))
		})
	}
}

func Test_Wrap_IfRange(t *testing.T) {
	t.Parallel()

	handler := newFileServer(t)

	const stale = "Wed, 01 Jan 2020 03:04:05 GMT"

	tests := []struct {
		name       string
		rangeField string
		ifRange    string
		wantRange  string
		wantBody   string
		wantStatus int
	}{
		// The validator matches: the range is served.
		{name: "matching date", rangeField: "bytes=0-4", ifRange: fileModified, wantStatus: 206, wantBody: "01234", wantRange: "bytes 0-4/20"},
		{name: "matching date, unit in upper case", rangeField: "BYTES=0-4", ifRange: fileModified, wantStatus: 206, wantBody: "01234", wantRange: "bytes 0-4/20"},
		{name: "matching date, unsatisfiable range", rangeField: "bytes=100-200", ifRange: fileModified, wantStatus: 416, wantBody: "Range Not Satisfiable"},

		// It does not: the whole file is sent (§13.1.5).
		{name: "stale date", rangeField: "bytes=0-4", ifRange: stale, wantStatus: 200, wantBody: fileContent},
		{name: "stale date, suffix range", rangeField: "bytes=-5", ifRange: stale, wantStatus: 200, wantBody: fileContent},
		{name: "stale date, unsatisfiable range", rangeField: "bytes=100-200", ifRange: stale, wantStatus: 200, wantBody: fileContent},
		{name: "entity-tag the file server never sent", rangeField: "bytes=0-4", ifRange: `"deadbeef"`, wantStatus: 200, wantBody: fileContent},
		{name: "weak entity-tag", rangeField: "bytes=0-4", ifRange: `W/"deadbeef"`, wantStatus: 200, wantBody: fileContent},
		{name: "not a validator", rangeField: "bytes=0-4", ifRange: "later", wantStatus: 200, wantBody: fileContent},

		// Ranges that were ignored anyway.
		{name: "matching date, several ranges", rangeField: "bytes=0-1,3-4", ifRange: fileModified, wantStatus: 200, wantBody: fileContent},
		{name: "stale date, several ranges", rangeField: "bytes=0-1,3-4", ifRange: stale, wantStatus: 200, wantBody: fileContent},
		{name: "stale date, unknown unit", rangeField: "items=0-4", ifRange: stale, wantStatus: 200, wantBody: fileContent},

		// If-Range means nothing without Range.
		{name: "no Range", ifRange: stale, wantStatus: 200, wantBody: fileContent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fields := map[string]string{fasthttp.HeaderIfRange: tc.ifRange}
			if tc.rangeField != "" {
				fields[fasthttp.HeaderRange] = tc.rangeField
			}
			got := request(t, handler, fasthttp.MethodGet, fields)

			require.Equal(t, tc.wantStatus, got.status())
			require.Equal(t, tc.wantBody, got.body)
			require.Equal(t, tc.wantRange, got.header(fasthttp.HeaderContentRange))
			if tc.wantStatus == 200 {
				// A whole representation, framed as one.
				require.Equal(t, "20", got.header("Content-Length"))
				require.Equal(t, fileModified, got.header(fasthttp.HeaderLastModified))
				require.Equal(t, "bytes", got.header("Accept-Ranges"))
			}
			require.Equal(t, tc.rangeField, string(got.ctx.Request.Header.Peek(fasthttp.HeaderRange)))
			require.Equal(t, tc.ifRange, string(got.ctx.Request.Header.Peek(fasthttp.HeaderIfRange)))
		})
	}
}

// A file modified less than strongAfter ago has no strong validator, so no
// If-Range date matches it (§8.8.2.2).
func Test_Wrap_IfRange_Strength(t *testing.T) {
	t.Parallel()

	handler := newFileServer(t)
	modifiedAt := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)

	for age, wantStatus := range map[time.Duration]int{
		0:                             200,
		time.Second:                   200,
		strongAfter - time.Nanosecond: 200,
		strongAfter:                   206,
		time.Hour:                     206,
	} {
		t.Run(age.String(), func(t *testing.T) {
			t.Parallel()

			now := func() time.Time { return modifiedAt.Add(age) }
			fields := map[string]string{fasthttp.HeaderRange: "bytes=0-4", fasthttp.HeaderIfRange: fileModified}
			require.Equal(t, wantStatus, requestAt(t, now, handler, fasthttp.MethodGet, fields).status())
		})
	}
}

// Headers set before the file server ran survive every outcome; those of a
// partial response do not.
func Test_Wrap_KeepsCallerHeaders(t *testing.T) {
	t.Parallel()

	handler := newFileServer(t)
	prepare := func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.Set("X-Request", "kept")
	}

	for name, fields := range map[string]map[string]string{
		"range served":               {fasthttp.HeaderRange: "bytes=0-4"},
		"range ignored":              {fasthttp.HeaderRange: "bytes=0-1,3-4"},
		"range dropped for If-Range": {fasthttp.HeaderRange: "bytes=0-4", fasthttp.HeaderIfRange: "Wed, 01 Jan 2020 03:04:05 GMT"},
		// The refusal wipes the response; the caller's headers are put back.
		"unsatisfiable range dropped for If-Range": {fasthttp.HeaderRange: "bytes=100-200", fasthttp.HeaderIfRange: "Wed, 01 Jan 2020 03:04:05 GMT"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := request(t, handler, fasthttp.MethodGet, fields, prepare)
			require.Equal(t, "kept", got.header("X-Request"))
			if got.status() == 200 {
				require.Empty(t, got.header(fasthttp.HeaderContentRange))
				require.Equal(t, "20", got.header("Content-Length"))
			}
		})
	}
}

func Benchmark_Wrap_NoRange(b *testing.B) {
	dir := b.TempDir()
	require.NoError(b, os.WriteFile(filepath.Join(dir, "file.txt"), []byte(fileContent), 0o600))
	server := &fasthttp.FS{Root: dir, AcceptByteRange: true}
	handler := Wrap(server.NewRequestHandler())

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/file.txt")

	b.ReportAllocs()
	for b.Loop() {
		handler(ctx)
		ctx.Response.Reset()
	}
}

func Benchmark_Wrap_SingleRange(b *testing.B) {
	dir := b.TempDir()
	require.NoError(b, os.WriteFile(filepath.Join(dir, "file.txt"), []byte(fileContent), 0o600))
	server := &fasthttp.FS{Root: dir, AcceptByteRange: true}
	handler := Wrap(server.NewRequestHandler())

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/file.txt")
	ctx.Request.Header.Set(fasthttp.HeaderRange, "bytes=0-4")

	b.ReportAllocs()
	for b.Loop() {
		handler(ctx)
		ctx.Response.Reset()
	}
}

// The Range field is put back even when the handler panics.
func Test_Wrap_RestoresRangeAfterPanic(t *testing.T) {
	t.Parallel()

	handler := Wrap(func(*fasthttp.RequestCtx) { panic("handler broke") })

	for _, tc := range []struct{ method, field string }{
		{method: fasthttp.MethodGet, field: "bytes=0-4"},
		{method: fasthttp.MethodGet, field: "Bytes=0-4"},
		{method: fasthttp.MethodGet, field: "bytes=0-1,3-4"},
		{method: fasthttp.MethodGet, field: "items=0-4"},
		{method: fasthttp.MethodHead, field: "bytes=0-4"},
	} {
		var req fasthttp.Request
		req.Header.SetMethod(tc.method)
		req.SetRequestURI("/file.txt")
		req.Header.Set(fasthttp.HeaderRange, tc.field)
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(&req, nil, discardLogger{})

		require.Panics(t, func() { handler(ctx) })
		require.Equal(t, tc.field, string(ctx.Request.Header.Peek(fasthttp.HeaderRange)), tc.method+" "+tc.field)
	}
}
