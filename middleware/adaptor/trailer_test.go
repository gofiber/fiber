package adaptor

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/stretchr/testify/require"
)

// trailerNames returns "prefix0, prefix1, ..." naming n fields.
func trailerNames(prefix string, n int) string {
	names := make([]string, n)
	for i := range names {
		names[i] = prefix + strconv.Itoa(i)
	}
	return strings.Join(names, ", ")
}

// Test_FiberHandler_TrailerNames pins that a Trailer field naming more than fieldname.MaxTrailers fields is
// refused before the handler runs, as the router refuses it, and that up to that many reach the handler.
// net/http keeps the Trailer field of a request that is not chunked in its header map, and fasthttp scans every
// name announced for each header it stores.
func Test_FiberHandler_TrailerNames(t *testing.T) {
	t.Parallel()
	maxNames := trailerNames("X-T", fieldname.MaxTrailers)
	for _, tt := range []struct {
		name   string
		lines  []string
		status int
		keys   int
	}{
		{name: "none", status: fiber.StatusNoContent},
		{name: "at the cap", lines: []string{maxNames}, status: fiber.StatusNoContent, keys: fieldname.MaxTrailers},
		{name: "at the cap over two lines", lines: []string{trailerNames("X-T", 8), trailerNames("X-U", 8)}, status: fiber.StatusNoContent, keys: 16},
		{name: "over the cap", lines: []string{trailerNames("X-T", fieldname.MaxTrailers+1)}, status: fiber.StatusRequestHeaderFieldsTooLarge},
		{name: "over the cap over two lines", lines: []string{maxNames, "X-Extra"}, status: fiber.StatusRequestHeaderFieldsTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := make(chan int, 1)
			h := FiberHandler(func(c fiber.Ctx) error {
				got <- len(c.Request().Header.PeekTrailerKeys())
				return c.SendStatus(fiber.StatusNoContent)
			})
			r := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			r.Header["Trailer"] = tt.lines
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			require.Equal(t, tt.status, rec.Code)
			if tt.status != fiber.StatusNoContent {
				require.Empty(t, got, "the handler ran for a refused request")
				return
			}
			require.Equal(t, tt.keys, apptest.Recv(t, got, time.Second))
		})
	}
}

// Test_FiberApp_TrailerNamesAreCapped pins that a request that names as many trailer fields as it has headers,
// which kept the header copy quadratic (about 6 s for these, 40 s for what fits in net/http's header limit), is
// refused at once instead of copied.
func Test_FiberApp_TrailerNamesAreCapped(t *testing.T) {
	t.Parallel()
	const n = 40_000
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	r := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	for i := range n {
		r.Header.Set("X-H"+strconv.Itoa(i), "v")
	}
	r.Header.Set("Trailer", trailerNames("X-T", n))
	rec := httptest.NewRecorder()
	start := time.Now()
	FiberApp(app).ServeHTTP(rec, r)
	require.Equal(t, fiber.StatusRequestHeaderFieldsTooLarge, rec.Code)
	require.Less(t, time.Since(start), 2*time.Second, "the names a Trailer field announces made the header copy quadratic")
}
