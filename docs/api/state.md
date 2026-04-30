---
id: state
title: 🗂️ State Management
sidebar_position: 8
---

State management provides a global key–value store for application dependencies and runtime data. The store is shared across the entire application and persists between requests. It's commonly used to store [Services](../api/services), which you can retrieve with the `GetService` or `MustGetService` functions.

:::warning
When prefork is enabled, each worker process has an independent state store, meaning state is not shared between them.
:::

## SharedState (Prefork-safe)

For data that must be shared across prefork workers or multiple app processes, use `app.SharedState()` backed by `fiber.Storage`.

Configure storage in `fiber.Config`:

```go
app := fiber.New(fiber.Config{
    AppName:           "billing-api",
    SharedStorage:     redisStorage, // any implementation of fiber.Storage
    SharedStatePrefix: "billing-shared-", // optional
})
```

If `SharedStatePrefix` is empty, Fiber derives a default namespace and includes `AppName` (when set) to reduce collisions between apps/services.

MsgPack and CBOR helpers require the corresponding `Config` encoders/decoders to be configured. If they are unavailable, the helper methods return an error instead of panicking.

:::warning Memory storage caveat
`SharedState` is only cross-worker / cross-process when the configured `SharedStorage` backend is shared.

If you use an in-memory backend (for example memory storage), data remains process-local. In prefork mode, each worker process has its own independent in-memory store.
:::

### SharedState Methods

```go title="Signature"
func (app *App) SharedState() *SharedState

func (s *SharedState) Set(key string, val []byte, ttl time.Duration) error
func (s *SharedState) SetWithContext(ctx context.Context, key string, val []byte, ttl time.Duration) error

func (s *SharedState) Get(key string) (val []byte, found bool, err error)
func (s *SharedState) GetWithContext(ctx context.Context, key string) (val []byte, found bool, err error)

func (s *SharedState) SetJSON(key string, v any, ttl time.Duration) error
func (s *SharedState) SetJSONWithContext(ctx context.Context, key string, v any, ttl time.Duration) error

func (s *SharedState) GetJSON(key string, out any) (raw []byte, found bool, err error)
func (s *SharedState) GetJSONWithContext(ctx context.Context, key string, out any) (raw []byte, found bool, err error)

func (s *SharedState) SetMsgPack(key string, v any, ttl time.Duration) error
func (s *SharedState) SetMsgPackWithContext(ctx context.Context, key string, v any, ttl time.Duration) error

func (s *SharedState) GetMsgPack(key string, out any) (raw []byte, found bool, err error)
func (s *SharedState) GetMsgPackWithContext(ctx context.Context, key string, out any) (raw []byte, found bool, err error)

func (s *SharedState) SetCBOR(key string, v any, ttl time.Duration) error
func (s *SharedState) SetCBORWithContext(ctx context.Context, key string, v any, ttl time.Duration) error

func (s *SharedState) GetCBOR(key string, out any) (raw []byte, found bool, err error)
func (s *SharedState) GetCBORWithContext(ctx context.Context, key string, out any) (raw []byte, found bool, err error)

func (s *SharedState) SetXML(key string, v any, ttl time.Duration) error
func (s *SharedState) SetXMLWithContext(ctx context.Context, key string, v any, ttl time.Duration) error

func (s *SharedState) GetXML(key string, out any) (raw []byte, found bool, err error)
func (s *SharedState) GetXMLWithContext(ctx context.Context, key string, out any) (raw []byte, found bool, err error)

func (s *SharedState) Delete(key string) error
func (s *SharedState) DeleteWithContext(ctx context.Context, key string) error

func (s *SharedState) Has(key string) (bool, error)
func (s *SharedState) HasWithContext(ctx context.Context, key string) (bool, error)

func (s *SharedState) Reset() error
func (s *SharedState) ResetWithContext(ctx context.Context) error
func (s *SharedState) Close() error
```

### SharedState Example

```go
type SessionSnapshot struct {
    UserID    string    `json:"user_id"`
    UpdatedAt time.Time `json:"updated_at"`
}

app.Post("/sessions/:id", func(c fiber.Ctx) error {
    key := "session:" + c.Params("id")
    value := SessionSnapshot{
        UserID:    c.Params("id"),
        UpdatedAt: time.Now().UTC(),
    }

    if err := app.SharedState().SetJSON(key, value, 30*time.Minute); err != nil {
        return err
    }

    return c.SendStatus(fiber.StatusAccepted)
})

app.Get("/sessions/:id", func(c fiber.Ctx) error {
    key := "session:" + c.Params("id")
    var snapshot SessionSnapshot

    _, found, err := app.SharedState().GetJSON(key, &snapshot)
    if err != nil {
        return err
    }
    if !found {
        return c.SendStatus(fiber.StatusNotFound)
    }

    return c.JSON(snapshot)
})
```

### SharedState with Context (timeouts/cancellation)

```go
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()

err := app.SharedState().SetJSONWithContext(ctx, "job:42", fiber.Map{
    "status": "queued",
}, 2*time.Minute)
if err != nil {
    // timeout, cancellation, storage error, or JSON serialization error
}
```

## State Type

`State` is a key–value store built on top of `sync.Map` to ensure safe concurrent access. It allows storage and retrieval of dependencies and configurations in a Fiber application as well as thread–safe access to runtime data.

### Definition

```go
// State is a key–value store for Fiber's app, used as a global storage for the app's dependencies.
// It is a thread–safe implementation of a map[string]any, using sync.Map.
type State struct {
    dependencies sync.Map
}
```

## Methods on State

### Set

Set adds or updates a key–value pair in the State.

```go
// Set adds or updates a key–value pair in the State.
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

Delete removes a key–value pair from the State.

```go title="Signature"
func (s *State) Delete(key string)
```

**Usage Example:**

```go
app.State().Delete("obsoleteKey")
```

### Reset

Reset removes all keys from the State, including those related to Services.

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

```go
// Len returns the number of keys in the State.
func (s *State) Len() int
```

**Usage Example:**

```go
fmt.Printf("Total State Entries: %d\n", app.State().Len())
```

### GetString

GetString retrieves a string value from the State. It returns the string and a boolean indicating a successful type assertion.

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

GetInt retrieves an integer value from the State. It returns the int and a boolean indicating a successful type assertion.

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

GetBool retrieves a boolean value from the State. It returns the bool and a boolean indicating a successful type assertion.

```go title="Signature"
func (s *State) GetBool(key string) (value, bool)
```

**Usage Example:**

```go
if debug, ok := app.State().GetBool("debugMode"); ok {
    fmt.Printf("Debug Mode: %v\n", debug)
}
```

### GetFloat64

GetFloat64 retrieves a float64 value from the State. It returns the float64 and a boolean indicating a successful type assertion.

```go title="Signature"
func (s *State) GetFloat64(key string) (float64, bool)
```

**Usage Example:**

```go title="Signature"
if ratio, ok := app.State().GetFloat64("scalingFactor"); ok {
    fmt.Printf("Scaling Factor: %f\n", ratio)
}
```

### GetUint

GetUint retrieves a `uint` value from the State.

```go title="Signature"
func (s *State) GetUint(key string) (uint, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetUint("maxConnections"); ok {
    fmt.Printf("Max Connections: %d\n", val)
}
```

### GetInt8

GetInt8 retrieves an `int8` value from the State.

```go title="Signature"
func (s *State) GetInt8(key string) (int8, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetInt8("threshold"); ok {
    fmt.Printf("Threshold: %d\n", val)
}
```

### GetInt16

GetInt16 retrieves an `int16` value from the State.

```go title="Signature"
func (s *State) GetInt16(key string) (int16, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetInt16("minValue"); ok {
    fmt.Printf("Minimum Value: %d\n", val)
}
```

### GetInt32

GetInt32 retrieves an `int32` value from the State.

```go title="Signature"
func (s *State) GetInt32(key string) (int32, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetInt32("portNumber"); ok {
    fmt.Printf("Port Number: %d\n", val)
}
```

### GetInt64

GetInt64 retrieves an `int64` value from the State.

```go title="Signature"
func (s *State) GetInt64(key string) (int64, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetInt64("fileSize"); ok {
    fmt.Printf("File Size: %d\n", val)
}
```

### GetUint8

GetUint8 retrieves a `uint8` value from the State.

```go title="Signature"
func (s *State) GetUint8(key string) (uint8, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetUint8("byteValue"); ok {
    fmt.Printf("Byte Value: %d\n", val)
}
```

### GetUint16

GetUint16 retrieves a `uint16` value from the State.

```go title="Signature"
func (s *State) GetUint16(key string) (uint16, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetUint16("limit"); ok {
    fmt.Printf("Limit: %d\n", val)
}
```

### GetUint32

GetUint32 retrieves a `uint32` value from the State.

```go title="Signature"
func (s *State) GetUint32(key string) (uint32, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetUint32("timeout"); ok {
    fmt.Printf("Timeout: %d\n", val)
}
```

### GetUint64

GetUint64 retrieves a `uint64` value from the State.

```go title="Signature"
func (s *State) GetUint64(key string) (uint64, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetUint64("maxSize"); ok {
    fmt.Printf("Max Size: %d\n", val)
}
```

### GetUintptr

GetUintptr retrieves a `uintptr` value from the State.

```go title="Signature"
func (s *State) GetUintptr(key string) (uintptr, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetUintptr("pointerValue"); ok {
    fmt.Printf("Pointer Value: %d\n", val)
}
```

### GetFloat32

GetFloat32 retrieves a `float32` value from the State.

```go title="Signature"
func (s *State) GetFloat32(key string) (float32, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetFloat32("scalingFactor32"); ok {
    fmt.Printf("Scaling Factor (float32): %f\n", val)
}
```

### GetComplex64

GetComplex64 retrieves a `complex64` value from the State.

```go title="Signature"
func (s *State) GetComplex64(key string) (complex64, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetComplex64("complexVal"); ok {
    fmt.Printf("Complex Value (complex64): %v\n", val)
}
```

### GetComplex128

GetComplex128 retrieves a `complex128` value from the State.

```go title="Signature"
func (s *State) GetComplex128(key string) (complex128, bool)
```

**Usage Example:**

```go
if val, ok := app.State().GetComplex128("complexVal128"); ok {
    fmt.Printf("Complex Value (complex128): %v\n", val)
}
```

## Generic Functions

Fiber provides generic functions to retrieve state values with type safety and fallback options.

### GetState

GetState retrieves a value from the State and casts it to the desired type. It returns the cast value and a boolean indicating if the cast was successful.

```go title="Signature"
func GetState[T any](s *State, key string) (T, bool)
```

**Usage Example:**

```go
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

```go
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
// Retrieve a value with a fallback.
requestCount := GetStateWithDefault[int](app.State(), "requestCount", 0)
fmt.Printf("Request Count: %d\n", requestCount)
```

### GetService

GetService retrieves a Service from the State and casts it to the desired type. It returns the cast value and a boolean indicating if the cast was successful.

```go title="Signature"
func GetService[T Service](s *State, key string) (T, bool) {
```

**Usage Example:**

```go
if srv, ok := fiber.GetService[*redisService](app.State(), "someService")
    fmt.Printf("Some Service: %s\n", srv.String())
}
```

### MustGetService

MustGetService retrieves a Service from the State and casts it to the desired type. It panics if the key is not found or if the type assertion fails.

```go title="Signature"
func MustGetService[T Service](s *State, key string) T
```

**Usage Example:**

```go
srv := fiber.MustGetService[*SomeService](app.State(), "someService")
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
        count, _ := c.App().State().GetInt("requestCount")
        app.State().Set("requestCount", count+1)
        return c.Next()
    })

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello World!")
    })

    app.Get("/stats", func(c fiber.Ctx) error {
        count, _ := c.App().State().Get("requestCount")
        return c.SendString(fmt.Sprintf("Total requests: %d", count))
    })

    app.Listen(":3000")
}
```

### Example: Environment–Specific Configuration

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
            "apiUrl":      fiber.GetStateWithDefault(c.App().State(), "apiUrl", ""),
            "debug":       fiber.GetStateWithDefault(c.App().State(), "debug", false),
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

        // Retrieve the Redis client from the global state.
        rdb, ok := fiber.GetState[*redis.Client](c.App().State(), "redis")
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

        rdb, ok := fiber.GetState[*redis.Client](c.App().State(), "redis")
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
