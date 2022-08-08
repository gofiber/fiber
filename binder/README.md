# Fiber Binders

Binder is new request/response binding feature for Fiber. By aganist old Fiber parsers, it supports custom binder registration, struct validation, **map[string]string**, **map[string][]string** and more. It's introduced in Fiber v3 and a replacement of:
- BodyParser
- ParamsParser
- GetReqHeaders
- GetRespHeaders
- AllParams
- QueryParser
- ReqHeaderParser


## Default Binders
- [Form](form.go)
- [Query](query.go)
- [URI](uri.go)
- [Header](header.go)
- [Response Header](resp_header.go)
- [Cookie](cookie.go)
- [JSON](json.go)
- [XML](xml.go)

## Guides

### Binding into the Struct
Fiber supports binding into the struct with [gorilla/schema](https://github.com/gorilla/schema). Here's an example for it:
```go
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

// Run tests with the following curl commands:

// curl -X POST -H "Content-Type: application/json" --data "{\"name\":\"john\",\"pass\":\"doe\"}" localhost:3000

// curl -X POST -H "Content-Type: application/xml" --data "<login><name>john</name><pass>doe</pass></login>" localhost:3000

// curl -X POST -H "Content-Type: application/x-www-form-urlencoded" --data "name=john&pass=doe" localhost:3000

// curl -X POST -F name=john -F pass=doe http://localhost:3000

// curl -X POST "http://localhost:3000/?name=john&pass=doe"
```

### Binding into the Map
Fiber supports binding into the **map[string]string** or **map[string][]string**. Here's an example for it:
```go
app.Get("/", func(c fiber.Ctx) error {
        p := make(map[string][]string)

        if err := c.Bind().Query(p); err != nil {
            return err
        }

        log.Println(p["name"])     // john
        log.Println(p["pass"])     // doe
        log.Println(p["products"]) // [shoe, hat]

        // ...
})
// Run tests with the following curl command:

// curl "http://localhost:3000/?name=john&pass=doe&products=shoe,hat"
```
### Behaviors of Should/Must
Normally, Fiber returns binder error directly. However; if you want to handle it automatically, you can prefer `Must()`. 

If there's an error it'll return error and 400 as HTTP status. Here's an example for it:
```go
// Field names should start with an uppercase letter
type Person struct {
    Name string `json:"name,required"`
    Pass string `json:"pass"`
}

app.Get("/", func(c fiber.Ctx) error {
        p := new(Person)

        if err := c.Bind().Must().JSON(p); err != nil {
            return err 
            // Status code: 400 
            // Response: Bad request: name is empty
        }

        // ...
})

// Run tests with the following curl command:

// curl -X GET -H "Content-Type: application/json" --data "{\"pass\":\"doe\"}" localhost:3000
```
### Defining Custom Binder
We didn't add much binder to make Fiber codebase minimal. But if you want to use your binders, it's easy to register and use them. Here's an example for TOML binder.
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

		// or you can use like:
		// if err := c.Bind().Custom("toml", out); err != nil {
		// 	 return err
		// }

		return c.SendString(out.Pass) // test
	})

	app.Listen(":3000")
}

// curl -X GET -H "Content-Type: application/toml" --data "name = 'bar'
// pass = 'test'" localhost:3000
```
### Defining Custom Validator
All Fiber binders supporting struct validation if you defined validator inside of the config. You can create own validator, or use [go-playground/validator](https://github.com/go-playground/validator), [go-ozzo/ozzo-validation](https://github.com/go-ozzo/ozzo-validation)... Here's an example of simple custom validator:
```go
type Query struct {
	Name string `query:"name"`
}

type structValidator struct{}

func (v *structValidator) Engine() any {
	return ""
}

func (v *structValidator) ValidateStruct(out any) error {
	out = reflect.ValueOf(out).Elem().Interface()
	sq := out.(Query)

	if sq.Name != "john" {
		return errors.New("you should have entered right name!")
	}

	return nil
}

func main() {
	app := fiber.New(fiber.Config{StructValidator: &structValidator{}})

	app.Get("/", func(c fiber.Ctx) error {
		out := new(Query)
		if err := c.Bind().Query(out); err != nil {
			return err // you should have entered right name!
		}
		return c.SendString(out.Name)
	})

	app.Listen(":3000")
}

// Run tests with the following curl command:

// curl "http://localhost:3000/?name=efe"
```