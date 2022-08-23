# Fiber Binders

Bind is new request/response binding feature for Fiber.
By against old Fiber parsers, it supports custom binder registration,
struct validation with high performance and easy to use.

It's introduced in Fiber v3 and a replacement of:

- BodyParser
- ParamsParser
- GetReqHeaders
- GetRespHeaders
- AllParams
- QueryParser
- ReqHeaderParser

## Guides

### Binding basic request info

Fiber supports binding basic request data into the struct:

all tags you can use are:

- respHeader
- header
- query
- param
- cookie

(binding for Request/Response header are case in-sensitive)

private and anonymous fields will be ignored.

```go
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	fiber "github.com/gofiber/fiber/v3"
)

type Req struct {
	ID    int       `param:"id"`
	Q     int       `query:"q"`
	Likes []int     `query:"likes"`
	T     time.Time `header:"x-time"`
	Token string    `header:"x-auth"`
}

func main() {
	app := fiber.New()

	app.Get("/:id", func(c fiber.Ctx) error {
		var req Req
		if err := c.Bind().Req(&req).Err(); err != nil {
			return err
		}
		return c.JSON(req)
	})

	req := httptest.NewRequest(http.MethodGet, "/1?&s=a,b,c&q=47&likes=1&likes=2", http.NoBody)
	req.Header.Set("x-auth", "ttt")
	req.Header.Set("x-time", "2022-08-08T08:11:39+08:00")
	resp, err := app.Test(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.StatusCode, string(b))
	// Output: 200 {"ID":1,"S":["a","b","c"],"Q":47,"Likes":[1,2],"T":"2022-08-08T08:11:39+08:00","Token":"ttt"}
}

```

### Defining Custom Binder

We support 2 types of Custom Binder

#### a `encoding.TextUnmarshaler` with basic tag config.

like the `time.Time` field in the previous example, if a field implement `encoding.TextUnmarshaler`, it will be called
to
unmarshal raw string we get from request's query/header/...

#### a `fiber.Binder` interface.

You don't need to set a field tag and it's binding tag will be ignored.

```
type Binder interface {
    UnmarshalFiberCtx(ctx fiber.Ctx) error
}
```

If your type implement `fiber.Binder`, bind will pass current request Context to your and you can unmarshal the info
you need.

### Parse Request Body

you can call `ctx.BodyJSON(v any) error` or `BodyXML(v any) error`

These methods will check content-type HTTP header and call configured JSON or XML decoder to unmarshal.

```golang
package main

type Body struct {
	ID    int       `json:"..."`
	Q     int       `json:"..."`
	Likes []int     `json:"..."`
	T     time.Time `json:"..."`
	Token string    `json:"..."`
}

func main() {
	app := fiber.New()

	app.Get("/:id", func(c fiber.Ctx) error {
		var data Body
		if err := c.Bind().JSON(&data).Err(); err != nil {
			return err
		}
		return c.JSON(data)
	})
}
```

### Bind With validation

Normally, `bind` will only try to unmarshal data from request and pass it to request handler.

you can call `.Validate()` to validate previous binding.

And you will need to set a validator in app Config, otherwise it will always return an error.

```go
package main

type Validator struct{}

func (validator *Validator) Validate(v any) error {
	return nil
}

func main() {
	app := fiber.New(fiber.Config{
		Validator: &Validator{},
	})

	app.Get("/:id", func(c fiber.Ctx) error {
		var req struct{}
		var body struct{}
		if err := c.Bind().Req(&req).Validate().JSON(&body).Validate().Err(); err != nil {
			return err
		}

		return nil
	})
}
```
