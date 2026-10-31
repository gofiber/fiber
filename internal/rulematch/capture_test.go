package rulematch

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_CaptureTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		input   string
		target  string
		want    string
	}{
		{"no capture groups", `^/old$`, "/old", "/new/$1", "/new/$1"},
		{"reordered and repeated captures", `^/users/(.*)/orders/(.*)$`, "/users/7/orders/9", "/$2/$1/$2", "/9/7/9"},
		{
			"two digit captures", `^/(a)/(b)/(c)/(d)/(e)/(f)/(g)/(h)/(i)/(j)/(k)$`, "/a/b/c/d/e/f/g/h/i/j/k",
			"/$11/$10/$1", "/k/j/a",
		},
		{"empty capture", `^/old/(.*)$`, "/old/", "/new/$1", "/new/"},
		{"unmatched optional capture", `^/old(?:/(.*))?$`, "/old", "/new/$1", "/new/"},
		{"trailing slash preserved", `^/old/(.*)$`, "/old/a/b/", "/new/$1", "/new/a/b/"},
		{"capture is not expanded again", `^/old/([^/]+)/([^/]+)$`, "/old/$2/value", "/$1/$2", "/$2/value"},
		{"unknown and braced tokens preserved", `^/old/(.*)$`, "/old/a%2Fb", "/$1/$0/$2/${1}", "/a%2Fb/$0/$2/${1}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			replacer := CaptureTokens(regexp.MustCompile(tc.pattern), tc.input, false)
			require.NotNil(t, replacer)
			require.Equal(t, tc.want, replacer.Replace(tc.target))
		})
	}
}

func Test_CaptureTokens_NoMatch(t *testing.T) {
	t.Parallel()
	pattern := regexp.MustCompile(`^/old/(.*)$`)
	for _, input := range []string{"/other/a", "/prefix/old/a", "/old"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			require.Nil(t, CaptureTokens(pattern, input, false))
		})
	}
}

func Test_CaptureTokens_Unescaped(t *testing.T) {
	t.Parallel()
	pattern := regexp.MustCompile(`^/old/(.*)$`)
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "", want: "/new/"},
		{input: "a/b/", want: "/new/a/b/"},
		{input: "100%", want: "/new/100%25"},
		{input: "a%2Fb", want: "/new/a%252Fb"},
		{input: "%2e%2e/secret", want: "/new/%252e%252e/secret"},
		{input: "a b", want: "/new/a%20b"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			replacer := CaptureTokens(pattern, "/old/"+tc.input, true)
			require.NotNil(t, replacer)
			require.Equal(t, tc.want, replacer.Replace("/new/$1"))
		})
	}
	require.Nil(t, CaptureTokens(pattern, "/other/a", true))
}

func Test_DotSegments(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want int
	}{
		{in: "", want: 0},
		{in: "/new/$1", want: 0},
		{in: "https://h/a/../b", want: 1},
		{in: "https://h/a/./../b", want: 2},
		{in: "https://h/a/..", want: 1},
		{in: "../../a", want: 2},
		{in: "/a/%2e/.%2E/%2e%2e/%2E.", want: 4},
		{in: `/a\..\b`, want: 1},
		{in: "/a/..%2Fb", want: 1},
		{in: "/a%2f..%5Cb", want: 1},
		{in: "/a/..?x", want: 1},
		{in: "/a/../b?/../c#/./d", want: 3},
		{in: "/a/.../..b/b../%2e%2e%2e/.$1", want: 0},
		{in: "/a/..%252Fb/%252e%252e/.%09./..%2", want: 0},
		{in: "/search?q=../x", want: 0},
	} {
		require.Equal(t, tc.want, DotSegments(tc.in), "input %q", tc.in)
	}
}
