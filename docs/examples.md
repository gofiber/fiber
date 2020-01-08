# Examples

## Multiple File Upload
```go

package main

import "github.com/fenny/fiber"

func main() {
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) {
		// Parse the multipart form
		if form := c.Form(); form != nil {
			// => *multipart.Form

			// Get all files from "documents" key
			files := form.File["documents"]
			// => []*multipart.FileHeader

			// Loop trough files
			for _, file := range files {
				fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
				// => "tutorial.pdf" 360641 "application/pdf"

				// Save the files to disk
				c.SaveFile(file, fmt.Sprintf("./%s", file.Filename))
			}
		}
	})
	app.Listen(8080)
}
```
## 404 Handling
```go
package main

import "github.com/fenny/fiber"

func main() {
	app := fiber.New()

	app.Get("./static")
	app.Get(notFound)

	app.Listen(8080)
}

func notFound(c *fiber.Ctx) {
	c.Status(404).Send("Not Found")
}
```
## Static Caching
```go
package main

import "github.com/fenny/fiber"

func main() {
	app := fiber.New()
	app.Get(cacheControl)
	app.Get("./static")
	app.Listen(8080)
}

func cacheControl(c *fiber.Ctx) {
	c.Set("Cache-Control", "max-age=2592000, public")
	c.Next()
}
```
## Enable CORS
```go
package main

import "./fiber"

func main() {
	app := fiber.New()

	app.All("/api", enableCors)
	app.Get("/api", apiHandler)

	app.Listen(8080)
}

func enableCors(c *fiber.Ctx) {
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "X-Requested-With")
	c.Next()
}
func apiHandler(c *fiber.Ctx) {
	c.Send("Hi, I'm API!")
}
```
## Returning JSON
```go
package main

import "./fiber"

type Data struct {
	Name string
	Age  int
}

func main() {
app := fiber.New()
	app.Get("/json", func(c *fiber.Ctx) {
		data := SomeData{
			Name: "John",
			Age:  20,
		}
		c.Json(data)
		// or
		err := c.Json(data)
		if err != nil {
			c.Send("Something went wrong!")
		}
	})
	app.Listen(8080)
}
```

*Caught a mistake? [Edit this page on GitHub!](https://github.com/Fenny/fiber/blob/master/docs/examples.md)*
