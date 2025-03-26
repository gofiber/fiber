# State Management

This document details the state management functionality provided by Fiber, a thread-safe global key-value store used to store application dependencies and runtime data. The implementation is based on Go's `sync.Map` ensuring concurrency safety.

Below is the detailed description of all public methods and usage examples.

## State Type

`State` is a key-value store built on top of `sync.Map`. It allows storage and retrieval of dependencies and configurations in a Fiber application as well as thread-safe access to runtime data.

### Definition

```go
// State is a key-value store for Fiber's app in order to be used as a global storage for the app's dependencies.
// It's a thread-safe implementation of a map[string]any, using sync.Map.
type State struct {
    dependencies sync.Map
}
```

## Methods on State

### Set

Set adds or updates a key-value pair in the State.

```go title="Signature"
func (s *State) Set(key string, value any)
```

**Usage Example:**

```go
app.State().Set("appName", "My Fiber App")
```

### Get

Get retrieves a value from the State.

```go title="Signature"
func (s *State) Get(key string) (any, bool)
```

**Usage Example:**

```go
value, ok := app.State().Get("appName")
if ok {
    fmt.Println("App Name:", value)
}
```

### MustGet

MustGet retrieves a value from the State and panics if the key is not found.

```go title="Signature"
func (s *State) MustGet(key string) any
```

**Usage Example:**

```go
appName := app.State().MustGet("appName")
fmt.Println("App Name:", appName)
```

### GetString

GetString retrieves a string value from the State. It returns the string and a boolean indicating successful type assertion.

```go title="Signature"
func (s *State) GetString(key string) (string, bool)
```

**Usage Example:**

```go
if appName, ok := app.State().GetString("appName"); ok {
    fmt.Println("App Name:", appName)
}
```

### GetInt

GetInt retrieves an integer value from the State. It returns the int and a boolean indicating successful type assertion.

```go title="Signature"
func (s *State) GetInt(key string) (int, bool)
```

**Usage Example:**

```go
if count, ok := app.State().GetInt("userCount"); ok {
    fmt.Printf("User Count: %d\n", count)
}
```

### GetBool

GetBool retrieves a boolean value from the State. It returns the bool and a boolean indicating successful type assertion.

```go title="Signature"
func (s *State) GetBool(key string) (value, ok bool)
```

**Usage Example:**

```go
if debug, ok := app.State().GetBool("debugMode"); ok {
    fmt.Printf("Debug Mode: %v\n", debug)
}
```

### GetFloat64

GetFloat64 retrieves a float64 value from the State. It returns the float64 and a boolean indicating successful type assertion.

```go title="Signature"
func (s *State) GetFloat64(key string) (float64, bool)
```

**Usage Example:**

```go title="Signature"
if ratio, ok := app.State().GetFloat64("scalingFactor"); ok {
    fmt.Printf("Scaling Factor: %f\n", ratio)
}
```

### Has

Has checks if a key exists in the State.

```go title="Signature"
func (s *State) Has(key string) bool
```

**Usage Example:**

```go
if app.State().Has("appName") {
    fmt.Println("App Name is set.")
}
```

### Delete

Delete removes a key-value pair from the State.

```go title="Signature"
func (s *State) Delete(key string)
```

**Usage Example:**

```go
app.State().Delete("obsoleteKey")
```

### Reset

Reset resets the State by removing all keys.

```go title="Signature"
func (s *State) Reset()
```

**Usage Example:**

```go
app.State().Reset()
```

### Keys

Keys returns a slice containing all keys present in the State.

```go title="Signature"
func (s *State) Keys() []string
```

**Usage Example:**

```go
keys := app.State().Keys()
fmt.Println("State Keys:", keys)
```

### Len

Len returns the number of keys in the State.

```go title="Signature"
func (s *State) Len() int
```

**Usage Example:**

```go
fmt.Printf("Total State Entries: %d\n", app.State().Len())
```

## Generic Functions

Fiber provides generic functions to retrieve state values with type-safety and fallback options.

### GetState

GetState retrieves a value from the State and casts it to the desired type. It returns the casted value and a boolean indicating if the cast was successful.

```go title="Signature"
func GetState[T any](s *State, key string) (T, bool)
```

**Usage Example:**

```go title="Signature"
// Retrieve an integer value safely.
userCount, ok := GetState[int](app.State(), "userCount")
if ok {
    fmt.Printf("User Count: %d\n", userCount)
}
```

### MustGetState

MustGetState retrieves a value from the State and casts it to the desired type. It panics if the key is not found or if the type assertion fails.

```go title="Signature"
func MustGetState[T any](s *State, key string) T
```

**Usage Example:**

```go title="Signature"
// Retrieve the value or panic if it is not present.
config := MustGetState[string](app.State(), "configFile")
fmt.Println("Config File:", config)
```

### GetStateWithDefault

GetStateWithDefault retrieves a value from the State, casting it to the desired type. If the key is not present, it returns the provided default value.

```go title="Signature"
func GetStateWithDefault[T any](s *State, key string, defaultVal T) T
```

**Usage Example:**

```go
// Retrieve a value with a default fallback.
requestCount := GetStateWithDefault[int](app.State(), "requestCount", 0)
fmt.Printf("Request Count: %d\n", requestCount)
```

## Comprehensive Examples

### Example: Request Counter

This example demonstrates how to track the number of requests using the State.

```go
package main

import (
    "fmt"
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Initialize state with a counter.
    app.State().Set("requestCount", 0)

    // Middleware: Increase counter for every request.
    app.Use(func(c fiber.Ctx) error {
        count := fiber.GetStateWithDefault(app.State(), "requestCount", 0)
        app.State().Set("requestCount", count+1)
        return c.Next()
    })

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello World!")
    })

    app.Get("/stats", func(c fiber.Ctx) error {
        count := fiber.GetStateWithDefault(c.App().State(), "requestCount", 0)
        return c.SendString(fmt.Sprintf("Total requests: %d", count))
    })

    app.Listen(":3000")
}
```

### Example: Environment-Specific Configuration

This example shows how to configure different settings based on the environment.

```go
package main

import (
    "os"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Determine environment.
    environment := os.Getenv("ENV")
    if environment == "" {
        environment = "development"
    }
    app.State().Set("environment", environment)

    // Set environment-specific configurations.
    if environment == "development" {
    app.State().Set("apiUrl", "http://localhost:8080/api")
        app.State().Set("debug", true)
    } else {
        app.State().Set("apiUrl", "https://api.production.com")
        app.State().Set("debug", false)
    }

    app.Get("/config", func(c fiber.Ctx) error {
        config := map[string]any{
            "environment": environment,
            "apiUrl":      fiber.GetStateWithDefault(app.State(), "apiUrl", ""),
            "debug":       fiber.GetStateWithDefault(app.State(), "debug", false),
        }
        return c.JSON(config)
    })

    app.Listen(":3000")
}
```

### Example: Dependency Injection with State Management

This example demonstrates how to use the State for dependency injection in a Fiber application.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/redis/go-redis/v9"
)

type User struct {
    ID    int    `query:"id"`
    Name  string `query:"name"`
    Email string `query:"email"`
}

func main() {
    app := fiber.New()
    ctx := context.Background()

    // Initialize Redis client.
    rdb := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
    })

    // Check the Redis connection.
    if err := rdb.Ping(ctx).Err(); err != nil {
        log.Fatalf("Could not connect to Redis: %v", err)
    }

    // Inject the Redis client into Fiber's State for dependency injection.
    app.State().Set("redis", rdb)

    app.Get("/user/create", func(c fiber.Ctx) error {
        var user User
        if err := c.Bind().Query(&user); err != nil {
            return c.Status(fiber.StatusBadRequest).SendString(err.Error())
        }

        // Save the user to the database.
        rdb, ok := fiber.GetState[*redis.Client](app.State(), "redis")
        if !ok {
            return c.Status(fiber.StatusInternalServerError).SendString("Redis client not found")
        }

        // Save the user to the database.
        key := fmt.Sprintf("user:%d", user.ID)
        err := rdb.HSet(ctx, key, "name", user.Name, "email", user.Email).Err()
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }

        return c.JSON(user)
    })

    app.Get("/user/:id", func(c fiber.Ctx) error {
        id := c.Params("id")

        rdb, ok := fiber.GetState[*redis.Client](app.State(), "redis")
        if !ok {
            return c.Status(fiber.StatusInternalServerError).SendString("Redis client not found")
        }

        key := fmt.Sprintf("user:%s", id)
        user, err := rdb.HGetAll(ctx, key).Result()
        if err == redis.Nil {
            return c.Status(fiber.StatusNotFound).SendString("User not found")
        } else if err != nil {
            return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }

        return c.JSON(user)
    })

    app.Listen(":3000")
}
```
