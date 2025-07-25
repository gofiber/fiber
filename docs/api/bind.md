---
id: bind
title: 📎 Bind
description: Binds the request and response items to a struct.
sidebar_position: 4
toc_max_heading_level: 4
---

Bindings are used to parse the request/response body, query parameters, cookies, and much more into a struct.

:::info
All binder returned values are only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## Binders

- [All](#all)
- [Body](#body)
  - [CBOR](#cbor)
  - [Form](#form)
  - [JSON](#json)
  - [MsgPack](#msgpack)
  - [XML](#xml)
- [Cookie](#cookie)
- [Header](#header)
- [Query](#query)
- [RespHeader](#respheader)
- [URI](#uri)

### All

The `All` function binds data from various sources (URL parameters, request body, query parameters, headers, and cookies) into the provided struct pointer `out`. It processes each source in a predefined order, applying data to the struct fields based on their tags.

#### Precedence Order

The binding sources have the following precedence:

1. **URL Parameters (URI)**
2. **Request Body (e.g., JSON or form data)**
3. **Query Parameters**
4. **Request Headers**
5. **Cookies**

```go title="Signature"
func (b *Bind) All(out any) error
```

``` go title="Example"
type User struct {
    Name      string                `query:"name" json:"name" form:"name"`
    Email     string                `json:"email" form:"email"`
    Role      string                `header:"X-User-Role"`
    SessionID string                `json:"session_id" cookie:"session_id"`
    ID        int                   `uri:"id" query:"id" json:"id" form:"id"`
}

app.Post("/users", func(c fiber.Ctx) error {
    user := new(User)

    if err := c.Bind().All(user); err != nil {
        return err
    }

    // All available data is now bound to the user struct
    return c.JSON(user)
})
```

### Body

Binds the request body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a JSON body with a field called `Pass`, you would use a struct field with `json:"pass"`.

| Content-Type                        | Struct Tag |
| ----------------------------------- | ---------- |
| `application/x-www-form-urlencoded` | `form`     |
| `multipart/form-data`               | `form`     |
| `application/json`                  | `json`     |
| `application/xml`                   | `xml`      |
| `text/xml`                          | `xml`      |
| `application/vnd.msgpack`           | `msgpack`  |

```go title="Signature"
func (b *Bind) Body(out any) error
```

```go title="Example"
type Person struct {
    Name string `json:"name" xml:"name" form:"name" msgpack:"name"`
    Pass string `json:"pass" xml:"pass" form:"pass" msgpack:"pass"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Body(p); err != nil {
        return err
    }
    
    log.Println(p.Name) // john
    log.Println(p.Pass) // doe
    
    // ...
})
```

Run tests with the following `curl` commands:

```bash
# JSON
curl -X POST -H "Content-Type: application/json" --data "{\"name\":\"john\",\"pass\":\"doe\"}" localhost:3000

# MsgPack
curl -X POST -H "Content-Type: application/vnd.msgpack" --data-binary $'\x82\xa4name\xa4john\xa4pass\xa3doe'  localhost:3000

# XML
curl -X POST -H "Content-Type: application/xml" --data "<login><name>john</name><pass>doe</pass></login>" localhost:3000

# Form URL-Encoded
curl -X POST -H "Content-Type: application/x-www-form-urlencoded" --data "name=john&pass=doe" localhost:3000

# Multipart Form
curl -X POST -F name=john -F pass=doe http://localhost:3000
```

### CBOR

> **Note:** Before using any CBOR-related features, make sure to follow the [CBOR setup instructions](../guide/advance-format.md#cbor).

Binds the request CBOR body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a CBOR body with a field called `Pass`, you would use a struct field with `cbor:"pass"`.

```go title="Signature"
func (b *Bind) CBOR(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name string `cbor:"name"`
    Pass string `cbor:"pass"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().CBOR(p); err != nil {
        return err
    }
    
    log.Println(p.Name) // john
    log.Println(p.Pass) // doe
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl -X POST -H "Content-Type: application/cbor" --data "\xa2dnamedjohndpasscdoe" localhost:3000
```

### Form

Binds the request or multipart form body data to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a form body with a field called `Pass`, you would use a struct field with `form:"pass"`.

```go title="Signature"
func (b *Bind) Form(out any) error
```

```go title="Example"
type Person struct {
    Name string `form:"name"`
    Pass string `form:"pass"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Form(p); err != nil {
        return err
    }
    
    log.Println(p.Name) // john
    log.Println(p.Pass) // doe
    
    // ...
})
```

Run tests with the following `curl` commands for both `application/x-www-form-urlencoded` and `multipart/form-data`:

```bash
curl -X POST -H "Content-Type: application/x-www-form-urlencoded" --data "name=john&pass=doe" localhost:3000
```

```bash
curl -X POST -H "Content-Type: multipart/form-data" -F "name=john" -F "pass=doe" localhost:3000
```

:::info
If you need to bind multipart file, you can use `*multipart.FileHeader`, `*[]*multipart.FileHeader` or `[]*multipart.FileHeader` as a field type.
:::

```go title="Example"
type Person struct {
    Name string                `form:"name"`
    Pass string                `form:"pass"`
    Avatar *multipart.FileHeader `form:"avatar"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Form(p); err != nil {
        return err
    }
    
    log.Println(p.Name) // john
    log.Println(p.Pass) // doe
    log.Println(p.Avatar.Filename) // file.txt
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl -X POST -H "Content-Type: multipart/form-data" -F "name=john" -F "pass=doe" -F 'avatar=@filename' localhost:3000
```

### JSON

Binds the request JSON body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a JSON body with a field called `Pass`, you would use a struct field with `json:"pass"`.

```go title="Signature"
func (b *Bind) JSON(out any) error
```

```go title="Example"
type Person struct {
    Name string `json:"name"`
    Pass string `json:"pass"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().JSON(p); err != nil {
        return err
    }

    log.Println(p.Name) // john
    log.Println(p.Pass) // doe
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl -X POST -H "Content-Type: application/json" --data "{\"name\":\"john\",\"pass\":\"doe\"}" localhost:3000
```

### MsgPack

> **Note:** Before using any MsgPack-related features, make sure to follow the [MsgPack setup instructions](../guide/advance-format.md#msgpack).

Binds the request MsgPack body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a Msgpack body with a field called `Pass`, you would use a struct field with `msgpack:"pass"`.

> Our library uses [shamaton-msgpack](https://github.com/shamaton/msgpack) which uses `msgpack` struct tags by default. If you want to use other libraries, you may need to update the struct tags accordingly.

```go title="Signature"
func (b *Bind) MsgPack(out any) error
```

```go title="Example"
type Person struct {
    Name string `msgpack:"name"`
    Pass string `msgpack:"pass"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().MsgPack(p); err != nil {
        return err
    }

    log.Println(p.Name) // john
    log.Println(p.Pass) // doe
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl -X POST -H "Content-Type: application/vnd.msgpack" --data-binary $'\x82\xa4name\xa4john\xa4pass\xa3doe'  localhost:3000
```

### XML

Binds the request XML body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse an XML body with a field called `Pass`, you would use a struct field with `xml:"pass"`.

```go title="Signature"
func (b *Bind) XML(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name string `xml:"name"`
    Pass string `xml:"pass"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().XML(p); err != nil {
        return err
    }
    
    log.Println(p.Name) // john
    log.Println(p.Pass) // doe
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl -X POST -H "Content-Type: application/xml" --data "<login><name>john</name><pass>doe</pass></login>" localhost:3000
```

### Cookie

This method is similar to [Body Binding](#body), but for cookie parameters.  
It is important to use the struct tag `cookie`. For example, if you want to parse a cookie with a field called `Age`, you would use a struct field with `cookie:"age"`.

```go title="Signature"
func (b *Bind) Cookie(out any) error
```

```go title="Example"
type Person struct {
    Name string `cookie:"name"`
    Age  int    `cookie:"age"`
    Job  bool   `cookie:"job"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Cookie(p); err != nil {
        return err
    }
    
    log.Println(p.Name)  // Joseph
    log.Println(p.Age)   // 23
    log.Println(p.Job)   // true
})
```

Run tests with the following `curl` command:

```bash
curl --cookie "name=Joseph; age=23; job=true" http://localhost:8000/
```

### Header

This method is similar to [Body Binding](#body), but for request headers.  
It is important to use the struct tag `header`. For example, if you want to parse a request header with a field called `Pass`, you would use a struct field with `header:"pass"`.

```go title="Signature"
func (b *Bind) Header(out any) error
```

```go title="Example"
type Person struct {
    Name     string   `header:"name"`
    Pass     string   `header:"pass"`
    Products []string `header:"products"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Header(p); err != nil {
        return err
    }
    
    log.Println(p.Name)     // john
    log.Println(p.Pass)     // doe
    log.Println(p.Products) // [shoe hat]
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl "http://localhost:3000/" -H "name: john" -H "pass: doe" -H "products: shoe,hat"
```

### Query

This method is similar to [Body Binding](#body), but for query parameters.  
It is important to use the struct tag `query`. For example, if you want to parse a query parameter with a field called `Pass`, you would use a struct field with `query:"pass"`.

```go title="Signature"
func (b *Bind) Query(out any) error
```

```go title="Example"
type Person struct {
    Name     string   `query:"name"`
    Pass     string   `query:"pass"`
    Products []string `query:"products"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Query(p); err != nil {
        return err
    }
    
    log.Println(p.Name)     // john
    log.Println(p.Pass)     // doe
    // Depending on fiber.Config{EnableSplittingOnParsers: false} - default
    log.Println(p.Products) // ["shoe,hat"]
    // With fiber.Config{EnableSplittingOnParsers: true}
    // log.Println(p.Products) // ["shoe", "hat"]
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl "http://localhost:3000/?name=john&pass=doe&products=shoe,hat"
```

:::info
For more parser settings, please refer to [Config](fiber.md#enablesplittingonparsers)
:::

### RespHeader

This method is similar to [Body Binding](#body), but for response headers.
It is important to use the struct tag `respHeader`. For example, if you want to parse a response header with a field called `Pass`, you would use a struct field with `respHeader:"pass"`.

```go title="Signature"
func (b *Bind) RespHeader(out any) error
```

```go title="Example"
type Person struct {
    Name     string   `respHeader:"name"`
    Pass     string   `respHeader:"pass"`
    Products []string `respHeader:"products"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().RespHeader(p); err != nil {
        return err
    }
    
    log.Println(p.Name)     // john
    log.Println(p.Pass)     // doe
    log.Println(p.Products) // [shoe hat]
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl "http://localhost:3000/" -H "name: john" -H "pass: doe" -H "products: shoe,hat"
```

### URI

This method is similar to [Body Binding](#body), but for path parameters.  
It is important to use the struct tag `uri`. For example, if you want to parse a path parameter with a field called `Pass`, you would use a struct field with `uri:"pass"`.

```go title="Signature"
func (b *Bind) URI(out any) error
```

```go title="Example"
// GET http://example.com/user/111
app.Get("/user/:id", func(c fiber.Ctx) error {
    param := struct {
        ID uint `uri:"id"`
    }{}
    
    if err := c.Bind().URI(&param); err != nil {
        return err
    }
    
    // ...
    return c.SendString(fmt.Sprintf("User ID: %d", param.ID))
})
```

## Custom

To use custom binders, you have to use this method.

You can register them using the [RegisterCustomBinder](./app.md#registercustombinder) method of the Fiber instance.

```go title="Signature"
func (b *Bind) Custom(name string, dest any) error
```

```go title="Example"
app := fiber.New()

// My custom binder
type customBinder struct{}

func (cb *customBinder) Name() string {
    return "custom"
}

func (cb *customBinder) MIMETypes() []string {
    return []string{"application/yaml"}
}

func (cb *customBinder) Parse(c fiber.Ctx, out any) error {
    // parse YAML body
    return yaml.Unmarshal(c.Body(), out)
}

// Register custom binder
app.RegisterCustomBinder(&customBinder{})

type User struct {
    Name string `yaml:"name"`
}

// curl -X POST http://localhost:3000/custom -H "Content-Type: application/yaml" -d "name: John"
app.Post("/custom", func(c fiber.Ctx) error {
    var user User
    // Use Custom binder by name
    if err := c.Bind().Custom("custom", &user); err != nil {
        return err
    }
    return c.JSON(user)
})
```

Internally, custom binders are also used in the [Body](#body) method.  
The `MIMETypes` method is used to check if the custom binder should be used for the given content type.

## Options

For more control over error handling, you can use the following methods.

### WithAutoHandling

If you want to handle binder errors automatically, you can use `WithAutoHandling`.  
If there's an error, it will return the error and set HTTP status to `400 Bad Request`.
This function does NOT panic therefore you must still return on error explicitly

```go title="Signature"
func (b *Bind) WithAutoHandling() *Bind
```

### WithoutAutoHandling

To handle binder errors manually, you can use the `WithoutAutoHandling` method.  
It's the default behavior of the binder.

```go title="Signature"
func (b *Bind) WithoutAutoHandling() *Bind
```

## SetParserDecoder

Allows you to configure the BodyParser/QueryParser decoder based on schema options, providing the possibility to add custom types for parsing.

```go title="Signature"
func SetParserDecoder(parserConfig fiber.ParserConfig{
    IgnoreUnknownKeys bool,
    ParserType        []fiber.ParserType{
        Customtype any,
        Converter  func(string) reflect.Value,
    },
    ZeroEmpty         bool,
    SetAliasTag       string,
})
```

```go title="Example"

type CustomTime time.Time

// String returns the time in string format
func (ct *CustomTime) String() string {
    t := time.Time(*ct).String()
    return t
}

// Converter for CustomTime type with format "2006-01-02"
var timeConverter = func(value string) reflect.Value {
    fmt.Println("timeConverter:", value)
    if v, err := time.Parse("2006-01-02", value); err == nil {
        return reflect.ValueOf(CustomTime(v))
    }
    return reflect.Value{}
}

customTime := fiber.ParserType{
    CustomType: CustomTime{},
    Converter:  timeConverter,
}

// Add custom type to the Decoder settings
fiber.SetParserDecoder(fiber.ParserConfig{
    IgnoreUnknownKeys: true,
    ParserType:        []fiber.ParserType{customTime},
    ZeroEmpty:         true,
})

// Example using CustomTime with non-RFC3339 format
type Demo struct {
    Date  CustomTime `form:"date" query:"date"`
    Title string     `form:"title" query:"title"`
    Body  string     `form:"body" query:"body"`
}

app.Post("/body", func(c fiber.Ctx) error {
    var d Demo
    if err := c.Bind().Body(&d); err != nil {
        return err
    }
    fmt.Println("d.Date:", d.Date.String())
    return c.JSON(d)
})

app.Get("/query", func(c fiber.Ctx) error {
    var d Demo
    if err := c.Bind().Query(&d); err != nil {
        return err
    }
    fmt.Println("d.Date:", d.Date.String())
    return c.JSON(d)
})

// Run tests with the following curl commands:

# Body Binding
curl -X POST -F title=title -F body=body -F date=2021-10-20 http://localhost:3000/body

# Query Binding
curl -X GET "http://localhost:3000/query?title=title&body=body&date=2021-10-20"
```

## Validation

Validation is also possible with the binding methods. You can specify your validation rules using the `validate` struct tag.

Specify your struct validator in the [config](./fiber.md#structvalidator).

### Setup Your Validator in the Config

```go title="Example"
import "github.com/go-playground/validator/v10"

type structValidator struct {
    validate *validator.Validate
}

// Validate method implementation
func (v *structValidator) Validate(out any) error {
    return v.validate.Struct(out)
}

// Setup your validator in the Fiber config
app := fiber.New(fiber.Config{
    StructValidator: &structValidator{validate: validator.New()},
})
```

### Usage of Validation in Binding Methods

```go title="Example"
type Person struct {
    Name string `json:"name" validate:"required"`
    Age  int    `json:"age" validate:"gte=18,lte=60"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().JSON(p); err != nil { // Receives validation errors
        return err
    }
})
```

## Default Fields

You can set default values for fields in the struct by using the `default` struct tag. Supported types:

- `bool`
- Float variants (`float32`, `float64`)
- Int variants (`int`, `int8`, `int16`, `int32`, `int64`)
- Uint variants (`uint`, `uint8`, `uint16`, `uint32`, `uint64`)
- `string`
- A slice of the above types. Use `|` to separate slice items.
- A pointer to one of the above types (**pointers to slices and slices of pointers are not supported**).

```go title="Example"
type Person struct {
    Name     string     `query:"name,default:john"`
    Pass     string     `query:"pass"`
    Products []string   `query:"products,default:shoe|hat"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)

    if err := c.Bind().Query(p); err != nil {
        return err
    }

    log.Println(p.Name)     // john
    log.Println(p.Pass)     // doe
    log.Println(p.Products) // ["shoe", "hat"]
    
    // ...
})
```

Run tests with the following `curl` command:

```bash
curl "http://localhost:3000/?pass=doe"
```
