# Adaptor
Adaptor for [Fiber](https://github.com/gofiber/fiber) Converter for net/http handlers to Fiber handlers, special thanks to [@arsmn](https://github.com/arsmn)!.

- [Signatures](#signatures)
- [Examples](#examples)

### Signatures
```go
func HTTPHandlerFunc(h http.HandlerFunc) fiber.Handler
func HTTPHandler(h http.Handler) fiber.Handler
```

### Example
Import the adaptor package that is part of the Fiber web framework
```go
import (
    "net/http"
    
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
func main() {
	app := fiber.New()

	// http.Handler -> fiber.Handler
	app.Get("/", adaptor.HTTPHandler(handler(greet)))

	// http.HandlerFunc -> fiber.Handler
	app.Get("/func", adaptor.HTTPHandlerFunc(greet))

	log.Fatal(app.Listen(":3000"))
}

func handler(f http.HandlerFunc) http.Handler {
	return http.HandlerFunc(f)
}

func greet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello World!")
}
```