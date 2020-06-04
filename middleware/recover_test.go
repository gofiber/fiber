package middleware

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// go test -run Test_Middleware_Recover
func Test_Middleware_Recover(t *testing.T) {
	app := fiber.New()

	app.Use(Recover())

	app.Get("/panic", func(ctx *fiber.Ctx) {
		ctx.Set("dummy", "this should not be here")
		panic("Hi, I'm an error!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "", resp.Header.Get("dummy"))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Hi, I'm an error!", string(body))
}
