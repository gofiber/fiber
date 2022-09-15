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

func Test_Router_CaseSensitive(t *testing.T) {
	app := New(Config{
		CaseSensitive: false,
	})
	router := NewRouter(RouterConfig{
		CaseSensitive: true,
	})

	app.Use("/router", router)

	app.Get("/abc", func(c *Ctx) error {
		return c.SendString(c.Path())
	})
	router.Get("/abc", func(c *Ctx) error {
		return c.SendString(c.Path())
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/AbC", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// wrong letters in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/router/AbC", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")

	// right letters in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/router/abc", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// check the detected path when the case insensitive recognition is activated
	router.config.CaseSensitive = false
	// check the case sensitive feature
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/router/AbC", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	// check the detected path result
	utils.AssertEqual(t, "/router/AbC", app.getString(body))
}

func Test_Router_Strict(t *testing.T) {
	app := New(Config{
		Strict: false,
	})
	router := NewRouter(RouterConfig{
		Strict: true,
	})

	app.Use("/router", router)

	app.Get("/fiber", testEmptyHandler)
	router.Get("/fiber", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/fiber/", nil))
	utils.AssertEqual(t, err, nil)
	utils.AssertEqual(t, resp.StatusCode, StatusOK)

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/router/fiber/", nil))
	utils.AssertEqual(t, err, nil)
	utils.AssertEqual(t, resp.StatusCode, StatusNotFound)

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
