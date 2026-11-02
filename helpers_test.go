// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Benchmark_PeekJoinedRequestHeader(b *testing.B) {
	for _, lines := range []int{1, 2, 3} {
		b.Run(strconv.Itoa(lines), func(b *testing.B) {
			var h fasthttp.RequestHeader
			for range lines {
				h.Add(HeaderAccept, "text/plain")
			}
			var joined []byte
			b.ReportAllocs()
			for b.Loop() {
				joined = peekJoinedRequestHeader(&h, HeaderAccept)
			}
			require.NotEmpty(b, joined)
		})
	}
}

// Test_JoinedInReceivedOrder pins the read-only scan to the values fasthttp parses.
func Test_JoinedInReceivedOrder(t *testing.T) {
	t.Parallel()
	const head = "GET / HTTP/1.1\r\nHost: example.com\r\n"
	tests := []struct {
		name     string
		lines    string
		combined string // empty: as stored
		want     bool
	}{
		{name: "in order", lines: "X-Forwarded-For: 1.1.1.1\r\nAccept: */*\r\nX-Forwarded-For: 2.2.2.2\r\n", want: true},
		{name: "OWS and spelling", lines: "x-forwarded-for:\t1.1.1.1 \r\nX-FORWARDED-FOR:2.2.2.2\t\r\n", want: true},
		{name: "longer name", lines: "X-Forwarded-For-Extra: 9.9.9.9\r\nX-Forwarded-For: 1.1.1.1\r\nX-Forwarded-For: 2.2.2.2\r\n", want: true},
		{name: "reordered", lines: "X-Forwarded-For: 1.1.1.1\r\nX-Forwarded-For: 2.2.2.2\r\n", combined: "2.2.2.2,1.1.1.1"},
		{name: "added", lines: "X-Forwarded-For: 1.1.1.1\r\nX-Forwarded-For: 2.2.2.2\r\n", combined: "1.1.1.1,2.2.2.2,3.3.3.3"},
		{name: "folded", lines: "X-Forwarded-For: 1.1.1.1,\r\n 3.3.3.3\r\nX-Forwarded-For: 2.2.2.2\r\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var req fasthttp.Request
			require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(head+tc.lines+"\r\n"))))
			combined := []byte(tc.combined)
			if tc.combined == "" {
				combined = peekJoinedRequestHeader(&req.Header, HeaderXForwardedFor)
			}
			require.Equal(t, tc.want, fieldname.JoinedInReceivedOrder(req.Header.RawHeaders(), HeaderXForwardedFor, combined))
		})
	}
}

func Test_Utils_GetOffer(t *testing.T) {
	t.Parallel()
	require.Empty(t, getOffer([]byte("hello"), acceptsOffer))
	require.Equal(t, "1", getOffer([]byte(""), acceptsOffer, "1"))
	require.Empty(t, getOffer([]byte("2"), acceptsOffer, "1"))

	require.Empty(t, getOffer([]byte(""), acceptsOfferType))
	require.Empty(t, getOffer([]byte("text/html"), acceptsOfferType))
	require.Empty(t, getOffer([]byte("text/html"), acceptsOfferType, "application/json"))
	require.Empty(t, getOffer([]byte("text/html;q=0"), acceptsOfferType, "text/html"))
	require.Empty(t, getOffer([]byte("application/json, */*; q=0"), acceptsOfferType, "image/png"))
	require.Equal(t, "application/xml", getOffer([]byte("text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"), acceptsOfferType, "application/xml", "application/json"))
	require.Equal(t, "text/html", getOffer([]byte("text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"), acceptsOfferType, "text/html"))
	require.Equal(t, "application/pdf", getOffer([]byte("text/plain;q=0,application/pdf;q=0.9,*/*;q=0.000"), acceptsOfferType, "application/pdf", "application/json"))
	require.Equal(t, "application/pdf", getOffer([]byte("text/plain;q=0,application/pdf;q=0.9,*/*;q=0.000"), acceptsOfferType, "application/pdf", "application/json"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("text/plain;a=1"), acceptsOfferType, "text/plain;a=1"))
	require.Empty(t, getOffer([]byte("text/plain;a=1;b=2"), acceptsOfferType, "text/plain;b=2"))

	// Spaces, quotes, out of order params, and case insensitivity
	require.Equal(t, "text/plain", getOffer([]byte("text/plain  "), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain", getOffer([]byte("text/plain;q=0.4  "), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain", getOffer([]byte("text/plain;q=0.4  ;"), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain", getOffer([]byte("text/plain;q=0.4  ; p=foo"), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain;b=2;a=1", getOffer([]byte("text/plain ;a=1;b=2"), acceptsOfferType, "text/plain;b=2;a=1"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("text/plain;   a=1   "), acceptsOfferType, "text/plain;a=1"))
	require.Equal(t, `text/plain;a="1;b=2\",text/plain"`, getOffer([]byte(`text/plain;a="1;b=2\",text/plain";q=0.9`), acceptsOfferType, `text/plain;a=1;b=2`, `text/plain;a="1;b=2\",text/plain"`))
	// A quoted-pair escapes exactly one byte, so the string ends at the genuine
	// closing quote and the later comma splits the ranges (RFC 9110 Section 5.6.4).
	require.Equal(t, "text/plain", getOffer([]byte(`text/html;p="a\"b", text/plain`), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain;A=CAPS", getOffer([]byte(`text/plain;a="caPs"`), acceptsOfferType, "text/plain;A=CAPS"))

	// The media type and subtype tokens are case-insensitive (RFC 9110 8.3.1)
	require.Equal(t, "application/json", getOffer([]byte("Application/JSON"), acceptsOfferType, "application/json"))
	require.Equal(t, "Application/JSON", getOffer([]byte("application/json"), acceptsOfferType, "Application/JSON"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("Text/Plain;a=1"), acceptsOfferType, "text/plain;a=1"))

	// Priority
	require.Equal(t, "text/plain", getOffer([]byte("text/plain"), acceptsOfferType, "text/plain", "text/plain;a=1"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("text/plain"), acceptsOfferType, "text/plain;a=1", "", "text/plain"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("text/plain,text/plain;a=1"), acceptsOfferType, "text/plain", "text/plain;a=1"))
	require.Equal(t, "text/plain", getOffer([]byte("text/plain;q=0.899,text/plain;a=1;q=0.898"), acceptsOfferType, "text/plain", "text/plain;a=1"))
	require.Equal(t, "text/plain;a=1;b=2", getOffer([]byte("text/plain,text/plain;a=1,text/plain;a=1;b=2"), acceptsOfferType, "text/plain", "text/plain;a=1", "text/plain;a=1;b=2"))

	// Takes the last value specified
	require.Equal(t, "text/plain;a=1;b=2", getOffer([]byte("text/plain;a=1;b=1;B=2"), acceptsOfferType, "text/plain;a=1;b=1", "text/plain;a=1;b=2"))

	require.Empty(t, getOffer([]byte("utf-8, iso-8859-1;q=0.5"), acceptsOffer))
	require.Empty(t, getOffer([]byte("utf-8, iso-8859-1;q=0.5"), acceptsOffer, "ascii"))
	require.Equal(t, "utf-8", getOffer([]byte("utf-8, iso-8859-1;q=0.5"), acceptsOffer, "utf-8"))
	require.Equal(t, "iso-8859-1", getOffer([]byte("utf-8;q=0, iso-8859-1;q=0.5"), acceptsOffer, "utf-8", "iso-8859-1"))

	// Accept-Charset wildcard coverage
	require.Equal(t, "utf-8", getOffer([]byte("utf-*"), acceptsOffer, "utf-8"))
	require.Equal(t, "UTF-16", getOffer([]byte("utf-*"), acceptsOffer, "UTF-16", "iso-8859-1"))
	require.Empty(t, getOffer([]byte("utf-*"), acceptsOffer, "iso-8859-1"))
	require.Empty(t, getOffer([]byte("utf-*"), acceptsOffer, "utf"))
	require.Empty(t, getOffer([]byte("utf-*"), acceptsOffer, "x-utf-8"))

	// Complex wildcard negotiation
	require.Equal(t, "utf-16le", getOffer([]byte("utf-8;q=0.4, utf-*;q=0.8, iso-8859-1;q=0.6"), acceptsOffer, "iso-8859-1", "utf-16le"))
	require.Equal(t, "iso-8859-1", getOffer([]byte("utf-*;q=0.9, iso-8859-1;q=1"), acceptsOffer, "x-utf-16", "iso-8859-1"))
	require.Empty(t, getOffer([]byte("utf-*;q=0.5, iso-8859-1;q=0.4"), acceptsOffer, "ascii", "us-ascii"))

	require.Equal(t, "deflate", getOffer([]byte("gzip, deflate"), acceptsOffer, "deflate"))
	require.Empty(t, getOffer([]byte("gzip, deflate;q=0"), acceptsOffer, "deflate"))

	// Accept-Language Basic Filtering
	require.Positive(t, acceptsLanguageOfferBasic("en", "en-US", nil))
	require.Zero(t, acceptsLanguageOfferBasic("en-US", "en", nil))
	require.Positive(t, acceptsLanguageOfferBasic("EN", "en-us", nil))
	require.Zero(t, acceptsLanguageOfferBasic("en", "en_US", nil))
	require.Equal(t, "en-US", getOffer([]byte("fr-CA;q=0.8, en-US"), acceptsLanguageOfferBasic, "en-US", "fr-CA"))
	require.Empty(t, getOffer([]byte("xx"), acceptsLanguageOfferBasic, "en"))
	require.Zero(t, acceptsLanguageOfferBasic("en-*", "en-US", nil))
	require.Positive(t, acceptsLanguageOfferBasic("*", "en-US", nil))

	// Accept-Language Extended Filtering
	require.Positive(t, acceptsLanguageOfferExtended("en", "en-US", nil))
	require.Positive(t, acceptsLanguageOfferExtended("en", "en-Latn-US", nil))
	require.Positive(t, acceptsLanguageOfferExtended("en-*", "en-US", nil))
	require.Positive(t, acceptsLanguageOfferExtended("*-US", "en-US", nil))
	require.Positive(t, acceptsLanguageOfferExtended("en-US-*", "en-US", nil))
	require.Positive(t, acceptsLanguageOfferExtended("en-*", "en-US-CA", nil))
	require.Zero(t, acceptsLanguageOfferExtended("en-US", "en-GB", nil))
	require.Zero(t, acceptsLanguageOfferExtended("fr", "en-US", nil))
	require.Zero(t, acceptsLanguageOfferExtended("", "en-US", nil))
	require.Zero(t, acceptsLanguageOfferExtended("en", "", nil))
	require.Positive(t, acceptsLanguageOfferExtended("*", "en-US", nil))
	require.Positive(t, acceptsLanguageOfferExtended("en-*", "en", nil))
	require.Equal(t, "en-US", getOffer([]byte("fr-CA;q=0.8, en-*"), acceptsLanguageOfferExtended, "en-US", "fr-CA"))

	// Sliding and singleton barriers
	require.Positive(t, acceptsLanguageOfferExtended("de-*-DE", "de-DE", nil))
	require.Positive(t, acceptsLanguageOfferExtended("de-*-DE", "de-DE-x-goethe", nil))
	require.Positive(t, acceptsLanguageOfferExtended("de-*-DE", "de-Latn-DE-1996", nil))
	require.Zero(t, acceptsLanguageOfferExtended("de-*-DE", "de", nil))
	require.Zero(t, acceptsLanguageOfferExtended("de-*-DE", "de-x-DE", nil))
	require.Positive(t, acceptsLanguageOfferExtended("*-CH", "de-CH", nil))
	require.Positive(t, acceptsLanguageOfferExtended("*-CH", "de-Latn-CH", nil))
}

func Test_ReadContentReturnsBytes(t *testing.T) {
	t.Parallel()
	content := []byte("fiber read content test")
	tempFile, err := os.CreateTemp("", "fiber-read-content-*.txt")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Remove(tempFile.Name())) })

	_, err = tempFile.Write(content)
	require.NoError(t, err)
	require.NoError(t, tempFile.Close())

	var buffer bytes.Buffer
	n, err := readContent(&buffer, tempFile.Name())
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), n)
	require.Equal(t, content, buffer.Bytes())
}

type wrappedListener struct {
	net.Listener
}

type tlsConfigMethodListener struct {
	net.Listener
	cfg *tls.Config
}

func (ln *tlsConfigMethodListener) TLSConfig() *tls.Config {
	return ln.cfg
}

type configMethodListener struct {
	net.Listener
	cfg *tls.Config
}

func (ln *configMethodListener) Config() *tls.Config {
	return ln.cfg
}

type nonStructListener int

func (nonStructListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (nonStructListener) Close() error              { return nil }
func (nonStructListener) Addr() net.Addr            { return &net.TCPAddr{} }

func Test_GetTLSConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		// wrap builds the listener under test on top of base. Closing it must
		// close base, so the subtest cleans up through it.
		wrap       func(base net.Listener, cfg *tls.Config) net.Listener
		name       string
		msg        string
		minVersion uint16
		wantConfig bool // false: no TLS config must be reported
	}{
		{
			name:       "tls listener",
			wrap:       tls.NewListener,
			minVersion: tls.VersionTLS12,
			wantConfig: true,
			msg:        "*tls.Listener should expose its TLS config",
		},
		{
			name: "wrapped tls listener",
			wrap: func(base net.Listener, cfg *tls.Config) net.Listener {
				return &wrappedListener{Listener: tls.NewListener(base, cfg)}
			},
			minVersion: tls.VersionTLS13,
			msg:        "wrapping without Config()-like methods should return nil",
		},
		{
			name: "listener with tls config method",
			wrap: func(base net.Listener, cfg *tls.Config) net.Listener {
				return &tlsConfigMethodListener{Listener: base, cfg: cfg}
			},
			minVersion: tls.VersionTLS13,
			wantConfig: true,
			msg:        "TLSConfig() should be preferred for TLS discovery",
		},
		{
			name: "listener with config method",
			wrap: func(base net.Listener, cfg *tls.Config) net.Listener {
				return &configMethodListener{Listener: base, cfg: cfg}
			},
			minVersion: tls.VersionTLS12,
			wantConfig: true,
			msg:        "Config() should be preferred for TLS discovery",
		},
		{
			name: "non tls listener",
			wrap: func(base net.Listener, _ *tls.Config) net.Listener { return base },
			msg:  "plain listeners should not report TLS config",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &tls.Config{MinVersion: tc.minVersion}
			ln := tc.wrap(newLocalListener(t), cfg)
			t.Cleanup(func() { require.NoError(t, ln.Close()) })

			if tc.wantConfig {
				require.Same(t, cfg, getTLSConfig(ln), tc.msg)
			} else {
				require.Nil(t, getTLSConfig(ln), tc.msg)
			}
		})
	}

	t.Run("non-struct listener", func(t *testing.T) {
		t.Parallel()
		ln := nonStructListener(7)
		require.NotPanics(t, func() {
			require.Nil(t, getTLSConfig(ln), "non-struct listeners should not panic and should not report TLS config")
		})
	})
}

func newLocalListener(t *testing.T) net.Listener {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	return ln
}

// Per RFC 9110 §12.5.1 the most specific matching media range decides an
// offer's acceptability, so a broader range with a higher quality must not
// override a more specific q=0 rejection. Before the fix, getOffer dropped
// q=0 ranges up front and returned an offer the client had explicitly refused.
func Test_Utils_GetOffer_QualityZeroRejection(t *testing.T) {
	t.Parallel()
	// A more specific q=0 range rejects the offer even though a broader
	// range (text/*, */*) with quality > 0 also matches it.
	require.Empty(t, getOffer([]byte("text/*, text/html;q=0"), acceptsOfferType, "text/html"))
	require.Empty(t, getOffer([]byte("*/*, application/json;q=0"), acceptsOfferType, "application/json"))

	// The broader range still selects offers that the q=0 range does not reject.
	require.Equal(t, "text/plain", getOffer([]byte("text/*, text/html;q=0"), acceptsOfferType, "text/plain"))

	// Accept-Encoding: the client accepts anything except gzip, so a server
	// offering gzip or deflate must pick deflate, not the refused gzip.
	require.Equal(t, "deflate", getOffer([]byte("*, gzip;q=0"), acceptsOffer, "gzip", "deflate"))
	require.Empty(t, getOffer([]byte("*, gzip;q=0"), acceptsOffer, "gzip"))

	// Accept-Language: "*" accepts any tag, but "en;q=0" rejects English.
	require.Empty(t, getOffer([]byte("*, en;q=0"), acceptsLanguageOfferBasic, "en"))
	require.Equal(t, "fr", getOffer([]byte("*, en;q=0"), acceptsLanguageOfferBasic, "en", "fr"))

	// Same coarse specificity class: a positive match and a q=0 refusal can share
	// the parsed specificity bucket, so the effective match specificity decides.

	// Media-type parameters (RFC 9110 §12.5.1): "text/html;level=1;q=0" is more
	// specific than "text/html", so an offer carrying that parameter is rejected.
	require.Empty(t, getOffer([]byte("text/html, text/html;level=1;q=0"), acceptsOfferType, "text/html;level=1"))
	// A less specific "text/html;q=0" must not override the more specific positive
	// "text/html;level=1" match.
	require.Equal(t, "text/html;level=1", getOffer([]byte("text/html;level=1, text/html;q=0"), acceptsOfferType, "text/html;level=1"))
	// An offer without the refused parameter is still accepted.
	require.Equal(t, "text/html", getOffer([]byte("text/html, text/html;level=1;q=0"), acceptsOfferType, "text/html"))

	// Accept-Language basic filtering buckets "en" and "en-US" the same, but an
	// exact "en-US;q=0" must override the prefix "en" match.
	require.Empty(t, getOffer([]byte("en, en-US;q=0"), acceptsLanguageOfferBasic, "en-US"))
	require.Equal(t, "en-GB", getOffer([]byte("en, en-US;q=0"), acceptsLanguageOfferBasic, "en-US", "en-GB"))
	// A broader "en;q=0" does not override the exact positive "en-US".
	require.Equal(t, "en-US", getOffer([]byte("en-US, en;q=0"), acceptsLanguageOfferBasic, "en-US"))
	// Extended filtering: a deeper refused range wins over a shorter positive one.
	require.Empty(t, getOffer([]byte("en, en-US;q=0"), acceptsLanguageOfferExtended, "en-US"))

	// Accept-Charset: an exact "utf-8;q=0" refusal overrides an earlier exact
	// "utf-8" of the same specificity, and a wildcard positive too.
	require.Empty(t, getOffer([]byte("utf-8, utf-8;q=0"), acceptsOffer, "utf-8"))
	require.Empty(t, getOffer([]byte("*, utf-8;q=0"), acceptsOffer, "utf-8"))
	require.Equal(t, "iso-8859-1", getOffer([]byte("*, utf-8;q=0"), acceptsOffer, "utf-8", "iso-8859-1"))

	// Optional whitespace may precede the list separator (RFC 9110 §5.6.1.2),
	// so the weight is still the last thing in the element and must be honored.
	require.Empty(t, getOffer([]byte("text/html;q=0 , text/plain"), acceptsOfferType, "text/html"))
	require.Equal(t, "text/plain", getOffer([]byte("text/html;q=0 , text/plain"), acceptsOfferType, "text/html", "text/plain"))
	require.Empty(t, getOffer([]byte("text/html;q=0\t, text/plain"), acceptsOfferType, "text/html"))
	require.Equal(t, "deflate", getOffer([]byte("gzip;q=0 , deflate"), acceptsOffer, "gzip", "deflate"))
	require.Empty(t, getOffer([]byte("en;q=0 , fr"), acceptsLanguageOfferBasic, "en"))
	// The same whitespace must not distort ordering between positive weights.
	require.Equal(t, "application/json", getOffer([]byte("text/plain;q=0.1 , application/json;q=0.9"), acceptsOfferType, "text/plain", "application/json"))

	// An empty offer is skipped on the resolve-by-specificity path too: the first
	// match (text/plain, demoted to 0.3 by its own range) loses to text/html.
	require.Equal(t, "text/html", getOffer([]byte("text/*;q=0.8, text/plain;q=0.3"), acceptsOfferType, "", "text/plain", "text/html"))
	require.Equal(t, "text/plain", getOffer([]byte("text/*;q=0.8, text/plain;q=0.3"), acceptsOfferType, "", "text/plain"))
}

func Benchmark_Utils_GetOffer(b *testing.B) {
	testCases := []struct {
		description string
		accept      string
		offers      []string
	}{
		{description: "simple", accept: "application/json", offers: []string{"application/json"}},
		{
			description: "6 offers",
			accept:      "text/plain",
			offers:      []string{"junk/a", "junk/b", "junk/c", "junk/d", "junk/e", "text/plain"},
		},
		{
			description: "1 parameter",
			accept:      "application/json; version=1",
			offers:      []string{"application/json;version=1"},
		},
		{
			description: "2 parameters",
			accept:      "application/json; version=1; foo=bar",
			offers:      []string{"application/json;version=1;foo=bar"},
		},
		{
			description: "3 parameters",
			accept:      "application/json; version=1; foo=bar; charset=utf-8",
			offers:      []string{"application/json;version=1;foo=bar;charset=utf-8"},
		},
		{
			description: "10 parameters",
			accept:      "text/plain;a=1;b=2;c=3;d=4;e=5;f=6;g=7;h=8;i=9;j=10",
			offers:      []string{"text/plain;a=1;b=2;c=3;d=4;e=5;f=6;g=7;h=8;i=9;j=10"},
		},
		{
			description: "6 offers w/params",
			accept:      "text/plain; format=flowed",
			offers: []string{
				"junk/a;a=b",
				"junk/b;b=c",
				"junk/c;c=d",
				"text/plain; format=justified",
				"text/plain; format=flat",
				"text/plain; format=flowed",
			},
		},
		{description: "mime extension", accept: "utf-8, iso-8859-1;q=0.5", offers: []string{"utf-8"}},
		{description: "mime extension", accept: "utf-8, iso-8859-1;q=0.5", offers: []string{"iso-8859-1"}},
		{description: "mime extension", accept: "utf-8, iso-8859-1;q=0.5", offers: []string{"iso-8859-1", "utf-8"}},
		{description: "mime extension", accept: "gzip, deflate", offers: []string{"deflate"}},
		{
			description: "web browser",
			accept:      "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			offers:      []string{"text/html", "application/xml", "application/xml+xhtml"},
		},
	}

	b.ReportAllocs()
	for _, tc := range testCases {
		accept := []byte(tc.accept)
		b.Run(tc.description, func(b *testing.B) {
			for b.Loop() {
				getOffer(accept, acceptsOfferType, tc.offers...)
			}
		})
	}
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Utils_ParamsMatch(t *testing.T) {
	testCases := []struct {
		description string
		accept      headerParams
		offer       string
		match       bool
	}{
		{description: "empty accept and offer", accept: nil, offer: "", match: true},
		{description: "accept is empty, offer has params", accept: make(headerParams), offer: ";foo=bar", match: true},
		{description: "offer is empty, accept has params", accept: headerParams{"foo": []byte("bar")}, offer: "", match: false},
		{description: "accept has extra parameters", accept: headerParams{"foo": []byte("bar"), "a": []byte("1")}, offer: ";foo=bar", match: false},
		{description: "matches regardless of order", accept: headerParams{"b": []byte("2"), "a": []byte("1")}, offer: ";b=2;a=1", match: true},
		{description: "case-insensitive", accept: headerParams{"ParaM": []byte("FoO")}, offer: ";pAram=foO", match: true},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.match, paramsMatch(tc.accept, tc.offer), tc.description)
	}
}

func Benchmark_Utils_ParamsMatch(b *testing.B) {
	var match bool

	specParams := headerParams{"appLe": []byte("orange"), "param": []byte("foo")}
	b.ReportAllocs()
	for b.Loop() {
		match = paramsMatch(specParams, `;param=foo; apple=orange`)
	}
	require.True(b, match)
}

func Test_Utils_AcceptsOfferType(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description string
		spec        string
		specParams  headerParams
		offerType   string
		accepts     bool
	}{
		{description: "no params, matching", spec: "application/json", offerType: "application/json", accepts: true},
		{description: "no params, mismatch", spec: "application/json", offerType: "application/xml", accepts: false},
		{description: "mismatch with subtype prefix", spec: "application/json", offerType: "application/json+xml", accepts: false},
		{
			description: "params match",
			spec:        "application/json",
			specParams:  headerParams{"format": []byte("foo"), "version": []byte("1")},
			offerType:   "application/json;version=1;format=foo;q=0.1",
			accepts:     true,
		},
		{
			description: "spec has extra params",
			spec:        "text/html",
			specParams:  headerParams{"charset": []byte("utf-8")},
			offerType:   "text/html",
			accepts:     false,
		},
		{description: "offer has extra params", spec: "text/html", offerType: "text/html;charset=utf-8", accepts: true},
		{
			description: "ignores optional whitespace",
			spec:        "application/json",
			specParams:  headerParams{"format": []byte("foo"), "version": []byte("1")},
			offerType:   "application/json;  version=1 ;    format=foo   ",
			accepts:     true,
		},
		{
			description: "ignores optional whitespace",
			spec:        "application/json",
			specParams:  headerParams{"format": []byte("foo bar"), "version": []byte("1")},
			offerType:   `application/json;version="1";format="foo bar"`,
			accepts:     true,
		},
	}
	for _, tc := range testCases {
		accepts := acceptsOfferType(tc.spec, tc.offerType, tc.specParams) > 0
		require.Equal(t, tc.accepts, accepts, tc.description)
	}
}

func Test_Utils_SortAcceptedTypes(t *testing.T) {
	t.Parallel()
	acceptedTypes := []acceptedType{
		{spec: "text/html", quality: 1, specificity: 3, order: 0},
		{spec: "text/*", quality: 0.5, specificity: 2, order: 1},
		{spec: "*/*", quality: 0.1, specificity: 1, order: 2},
		{spec: "application/xml", quality: 1, specificity: 3, order: 4},
		{spec: "application/pdf", quality: 1, specificity: 3, order: 5},
		{spec: "image/png", quality: 1, specificity: 3, order: 6},
		{spec: "image/jpeg", quality: 1, specificity: 3, order: 7},
		{spec: "image/*", quality: 1, specificity: 2, order: 8},
		{spec: "image/gif", quality: 1, specificity: 3, order: 9},
		{spec: "text/plain", quality: 1, specificity: 3, order: 10},
		{spec: "application/json", quality: 0.999, specificity: 3, params: headerParams{"a": []byte("1")}, order: 11},
		{spec: "application/json", quality: 0.999, specificity: 3, order: 3},
	}
	sortAcceptedTypes(acceptedTypes)
	require.Equal(t, []acceptedType{
		{spec: "text/html", quality: 1, specificity: 3, order: 0},
		{spec: "application/xml", quality: 1, specificity: 3, order: 4},
		{spec: "application/pdf", quality: 1, specificity: 3, order: 5},
		{spec: "image/png", quality: 1, specificity: 3, order: 6},
		{spec: "image/jpeg", quality: 1, specificity: 3, order: 7},
		{spec: "image/gif", quality: 1, specificity: 3, order: 9},
		{spec: "text/plain", quality: 1, specificity: 3, order: 10},
		{spec: "image/*", quality: 1, specificity: 2, order: 8},
		{spec: "application/json", quality: 0.999, specificity: 3, params: headerParams{"a": []byte("1")}, order: 11},
		{spec: "application/json", quality: 0.999, specificity: 3, order: 3},
		{spec: "text/*", quality: 0.5, specificity: 2, order: 1},
		{spec: "*/*", quality: 0.1, specificity: 1, order: 2},
	}, acceptedTypes)
}

// Every element ranks above all the ones before it: one swap per pair for an insertion sort.
func Test_Utils_SortAcceptedTypes_NotQuadratic(t *testing.T) {
	t.Parallel()
	const n = 1 << 17
	acceptedTypes := make([]acceptedType, n)
	for i := range acceptedTypes {
		acceptedTypes[i] = acceptedType{quality: float64(i), specificity: 4, order: i}
	}

	start := time.Now()
	sortAcceptedTypes(acceptedTypes)
	// Quadratic takes tens of seconds here, n log n a few milliseconds.
	require.Less(t, time.Since(start), 5*time.Second)

	for i := range acceptedTypes {
		require.Equal(t, n-1-i, acceptedTypes[i].order)
	}
}

func Test_Utils_SortAcceptedTypes_LongMatchesShort(t *testing.T) {
	t.Parallel()
	pattern := []acceptedType{
		{quality: 0.5, specificity: 3},
		{quality: 1, specificity: 1},
		{quality: 1, specificity: 3},
		{quality: 1, specificity: 3, params: headerParams{"a": []byte("1")}},
		{quality: 0, specificity: 4},
	}
	long := make([]acceptedType, 0, 4*maxInsertionSortedTypes)
	for i := range cap(long) {
		element := pattern[i%len(pattern)]
		element.order = i
		long = append(long, element)
	}
	sortAcceptedTypes(long)

	for i := 1; i < len(long); i++ {
		pair := []acceptedType{long[i], long[i-1]}
		sortAcceptedTypes(pair)
		require.Equal(t, long[i-1].order, pair[0].order)
	}
}

func Test_Utils_GetOffer_ManyRanges(t *testing.T) {
	t.Parallel()
	// 1500 wildcards, then 2000 empty members that sort in front of every one of them.
	header := []byte(strings.Repeat("*/*,", 1500) + strings.Repeat(",", 2000))

	require.Equal(t, "text/html", getOffer(header, acceptsOfferType, "text/html"))
	require.Equal(t, "application/json", getOffer(header, acceptsOfferType, "application/json", "text/html"))
}

func Benchmark_Utils_SortAcceptedTypes_Long(b *testing.B) {
	const n = 6000
	acceptedTypes := make([]acceptedType, n)
	b.ReportAllocs()
	for b.Loop() {
		// 2000 wildcards, then 4000 more specific members.
		for i := range acceptedTypes {
			specificity := 4
			if i < n/3 {
				specificity = 1
			}
			acceptedTypes[i] = acceptedType{quality: 1, specificity: specificity, order: i}
		}
		sortAcceptedTypes(acceptedTypes)
	}
	require.Equal(b, n/3, acceptedTypes[0].order)
	require.Equal(b, 0, acceptedTypes[n-n/3].order)
}

func Benchmark_Utils_SortAcceptedTypes_Sorted(b *testing.B) {
	acceptedTypes := make([]acceptedType, 3)
	b.ReportAllocs()
	for b.Loop() {
		acceptedTypes[0] = acceptedType{spec: "text/html", quality: 1, specificity: 1, order: 0}
		acceptedTypes[1] = acceptedType{spec: "text/*", quality: 0.5, specificity: 1, order: 1}
		acceptedTypes[2] = acceptedType{spec: "*/*", quality: 0.1, specificity: 1, order: 2}
		sortAcceptedTypes(acceptedTypes)
	}
	require.Equal(b, "text/html", acceptedTypes[0].spec)
	require.Equal(b, "text/*", acceptedTypes[1].spec)
	require.Equal(b, "*/*", acceptedTypes[2].spec)
}

func Benchmark_Utils_SortAcceptedTypes_Unsorted(b *testing.B) {
	acceptedTypes := make([]acceptedType, 11)
	b.ReportAllocs()
	for b.Loop() {
		acceptedTypes[0] = acceptedType{spec: "text/html", quality: 1, specificity: 3, order: 0}
		acceptedTypes[1] = acceptedType{spec: "text/*", quality: 0.5, specificity: 2, order: 1}
		acceptedTypes[2] = acceptedType{spec: "*/*", quality: 0.1, specificity: 1, order: 2}
		acceptedTypes[3] = acceptedType{spec: "application/json", quality: 0.999, specificity: 3, order: 3}
		acceptedTypes[4] = acceptedType{spec: "application/xml", quality: 1, specificity: 3, order: 4}
		acceptedTypes[5] = acceptedType{spec: "application/pdf", quality: 1, specificity: 3, order: 5}
		acceptedTypes[6] = acceptedType{spec: "image/png", quality: 1, specificity: 3, order: 6}
		acceptedTypes[7] = acceptedType{spec: "image/jpeg", quality: 1, specificity: 3, order: 7}
		acceptedTypes[8] = acceptedType{spec: "image/*", quality: 1, specificity: 2, order: 8}
		acceptedTypes[9] = acceptedType{spec: "image/gif", quality: 1, specificity: 3, order: 9}
		acceptedTypes[10] = acceptedType{spec: "text/plain", quality: 1, specificity: 3, order: 10}
		sortAcceptedTypes(acceptedTypes)
	}
	require.Equal(b, []acceptedType{
		{spec: "text/html", quality: 1, specificity: 3, order: 0},
		{spec: "application/xml", quality: 1, specificity: 3, order: 4},
		{spec: "application/pdf", quality: 1, specificity: 3, order: 5},
		{spec: "image/png", quality: 1, specificity: 3, order: 6},
		{spec: "image/jpeg", quality: 1, specificity: 3, order: 7},
		{spec: "image/gif", quality: 1, specificity: 3, order: 9},
		{spec: "text/plain", quality: 1, specificity: 3, order: 10},
		{spec: "image/*", quality: 1, specificity: 2, order: 8},
		{spec: "application/json", quality: 0.999, specificity: 3, order: 3},
		{spec: "text/*", quality: 0.5, specificity: 2, order: 1},
		{spec: "*/*", quality: 0.1, specificity: 1, order: 2},
	}, acceptedTypes)
}

func Test_Utils_getGroupPath(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name, prefix, path, want string
	}{
		{name: "root path after prefix", prefix: "/v1", path: "/", want: "/v1/"},
		{name: "prefix with trailing slash", prefix: "/v1/", path: "/", want: "/v1/"},
		{name: "root prefix and root path", prefix: "/", path: "/", want: "/"},
		{name: "nested prefix with trailing slash", prefix: "/v1/api/", path: "/", want: "/v1/api/"},
		{name: "relative path", prefix: "/v1/api", path: "group", want: "/v1/api/group"},
		{name: "empty path", prefix: "/v1/api", path: "", want: "/v1/api"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, getGroupPath(tc.prefix, tc.path))
		})
	}
}

func Benchmark_Utils_getGroupPath(b *testing.B) {
	var res string
	b.ReportAllocs()
	for b.Loop() {
		_ = getGroupPath("/v1/long/path/john/doe", "/why/this/name/is/so/awesome")
		_ = getGroupPath("/v1", "/")
		_ = getGroupPath("/v1", "/api")
		res = getGroupPath("/v1", "/api/register/:project")
	}
	require.Equal(b, "/v1/api/register/:project", res)
}

func Benchmark_Utils_Unescape(b *testing.B) {
	unescaped := ""
	dst := make([]byte, 0)
	b.ReportAllocs()
	for b.Loop() {
		source := "/cr%C3%A9er"
		pathBytes := utils.UnsafeBytes(source)
		pathBytes = fasthttp.AppendUnquotedArg(dst[:0], pathBytes)
		unescaped = utils.UnsafeString(pathBytes)
	}

	require.Equal(b, "/créer", unescaped)
}

func Test_Utils_Parse_Address(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		addr, host, port string
	}{
		{addr: "[::1]:3000", host: "[::1]", port: "3000"},
		{addr: "127.0.0.1:3000", host: "127.0.0.1", port: "3000"},
		{addr: "[::1]", host: "[::1]", port: ""},
		{addr: "2001:db8::1", host: "2001:db8::1", port: ""},
		{addr: "/path/to/unix/socket", host: "/path/to/unix/socket", port: ""},
		{addr: "127.0.0.1", host: "127.0.0.1", port: ""},
		{addr: "localhost:8080", host: "localhost", port: "8080"},
		{addr: "example.com", host: "example.com", port: ""},
		{addr: "[fe80::1%lo0]:1234", host: "[fe80::1%lo0]", port: "1234"},
		{addr: "[fe80::1%lo0]", host: "[fe80::1%lo0]", port: ""},
		{addr: ":9090", host: "", port: "9090"},
		{addr: " 127.0.0.1:8080 ", host: "127.0.0.1", port: "8080"},
		{addr: "", host: "", port: ""},
	}

	for _, c := range testCases {
		host, port := parseAddr(c.addr)
		require.Equal(t, c.host, host, "addr host: %q", c.addr)
		require.Equal(t, c.port, port, "addr port: %q", c.addr)
	}
}

func Test_Utils_TestConn_Deadline(t *testing.T) {
	t.Parallel()
	conn := &testConn{}
	require.NoError(t, conn.SetDeadline(time.Time{}))
	require.NoError(t, conn.SetReadDeadline(time.Time{}))
	require.NoError(t, conn.SetWriteDeadline(time.Time{}))
}

func Test_Utils_TestConn_ReadWrite(t *testing.T) {
	t.Parallel()
	conn := &testConn{}

	_, err := conn.r.Write([]byte("Request"))
	require.NoError(t, err)

	req := make([]byte, 7)
	_, err = conn.Read(req)
	require.NoError(t, err)
	require.Equal(t, []byte("Request"), req)

	_, err = conn.Write([]byte("Response"))
	require.NoError(t, err)

	res := make([]byte, 8)
	_, err = conn.w.Read(res)
	require.NoError(t, err)
	require.Equal(t, []byte("Response"), res)
}

func Test_Utils_TestConn_Closed_Write(t *testing.T) {
	t.Parallel()
	conn := &testConn{}

	_, err := conn.Write([]byte("Response 1\n"))
	require.NoError(t, err)

	// Close early, write should fail
	conn.Close() //nolint:errcheck // It is fine to ignore the error here
	_, err = conn.Write([]byte("Response 2\n"))
	require.ErrorIs(t, err, errTestConnClosed)

	res := make([]byte, 11)
	_, err = conn.w.Read(res)
	require.NoError(t, err)
	require.Equal(t, []byte("Response 1\n"), res)
}

func Test_Utils_IsNoCache(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		string
		bool
	}{
		{string: "public", bool: false},
		{string: "no-cache", bool: true},
		{string: "public, no-cache, max-age=30", bool: true},
		{string: "public,no-cache", bool: true},
		{string: "public,no-cacheX", bool: false},
		{string: "no-cache, public", bool: true},
		{string: "Xno-cache, public", bool: false},
		{string: "max-age=30, no-cache,public", bool: true},
		{string: "NO-CACHE", bool: true},
		{string: "public, NO-CACHE", bool: true},
		// RFC 9111 §5.2.2.4: no-cache with field-name argument
		{string: "no-cache=\"Set-Cookie\"", bool: true},
		{string: "public, no-cache=\"Set-Cookie, Set-Cookie2\"", bool: true},
		{string: "no-cache=Set-Cookie", bool: true},
		// Edge cases with spaces
		{string: "no-cache ,public", bool: true},
		{string: "public, no-cache =field", bool: true},
	}

	for _, c := range testCases {
		ok := isNoCache(c.string)
		require.Equal(t, c.bool, ok, "want %t, got isNoCache(%s)=%t", c.bool, c.string, ok)
	}
}

func Benchmark_Utils_IsNoCache(b *testing.B) {
	var ok bool
	b.ReportAllocs()
	for b.Loop() {
		_ = isNoCache("public")
		_ = isNoCache("no-cache")
		_ = isNoCache("public, no-cache, max-age=30")
		_ = isNoCache("public,no-cache")
		_ = isNoCache("no-cache, public")
		ok = isNoCache("max-age=30, no-cache,public")
	}
	require.True(b, ok)
}

// miscParseIntCase is a case of the integer parsing tests and benchmarks: value
// needs an integer of at least bits width.
type miscParseIntCase[N int64 | uint64] struct {
	value N
	bits  int
}

// miscParseInts, miscParseUints, miscParseFloats, miscParseBytes and
// miscParseBools are what the genericParseType tests parse, and its benchmarks
// would time.
var (
	miscParseInts = []miscParseIntCase[int64]{
		{value: 0, bits: 8},
		{value: 1, bits: 8},
		{value: 2, bits: 8},
		{value: 3, bits: 8},
		{value: 4, bits: 8},
		{value: -1, bits: 8},
		{value: math.MaxInt8, bits: 8},
		{value: math.MinInt8, bits: 8},
		{value: math.MaxInt16, bits: 16},
		{value: math.MinInt16, bits: 16},
		{value: math.MaxInt32, bits: 32},
		{value: math.MinInt32, bits: 32},
		{value: math.MaxInt64, bits: 64},
		{value: math.MinInt64, bits: 64},
	}
	miscParseUints = []miscParseIntCase[uint64]{
		{value: 0, bits: 8},
		{value: 1, bits: 8},
		{value: 2, bits: 8},
		{value: 3, bits: 8},
		{value: 4, bits: 8},
		{value: math.MaxUint8, bits: 8},
		{value: math.MaxUint16, bits: 16},
		{value: math.MaxUint16, bits: 16},
		{value: math.MaxUint32, bits: 32},
		{value: math.MaxUint64, bits: 64},
	}
	miscParseFloats = []struct {
		str   string
		value float64
	}{{value: 3.1415, str: "3.1415"}, {value: 1.234, str: "1.234"}, {value: 2, str: "2"}, {value: 3, str: "3"}}
	miscParseBytes = []struct {
		str   string
		err   error
		value []byte
	}{
		{value: []byte("alex"), str: "alex"},
		{value: []byte("32.23"), str: "32.23"},
		{value: []byte("john"), str: "john"},
		{value: []byte(nil), str: "", err: errParsedEmptyBytes},
	}
	miscParseBools = []struct {
		str   string
		value bool
	}{
		{str: "True", value: true},
		{str: "False", value: false},
		{str: "true", value: true},
		{str: "false", value: false},
	}
)

func Test_GenericParseTypeInts(t *testing.T) {
	t.Parallel()
	testGenericTypeInteger[int8](t, "test_genericParseTypeInt8s", miscParseInts)
	testGenericTypeInteger[int16](t, "test_genericParseTypeInt16s", miscParseInts)
	testGenericTypeInteger[int32](t, "test_genericParseTypeInt32s", miscParseInts)
	testGenericTypeInteger[int64](t, "test_genericParseTypeInt64s", miscParseInts)
	testGenericTypeInteger[int](t, "test_genericParseTypeInts", miscParseInts)
}

// testGenericTypeInteger parses the decimal form of every value in cases as V:
// a value that fits in V must parse to itself, any other one must be rejected.
func testGenericTypeInteger[V GenericTypeInteger, N int64 | uint64](
	t *testing.T, name string, cases []miscParseIntCase[N],
) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		for _, tc := range cases {
			v, err := genericParseType[V](fmt.Sprint(tc.value))
			if tc.bits <= int(unsafe.Sizeof(V(0)))*8 {
				require.NoError(t, err, "value %d", tc.value)
				require.Equal(t, V(tc.value), v, "value %d", tc.value)
			} else {
				require.ErrorIs(t, err, errParseValue, "value %d", tc.value)
			}
		}
		testGenericParseError[V](t)
	})
}

func Test_GenericParseTypeUints(t *testing.T) {
	t.Parallel()
	testGenericTypeInteger[uint8](t, "test_genericParseTypeUint8s", miscParseUints)
	testGenericTypeInteger[uint16](t, "test_genericParseTypeUint16s", miscParseUints)
	testGenericTypeInteger[uint32](t, "test_genericParseTypeUint32s", miscParseUints)
	testGenericTypeInteger[uint64](t, "test_genericParseTypeUint64s", miscParseUints)
	testGenericTypeInteger[uint](t, "test_genericParseTypeUints", miscParseUints)
}

func Test_GenericParseTypeFloats(t *testing.T) {
	t.Parallel()
	t.Run("test_genericParseTypeFloat32s", func(t *testing.T) {
		t.Parallel()
		for _, test := range miscParseFloats {
			v, err := genericParseType[float32](test.str)
			require.NoError(t, err)
			require.InEpsilon(t, float32(test.value), v, epsilon)
		}
		testGenericParseError[float32](t)
	})

	t.Run("test_genericParseTypeFloat64s", func(t *testing.T) {
		t.Parallel()
		for _, test := range miscParseFloats {
			v, err := genericParseType[float64](test.str)
			require.NoError(t, err)
			require.InEpsilon(t, test.value, v, epsilon)
		}
		testGenericParseError[float64](t)
	})
}

func Test_GenericParseTypeBytes(t *testing.T) {
	t.Parallel()
	t.Run("test_genericParseTypeBytes", func(t *testing.T) {
		t.Parallel()
		for _, test := range miscParseBytes {
			v, err := genericParseType[[]byte](test.str)
			if test.err == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, test.err)
			}
			require.Equal(t, test.value, v)
		}
	})
}

func Test_GenericParseTypeString(t *testing.T) {
	t.Parallel()
	tests := []string{"john", "doe", "hello", "fiber"}

	for _, test := range tests {
		t.Run("test_genericParseTypeString", func(t *testing.T) {
			t.Parallel()
			v, err := genericParseType[string](test)
			require.NoError(t, err)
			require.Equal(t, test, v)
		})
	}
}

func Test_GenericParseTypeBoolean(t *testing.T) {
	t.Parallel()
	t.Run("test_genericParseTypeBoolean", func(t *testing.T) {
		t.Parallel()
		for _, test := range miscParseBools {
			v, err := genericParseType[bool](test.str)
			require.NoError(t, err)
			if test.value {
				require.True(t, v)
			} else {
				require.False(t, v)
			}
		}
		testGenericParseError[bool](t)
	})
}

func testGenericParseError[V GenericType](t *testing.T) {
	t.Helper()
	var expected V
	v, err := genericParseType[V]("invalid-string")
	require.Error(t, err)
	require.Equal(t, expected, v)
}

func Benchmark_GenericParseTypeInts(b *testing.B) {
	b.Skip("Skipped: too fast to compare reliably (results in sub-ns range are unstable)")
	for _, test := range miscParseInts {
		benchGenericParseTypeInt[int8](b, "bench_genericParseTypeInt8s", test)
		benchGenericParseTypeInt[int16](b, "bench_genericParseTypeInt16s", test)
		benchGenericParseTypeInt[int32](b, "bench_genericParseTypeInt32s", test)
		benchGenericParseTypeInt[int64](b, "bench_genericParseTypeInt64s", test)
		benchGenericParseTypeInt[int](b, "bench_genericParseTypeInts", test)
	}
}

// conventions:ignore bench-loop: helper inherited from main; new benchmarks keep their loop
func benchGenericParseTypeInt[V GenericTypeInteger](b *testing.B, name string, test miscParseIntCase[int64]) {
	b.Helper()
	b.Run(name, func(t *testing.B) {
		var v V
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				v, err = genericParseType[V](strconv.FormatInt(test.value, 10))
			}
		})
		if test.bits <= int(unsafe.Sizeof(V(0)))*8 {
			require.NoError(t, err)
			require.Equal(t, V(test.value), v)
		} else {
			require.ErrorIs(t, err, errParseValue)
		}
	})
}

func Benchmark_GenericParseTypeUints(b *testing.B) {
	b.Skip("Skipped: too fast to compare reliably (results in sub-ns range are unstable)")
	for _, test := range miscParseUints {
		benchGenericParseTypeUInt[uint8](b, "benchmark_genericParseTypeUint8s", test)
		benchGenericParseTypeUInt[uint16](b, "benchmark_genericParseTypeUint16s", test)
		benchGenericParseTypeUInt[uint32](b, "benchmark_genericParseTypeUint32s", test)
		benchGenericParseTypeUInt[uint64](b, "benchmark_genericParseTypeUint64s", test)
		benchGenericParseTypeUInt[uint](b, "benchmark_genericParseTypeUints", test)
	}
}

// conventions:ignore bench-loop: helper inherited from main; new benchmarks keep their loop
func benchGenericParseTypeUInt[V GenericTypeInteger](b *testing.B, name string, test miscParseIntCase[uint64]) {
	b.Helper()
	b.Run(name, func(t *testing.B) {
		var v V
		var err error
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				v, err = genericParseType[V](strconv.FormatUint(test.value, 10))
			}
		})
		if test.bits <= int(unsafe.Sizeof(V(0)))*8 {
			require.NoError(t, err)
			require.Equal(t, V(test.value), v)
		} else {
			require.ErrorIs(t, err, errParseValue)
		}
	})
}

func Benchmark_GenericParseTypeFloats(b *testing.B) {
	b.Skip("Skipped: too fast to compare reliably (results in sub-ns range are unstable)")
	for _, test := range miscParseFloats {
		b.Run("benchmark_genericParseTypeFloat32s", func(b *testing.B) {
			var v float32
			var err error
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					v, err = genericParseType[float32](test.str)
				}
			})
			require.NoError(b, err)
			require.InEpsilon(b, float32(test.value), v, epsilon)
		})
	}

	for _, test := range miscParseFloats {
		b.Run("benchmark_genericParseTypeFloat64s", func(b *testing.B) {
			var v float64
			var err error
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					v, err = genericParseType[float64](test.str)
				}
			})
			require.NoError(b, err)
			require.InEpsilon(b, test.value, v, epsilon)
		})
	}
}

func Benchmark_GenericParseTypeBytes(b *testing.B) {
	b.Skip("Skipped: too fast to compare reliably (results in sub-ns range are unstable)")
	for _, test := range miscParseBytes {
		b.Run("benchmark_genericParseTypeBytes", func(b *testing.B) {
			var v []byte
			var err error
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					v, err = genericParseType[[]byte](test.str)
				}
			})
			if test.err == nil {
				require.NoError(b, err)
			} else {
				require.ErrorIs(b, err, test.err)
			}
			require.Equal(b, test.value, v)
		})
	}
}

func Benchmark_GenericParseTypeString(b *testing.B) {
	b.Skip("Skipped: too fast to compare reliably (results in sub-ns range are unstable)")
	tests := []string{"john", "doe", "hello", "fiber"}

	for _, test := range tests {
		b.Run("benchmark_genericParseTypeString", func(b *testing.B) {
			var v string
			var err error
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					v, err = genericParseType[string](test)
				}
			})
			require.NoError(b, err)
			require.Equal(b, test, v)
		})
	}
}

func Benchmark_GenericParseTypeBoolean(b *testing.B) {
	b.Skip("Skipped: too fast to compare reliably (results in sub-ns range are unstable)")
	for _, test := range miscParseBools {
		b.Run("benchmark_genericParseTypeBoolean", func(b *testing.B) {
			var v bool
			var err error
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					v, err = genericParseType[bool](test.str)
				}
			})
			require.NoError(b, err)
			if test.value {
				require.True(b, v)
			} else {
				require.False(b, v)
			}
		})
	}
}

func Test_ParamsMatch_InvalidEscape(t *testing.T) {
	t.Parallel()
	match := paramsMatch(headerParams{"foo": []byte("bar")}, `;foo="bar\\`)
	require.False(t, match)
}

func Test_IsEtagStale(t *testing.T) {
	t.Parallel()
	app := New()

	// Invalid/unquoted tags are considered a mismatch, so it's stale
	require.True(t, app.isEtagStale(`"a"`, []byte("b")))
	require.True(t, app.isEtagStale(`"a"`, []byte("a")))

	// Matching tags, not stale
	require.False(t, app.isEtagStale(`"a"`, []byte(`"a"`)))
	require.False(t, app.isEtagStale(`W/"a"`, []byte(`"a"`)))

	// List of tags, not stale
	require.False(t, app.isEtagStale(`"c"`, []byte(`"a", "b", "c"`)))
	require.False(t, app.isEtagStale(`W/"c"`, []byte(`"a", "b", "c"`)))
	require.False(t, app.isEtagStale(`"c"`, []byte(`"a", "b", W/"c"`)))
	require.False(t, app.isEtagStale(`"c"`, []byte(`"c", "b", "a"`)))
	require.False(t, app.isEtagStale(`"c"`, []byte(`  "a",   "c"   , "b"  `)))

	// List of tags, stale
	require.True(t, app.isEtagStale(`"d"`, []byte(`"a", "b", "c"`)))
	require.True(t, app.isEtagStale(`W/"d"`, []byte(`"a", "b", "c"`)))

	// Wildcard
	require.False(t, app.isEtagStale(`"a"`, []byte("*")))
	require.False(t, app.isEtagStale(`"a"`, []byte(" *   ")))
	require.False(t, app.isEtagStale(`W/"a"`, []byte("*")))

	// Empty case
	require.True(t, app.isEtagStale(`"a"`, []byte("")))
	require.True(t, app.isEtagStale(`"a"`, []byte("   ")))

	// Weak vs. weak
	require.False(t, app.isEtagStale(`W/"a"`, []byte(`W/"a"`)))

	// etagc permits "," inside the opaque-tag (RFC 9110 §8.8.3): a quoted
	// comma is part of the tag, not a list separator.
	require.False(t, app.isEtagStale(`"v1,v2"`, []byte(`"v1,v2"`)))
	require.False(t, app.isEtagStale(`"v1,v2"`, []byte(`"a", "v1,v2", "b"`)))
	require.False(t, app.isEtagStale(`W/"v1,v2"`, []byte(`"v1,v2"`)))
	require.True(t, app.isEtagStale(`"v1"`, []byte(`"v1,v2"`)))
	require.True(t, app.isEtagStale(`"v2"`, []byte(`"v1,v2"`)))
}

func TestStoreInContext(t *testing.T) {
	t.Parallel()
	app := New(Config{PassLocalsToContext: true})
	raw := &fasthttp.RequestCtx{}
	c := apptest.CtxFor(t, app, raw)

	StoreInContext(c, "key", "value")

	localValue, ok := c.Locals("key").(string)
	require.True(t, ok)
	require.Equal(t, "value", localValue)

	contextValue, ok := c.Context().Value("key").(string)
	require.True(t, ok)
	require.Equal(t, "value", contextValue)
}

func TestValueFromContext(t *testing.T) {
	t.Parallel()
	// The lookups behave the same for the default and for a custom context.
	for _, tc := range []struct {
		newApp func() *App
		name   string
	}{
		{name: "fiber.Ctx", newApp: func() *App { return New() }},
		{name: "fiber.CustomCtx", newApp: func() *App {
			return NewWithCustomCtx(func(app *App) CustomCtx { return &customCtx{DefaultCtx: *NewDefaultCtx(app)} })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := tc.newApp()
			raw := &fasthttp.RequestCtx{}
			c := apptest.CtxFor(t, app, raw)

			c.Locals("key", "value")

			value, ok := ValueFromContext[string](c, "key")
			require.True(t, ok)
			require.Equal(t, "value", value)
		})

		t.Run("released "+tc.name, func(t *testing.T) {
			t.Parallel()
			app := tc.newApp()
			raw := &fasthttp.RequestCtx{}
			c := app.AcquireCtx(raw)

			c.Locals("key", "value")

			valueBeforeRelease, okBeforeRelease := ValueFromContext[string](c, "key")
			require.True(t, okBeforeRelease)
			require.Equal(t, "value", valueBeforeRelease)

			app.ReleaseCtx(c)

			require.NotPanics(t, func() {
				valueAfterRelease, okAfterRelease := ValueFromContext[string](c, "key")
				require.False(t, okAfterRelease)
				require.Empty(t, valueAfterRelease)
			})
		})
	}

	t.Run("fasthttp request ctx", func(t *testing.T) {
		t.Parallel()
		raw := &fasthttp.RequestCtx{}
		raw.SetUserValue("key", "value")

		value, ok := ValueFromContext[string](raw, "key")
		require.True(t, ok)
		require.Equal(t, "value", value)
	})

	t.Run("context.Context", func(t *testing.T) {
		t.Parallel()
		type testContextKey struct{}

		ctx := context.WithValue(context.Background(), testContextKey{}, "value")

		value, ok := ValueFromContext[string](ctx, testContextKey{})
		require.True(t, ok)
		require.Equal(t, "value", value)
	})

	t.Run("unsupported ctx", func(t *testing.T) {
		t.Parallel()
		value, ok := ValueFromContext[string](42, "key")
		require.False(t, ok)
		require.Empty(t, value)
	})
}

func Test_IsMethodSafe_Idempotent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		is       func(string) bool
		name     string
		yes, not []string
	}{
		{
			name: "IsMethodSafe", is: IsMethodSafe,
			yes: []string{MethodGet, MethodHead, MethodOptions, MethodTrace, MethodQuery},
			not: []string{MethodPost, MethodPut, MethodPatch, MethodDelete, MethodConnect},
		},
		{
			name: "IsMethodIdempotent", is: IsMethodIdempotent,
			yes: []string{MethodGet, MethodHead, MethodOptions, MethodTrace, MethodQuery, MethodPut, MethodDelete},
			not: []string{MethodPost, MethodPatch, MethodConnect},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, m := range tc.yes {
				require.True(t, tc.is(m), "%s should be accepted", m)
			}
			for _, m := range tc.not {
				require.False(t, tc.is(m), "%s should not be accepted", m)
			}
		})
	}
}

// localsRecordingCtx is a custom context whose Locals is observable, so the
// interface fallback in setLocal can be told apart from the concrete fast path.
type localsRecordingCtx struct {
	DefaultCtx
	calls int
}

// Locals counts each call before forwarding it, so a test can tell whether
// the override ran or the concrete fast path bypassed it.
func (c *localsRecordingCtx) Locals(key any, value ...any) any {
	c.calls++
	return c.DefaultCtx.Locals(key, value...)
}

// Test_setLocal_UsesCustomCtxLocals pins the fallback in setLocal. The concrete
// *DefaultCtx path exists only to keep the variadic slice off the heap; it must
// never take precedence over a custom context's own Locals, which a type
// embedding DefaultCtx is entitled to override.
func Test_setLocal_UsesCustomCtxLocals(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, NewWithCustomCtx(func(app *App) CustomCtx {
		return &localsRecordingCtx{DefaultCtx: *NewDefaultCtx(app)}
	}))

	custom, ok := c.(*localsRecordingCtx)
	require.True(t, ok, "the app must hand out the custom context")

	require.Equal(t, "v", setLocal(c, "k", "v"), "setLocal returns what Locals returned")
	require.Equal(t, 1, custom.calls, "the overridden Locals must be the one that ran")
	require.Equal(t, "v", c.Locals("k"), "and the value must actually be stored")
}

// Test_setLocal_UsesDefaultCtxDirectly is the other half: the default context
// takes the concrete path, and the value is stored and returned unchanged.
func Test_setLocal_UsesDefaultCtxDirectly(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	_, isDefault := c.(*DefaultCtx)
	require.True(t, isDefault, "the default app must hand out *DefaultCtx")

	require.Equal(t, 42, setLocal(c, "n", 42))
	require.Equal(t, 42, c.Locals("n"))
}

// Test_appendCopyLowerASCII pins appendCopyLowerASCII against the copy, the
// fold and the slash count it fuses, at every length across the SWAR word
// boundaries, on fresh and on reused destinations.
func Test_appendCopyLowerASCII(t *testing.T) {
	t.Parallel()
	cases := []string{
		"", "/", "A", "/abc", "/AbC", "/ABCDEFG", "/ABCDEFGH/XYZ",
		"/API/V1/UsersAndGroups", "/a1-B2_c3{~}", "/CAF\xC3\xA9/\xC3\x89",
		"/repos/GoFiber/Fiber/issues/4662/comments", "////////", "/////////",
	}
	// Every length across the word boundaries, so the main loop, the
	// overlapping tail word and the byte-wise path are all covered, with the
	// slashes at every offset of the tail word.
	for n := range 40 {
		cases = append(cases, strings.Repeat("aB/", n), "/"+strings.Repeat("aB/", n), "/x"+strings.Repeat("aB/", n))
	}

	for _, in := range cases {
		t.Run(strconv.Itoa(len(in)), func(t *testing.T) {
			t.Parallel()
			// It must agree with the two operations it replaces.
			wantPath := append([]byte(nil), in...)
			wantLower := appendLowerASCII(nil, wantPath)

			// Fresh destinations (forces growth) and reused oversized ones
			// (exercises the cap(dst) >= n path).
			wantSlashes := strings.Count(in, "/")
			gotPath, gotLower, gotSlashes := appendCopyLowerASCII(nil, nil, in)
			require.Equal(t, string(wantPath), string(gotPath))
			require.Equal(t, string(wantLower), string(gotLower))
			require.Equal(t, wantSlashes, gotSlashes)

			reusedPath := make([]byte, 0, 128)
			reusedLower := make([]byte, 0, 128)
			gotPath, gotLower, gotSlashes = appendCopyLowerASCII(reusedPath, reusedLower, in)
			require.Equal(t, string(wantPath), string(gotPath))
			require.Equal(t, string(wantLower), string(gotLower))
			require.Equal(t, wantSlashes, gotSlashes)
			if in != "" {
				require.Equal(t, 128, cap(gotPath), "reused buffer must not be reallocated")
				require.Equal(t, 128, cap(gotLower), "reused buffer must not be reallocated")
			}
		})
	}
}

func Test_appendCopyLowerASCII_AliasedSubstring(t *testing.T) {
	t.Parallel()
	path := []byte("/api/bar/fooX")
	src := utils.UnsafeString(path[4:])
	gotPath, gotLower, gotSlashes := appendCopyLowerASCII(path[:0], nil, src)

	require.Equal(t, "/bar/fooX", string(gotPath))
	require.Equal(t, "/bar/foox", string(gotLower))
	require.Equal(t, 2, gotSlashes)
}

func Test_appendLowerASCII(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{"empty", "", ""},
		{"short lower", "/abc", "/abc"},
		{"short mixed", "/AbC", "/abc"},
		{"exactly one word", "/ABCDEFG", "/abcdefg"},
		{"multi word", "/API/V1/UsersAndGroups", "/api/v1/usersandgroups"},
		{"word plus tail", "/ABCDEFGH/XYZ", "/abcdefgh/xyz"},
		{"non-letters unchanged", "/a1-B2_c3{~}", "/a1-b2_c3{~}"},
		{"non-ascii passthrough", "/CAFé\xC3\xA9/É", "/caf\xC3\xA9\xC3\xA9/\xC3\x89"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Fresh destination (forces growth) and reused oversized
			// destination (exercises the cap(dst) >= n path).
			require.Equal(t, tc.out, string(appendLowerASCII(nil, []byte(tc.in))))
			reused := make([]byte, 0, 128)
			got := appendLowerASCII(reused, []byte(tc.in))
			require.Equal(t, tc.out, string(got))
			if tc.in != "" {
				require.Equal(t, 128, cap(got), "reused buffer must not be reallocated")
			}
		})
	}
}

// Test_bindMediaType pins the media type Bind dispatches on: the canonical
// spellings answer directly, every other spelling is folded, vendor suffixes
// resolve, and parameters are dropped, with and without the fast path.
func Test_bindMediaType(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		ct, want string
	}{
		{ct: "application/json", want: MIMEApplicationJSON},
		{ct: "application/json; charset=utf-8", want: MIMEApplicationJSON},
		{ct: "application/json;charset=utf-8", want: MIMEApplicationJSON},
		{ct: "application/json ", want: MIMEApplicationJSON},
		{ct: "Application/JSON", want: MIMEApplicationJSON},
		{ct: "APPLICATION/JSON; CHARSET=UTF-8", want: MIMEApplicationJSON},
		{ct: "application/vnd.api+json", want: MIMEApplicationJSON},
		{ct: "application/jsonx", want: "application/jsonx"},
		{ct: "application/x-www-form-urlencoded", want: MIMEApplicationForm},
		{ct: "application/x-www-form-urlencoded; charset=UTF-8", want: MIMEApplicationForm},
		{ct: "Application/X-WWW-Form-Urlencoded", want: MIMEApplicationForm},
		{ct: "multipart/form-data; boundary=X", want: MIMEMultipartForm},
		{ct: "text/xml", want: MIMETextXML},
		{ct: "", want: ""},
	} {
		h := &fasthttp.RequestHeader{}
		h.SetContentType(tc.ct)
		require.Equal(t, tc.want, bindMediaType(h), "content type %q", tc.ct)
	}

	// A parameterized form type still takes the folding walk, which lowercases
	// the parameter names in place for fasthttp's own parser.
	h := &fasthttp.RequestHeader{}
	h.SetContentType("application/x-www-form-urlencoded; Charset=UTF-8")
	require.Equal(t, MIMEApplicationForm, bindMediaType(h))
	require.Equal(t, "application/x-www-form-urlencoded; charset=UTF-8", string(h.ContentType()))
}

func Benchmark_bindMediaType(b *testing.B) {
	for _, ct := range []string{
		"application/json", "application/json; charset=utf-8", "Application/JSON", "application/x-www-form-urlencoded",
	} {
		b.Run(ct, func(b *testing.B) {
			h := &fasthttp.RequestHeader{}
			h.SetContentType(ct)
			b.ReportAllocs()
			var got string
			for b.Loop() {
				got = bindMediaType(h)
			}
			require.NotEmpty(b, got)
		})
	}
}

func Test_sameFunc(t *testing.T) {
	t.Parallel()
	newCallback := func(limit int) func(*fasthttp.RequestHeader) fasthttp.RequestConfig {
		return func(*fasthttp.RequestHeader) fasthttp.RequestConfig {
			return fasthttp.RequestConfig{MaxRequestBodySize: limit}
		}
	}
	a, b := newCallback(1), newCallback(1)

	require.True(t, sameFunc(a, a))
	require.False(t, sameFunc(a, b), "closures of one literal are different values")
	require.False(t, sameFunc(a, nil))
	require.False(t, sameFunc(nil, a))
	require.False(t, sameFunc(nil, nil))
}

func Test_hasHeaderField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		raw   string
		field string // HeaderContentLength when empty
		want  bool
	}{
		{name: "empty", raw: "", want: false},
		{name: "no fields", raw: "\r\n", want: false},
		{name: "only field", raw: "Content-Length: 4\r\n\r\n", want: true},
		{name: "among others", raw: "Host: x\r\nContent-Length: 4\r\nTransfer-Encoding: chunked\r\n\r\n", want: true},
		{name: "last field", raw: "Host: x\r\nContent-Length: 4", want: true},
		{name: "lower case", raw: "Host: x\r\ncontent-length: 4\r\n\r\n", want: true},
		{name: "upper case", raw: "Host: x\r\nCONTENT-LENGTH: 4\r\n\r\n", want: true},
		{name: "bare LF line ends", raw: "Host: x\nContent-Length: 4\n\n", want: true},
		{name: "no space after the colon", raw: "Host: x\r\nContent-Length:4\r\n\r\n", want: true},
		{name: "empty value", raw: "Host: x\r\nContent-Length:\r\n\r\n", want: true},
		{name: "longer name", raw: "Host: x\r\nContent-Length-Extra: 4\r\n\r\n", want: false},
		{name: "longer name ending in it", raw: "Host: x\r\nX-Content-Length: 4\r\n\r\n", want: false},
		{name: "shorter name", raw: "Host: x\r\nContent-Len: 4\r\n\r\n", want: false},
		{name: "name inside a value", raw: "Host: x\r\nX-Note: Content-Length: 4\r\n\r\n", want: false},
		{name: "folded continuation line", raw: "Host: x\r\nX-Note: a\r\n Content-Length: 4\r\n\r\n", want: false},
		{name: "name without a colon", raw: "Host: x\r\nContent-Length\r\n\r\n", want: false},
		{name: "name only", raw: "Content-Length", want: false},
		{name: "Transfer-Encoding", raw: "Host: x\r\nTransfer-Encoding: identity\r\n\r\n", field: HeaderTransferEncoding, want: true},
		{name: "Transfer-Encoding lower case", raw: "Host: x\r\ntransfer-encoding: identity\r\n\r\n", field: HeaderTransferEncoding, want: true},
		{name: "Transfer-Encoding absent", raw: "Host: x\r\nContent-Length: 4\r\n\r\n", field: HeaderTransferEncoding, want: false},
		{name: "Transfer-Encoding longer name", raw: "Host: x\r\nX-Transfer-Encoding: chunked\r\n\r\n", field: HeaderTransferEncoding, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			field := tc.field
			if field == "" {
				field = HeaderContentLength
			}
			require.Equal(t, tc.want, hasHeaderField([]byte(tc.raw), field))
		})
	}
}

// fasthttp reports -1 for a chunked request, the Content-Length value for one it
// frames by Content-Length ("identity" included) and -2 for neither; the field
// that is not reflected in that is looked up in the header block.
func Test_bothFramingFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "neither", raw: "GET / HTTP/1.1\r\nHost: x\r\n\r\n", want: false},
		{name: "Content-Length only", raw: "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 4\r\n\r\nabcd", want: false},
		{name: "chunked only", raw: "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n", want: false},
		{name: "identity only", raw: "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: identity\r\n\r\n", want: false},
		{name: "chunked after Content-Length", raw: "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 4\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n", want: true},
		{name: "chunked before Content-Length", raw: "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\nContent-Length: 4\r\n\r\n0\r\n\r\n", want: true},
		{name: "identity after Content-Length", raw: "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 4\r\nTransfer-Encoding: identity\r\n\r\nabcd", want: true},
		{name: "identity before Content-Length", raw: "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: identity\r\nContent-Length: 4\r\n\r\nabcd", want: true},
		{name: "identity with an empty body", raw: "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 0\r\nTransfer-Encoding: identity\r\n\r\n", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var h fasthttp.RequestHeader
			require.NoError(t, h.Read(bufio.NewReader(strings.NewReader(tc.raw))))
			require.Equal(t, tc.want, bothFramingFields(&h))
		})
	}
}

func Benchmark_hasHeaderField(b *testing.B) {
	raw := []byte("Host: example.com\r\nUser-Agent: Go-http-client/1.1\r\nAccept: */*\r\nAccept-Encoding: gzip\r\nTransfer-Encoding: chunked\r\nContent-Length: 4\r\n\r\n")

	b.ReportAllocs()
	for b.Loop() {
		if !hasHeaderField(raw, HeaderContentLength) {
			b.Fatal("Content-Length field not found")
		}
	}
}

func Benchmark_bothFramingFields(b *testing.B) {
	const fields = "Host: example.com\r\nUser-Agent: Go-http-client/1.1\r\nAccept: */*\r\nAccept-Encoding: gzip\r\n"
	for _, tc := range []struct{ name, raw string }{
		{"no body length", "GET / HTTP/1.1\r\n" + fields + "\r\n"},
		{"Content-Length", "POST / HTTP/1.1\r\n" + fields + "Content-Length: 4\r\n\r\nabcd"},
		{"chunked", "POST / HTTP/1.1\r\n" + fields + "Transfer-Encoding: chunked\r\n\r\n0\r\n\r\n"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var h fasthttp.RequestHeader
			require.NoError(b, h.Read(bufio.NewReader(strings.NewReader(tc.raw))))

			b.ReportAllocs()
			for b.Loop() {
				if bothFramingFields(&h) {
					b.Fatal("a single framing field was taken for both")
				}
			}
		})
	}
}
