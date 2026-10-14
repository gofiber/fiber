---
id: services
title: 🧩 Services
sidebar_position: 9
---

Services wrap external dependencies. List them in the application's configuration, and Fiber starts and stops them automatically—useful during development and testing.

Fiber starts the configured services while `fiber.New` runs and stops them during shutdown. Retrieve a started service from state with `GetService` or `MustGetService`, using the name its `String` method returns (see [State Management](./state)).

## Service Interface

The `Service` interface defines methods a service must implement.

### Definition

```go
type Service interface {
    // Start starts the service, returning an error if it fails.
    Start(ctx context.Context) error

    // String returns a string representation of the service.
    // It is used to print a human-readable name of the service in the startup message.
    String() string

    // State returns the current state of the service.
    State(ctx context.Context) (string, error)

    // Terminate terminates the service, returning an error if it fails.
    Terminate(ctx context.Context) error
}
```

## Service Methods

### Start

Starts the service. Fiber calls this while `fiber.New` runs, with the context returned by `ServicesStartupContextProvider`. See [Startup](#startup) for what happens when it returns an error.

```go
func (s *SomeService) Start(ctx context.Context) error
```

### String

Returns a string representation of the service, used to print the service in the startup message and as the key the service is registered under in the application state. Names must be unique across the configured services: `fiber.New` panics when two services report the same name, since one would otherwise overwrite the other and never be terminated.

```go
func (s *SomeService) String() string
```

### State

Reports the current state of the service for the startup message. Fiber calls it with the startup context when it prints the message, which shows the returned value in upper case, or `ERROR` when the call returns an error.

```go
func (s *SomeService) State(ctx context.Context) (string, error)
```

### Terminate

Stops the service after the application shuts down using a post-shutdown hook, with the context returned by `ServicesShutdownContextProvider`. Services are terminated in reverse start order, so a service can rely on the ones started before it still being available while it shuts down. See [Shutdown](#shutdown) for what happens when it returns an error.

```go
func (s *SomeService) Terminate(ctx context.Context) error
```

## Configuration

Services are configured with these `fiber.Config` fields:

| Property | Type | Description | Default |
| --- | --- | --- | --- |
| `Services` | `[]fiber.Service` | Services that `fiber.New` starts and that Fiber terminates on shutdown. | `nil` |
| `ServicesStartupContextProvider` | `func() context.Context` | Returns the context passed to `Start`. Fiber also passes it to `State` when it prints the startup message. | `context.Background()` |
| `ServicesShutdownContextProvider` | `func() context.Context` | Returns the context passed to `Terminate`. | `context.Background()` |

Fiber calls a provider each time it needs a context, so build a context that carries a deadline inside the provider. A context created ahead of time may already be expired when the application shuts down, and `Terminate` would then never run.

## Lifecycle

### Startup

`fiber.New` starts the services one after the other, in the order of `Config.Services`, and registers each started service in the application's [state](./state). It panics in these cases:

- The slice holds a `nil` entry or two services whose `String` method returns the same name. Nothing is started.
- A `Start` call returns an error. Fiber still tries the remaining services, then panics with all the collected errors, unless the error wraps `context.Canceled` or `context.DeadlineExceeded`: then startup stops at once and the panic carries only that error.
- The startup context is canceled or reaches its deadline. Startup stops at once and the panic carries the context error.

:::warning
With `ListenConfig.EnablePrefork`, the master process and every child process run your `main` function, and with it `fiber.New`, so each process starts its own copy of every service.
:::

### Shutdown

Fiber registers a post-shutdown hook while `fiber.New` runs, so `Shutdown`, `ShutdownWithTimeout`, `ShutdownWithContext`, and a graceful shutdown started through `ListenConfig.GracefulContext` all terminate the services. The hook runs before any `OnPostShutdown` hook you register afterwards. When `Config.SharedStorage` is set, the hook closes it once the services are done (see [SharedState](./state#sharedstate-prefork-safe)).

Services are terminated in reverse start order, and a terminated service is removed from the state. Termination is best effort:

- If `Terminate` returns an error, Fiber logs it (`Shutdown` does not return it) and goes on with the remaining services. The failed service stays registered, so a later shutdown retries it.
- Once the shutdown context is done, the remaining services are not terminated, and the logged error carries the context error for each of them.
- Each started service is terminated once, even when shutdown calls overlap.

## Comprehensive Examples

### Example: Adding a Service

This example demonstrates how to add a Redis store as a service to the application, backed by the Testcontainers Redis Go module.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/redis/go-redis/v9"
    tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const redisServiceName = "redis-store"

type redisService struct {
    ctr *tcredis.RedisContainer
}

// Start initializes and starts the service. It implements the [fiber.Service] interface.
func (s *redisService) Start(ctx context.Context) error {
    // start the service
    c, err := tcredis.Run(ctx, "redis:latest")
    if err != nil {
        return err
    }

    s.ctr = c
    return nil
}

// String returns a string representation of the service.
// It is used to print a human-readable name of the service in the startup message.
// It implements the [fiber.Service] interface.
func (s *redisService) String() string {
    return redisServiceName
}

// State returns the current state of the service.
// It implements the [fiber.Service] interface.
func (s *redisService) State(ctx context.Context) (string, error) {
    state, err := s.ctr.State(ctx)
    if err != nil {
        return "", fmt.Errorf("container state: %w", err)
    }

    // Status is a named string type (container.ContainerState) since testcontainers-go v0.42.0.
    return string(state.Status), nil
}

// Terminate stops and removes the service. It implements the [fiber.Service] interface.
func (s *redisService) Terminate(ctx context.Context) error {
    // stop the service
    return s.ctr.Terminate(ctx)
}

func main() {
    cfg := &fiber.Config{}

    // Initialize service.
    cfg.Services = append(cfg.Services, &redisService{})

    // Define a context provider for the services startup.
    // This is useful to cancel the startup of the services if the context is canceled.
    // Fiber calls the provider to start the services and again when it prints the startup
    // message, so create the context inside it to give each call a fresh timeout.
    // Default is context.Background().
    cfg.ServicesStartupContextProvider = func() context.Context {
        startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
        context.AfterFunc(startupCtx, cancelStartup) // the deadline releases the timer; this also keeps go vet from reporting the unused cancel function
        return startupCtx
    }

    // Define a context provider for the services shutdown.
    // This is useful to cancel the shutdown of the services if the context is canceled.
    // Fiber calls the provider from the post-shutdown hook, once the server has stopped, so
    // create the context inside it: a context created here would already be expired if the
    // app ran longer than the timeout.
    // Default is context.Background().
    cfg.ServicesShutdownContextProvider = func() context.Context {
        shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
        context.AfterFunc(shutdownCtx, cancelShutdown) // the deadline releases the timer; this also keeps go vet from reporting the unused cancel function
        return shutdownCtx
    }

    app := fiber.New(*cfg)

    ctx := context.Background()

    // Obtain the Redis service from the application's State.
    redisSrv, ok := fiber.GetService[*redisService](app.State(), redisServiceName)
    if !ok || redisSrv == nil {
        log.Printf("Redis service not found")
        return
    }

    // Obtain the connection string from the service.
    connString, err := redisSrv.ctr.ConnectionString(ctx)
    if err != nil {
        log.Printf("Could not get connection string: %v", err)
        return
    }

    // Parse the connection string to create a Redis client.
    options, err := redis.ParseURL(connString)
    if err != nil {
        log.Printf("failed to parse connection string: %s", err)
        return
    }

    // Initialize the Redis client.
    rdb := redis.NewClient(options)

    // Check the Redis connection.
    if err := rdb.Ping(ctx).Err(); err != nil {
        log.Fatalf("Could not connect to Redis: %v", err)
    }

    app.Listen(":3000")
}

```

### Example: Add a service with the Store middleware

This example shows how to use services with the Store middleware for dependency injection. It uses a Redis store backed by the Testcontainers Redis module.

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/logger"
    redisStore "github.com/gofiber/storage/redis/v3"
    tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
    redisServiceName = "redis-store"
)

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type redisService struct {
    ctr *tcredis.RedisContainer
}

// Start initializes and starts the service. It implements the [fiber.Service] interface.
func (s *redisService) Start(ctx context.Context) error {
    // start the service
    c, err := tcredis.Run(ctx, "redis:latest")
    if err != nil {
        return err
    }

    s.ctr = c
    return nil
}

// String returns a string representation of the service.
// It is used to print a human-readable name of the service in the startup message.
// It implements the [fiber.Service] interface.
func (s *redisService) String() string {
    return redisServiceName
}

// State returns the current state of the service.
// It implements the [fiber.Service] interface.
func (s *redisService) State(ctx context.Context) (string, error) {
    state, err := s.ctr.State(ctx)
    if err != nil {
        return "", fmt.Errorf("container state: %w", err)
    }

    // Status is a named string type (container.ContainerState) since testcontainers-go v0.42.0.
    return string(state.Status), nil
}

// Terminate stops and removes the service. It implements the [fiber.Service] interface.
func (s *redisService) Terminate(ctx context.Context) error {
    // stop the service
    return s.ctr.Terminate(ctx)
}

func main() {
    cfg := &fiber.Config{}

    // Initialize service.
    cfg.Services = append(cfg.Services, &redisService{})

    // Define a context provider for the services startup.
    // This is useful to cancel the startup of the services if the context is canceled.
    // Fiber calls the provider to start the services and again when it prints the startup
    // message, so create the context inside it to give each call a fresh timeout.
    // Default is context.Background().
    cfg.ServicesStartupContextProvider = func() context.Context {
        startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
        context.AfterFunc(startupCtx, cancelStartup) // the deadline releases the timer; this also keeps go vet from reporting the unused cancel function
        return startupCtx
    }

    // Define a context provider for the services shutdown.
    // This is useful to cancel the shutdown of the services if the context is canceled.
    // Fiber calls the provider from the post-shutdown hook, once the server has stopped, so
    // create the context inside it: a context created here would already be expired if the
    // app ran longer than the timeout.
    // Default is context.Background().
    cfg.ServicesShutdownContextProvider = func() context.Context {
        shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
        context.AfterFunc(shutdownCtx, cancelShutdown) // the deadline releases the timer; this also keeps go vet from reporting the unused cancel function
        return shutdownCtx
    }

    app := fiber.New(*cfg)

    // Initialize default config
    app.Use(logger.New())

    ctx := context.Background()

    // Obtain the Redis service from the application's State.
    redisSrv, ok := fiber.GetService[*redisService](app.State(), redisServiceName)
    if !ok || redisSrv == nil {
        log.Printf("Redis service not found")
        return
    }

    // Obtain the connection string from the service.
    connString, err := redisSrv.ctr.ConnectionString(ctx)
    if err != nil {
        log.Printf("Could not get connection string: %v", err)
        return
    }

    // define a GoFiber session store, backed by the Redis service
    store := redisStore.New(redisStore.Config{
        URL: connString,
    })

    app.Post("/user/create", func(c fiber.Ctx) error {
        var user User
        if err := c.Bind().JSON(&user); err != nil {
            return c.Status(fiber.StatusBadRequest).SendString(err.Error())
        }

        json, err := json.Marshal(user)
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }

        // Save the user to the database.
        err = store.Set(user.Email, json, time.Hour*24)
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }

        return c.JSON(user)
    })

    app.Get("/user/:id", func(c fiber.Ctx) error {
        id := c.Params("id")

        user, err := store.Get(id)
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }

        // The storage returns nil, nil when the key does not exist.
        if user == nil {
            return c.Status(fiber.StatusNotFound).SendString("User not found")
        }

        return c.JSON(string(user))
    })

    app.Listen(":3000")
}

```
