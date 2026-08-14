package session

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// go test -v -run=^$ -bench=Benchmark_Session_ReadOnly -benchmem -count=4
func Benchmark_Session_ReadOnly(b *testing.B) {
	app := fiber.New()
	store := NewStore()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("session_id", "12356789")

	// seed the stored session once
	sess, err := store.Get(c)
	if err != nil {
		b.Fatal(err)
	}
	sess.Set("uid", "1337")
	if err := sess.Save(); err != nil {
		b.Fatal(err)
	}
	sess.Release()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		sess, err := store.Get(c)
		if err != nil {
			b.Fatal(err)
		}
		if got, ok := sess.Get("uid").(string); !ok || got != "1337" {
			b.Fatalf("unexpected session value: %v", sess.Get("uid"))
		}
		if err := sess.Save(); err != nil {
			b.Fatal(err)
		}
		sess.Release()
	}
}
