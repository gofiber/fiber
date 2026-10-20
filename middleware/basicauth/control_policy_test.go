package basicauth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_ControlPolicies_AllBytes(t *testing.T) {
	t.Parallel()
	for n := 1; n <= 33; n++ {
		for pos := range n {
			for value := range 256 {
				input := []byte(strings.Repeat("a", n))
				input[pos] = byte(value)
				s := string(input)
				wantHeader := value < 0x20 && value != '\t' || value >= 0x7f
				if got := containsInvalidHeaderChars(s); got != wantHeader {
					t.Fatalf("header n=%d pos=%d byte=%02x: got %v, want %v", n, pos, value, got, wantHeader)
				}
				wantControl := strings.IndexFunc(s, unicode.IsControl) != -1
				if got := containsCTL(s); got != wantControl {
					t.Fatalf("credentials n=%d pos=%d byte=%02x: got %v, want %v", n, pos, value, got, wantControl)
				}
			}
		}
	}
	for pos := range 33 {
		for _, value := range []string{"é", "\u0085", "\u009f", "\u2028", "é\x00", "é\u0085"} {
			s := strings.Repeat("a", pos) + value + strings.Repeat("z", 33-pos)
			if got, want := containsCTL(s), strings.IndexFunc(s, unicode.IsControl) != -1; got != want {
				t.Fatalf("credentials %q: got %v, want %v", s, got, want)
			}
		}
	}
}

func Test_BasicAuth_UnicodeControlPolicies(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		credentials string
		status      int
		authorized  bool
	}{
		{"Unicode password", "john:pässwörd", fiber.StatusTeapot, true},
		{"tab in username", "jo\thn:doe", fiber.StatusBadRequest, false},
		{"C1 after Unicode", "john:päss\u0085tail", fiber.StatusBadRequest, false},
		{"DEL after Unicode", "john:päss\x7f", fiber.StatusBadRequest, false},
		{"C0 after word boundary", "abcdefghé\x00tail:doe", fiber.StatusBadRequest, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			called := false
			app := fiber.New()
			app.Use(New(Config{
				Charset:    "UTF-8",
				Authorizer: func(_, _ string, _ fiber.Ctx) bool { called = true; return true },
			}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })
			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Header.Set(fiber.HeaderAuthorization, "Basic "+base64.StdEncoding.EncodeToString([]byte(test.credentials)))
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, test.status, resp.StatusCode)
			require.Equal(t, test.authorized, called)
		})
	}
}
