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
		{
			name: "no capture groups", pattern: `^/old$`, input: "/old",
			target: "/new/$1", want: "/new/$1",
		},
		{
			name: "reordered and repeated captures", pattern: `^/users/(.*)/orders/(.*)$`, input: "/users/7/orders/9",
			target: "/$2/$1/$2", want: "/9/7/9",
		},
		{
			name: "two digit captures", pattern: `^/(a)/(b)/(c)/(d)/(e)/(f)/(g)/(h)/(i)/(j)/(k)$`, input: "/a/b/c/d/e/f/g/h/i/j/k",
			target: "/$11/$10/$1", want: "/k/j/a",
		},
		{
			name: "empty capture", pattern: `^/old/(.*)$`, input: "/old/",
			target: "/new/$1", want: "/new/",
		},
		{
			name: "unmatched optional capture", pattern: `^/old(?:/(.*))?$`, input: "/old",
			target: "/new/$1", want: "/new/",
		},
		{
			name: "trailing slash preserved", pattern: `^/old/(.*)$`, input: "/old/a/b/",
			target: "/new/$1", want: "/new/a/b/",
		},
		{
			name: "capture is not expanded again", pattern: `^/old/([^/]+)/([^/]+)$`, input: "/old/$2/value",
			target: "/$1/$2", want: "/$2/value",
		},
		{
			name: "unknown and braced tokens preserved", pattern: `^/old/(.*)$`, input: "/old/a%2Fb",
			target: "/$1/$0/$2/${1}", want: "/a%2Fb/$0/$2/${1}",
		},
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
