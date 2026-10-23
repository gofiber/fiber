---
id: bind
title: 📎 Bind
description: Binds the request and response items to a struct.
sidebar_position: 4
toc_max_heading_level: 4
---

Bindings parse request bodies, query parameters, cookies, headers, path parameters, and response headers into structs or [maps](#binding-into-maps).

:::info
Bound string values alias the request buffers, so they are valid only within the handler:
copy the data you need to keep. The [**`Immutable`**](./ctx.md) setting does not apply to
bound values. [Read more...](../#zero-allocation)
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

The `All` function binds data from URL parameters, the request body, query parameters, headers, and cookies into `out`. Sources are applied in the following order using struct field tags.

#### Precedence Order

The binding sources have the following precedence:

1. **URL Parameters (URI)**
2. **Request Body (e.g., JSON or form data)**
3. **Query Parameters**
4. **Request Headers**
5. **Cookies**

:::info
The request body is only included as a binding source when the request has both a non-empty body **and** a non-empty `Content-Type` header.
:::

Each source fills only the top-level fields of `out` that are still at their zero value, so the first source in the order that provides a non-zero value for a field wins. A non-zero value you set on `out` before the call is never overwritten. A zero one counts as unset, so a source can fill it, and a zero value from an earlier source (`0`, `""`, `false`) does not stop a later source from filling the field. A nested struct, slice or map that an earlier source set, even partly, is kept as a whole, so a later source cannot fill its remaining zero fields. A configured [`StructValidator`](#validation) runs once, on the merged struct.

`out` must be a non-nil pointer to a struct. Anything else, such as a map, returns `fiber.ErrUnprocessableEntity`. If a source fails to bind, `All` stops and returns that error without trying the remaining sources.

```go title="Signature"
func (b *Bind) All(out any) error
```

```go title="Example"
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

#### Custom Precedence

By default, the `All` method binds data in the following precedence order: `URI params -> Body -> Query -> Headers -> Cookies`.
If you need to override this behavior, you can define a custom precedence order for your struct using the `binding_source` tag.

```go title="Example"
type CustomPrecedenceReq struct {
    // Specify the priority for binding using the binding_source tag.
    // In this example, query parameters take highest priority, followed by headers.
    // The tag can be placed on any top-level field in the struct.
    Name string `binding_source:"query,header,cookie,body,uri" query:"name" header:"x-name" json:"name"`
}

app.Post("/users", func(c fiber.Ctx) error {
    req := new(CustomPrecedenceReq)
    if err := c.Bind().All(req); err != nil {
        return err
    }
    // req.Name will take the value from the query parameter if it exists, otherwise header, etc.
    return c.JSON(req)
})
```

:::info
For maximum performance, Fiber caches the precedence resolution per `reflect.Type` to reduce repeated parsing overhead.
Additionally, the `binding_source` tag must be placed on a **top-level** field of the struct (tags inside embedded structs are ignored), and a struct may only contain **one** `binding_source` tag.

- **Source Names**: The valid names are `uri`, `body`, `query`, `header`, and `cookie`. They are case-sensitive, surrounding spaces are ignored, and a name listed twice counts once, at its first position.
- **Partial Lists**: If you omit sources from the tag (e.g., `binding_source:"query"`), the omitted sources (body, header, etc.) will **not** be bound at all.
- **Unrecognized Sources**: If an invalid source name is provided in the tag (e.g., `binding_source:"invalid"`), or the struct carries more than one `binding_source` tag, `All` returns a plain error. It is not a `*BindError`: the mistake is in your struct, not in the request, so the default error handler answers with 500, even when `WithAutoHandling` is on.

:::

### Body

Binds the request body to a struct.

Use tags that match the content type. For example, to parse a JSON body with a `Pass` field, declare `json:"pass"`.

| Content-Type                        | Struct Tag |
| ----------------------------------- | ---------- |
| `application/x-www-form-urlencoded` | `form`     |
| `multipart/form-data`               | `form`     |
| `application/json`                  | `json`     |
| `application/xml`                   | `xml`      |
| `text/xml`                          | `xml`      |
| `application/vnd.msgpack`           | `msgpack`  |
| `application/cbor`                  | `cbor`     |

The decoder is chosen from the `Content-Type` header. The media type is matched case-insensitively and its parameters, such as `charset`, are ignored. A structured syntax suffix selects the matching format, so `application/vnd.api+json` binds as JSON, `application/problem+xml` as XML and `application/foo+cbor` as CBOR. MsgPack matches only `application/vnd.msgpack`, not a `+msgpack` suffix. [Custom binders](#mimetypes-and-body) are consulted first. If no decoder matches, or the header is empty, `Body` returns `fiber.ErrUnprocessableEntity` (422). MsgPack and CBOR bodies also need a decoder in the app config, see [MsgPack](#msgpack) and [CBOR](#cbor).

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

Test the handler with these `curl` commands:

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

> **Note:** Before using any CBOR-related features, make sure to follow the [CBOR setup instructions](../guide/advance-format.md#cbor). Fiber does not ship a CBOR implementation: until `CBORDecoder` is set in the app config, binding fails with `binder.ErrUnimplementedCBOR`.

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

Test it with this `curl` command, once `CBORDecoder` is set:

```bash
curl -X POST -H "Content-Type: application/cbor" --data-binary $'\xa2dnamedjohndpasscdoe' localhost:3000
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

:::info
`Form` reads only the request body, never the query string. A `multipart/form-data` body is read with the app's [`BodyLimit`](fiber.md#bodylimit) as its size limit, which also bounds the size of a `Content-Encoding: gzip` body after decompression. A larger body fails with an error that wraps `fasthttp.ErrBodyTooLarge`.

On a form request, `Form` and `Body` also lowercase the case-insensitive parts of the request's own `Content-Type` (the media type and the parameter names, never the `boundary` value), so copy a value you read earlier from `Get(HeaderContentType)` if you need it to outlive the call, see [FormValue](./ctx.md#formvalue). Other body types leave the header untouched.
:::

#### Nested and Array Form Fields

A form body is a flat list of key/value pairs, so structure lives in the field name. The notations below work for both `application/x-www-form-urlencoded` and `multipart/form-data`, and identically for [query parameters](#query).

| Notation           | Example                      | Binds to                             |
| ------------------ | ---------------------------- | ------------------------------------ |
| Repeated key       | `colors=red&colors=blue`     | slice of scalars                     |
| Empty brackets     | `colors[]=red&colors[]=blue` | slice of scalars                     |
| Bracket field      | `address[city]=Berlin`       | field of a nested struct             |
| Dot field          | `address.city=Berlin`        | field of a nested struct             |
| Indexed brackets   | `items[0][sku]=A1`           | field of a struct slice element      |
| Dot index          | `items.0.sku=A1`             | field of a struct slice element      |
| Mixed index/field  | `items[0].sku=A1`            | field of a struct slice element      |
| Map key            | `attributes[color]=red`      | entry of a [map field](#map-fields)  |
| Dot map key        | `attributes.color=red`       | entry of a [map field](#map-fields)  |

Bracket and dot notation are interchangeable and can be nested to any depth:

```go title="Struct"
type Address struct {
    City string `form:"city"`
    Zip  string `form:"zip"`
}

type Item struct {
    SKU string `form:"sku"`
    Qty int    `form:"qty"`
}

type Order struct {
    Colors  []string `form:"colors"`
    Address Address  `form:"address"`
    Items   []Item   `form:"items"`
}
```

```bash title="curl"
curl -X POST http://localhost:3000/orders \
  --data "colors[]=red&colors[]=blue" \
  --data "address[city]=Berlin&address.zip=10115" \
  --data "items[0][sku]=A1&items[0].qty=2&items.1.sku=B2&items.1.qty=5"
```

```go title="Result"
Order{
    Colors:  []string{"red", "blue"},
    Address: Address{City: "Berlin", Zip: "10115"},
    Items:   []Item{{SKU: "A1", Qty: 2}, {SKU: "B2", Qty: 5}},
}
```

:::caution
An index only ever addresses a struct field. `colors[0]=red` and `colors.0=red` do **not** fill a `[]string`. With the default decoder the value is dropped and no error is returned; after `SetParserDecoder(ParserConfig{IgnoreUnknownKeys: false})` the same input fails with `schema: invalid path "colors.0"`. Use a repeated key or `colors[]` for slices of scalars.

Indexes above 1000 are rejected: `items.1001.sku=A1` fails with `schema: index exceeds parser limit`.
:::

:::note
The struct field tag never contains the brackets or the index. `colors[]`, `colors[0]` and `colors.0` all resolve against `form:"colors"`.
:::

#### Map Fields

A `map[string]T` field collects every key under its name. Everything after the field name becomes the map key, as sent: `attributes[color]` and `attributes.color` both give `color`, and `attributes[a][b]` gives `a.b`. Values are converted like any other field, so `T` can be any scalar a struct field could hold, or a slice of one. Other map fields, such as `map[string]Address`, `map[int]string`, `map[string]any`, a nested map or a pointer to a map, are skipped without an error. Query strings work the same with `query` tags.

```go title="Struct"
type Filter struct {
    Attributes map[string]string   `form:"attributes"`
    Tags       map[string][]string `form:"tags"`
    Limits     map[string]int      `form:"limits"`
}
```

```bash title="curl"
curl -X POST http://localhost:3000/products \
  --data "attributes[color]=red&attributes.size=m" \
  --data "tags[sale]=yes&tags[sale]=new&limits[max]=5"
```

```go title="Result"
Filter{
    Attributes: map[string]string{"color": "red", "size": "m"},
    Tags:       map[string][]string{"sale": {"yes", "new"}},
    Limits:     map[string]int{"max": 5},
}
```

A map of slices keeps repeated values, and splits comma-separated ones when `EnableSplittingOnParsers` is on. A map of single values keeps the last value. A value that can't be converted fails the bind with the full key in the error, such as `"limits.max"`, and a map with no matching keys is left `nil`, or fails the bind when the tag has `required`. A map that is already set is added to, not replaced, so don't pre-fill the field with a map other requests share.

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

> Fiber does not bundle a MsgPack implementation. Set `MsgPackDecoder` in the app config to the `Unmarshal` function of the library you picked; until then, binding fails with `binder.ErrMsgPackNotConfigured`. Libraries such as [shamaton-msgpack](https://github.com/shamaton/msgpack) read `msgpack` struct tags. If you use another library, you may need to update the struct tags accordingly.

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

Run tests with the following `curl` command, once `MsgPackDecoder` is set:

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
    // Depending on fiber.Config{EnableSplittingOnParsers: false} - default
    log.Println(p.Products) // ["shoe,hat"]
    // With fiber.Config{EnableSplittingOnParsers: true}
    // log.Println(p.Products) // ["shoe", "hat"]

    // ...
})
```

Run tests with the following `curl` command:

```bash
curl "http://localhost:3000/" -H "name: john" -H "pass: doe" -H "products: shoe,hat"
```

Header names are matched case-insensitively. A header sent on several lines, such as `-H "products: shoe" -H "products: hat"`, fills a slice with one element per line, with or without `EnableSplittingOnParsers`.

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
`EnableSplittingOnParsers` applies to the Cookie, Form, Header, Query, and RespHeader binders. For more parser settings, please refer to [Config](fiber.md#enablesplittingonparsers)
:::

#### Array Query Parameters

Fiber supports several formats for passing array values via query parameters. The following table gives an overview:

| Format                   | Example                                        | Requires `EnableSplittingOnParsers` |
| ------------------------ | ---------------------------------------------- | ----------------------------------- |
| Repeated key             | `?colors=red&colors=blue`                      | No                                  |
| Bracket notation         | `?colors[]=red&colors[]=blue`                  | No                                  |
| Comma-separated          | `?colors=red,blue`                             | **Yes**                             |
| Indexed bracket notation | `?posts[0][title]=Hello&posts[1][title]=World` | No                                  |
| Nested bracket notation  | `?preferences[tags]=golang,api`                | No (comma splitting: **Yes**)       |
| Dot notation             | `?user.name=Alice&posts.0.title=Hello`         | No                                  |
| [Map key](#map-fields)   | `?attributes[color]=red&attributes.size=m`     | No (comma splitting: **Yes**)       |

##### Repeated Key

The most common approach. Repeat the same query key for each value:

```text
GET /api?colors=red&colors=blue&colors=green
```

```go title="Struct"
type Filter struct {
    Colors []string `query:"colors"`
}
// Result: Colors = ["red", "blue", "green"]
```

```bash title="curl"
curl "http://localhost:3000/api?colors=red&colors=blue&colors=green"
```

A repeated key bound into a field that is not a slice keeps its **last** value, so `?name=a&name=b` gives `b`.

##### Bracket Notation

Append `[]` to the key name. This is common in PHP-style and JavaScript frameworks:

```text
GET /api?colors[]=red&colors[]=blue&colors[]=green
```

```go title="Struct"
type Filter struct {
    Colors []string `query:"colors"`
}
// Result: Colors = ["red", "blue", "green"]
```

```bash title="curl"
curl "http://localhost:3000/api?colors[]=red&colors[]=blue&colors[]=green"
```

:::note
The struct field tag stays `query:"colors"` (without brackets). Fiber strips the `[]` automatically.
:::

##### Comma-Separated Values

Pass multiple values in a single parameter, separated by commas. This format requires [`EnableSplittingOnParsers`](fiber.md#enablesplittingonparsers) to be set to `true`.

```text
GET /api?colors=red,blue,green
```

```go title="Struct"
type Filter struct {
    Colors []string `query:"colors"`
}
```

```go title="App Setup"
// EnableSplittingOnParsers is required for comma splitting
app := fiber.New(fiber.Config{
    EnableSplittingOnParsers: true,
})
// Result: Colors = ["red", "blue", "green"]
```

Without `EnableSplittingOnParsers`, the entire string `"red,blue,green"` is treated as a **single** element. With it, only slice fields are split: a `string` field keeps its commas.

```go title="Default behavior (EnableSplittingOnParsers: false)"
// GET /api?colors=red,blue,green
// Result: Colors = ["red,blue,green"]  ← single element
```

```bash title="curl"
curl "http://localhost:3000/api?colors=red,blue,green"
```

You can also mix comma-separated values with repeated keys when splitting is enabled:

```text
GET /api?hobby=soccer&hobby=basketball,football
```

```go
type Query struct {
    Hobby []string `query:"hobby"`
}
// With EnableSplittingOnParsers: true
// Result: Hobby = ["soccer", "basketball", "football"]  ← 3 elements
```

##### Indexed Bracket Notation (Nested Structs)

Use indexed brackets to bind arrays of nested structs:

```text
GET /api?posts[0][title]=Hello&posts[0][author]=Alice&posts[1][title]=World&posts[1][author]=Bob
```

```go title="Struct"
type Post struct {
    Title  string `query:"title"`
    Author string `query:"author"`
}

type Request struct {
    Posts []Post `query:"posts"`
}
// Result: Posts = [{Title: "Hello", Author: "Alice"}, {Title: "World", Author: "Bob"}]
```

```bash title="curl"
curl "http://localhost:3000/api?posts[0][title]=Hello&posts[0][author]=Alice&posts[1][title]=World&posts[1][author]=Bob"
```

##### Nested Bracket Notation (Without Index)

Use bracket notation to access fields of a nested struct:

```text
GET /api?preferences[tags]=golang,api
```

```go title="Struct"
type Preferences struct {
    Tags *[]string `query:"tags"`
}

type Profile struct {
    Prefs *Preferences `query:"preferences"`
}
// With EnableSplittingOnParsers: true
// Result: *Prefs.Tags = ["golang", "api"]
```

```bash title="curl"
curl "http://localhost:3000/api?preferences[tags]=golang,api"
```

:::note
Pointer fields (`*[]string`, `*Preferences`) let you distinguish between a missing parameter (`nil`) and an empty one. When the parameter is present, Fiber allocates the pointer automatically.
:::

##### Dot Notation

Nested fields and indexes have a dot equivalent, and the two can be mixed within one request. Empty brackets are the exception, `colors[]` has no dot form:

```text
GET /api?user.name=Alice&posts.0.title=Hello&posts[1].title=World
```

```go title="Struct"
type Post struct {
    Title string `query:"title"`
}

type User struct {
    Name string `query:"name"`
}

type Request struct {
    User  User   `query:"user"`
    Posts []Post `query:"posts"`
}
// Result: User = {Name: "Alice"}, Posts = [{Title: "Hello"}, {Title: "World"}]
```

Query and form binding share the same decoder, so the full notation reference in [Nested and Array Form Fields](#nested-and-array-form-fields) applies here too, including the caveat that an index never fills a slice of scalars.

### RespHeader

This method is similar to [Body Binding](#body), but for response headers.
It is important to use the struct tag `respHeader`. For example, if you want to parse a response header with a field called `Pass`, you would use a struct field with `respHeader:"pass"`.

It reads the headers of the response being built, such as those set by earlier handlers or middleware, not the request headers.

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
    // Response headers, set here for the example
    c.Set("name", "john")
    c.Set("pass", "doe")
    c.Set("products", "shoe,hat")

    p := new(Person)

    if err := c.Bind().RespHeader(p); err != nil {
        return err
    }

    log.Println(p.Name)     // john
    log.Println(p.Pass)     // doe
    // Depending on fiber.Config{EnableSplittingOnParsers: false} - default
    log.Println(p.Products) // ["shoe,hat"]
    // With fiber.Config{EnableSplittingOnParsers: true}
    // log.Println(p.Products) // ["shoe", "hat"]

    // ...
    return nil
})
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

:::note
`URI` binds the values as `c.Params` returns them. With [`UnescapePath`](fiber.md#unescapepath) off (the default) they are still percent-encoded, so `/user/a%20b` binds `a%20b`.
:::

## Binding into Maps

The Cookie, Form, Header, Query, RespHeader, and URI binders can fill a `map[string]string` or a `map[string][]string` instead of a struct. Pass the map or a pointer to it. The keys are the names found in the request, so no struct tags are needed. `Body` does the same for form content types; JSON, XML, CBOR, and MsgPack bodies are decoded by their own library.

```go title="Example"
// GET /?color=red&color=blue&size=m
app.Get("/", func(c fiber.Ctx) error {
    all := make(map[string][]string)
    if err := c.Bind().Query(&all); err != nil {
        return err
    }
    log.Println(all["color"]) // [red blue]

    last := make(map[string]string)
    if err := c.Bind().Query(&last); err != nil {
        return err
    }
    log.Println(last["color"]) // blue

    return nil
})
```

- `map[string]string` keeps only the **last** value of a repeated key and never splits values on commas. `map[string][]string` keeps every value and, with [`EnableSplittingOnParsers`](fiber.md#enablesplittingonparsers), splits comma-separated ones.
- Bound keys are added to the map and replace entries of the same name. Other entries stay in place.
- A nil map passed by value fails with `binder.ErrMapNilDestination`. Pass a pointer, such as `&m`, and Fiber allocates the map. A nil pointer to a map fails as well.
- Only `map[string]string` and `map[string][]string` themselves are filled. A map with string or slice elements of any other type, such as `map[string][]int` or a named type like `type Headers map[string]string`, fails with `binder.ErrMapNotConvertible`. Maps with other element kinds, for example `map[string]int` or `map[string]any`, are left untouched and no error is returned. A map field of a struct follows [Map Fields](#map-fields) instead.
- `Query` and `Form` rewrite bracket notation to dots, so the key `filter[name]` is stored as `filter.name` and `ids[]` as `ids`.
- File fields of a multipart form are not added to a map.
- [`All`](#all) needs a pointer to a struct, and the [struct validator](#validation) is skipped for maps.

## BindError

When a bind method fails to parse (e.g. invalid JSON, bad type conversion), the behavior depends on the error-handling mode. In **manual handling** (the default), the binder returns a `*BindError` wrapping the underlying error — use `errors.As` to extract it and branch on the binding source or field. An error that is, or wraps, a `*fiber.Error`, such as one returned by a custom binder's `Parse` or by a custom `JSONDecoder`, is returned as that `*fiber.Error` instead. In **automatic handling** (enabled via `WithAutoHandling`), parse failures are instead converted to a `*fiber.Error` with HTTP status 400; `*BindError` is never surfaced to the caller in that mode. If you are using `WithAutoHandling`, check for `*fiber.Error` or an HTTP 400 response rather than using `errors.As` for `*BindError`. The default error handler answers a `*BindError` with `500 Internal Server Error`, so return a client error yourself, as in the example below, when the request is at fault.

```go
type BindError struct {
    Err    error  // underlying error; use errors.As to inspect
    Source string // "uri", "query", "body", "header", "cookie", or "respHeader"; the binder's name for Custom
    Field  string // struct field or tag key that failed (best-effort, may be empty)
}
```

Source constants: `BindSourceURI`, `BindSourceQuery`, `BindSourceHeader`, `BindSourceCookie`, `BindSourceBody`, `BindSourceRespHeader`.

### Branching on source

Use `errors.As` to extract `*BindError` and branch on `Source` for RFC-correct status codes (e.g. 404 for URI failures vs 400 for body/query):

```go title="Example"
// With manual handling mode (default behavior)
// Will not work with WithAutoHandling()
var req struct {
    ID   int    `uri:"id"`
    Name string `json:"name"`
}
if err := c.Bind().All(&req); err != nil {
    var be *fiber.BindError
    if errors.As(err, &be) && be.Source == fiber.BindSourceURI {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
    }
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
}
```

### Inspecting the underlying error

`BindError` unwraps to the error of the decoder, so `errors.As` and `errors.Is` see through it. Fiber re-exports the types you are most likely to look for:

- The Cookie, Form, Header, Query, RespHeader, and URI binders (and `Body` for form content) fail with a `fiber.MultiError`, a `map[string]error` keyed by the request key that failed. Its entries are a `fiber.ConversionError` when a value cannot be converted to the field type, a `fiber.EmptyFieldError` when a [`required`](#required-fields) key is missing or empty, and a `fiber.UnknownKeyError` for a key that names no field (only reported when `IgnoreUnknownKeys` is `false`, see [SetParserDecoder](#setparserdecoder)). Not every failure is a `MultiError`: a key with unmatched square brackets, such as `?a[=1`, is refused earlier with a plain `binder.ErrUnmatchedBrackets`, so keep a fallback for other errors.
- With the default `JSONDecoder`, a JSON body fails with the errors of `encoding/json`: `*fiber.UnmarshalTypeError` for a value of the wrong JSON type, `*fiber.SyntaxError` for malformed JSON, and `*fiber.InvalidUnmarshalError` for a destination that is not a non-nil pointer.

`Field` is taken from a `ConversionError`, `UnknownKeyError`, `EmptyFieldError`, `MultiError` (one of its keys, so any of them when several fields failed), or `UnmarshalTypeError`. For any other error it stays empty.

```go title="Example"
var req struct {
    Age int `query:"age,required"`
}
if err := c.Bind().Query(&req); err != nil {
    var fields fiber.MultiError
    if errors.As(err, &fields) {
        problems := make(map[string]string, len(fields))
        for key, fieldErr := range fields {
            var empty fiber.EmptyFieldError
            if errors.As(fieldErr, &empty) {
                problems[key] = "required"
            } else {
                problems[key] = "invalid"
            }
        }
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": problems})
    }
    return err
}
```

### Validation vs binding errors

Validation errors (from `StructValidator`) are **not** wrapped in `BindError`. Use `errors.As(err, &be)` to distinguish: it succeeds only for parsing/binding failures, not for validation failures.

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

`Custom` returns `fiber.ErrCustomBinderNotFound` when no binder is registered under `name`. With manual handling, an error returned by `Parse` comes back as a `*BindError` whose `Source` is the binder's name (when the binder is reached through `Body`, the `Source` is `BindSourceBody`). An error that is, or wraps, a `*fiber.Error` is the exception: that `*fiber.Error` comes back as it is, without a `*BindError` around it, and the error that wrapped it is dropped. Return `fiber.NewError(...)` from `Parse` to choose the status of the response. With `WithAutoHandling`, every error from `Parse`, a `*fiber.Error` included, comes back as a `*fiber.Error` with status `400` and the message `Bad request: ...`. The [struct validator](#validation) runs on `dest` after a successful `Parse`.

### MIMETypes and Body

[`Body`](#body) consults the registered custom binders before its own content-type
switch, so `MIMETypes` decides how a binder can be reached:

| `MIMETypes()` returns              | `Bind().Custom(name, dest)` | `Bind().Body(dest)`               |
| ---------------------------------- | --------------------------- | --------------------------------- |
| a content type no built-in handles | yes                         | yes, for that content type        |
| a content type a built-in handles  | yes                         | yes, the built-in no longer runs  |
| `nil`                              | yes                         | no                                |

Returning `nil` makes a binder opt-in: it is reachable only by name, and the
built-in decoders keep handling `Body`.

`Body` compares each entry of `MIMETypes()` with the media type normalized as
described under [Body](#body): lower case, without parameters, and with a
structured syntax suffix resolved. List the entries in that form.
`Application/YAML` and `application/toml; charset=utf-8` never match, and neither
does `application/vnd.api+json`, which a request reaches as `application/json`.

### Overriding a built-in format

Claiming a content type that already has a decoder takes precedence over the
built-in one for every `Bind().Body()` call in the application, not only on the
route you had in mind. `Body` uses the first registered binder whose `MIMETypes()`
contains the request's content type, so a second binder claiming
`fiber.MIMEApplicationJSON` never runs.

To decode strictly on selected routes instead, return `nil` and call the binder
by name:

```go title="Example"
type strictJSON struct{}

func (strictJSON) Name() string        { return "strict" }
func (strictJSON) MIMETypes() []string { return nil }

func (strictJSON) Parse(c fiber.Ctx, out any) error {
    d := json.NewDecoder(bytes.NewReader(c.Body()))
    d.DisallowUnknownFields()
    return d.Decode(out)
}

app.RegisterCustomBinder(strictJSON{})

// curl -X POST http://localhost:3000/strict -H "Content-Type: application/json" -d '{"name":"John","admin":true}'
app.Post("/strict", func(c fiber.Ctx) error {
    var user User
    // Rejects the unknown "admin" field; c.Bind().JSON(&user) still accepts it
    if err := c.Bind().Custom("strict", &user); err != nil {
        return err
    }
    return c.JSON(user)
})
```

## Options

For more control over error handling, you can use the following methods.

:::note
`c.Bind()` returns the same `Bind` for the whole request, so a setting made with one of these methods stays in effect for every later bind of that request, in a handler or in middleware, until you change it back.
:::

### WithAutoHandling

If you want to handle binder errors automatically, you can use `WithAutoHandling`.
If binding fails, it will return a `*fiber.Error` with the message `Bad request: ...` and set HTTP status to `400 Bad Request`.
This function does NOT panic therefore you must still return on error explicitly

```go title="Signature"
func (b *Bind) WithAutoHandling() *Bind
```

Auto handling covers failures to bind the request data. A validation error is converted to `400 Bad Request` only by [`All`](#all); the other bind methods return the validator's error as is, which the default error handler answers with 500. Errors that point at a mistake in your code, such as `ErrCustomBinderNotFound` or an invalid `binding_source` tag, are returned unchanged as well.

### WithoutAutoHandling

To handle binder errors manually, you can use the `WithoutAutoHandling` method.
It's the default behavior of the binder.

```go title="Signature"
func (b *Bind) WithoutAutoHandling() *Bind
```

### SkipValidation

To enable or disable validation, use `SkipValidation`.
By default, validation is enabled (`skip = false`).
The setting is not limited to one call: it applies to every later bind of the request until you call `SkipValidation` again.

```go title="Signature"
func (b *Bind) SkipValidation(skip bool) *Bind
```

## SetParserDecoder

Allows you to configure the decoder behind the Cookie, Form, Header, Query, RespHeader, and URI binders (and `Body` for form content) based on schema options, providing the possibility to add custom types for parsing. JSON, XML, CBOR, and MsgPack bodies are not affected. The function lives in the `github.com/gofiber/fiber/v3/binder` package.

```go title="Signature"
func SetParserDecoder(parserConfig binder.ParserConfig)
```

The call is global and replaces the whole configuration. Fields you leave out are not kept at Fiber's defaults, they fall back to their zero values. Fiber starts with `IgnoreUnknownKeys: true` and `ZeroEmpty: true`, so set both explicitly when you only want to add a converter.

`binder.ParserConfig` has the following fields:

```go
type ParserConfig struct {
    IgnoreUnknownKeys bool
    ParserType        []ParserType
    ZeroEmpty         bool
    SetAliasTag       string // ignored: every binder uses its own tag (query, form, header, ...)
}

type ParserType struct {
    CustomType any
    Converter  func(string) reflect.Value
}
```

- `IgnoreUnknownKeys`: when `false`, a request key that names no field fails the bind with `schema: invalid path "<key>"`. Fiber's default is `true`.
- `ParserType`: registers a `Converter` for the type of `CustomType`. The converter returns `reflect.Value{}` to reject a value, which fails the bind with a `fiber.ConversionError`.
- `ZeroEmpty`: when `true`, an empty value sets the field to its zero value, which for a slice field is one zero element and for a pointer field a non-nil pointer to the zero value. When `false`, an empty value leaves a plain field unchanged and a slice field empty, while a pointer field still gets a non-nil pointer to the zero value. Fiber's default is `true`.

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

customTime := binder.ParserType{
    CustomType: CustomTime{},
    Converter:  timeConverter,
}

// Add custom type to the Decoder settings
binder.SetParserDecoder(binder.ParserConfig{
    IgnoreUnknownKeys: true,
    ParserType:        []binder.ParserType{customTime},
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
```

Run tests with the following `curl` commands:

```bash
# Body Binding
curl -X POST -F title=title -F body=body -F date=2021-10-20 http://localhost:3000/body

# Query Binding
curl -X GET "http://localhost:3000/query?title=title&body=body&date=2021-10-20"
```

## Validation

Validation is also possible with the binding methods. You can specify your validation rules using the `validate` struct tag.

Specify your struct validator in the [config](./fiber.md#structvalidator).

The validator must implement the `StructValidator` interface, which requires a `Validate` method that takes an `any` type and returns an error.

```go title="Interface"
type StructValidator interface {
    Validate(out any) error
}
```

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

Fiber only runs `StructValidator` for struct destinations (or pointers to structs).
Binding into maps and other non-struct types skips the validator step.

Validation runs after a bind succeeds, on the value you passed in. If binding fails, the validator is not called. [`All`](#all) validates once, on the merged struct. The validator's error is returned as is: it is not wrapped in a [`BindError`](#validation-vs-binding-errors), and the default error handler answers it with `500 Internal Server Error` (only [`All`](#all) under [`WithAutoHandling`](#withautohandling) converts it to a `400 Bad Request`), so return a client error yourself when the request is at fault, as shown in the [Validation guide](../guide/validation.md). To bypass the validator, use [`SkipValidation`](#skipvalidation).

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

    return c.JSON(p)
})
```

## Default Fields

You can set default values for fields in the struct with the `default:` option of the field's tag, as in `query:"name,default:john"`. A separate `default` struct tag is not read. Supported types:

- `bool`
- Float variants (`float32`, `float64`)
- Int variants (`int`, `int8`, `int16`, `int32`, `int64`)
- Uint variants (`uint`, `uint8`, `uint16`, `uint32`, `uint64`)
- `string`
- A slice of the above types. Use `|` to separate slice items.
- A pointer to one of the above types (**pointers to slices and slices of pointers are not supported**).

Defaults are applied by the decoder behind the `query`, `form`, `header`, `cookie`, `respHeader`, and `uri` tags. They have no effect on JSON, XML, CBOR, or MsgPack bodies. A default applies when the key is absent from the source. A key that is present but empty, such as `?name=`, does not get it: with Fiber's default `ZeroEmpty` a plain field gets its zero value, a slice field gets one zero element (`?ids=` gives `[0]` for an `[]int`) and a pointer field gets a non-nil pointer to the zero value. The `uri` tag is the exception: the binder supplies every parameter of the matched route, an optional one the request omitted as an empty string, so a default never applies to a route parameter. It applies only to a `uri` name that the route does not declare.

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

## Required Fields

Add the `required` option to a tag to make the bind fail when the key is missing or empty. It works with the `query`, `form`, `header`, `cookie`, `respHeader`, and `uri` tags. On a `uri` tag, a route parameter that the request omitted, such as an optional `:id?`, counts as empty. A field cannot have both `required` and `default`.

```go title="Example"
type Search struct {
    Term string `query:"term,required"`
}
```

A missing `term` fails with a [`*BindError`](#binderror) that wraps a `fiber.MultiError` holding a `fiber.EmptyFieldError` (message `term is empty`). With `WithAutoHandling`, the response is `400 Bad Request` with the message `Bad request: term is empty`.
