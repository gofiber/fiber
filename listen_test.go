package fiber

import (
	"testing"
	"time"
)

func Test_Connect(t *testing.T) {
	app := New()
	app.Banner = false
	app.Get("/", func(c *Ctx) {

	})
	go func() {
		app.Listen(":8085")
	}()
	time.Sleep(1 * time.Second)
	err := app.Shutdown()
	if err != nil {
		t.Fatalf(`%s: Failed to shutdown server %v`, t.Name(), err)
	}
	return
}
