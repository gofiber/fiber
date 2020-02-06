package fiber

import (
	"net/http"
	"strings"
	"testing"
)

func Test_Append(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Append("X-Test", "hel")
		c.Append("X-Test", "lo", "world")
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if !strings.Contains(res, "X-Test: hel, lo, world") {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "X-Test: hel, lo, world")
	}
}

// TODO: add all functions from response.go
