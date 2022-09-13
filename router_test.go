// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📃 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3/utils"
)

//TODO: Add more router tests

func Test_Router_Config(t *testing.T) {
	t.Parallel()
	{
		r := NewRouter()
		utils.AssertEqual(t, r.config, DefaultRouterConfig)
	}
	{
		cfg := RouterConfig{
			CaseSensitive: true,
			MergeParams:   true,
			Strict:        true,
		}
		r := NewRouter(cfg)
		utils.AssertEqual(t, r.config.CaseSensitive, cfg.CaseSensitive)
	}
}

func Test_Router_Is_Added(t *testing.T) {
	app := New()
	router := NewRouter()
	app.Use("/router", router)

	utils.AssertEqual(t, router.app != nil, true)
	utils.AssertEqual(t, app.routerList["/router"] != nil, true)
}

func Test_Router_Methods(t *testing.T) {
	dummyHandler := testEmptyHandler

	app := New()
	router := NewRouter()
	app.Use("/users", router)

	router.Connect("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", "CONNECT")

	router.Put("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodPut)

	router.Post("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodPost)

	router.Delete("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodDelete)

	router.Head("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodHead)

	router.Patch("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodPatch)

	router.Options("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodOptions)

	router.Trace("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodTrace)

	router.Get("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodGet)

	router.All("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodPost)

	router.Use("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/users/john/doe", MethodGet)
}

func Test_Router_MergeParams(t *testing.T) {
	t.Parallel()
	{
		app := New()
		router := NewRouter(RouterConfig{
			MergeParams: true,
		})
		app.Use("/users/:id", router)

		router.Get("/:name", func(c *Ctx) error {
			utils.AssertEqual(t, c.Params("id"), "1")
			utils.AssertEqual(t, c.Params("name"), "eren")
			return nil
		})

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/users/1/eren", nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, StatusOK, resp.StatusCode)
	}
	{
		app := New()
		router := NewRouter(RouterConfig{
			MergeParams: false,
		})
		app.Use("/users/:id", router)

		router.Get("/:name", func(c *Ctx) error {
			utils.AssertEqual(t, c.Params("id"), "")
			utils.AssertEqual(t, c.Params("name"), "eren")
			return nil
		})

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/users/1/eren", nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, StatusOK, resp.StatusCode)
	}
}

func Test_Router_ErrorHandler(t *testing.T) {
	app := New()
	router := NewRouter()

	app.Use("/router", router)

	router.Use(func(c *Ctx, err error) error {
		return c.Status(500).SendString("I'm router error handler")
	})

	router.Get("/", func(c *Ctx) error {
		return NewError(StatusBadRequest, "")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/router", nil))
	body, _ := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, string(body), "I'm router error handler")
}
