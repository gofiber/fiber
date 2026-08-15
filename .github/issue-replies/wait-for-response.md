Thanks for the report. To take this further we need a **minimal reproducible example**: the smallest program that still shows the behaviour.

Please share:

- a runnable `main.go`, not a fragment - imports, `app := fiber.New()`, the route, and `app.Listen`
- the request you send (a `curl` line is ideal) and what you expected instead
- your Fiber version (`go list -m github.com/gofiber/fiber/v3`) and `go version`

If the behaviour only shows up with a middleware or a storage driver, keep that in and drop everything else.
