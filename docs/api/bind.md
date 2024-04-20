---
id: bind
title: 📎 Bind
description: Binds the request and response items to a struct.
sidebar_position: 4
toc_max_heading_level: 4
---

Bindings are used to parse the request/response body, query parameters, cookies and much more into a struct.

:::info

All binder returned value are only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)

:::


## Binders

- [Body](#body)
    - [Form](#form)
    - [JSON](#json)
    - [MultipartForm](#multipartform)
    - [XML](#xml)
- [Cookie](#cookie)
- [Header](#header)
- [Query](#query)
- [RespHeader](#respheader)
- [URI](#uri)

### Body

Binds the request body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a JSON body with a field called Pass, you would use a struct field of `json:"pass"`.

| content-type                        | struct tag |
| ----------------------------------- | ---------- |
| `application/x-www-form-urlencoded` | form       |
| `multipart/form-data`               | form       |
| `application/json`                  | json       |
| `application/xml`                   | xml        |
| `text/xml`                          | xml        |

```go title="Signature"
func (b *Bind) Body(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name string `json:"name" xml:"name" form:"name"`
    Pass string `json:"pass" xml:"pass" form:"pass"`
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

// Run tests with the following curl commands

// curl -X POST -H "Content-Type: application/json" --data "{\"name\":\"john\",\"pass\":\"doe\"}" localhost:3000

// curl -X POST -H "Content-Type: application/xml" --data "<login><name>john</name><pass>doe</pass></login>" localhost:3000

// curl -X POST -H "Content-Type: application/x-www-form-urlencoded" --data "name=john&pass=doe" localhost:3000

// curl -X POST -F name=john -F pass=doe http://localhost:3000

// curl -X POST "http://localhost:3000/?name=john&pass=doe"
```


**The methods for the various bodies can also be used directly:**

#### Form

Binds the request form body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a Form body with a field called Pass, you would use a struct field of `form:"pass"`.


```go title="Signature"
func (b *Bind) Form(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
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

// Run tests with the following curl commands

// curl -X POST -H "Content-Type: application/x-www-form-urlencoded" --data "name=john&pass=doe" localhost:3000
```

#### JSON

Binds the request json body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a JSON body with a field called Pass, you would use a struct field of `json:"pass"`.


```go title="Signature"
func (b *Bind) JSON(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
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

// Run tests with the following curl commands

// curl -X POST -H "Content-Type: application/json" --data "{\"name\":\"john\",\"pass\":\"doe\"}" localhost:3000

```

#### MultipartForm

Binds the request multipart form body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a MultipartForm body with a field called Pass, you would use a struct field of `form:"pass"`.


```go title="Signature"
func (b *Bind) MultipartForm(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name string `form:"name"`
    Pass string `form:"pass"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().MultipartForm(p); err != nil {
        return err
    }
    
    log.Println(p.Name) // john
    log.Println(p.Pass) // doe
    
    // ...
})

// Run tests with the following curl commands

// curl -X POST -H "Content-Type: multipart/form-data" -F "name=john" -F "pass=doe" localhost:3000

```

#### XML

Binds the request xml form body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a XML body with a field called Pass, you would use a struct field of `xml:"pass"`.


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

// Run tests with the following curl commands

// curl -X POST -H "Content-Type: application/xml" --data "<login><name>john</name><pass>doe</pass></login>" localhost:3000
```


### Cookie

This method is similar to [Body-Binding](#body), but for cookie parameters.
It is important to use the struct tag "cookie". For example, if you want to parse a cookie with a field called Age, you would use a struct field of `cookie:"age"`.

```go title="Signature"
func (b *Bind) Cookie(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name     string  `cookie:"name"`
    Age      int     `cookie:"age"`
    Job      bool    `cookie:"job"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Cookie(p); err != nil {
        return err
    }
    
    log.Println(p.Name)     // Joseph
    log.Println(p.Age)      // 23
    log.Println(p.Job)      // true
})
// Run tests with the following curl command
// curl.exe --cookie "name=Joseph; age=23; job=true" http://localhost:8000/
```


### Header

This method is similar to [Body-Binding](#body), but for request headers.
It is important to use the struct tag "header". For example, if you want to parse a request header with a field called Pass, you would use a struct field of `header:"pass"`.

```go title="Signature"
func (b *Bind) Header(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name     string     `header:"name"`
    Pass     string     `header:"pass"`
    Products []string   `header:"products"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Header(p); err != nil {
        return err
    }
    
    log.Println(p.Name)     // john
    log.Println(p.Pass)     // doe
    log.Println(p.Products) // [shoe, hat]
    
    // ...
})
// Run tests with the following curl command

// curl "http://localhost:3000/" -H "name: john" -H "pass: doe" -H "products: shoe,hat"
```


### Query

This method is similar to [Body-Binding](#body), but for query parameters.
It is important to use the struct tag "query". For example, if you want to parse a query parameter with a field called Pass, you would use a struct field of `query:"pass"`.

```go title="Signature"
func (b *Bind) Query(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name     string     `query:"name"`
    Pass     string     `query:"pass"`
    Products []string   `query:"products"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().Query(p); err != nil {
        return err
    }
    
    log.Println(p.Name)        // john
    log.Println(p.Pass)        // doe
    // fiber.Config{EnableSplittingOnParsers: false} - default
    log.Println(p.Products)    // ["shoe,hat"]
    // fiber.Config{EnableSplittingOnParsers: true}
    // log.Println(p.Products) // ["shoe", "hat"]
    
    
    // ...
})
// Run tests with the following curl command

// curl "http://localhost:3000/?name=john&pass=doe&products=shoe,hat"
```

:::info
For more parser settings please look here [Config](fiber.md#enablesplittingonparsers)
:::


### RespHeader

This method is similar to [Body-Binding](#body), but for response headers.
It is important to use the struct tag "respHeader". For example, if you want to parse a request header with a field called Pass, you would use a struct field of `respHeader:"pass"`.

```go title="Signature"
func (b *Bind) Header(out any) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name     string     `respHeader:"name"`
    Pass     string     `respHeader:"pass"`
    Products []string   `respHeader:"products"`
}

app.Get("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().RespHeader(p); err != nil {
        return err
    }
    
    log.Println(p.Name)     // john
    log.Println(p.Pass)     // doe
    log.Println(p.Products) // [shoe, hat]
    
    // ...
})
// Run tests with the following curl command

// curl "http://localhost:3000/" -H "name: john" -H "pass: doe" -H "products: shoe,hat"
```

### URI

This method is similar to [Body-Binding](#body), but for path parameters. It is important to use the struct tag "uri". For example, if you want to parse a path parameter with a field called Pass, you would use a struct field of uri:"pass"

```go title="Signature"
func (b *Bind) URI(out any) error
```

```go title="Example"
// GET http://example.com/user/111
app.Get("/user/:id", func(c fiber.Ctx) error {
    param := struct {ID uint `uri:"id"`}{}
    
    c.Bind().URI(&param) // "{"id": 111}"
    
    // ...
})

```

## Custom

To use custom binders, you have to use this method.

You can register them from [RegisterCustomBinder](./app.md#registercustombinder) method of Fiber instance.

```go title="Signature"
func (b *Bind) Custom(name string, dest any) error
```

```go title="Example"
app := fiber.New()

// My custom binder
customBinder := &customBinder{}
// Name of custom binder, which will be used as Bind().Custom("name")
func (*customBinder) Name() string {
    return "custom"
}
// Is used in the Body Bind method to check if the binder should be used for custom mime types
func (*customBinder) MIMETypes() []string {
    return []string{"application/yaml"}
}
// Parse the body and bind it to the out interface
func (*customBinder) Parse(c Ctx, out any) error {
    // parse yaml body
    return yaml.Unmarshal(c.Body(), out)
}
// Register custom binder
app.RegisterCustomBinder(customBinder)

// curl -X POST http://localhost:3000/custom -H "Content-Type: application/yaml" -d "name: John"
app.Post("/custom", func(c Ctx) error {
    var user User
    // output: {Name:John}
    // Custom binder is used by the name
    if err := c.Bind().Custom("custom", &user); err != nil {
        return err
    }
    // ...
    return c.JSON(user)
})
```

Internally they are also used in the [Body](#body) method.
For this the MIMETypes method is used to check if the custom binder should be used for the given content type.

## Options

For more control over the error handling, you can use the following methods.

### Must

If you want to handle binder errors automatically, you can use Must.
If there's an error it'll return error and 400 as HTTP status.

```go title="Signature"
func (b *Bind) Must() *Bind
```

### Should

To handle binder errors manually, you can prefer Should method.
It's default behavior of binder.

```go title="Signature"
func (b *Bind) Should() *Bind
```


## SetParserDecoder

Allow you to config BodyParser/QueryParser decoder, base on schema's options, providing possibility to add custom type for parsing.

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

// String() returns the time in string
func (ct *CustomTime) String() string {
    t := time.Time(*ct).String()
        return t
    }
    
    // Register the converter for CustomTime type format as 2006-01-02
    var timeConverter = func(value string) reflect.Value {
    fmt.Println("timeConverter", value)
    if v, err := time.Parse("2006-01-02", value); err == nil {
        return reflect.ValueOf(v)
    }
    return reflect.Value{}
}

customTime := fiber.ParserType{
    Customtype: CustomTime{},
    Converter:  timeConverter,
}

// Add setting to the Decoder
fiber.SetParserDecoder(fiber.ParserConfig{
    IgnoreUnknownKeys: true,
    ParserType:        []fiber.ParserType{customTime},
    ZeroEmpty:         true,
})

// Example to use CustomType, you pause custom time format not in RFC3339
type Demo struct {
    Date  CustomTime `form:"date" query:"date"`
    Title string     `form:"title" query:"title"`
    Body  string     `form:"body" query:"body"`
}

app.Post("/body", func(c fiber.Ctx) error {
    var d Demo
    c.BodyParser(&d)
    fmt.Println("d.Date", d.Date.String())
    return c.JSON(d)
})

app.Get("/query", func(c fiber.Ctx) error {
    var d Demo
    c.QueryParser(&d)
    fmt.Println("d.Date", d.Date.String())
    return c.JSON(d)
})

// curl -X POST -F title=title -F body=body -F date=2021-10-20 http://localhost:3000/body

// curl -X GET "http://localhost:3000/query?title=title&body=body&date=2021-10-20"

```


## Validation

Validation is also possible with the binding methods. You can specify your validation rules using the `validate` struct tag.

Specify your struct validator in the [config](./fiber.md#structvalidator)

Setup your validator in the config:

```go title="Example"
import "github.com/go-playground/validator/v10"

type structValidator struct {
    validate *validator.Validate
}

// Validator needs to implement the Validate method
func (v *structValidator) Validate(out any) error {
    return v.validate.Struct(out)
}

// Setup your validator in the config
app := fiber.New(fiber.Config{
    StructValidator: &structValidator{validate: validator.New()},
})
```

Usage of the validation in the binding methods:

```go title="Example"
type Person struct {
    Name string `json:"name" validate:"required"`
    Age  int    `json:"age" validate:"gte=18,lte=60"`
}

app.Post("/", func(c fiber.Ctx) error {
    p := new(Person)
    
    if err := c.Bind().JSON(p); err != nil {// <- here you receive the validation errors
        return err
    }
})
```



