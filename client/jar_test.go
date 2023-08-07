package client

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// tNow is the synthetic current time used as now during testing.
var tNow = time.Date(2013, 1, 1, 12, 0, 0, 0, time.UTC)

var hasDotSuffixTests = [...]struct {
	s, suffix string
}{
	{"", ""},
	{"", "."},
	{"", "x"},
	{".", ""},
	{".", "."},
	{".", ".."},
	{".", "x"},
	{".", "x."},
	{".", ".x"},
	{".", ".x."},
	{"x", ""},
	{"x", "."},
	{"x", ".."},
	{"x", "x"},
	{"x", "x."},
	{"x", ".x"},
	{"x", ".x."},
	{".x", ""},
	{".x", "."},
	{".x", ".."},
	{".x", "x"},
	{".x", "x."},
	{".x", ".x"},
	{".x", ".x."},
	{"x.", ""},
	{"x.", "."},
	{"x.", ".."},
	{"x.", "x"},
	{"x.", "x."},
	{"x.", ".x"},
	{"x.", ".x."},
	{"com", ""},
	{"com", "m"},
	{"com", "om"},
	{"com", "com"},
	{"com", ".com"},
	{"com", "x.com"},
	{"com", "xcom"},
	{"com", "xorg"},
	{"com", "org"},
	{"com", "rg"},
	{"foo.com", ""},
	{"foo.com", "m"},
	{"foo.com", "om"},
	{"foo.com", "com"},
	{"foo.com", ".com"},
	{"foo.com", "o.com"},
	{"foo.com", "oo.com"},
	{"foo.com", "foo.com"},
	{"foo.com", ".foo.com"},
	{"foo.com", "x.foo.com"},
	{"foo.com", "xfoo.com"},
	{"foo.com", "xfoo.org"},
	{"foo.com", "foo.org"},
	{"foo.com", "oo.org"},
	{"foo.com", "o.org"},
	{"foo.com", ".org"},
	{"foo.com", "org"},
	{"foo.com", "rg"},
}

func TestHasDotSuffix(t *testing.T) {
	for _, tc := range hasDotSuffixTests {
		got := hasDotSuffix([]byte(tc.s), []byte(tc.suffix))

		want := strings.HasSuffix(tc.s, "."+tc.suffix)
		require.Equal(t, want, got)
	}
}

var canonicalHostTests = map[string]string{
	"www.example.com":         "www.example.com",
	"WWW.EXAMPLE.COM":         "www.example.com",
	"wWw.eXAmple.CoM":         "www.example.com",
	"www.example.com:80":      "www.example.com",
	"192.168.0.10":            "192.168.0.10",
	"192.168.0.5:8080":        "192.168.0.5",
	"2001:4860:0:2001::68":    "2001:4860:0:2001::68",
	"[2001:4860:0:::68]:8080": "2001:4860:0:::68",
	"www.bücher.de":           "www.xn--bcher-kva.de",
	"www.example.com.":        "www.example.com",
	// TODO: Fix canonicalHost so that all of the following malformed
	// domain names trigger an error. (This list is not exhaustive, e.g.
	// malformed internationalized domain names are missing.)
	".":                       "",
	"..":                      ".",
	"...":                     "..",
	".net":                    ".net",
	".net.":                   ".net",
	"a..":                     "a.",
	"b.a..":                   "b.a.",
	"weird.stuff...":          "weird.stuff..",
	"[bad.unmatched.bracket:": "error",
}

var jarKeyTests = map[string]string{
	"foo.www.example.com": "example.com",
	"www.example.com":     "example.com",
	"example.com":         "example.com",
	"com":                 "com",
	"foo.www.bbc.co.uk":   "co.uk",
	"www.bbc.co.uk":       "co.uk",
	"bbc.co.uk":           "co.uk",
	"co.uk":               "co.uk",
	"uk":                  "uk",
	"192.168.0.5":         "192.168.0.5",
	// The following are actual outputs of canonicalHost for
	// malformed inputs to canonicalHost.
	"":              "",
	".":             ".",
	"..":            "..",
	".net":          ".net",
	"a.":            "a.",
	"b.a.":          "a.",
	"weird.stuff..": "stuff..",
}

func TestJarKey(t *testing.T) {
	for host, want := range jarKeyTests {
		got := jarKey([]byte(host))

		require.Equal(t, want, got)
	}
}

// expiresIn creates an expires attribute delta seconds from tNow.
func expiresIn(delta int) string {
	t := tNow.Add(time.Duration(delta) * time.Second)
	return "expires=" + t.Format(time.RFC1123)
}

// mustParseURL parses s to an URL and panics on error.
func mustParseURL(s string) *fasthttp.URI {
	u := fasthttp.AcquireURI()
	err := u.Parse(nil, utils.UnsafeBytes(s))

	if err != nil || utils.UnsafeString(u.Scheme()) == "" || utils.UnsafeString(u.Hash()) == "" {
		panic(fmt.Sprintf("Unable to parse URL %s.", s))
	}
	return u
}

// jarTest encapsulates the following actions on a jar:
//  1. Perform SetCookies with fromURL and the cookies from setCookies.
//     (Done at time tNow + 0 ms.)
//  2. Check that the entries in the jar matches content.
//     (Done at time tNow + 1001 ms.)
//  3. For each query in tests: Check that Cookies with toURL yields the
//     cookies in want.
//     (Query n done at tNow + (n+2)*1001 ms.)
type jarTest struct {
	description string   // The description of what this test is supposed to test
	fromURL     string   // The full URL of the request from which Set-Cookie headers where received
	setCookies  []string // All the cookies received from fromURL
	content     string   // The whole (non-expired) content of the jar
	queries     []query  // Queries to test the Jar.Cookies method
}

// query contains one test of the cookies returned from Jar.Cookies.
type query struct {
	toURL string // the URL in the Cookies call
	want  string // the expected list of cookies (order matters)
}

// run runs the jarTest.
func (test jarTest) run(t *testing.T, jar *jar) {
	// now := tNow

	// // Populate jar with cookies.
	// setCookies := make([]*fasthttp.Cookie, len(test.setCookies))
	// for i, cs := range test.setCookies {
	// 	resp := fasthttp.AcquireResponse()
	// 	cookies := (&http.Response{Header: http.Header{"Set-Cookie": {cs}}}).Cookies()
	// 	if len(cookies) != 1 {
	// 		panic(fmt.Sprintf("Wrong cookie line %q: %#v", cs, cookies))
	// 	}
	// 	setCookies[i] = cookies[0]
	// }
	// jar.setCookies(mustParseURL(test.fromURL), setCookies, now)
	// now = now.Add(1001 * time.Millisecond)

	// // Serialize non-expired entries in the form "name1=val1 name2=val2".
	// var cs []string
	// for _, submap := range jar.entries {
	// 	for _, cookie := range submap {
	// 		if !cookie.Expires.After(now) {
	// 			continue
	// 		}
	// 		cs = append(cs, cookie.Name+"="+cookie.Value)
	// 	}
	// }
	// sort.Strings(cs)
	// got := strings.Join(cs, " ")

	// // Make sure jar content matches our expectations.
	// if got != test.content {
	// 	t.Errorf("Test %q Content\ngot  %q\nwant %q",
	// 		test.description, got, test.content)
	// }

	// // Test different calls to Cookies.
	// for i, query := range test.queries {
	// 	now = now.Add(1001 * time.Millisecond)
	// 	var s []string
	// 	for _, c := range jar.cookies(mustParseURL(query.toURL), now) {
	// 		s = append(s, c.Name+"="+c.Value)
	// 	}
	// 	if got := strings.Join(s, " "); got != query.want {
	// 		t.Errorf("Test %q #%d\ngot  %q\nwant %q", test.description, i, got, query.want)
	// 	}
	// }
}
