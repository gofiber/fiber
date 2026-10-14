# Fiber Binders

**Binder** is a new request/response binding feature for Fiber introduced in Fiber v3. It replaces the old Fiber parsers and offers enhanced capabilities such as custom binder registration, struct validation, support for `map[string]string`, `map[string][]string`, and more. Binder replaces the following components:

- `BodyParser`
- `ParamsParser`
- `GetReqHeaders`
- `GetRespHeaders`
- `AllParams`
- `QueryParser`
- `ReqHeaderParser`

## Default Binders

Fiber provides several default binders out of the box:

- [Form](form.go)
- [Query](query.go)
- [URI](uri.go)
- [Header](header.go)
- [Response Header](resp_header.go)
- [Cookie](cookie.go)
- [JSON](json.go)
- [XML](xml.go)
- [CBOR](cbor.go)
- [MsgPack](msgpack.go)

## Guides

### Binding into a Struct

Fiber supports binding request data directly into a struct using [gofiber/schema](https://github.com/gofiber/schema). `Body` picks the decoder from the request's `Content-Type` header and returns `fiber.ErrUnprocessableEntity` (422) when none matches. Here's an example:

```go
// Field names must start with an uppercase letter
type Person struct {
    Name string `json:"name" xml:"name" form:"name"`
    Pass string `json:"pass" xml:"pass" form:"pass"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)

    if err := c.Bind().Body(p); err != nil {
        return err
    }

    log.Println(p.Name) // Output: john
    log.Println(p.Pass) // Output: doe

    // Additional logic...
})

// Run tests with the following curl commands:

// JSON
curl -X POST -H "Content-Type: application/json" --data "{\"name\":\"john\",\"pass\":\"doe\"}" localhost:3000

// XML
curl -X POST -H "Content-Type: application/xml" --data "<login><name>john</name><pass>doe</pass></login>" localhost:3000

// URL-Encoded Form
curl -X POST -H "Content-Type: application/x-www-form-urlencoded" --data "name=john&pass=doe" localhost:3000

// Multipart Form
curl -X POST -F name=john -F pass=doe http://localhost:3000
```

`Body` does not read the query string. Bind query parameters with `c.Bind().Query(p)` and tags such as `query:"name"` instead.

### Binding into a Map

Fiber allows binding request data into a `map[string]string` or `map[string][]string`. Here's an example:

```go
app.Get("/", func(c fiber.Ctx) error {
    params := make(map[string][]string)

    if err := c.Bind().Query(params); err != nil {
        return err
    }

    log.Println(params["name"])     // Output: [john]
    log.Println(params["pass"])     // Output: [doe]
    log.Println(params["products"]) // Output: [shoe hat]

    // Additional logic...
    return nil
})

// Run tests with the following curl command:

curl "http://localhost:3000/?name=john&pass=doe&products=shoe&products=hat"
```

### Automatic Error Handling with `WithAutoHandling`

By default, Fiber returns binder errors directly. To have a binding failure set a `400 Bad Request` status and come back as a `*fiber.Error` with the message `Bad request: ...`, use the `WithAutoHandling()` method. You must still return the error.

The `required` option belongs to the tags of the form, query, header, cookie, response header and URI binders. The JSON, XML, CBOR and MsgPack decoders do not check it.

**Example:**

```go
// Field names must start with an uppercase letter
type Person struct {
    Name string `query:"name,required"`
    Pass string `query:"pass"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)

    if err := c.Bind().WithAutoHandling().Query(p); err != nil {
        return err 
        // Automatically sets status code 400
        // Response: Bad request: name is empty
    }

    // Additional logic...
    return nil
})

// Run tests with the following curl command:

curl "http://localhost:3000/?pass=doe"
```

`c.Bind()` returns the same `Bind` for the whole request, so `WithAutoHandling()` stays in effect for every later bind of that request until you call `WithoutAutoHandling()`.

### Defining a Custom Binder

Fiber maintains a minimal codebase by not including every possible binder. If you need to use a custom binder, you can easily register and utilize it. Here's an example of creating a `toml` binder.

```go
type Person struct {
    Name string `toml:"name"`
    Pass string `toml:"pass"`
}

type tomlBinding struct{}

func (b *tomlBinding) Name() string {
    return "toml"
}

func (b *tomlBinding) MIMETypes() []string {
    return []string{"application/toml"}
}

func (b *tomlBinding) Parse(c fiber.Ctx, out any) error {
    return toml.Unmarshal(c.Body(), out)
}

func main() {
    app := fiber.New()
    app.RegisterCustomBinder(&tomlBinding{})

    app.Get("/", func(c fiber.Ctx) error {
        out := new(Person)
        if err := c.Bind().Body(out); err != nil {
            return err
        }

        // Alternatively, specify the custom binder:
        // if err := c.Bind().Custom("toml", out); err != nil {
        //     return err
        // }

        return c.SendString(out.Pass) // Output: test
    })

    app.Listen(":3000")
}

// Run tests with the following curl command:

curl -X GET -H "Content-Type: application/toml" --data "name = 'bar'
pass = 'test'" localhost:3000
```

### Defining a Custom Validator

All Fiber binders support struct validation if a validator is defined in the configuration. The validator implements `fiber.StructValidator`, a single method: `Validate(out any) error`. It runs after a bind succeeds, for destinations that are structs (or pointers to structs). You can create your own validator or use existing ones like [go-playground/validator](https://github.com/go-playground/validator) or [go-ozzo/ozzo-validation](https://github.com/go-ozzo/ozzo-validation). Here's an example of a simple custom validator:

```go
type Query struct {
    Name string `query:"name"`
}

type structValidator struct{}

func (v *structValidator) Validate(out any) error {
    query, ok := out.(*Query)
    if !ok {
        return nil
    }

    if query.Name != "john" {
        return errors.New("you should have entered the correct name!")
    }

    return nil
}

func main() {
    app := fiber.New(fiber.Config{
        StructValidator: &structValidator{},
    })

    app.Get("/", func(c fiber.Ctx) error {
        out := new(Query)
        if err := c.Bind().Query(out); err != nil {
            return err // Returns: you should have entered the correct name!
        }
        return c.SendString(out.Name)
    })

    app.Listen(":3000")
}

// Run tests with the following curl command:

curl "http://localhost:3000/?name=efe"
```

The validator's error is returned as is, so the default error handler answers it with `500 Internal Server Error`. Return a `*fiber.Error` such as `fiber.NewError(fiber.StatusBadRequest, "...")` from `Validate` when the request is at fault. `SkipValidation(true)` bypasses the validator for the rest of the request.
