package middleware

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// go test -run Test_Middleware_Timeout -race
func Test_Middleware_Timeout(t *testing.T) {
	app := fiber.New(&fiber.Settings{DisableStartupMessage: true})

	h := Timeout(
		func(c *fiber.Ctx) {
			sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
			time.Sleep(sleepTime)
			c.SendString("After " + c.Params("sleepTime") + "ms sleeping")
		},
		5*time.Millisecond,
	)
	app.Get("/test/:sleepTime", h)

	testTimeout := func(timeoutStr string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/test/"+timeoutStr, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusRequestTimeout, resp.StatusCode, "Status code")

		body, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Request Timeout", string(body))
	}

	testSucces := func(timeoutStr string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/test/"+timeoutStr, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")

		body, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "After "+timeoutStr+"ms sleeping", string(body))
	}

	testTimeout("15")
	testTimeout("30")
	testSucces("2")
	testSucces("3")
}

func Test_Middleware_Timeout_Invalid_TimeoutDuration(t *testing.T) {
	app := fiber.New(&fiber.Settings{DisableStartupMessage: true})

	h := Timeout(
		func(c *fiber.Ctx) {
			c.Set("dummy", "👋")
		},
		-5*time.Millisecond,
	)
	app.Get("/test", h)

	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "👋", resp.Header.Get("dummy"))
}

// go test -run Test_Middleware_Timeout_Panic -race
func Test_Middleware_Timeout_Panic(t *testing.T) {
	app := fiber.New(&fiber.Settings{DisableStartupMessage: true})

	h := Timeout(
		func(c *fiber.Ctx) {
			c.Set("dummy", "this should not be here")
			panic("panic in timeout handler")
		},
		5*time.Millisecond,
	)
	app.Get("/panic", Recover(), h)

	resp, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "", resp.Header.Get("dummy"))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "panic in timeout handler", string(body))
}

// go test -v -run=^$ -bench=Benchmark_Middleware_Timeout -benchmem -count=4
func Benchmark_Middleware_Timeout(b *testing.B) {
	app := fiber.New()
	app.Use(Timeout(
		func(c *fiber.Ctx) {},
		5*time.Second,
	))

	handler := app.Handler()

	c := &fasthttp.RequestCtx{}
	c.Request.SetRequestURI("/")

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		handler(c)
	}
}
