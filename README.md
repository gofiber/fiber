<p align="center">
  <img height="150" src="https://gofiber.github.io/fiber/static/logo.jpg">
</p>

# Fiber ![](https://img.shields.io/github/release/gofiber/fiber) ![](https://img.shields.io/github/issues/gofiber/fiber) ![](https://img.shields.io/github/stars/gofiber/fiber) ![](https://godoc.org/github.com/valyala/fasthttp?status.svg) ![](https://goreportcard.com/badge/github.com/gofiber/fiber) ![](https://img.shields.io/github/languages/top/gofiber/fiber) ![](https://img.shields.io/github/languages/code-size/gofiber/fiber)

**[Fiber](https://github.com/gofiber/fiber)** is an **[Express](https://expressjs.com/en/4x/api.html)** style HTTP framework implementation running on **[FastHTTP](https://github.com/valyala/fasthttp)**, the **fastest** HTTP engine for **[Go](https://golang.org/doc/)**. The package make use of similar framework convention as they are in express. People switching from **[Nodejs](https://nodejs.org/en/about/)** to **[Go](https://golang.org/doc/)** often end up in a bad learning curve to start building their webapps, this project is meant to **ease** things up for **fast** development, but with **zero memory allocation** and **performance** in mind.

![](https://gofiber.github.io/fiber/static/benchmarks/benchmark-pipeline.png?v=12) 
## Features
* Optimized for speed and low memory usage.
* Rapid Server-Side Programming
* Easy routing with parameters
* Static files with custom prefix
* Middleware support
* Supports Express API endpoints
* Extended [API Documentation](https://gofiber.github.io/fiber/)

## API Documentation
**[Click here](https://gofiber.github.io/fiber/)**

## Installing
Assuming you’ve already installed **[Go](https://golang.org/doc/)**, install the **[Fiber](https://github.com/gofiber/fiber)** package by calling the following command:
```bash
$ go get -u github.com/gofiber/fiber
```

## Hello world
Embedded below is essentially the simplest Fiber app you can create.
```bash
$ create server.go
```
```go
package main

import "github.com/gofiber/fiber"

func main() {
  app := fiber.New()
  app.Get("/", func(c *fiber.Ctx) {
    c.Send("Hello, World!")
  })
  app.Listen(8080)
}
```
```bash
$ go run server.go
```
Browse to **http://localhost:8080** and you should see Hello, World! on the page.

## Static files
To serve static files such as images, CSS files, and JavaScript files, replace your function handler with a file or directory string.
```go
// Function signature
app.Static(root string)
app.Static(prefix, root string)
```
For example, use the following code to serve images, CSS files, and JavaScript files in a directory named public:

```go
app.Static("./public")
```
Now, you can load the files that are in the public directory:
```shell
http://localhost:8080/images/kitten.jpg
http://localhost:8080/css/style.css
http://localhost:8080/js/app.js
http://localhost:8080/images/bg.png
http://localhost:8080/hello.html
```
To use multiple static assets directories, call the express.static middleware function multiple times:
```go
app.Static("./public")
app.Static("./files")
```
?>For best results, use a reverse proxy cache like [NGINX](https://www.nginx.com/resources/wiki/start/topics/examples/reverseproxycachingexample/) to improve performance of serving static assets.  

To create a virtual path prefix (where the path does not actually exist in the file system) for files that are served by the express.static function, specify a mount path for the static directory, as shown below:
```go
app.Static("/static", "./public")
```
Now, you can load the files that are in the public directory from the /static path prefix.
```shell
http://localhost:8080/static/images/kitten.jpg
http://localhost:8080/static/css/style.css
http://localhost:8080/static/js/app.js
http://localhost:8080/static/images/bg.png
http://localhost:8080/static/hello.html
```

*Caught a mistake? [Edit this page on GitHub!](https://github.com/gofiber/fiber/blob/master/README.md)*
