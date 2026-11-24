package escapedpath

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Resolve(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ base, location, want string }{
		{"http://example.com/a", "/b/..%2F..%2Fc", "http://example.com/b/..%2F..%2Fc"},
		{"http://example.com/files/a%2Fb?v=1", "next?v=2", "http://example.com/files/next?v=2"},
		{"https://example.com/a", "//other.example/b%2Fc", "https://other.example/b%2Fc"},
		{"http://example.com/a/b", "../c", "http://example.com/c"},
		{"http://example.com/", "/discount/100%", "http://example.com/discount/100%25"},
		{"http://example.com/", "/my file/café", "http://example.com/my%20file/caf%C3%A9"},
		{"http://example.com/", "http://u:p@other.example/x", "http://other.example/x"},
	} {
		next, err := Resolve(tc.base, []byte(tc.location))
		require.NoError(t, err, tc.location)
		require.Equal(t, tc.want, next.String(), tc.location)
	}
	for _, location := range []string{"http://[::1/", "file:///etc/passwd"} {
		_, err := Resolve("http://example.com/", []byte(location))
		require.ErrorIs(t, err, fasthttp.ErrorInvalidURI, location)
	}
}
