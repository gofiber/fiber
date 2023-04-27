package minify

import (
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

var filedata struct {
	html             []byte
	htmlWithCss      []byte
	htmlWithJs       []byte
	htmlWithCssAndJs []byte
}

var expectedData struct {
	html             []byte
	htmlWithCss      []byte
	htmlWithJs       []byte
	htmlWithCssAndJs []byte
}

func init() {
	html, err := os.ReadFile("../../.github/testdata2/template.html")
	if err != nil {
		panic(err)
	}
	filedata.html = html
	filedata.htmlWithCss, err = os.ReadFile("../../.github/testdata2/template-with-css.html")
	if err != nil {
		panic(err)
	}
	filedata.htmlWithJs, err = os.ReadFile("../../.github/testdata2/template-with-js.html")
	if err != nil {
		panic(err)
	}
	filedata.htmlWithCssAndJs, err = os.ReadFile("../../.github/testdata2/template-with-css-and-js.html")
	if err != nil {
		panic(err)
	}
}
func addExpectedData(expected string) {

	switch expected {
	case "html":
		expectedData.html = []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><meta http-equiv="X-UA-Compatible" content="IE=edge"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Document</title></head><body><p>Hello, Fiber!</p><input type="hidden" disabled><br></body></html>`)
	case "htmlWithCss":
		expectedData.htmlWithCss = []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><meta http-equiv="X-UA-Compatible" content="IE=edge"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Document</title><style>body{font-family:sans-serif;font-size:16px;color:#333;background-color:#fff}</style></head><body><p>Hello, Fiber!</p></body></html>`)

	case "htmlWithJs":
		expectedData.htmlWithJs = []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><meta http-equiv="X-UA-Compatible" content="IE=edge"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Document</title><script>alert('Hello, Fiber!')</script></head><body><p>Hello, Fiber!</p></body></html>`)

	case "htmlWithCssAndJs":
		expectedData.htmlWithCssAndJs = []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><meta http-equiv="X-UA-Compatible" content="IE=edge"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Document</title><style>body{font-family:sans-serif;font-size:16px;color:#333;background-color:#fff}</style></head><body><p>Hello, Fiber!</p><script>alert('Hello, Fiber!')</script></body></html>`)

	case "Config_MinifyHTMLOptions_MinifyStyles_false":
		expectedData.htmlWithCssAndJs = []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><meta http-equiv="X-UA-Compatible" content="IE=edge"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Document</title><style>
    /* This a css comment */
    body {
      font-family: sans-serif;
      font-size: 16px;
      color: #333333;
      /*
      Sets the background color to white
      and the text color to dark gray
      */
      background-color: #ffffff;
    }
  </style></head><body><p>Hello, Fiber!</p><script>alert('Hello, Fiber!')</script></body></html>`)
	}

}

func Test_Minify_HTML(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.html)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)
	addExpectedData("html")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.html, body)

}

func Test_Minify_HTML_With_CSS(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCss)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)
	addExpectedData("htmlWithCss")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.htmlWithCss, body)

}

func Test_Minify_HTML_With_JS(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)
	addExpectedData("htmlWithJs")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.htmlWithJs, body)
}

func Test_Minify_HTML_With_Css_And_JS(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCssAndJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)
	addExpectedData("htmlWithCssAndJs")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.htmlWithCssAndJs, body)
}

func Test_Minify_HTML_Config_Options_MinifyStyles_false(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(
		Config{
			MinifyHTML: true,
			MinifyHTMLOptions: MinifyHTMLOptions{
				MinifyStyles:  false,
				MinifyScripts: true,
			},
		},
	))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCssAndJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)
	fmt.Println("body", string(body))
	addExpectedData("Config_MinifyHTMLOptions_MinifyStyles_false")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.htmlWithCssAndJs, body)
}
