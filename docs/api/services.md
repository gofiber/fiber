---
id: services
title: 🥡 Services
sidebar_position: 9
---

Services provide external services needed to run the application. They are supposed to be used while developing and testing the application.

## Service Interface

`Service` is an interface that defines the methods for a service.

### Definition

```go
// Service is an interface that defines the methods for a service.
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

## Methods on the Service

### Start

Starts the service, returning an error if it fails. This method is automatically called when the application starts.

```go
func (s *SomeService) Start(ctx context.Context) error
```

### String

Returns a string representation of the service, used to print the service in the startup message.

```go
func (s *SomeService) String() string
```

### State

Returns the current state of the service, used to print the service in the startup message.

```go
func (s *SomeService) State(ctx context.Context) (string, error)
```

### Terminate

Terminate terminates the service after the application shuts down using a post shutdown hook, returning an error if it fails.

```go
func (s *SomeService) Terminate(ctx context.Context) error
```

## Comprehensive Examples

### Example: Adding a service

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
    return "redis-store"
}

// State returns the current state of the service.
// It implements the [fiber.Service] interface.
func (s *redisService) State(ctx context.Context) (string, error) {
    state, err := s.ctr.State(ctx)
    if err != nil {
        return "", fmt.Errorf("container state: %w", err)
    }

    return state.Status, nil
}

// Terminate stops and removes the service. It implements the [fiber.Service] interface.
func (s *redisService) Terminate(ctx context.Context) error {
    // stop the service
    return s.ctr.Terminate(ctx)
}

func main() {
    cfg := &fiber.Config{}

    // Initialize service.
    redisSrv := &redisService{}
    cfg.Services = append(cfg.Services, redisSrv)

    // Define a context provider for the services startup.
    // This is useful to cancel the startup of the services if the context is canceled.
    // Default is context.Background().
    startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cfg.ServicesStartupContextProvider = func() context.Context {
        return startupCtx
    }

    // Define a context provider for the services shutdown.
    // This is useful to cancel the shutdown of the services if the context is canceled.
    // Default is context.Background().
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cfg.ServicesShutdownContextProvider = func() context.Context {
        return shutdownCtx
    }

    app := fiber.New(*cfg)

    ctx := context.Background()

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

### Example: Adding a service with a Store Middleware

This example demonstrates how to use Services with the Store Middleware for dependency injection in a Fiber application.

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
    "github.com/redis/go-redis/v9"
    tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
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
    return "redis-store"
}

// State returns the current state of the service.
// It implements the [fiber.Service] interface.
func (s *redisService) State(ctx context.Context) (string, error) {
    state, err := s.ctr.State(ctx)
    if err != nil {
        return "", fmt.Errorf("container state: %w", err)
    }

    return state.Status, nil
}

// Terminate stops and removes the service. It implements the [fiber.Service] interface.
func (s *redisService) Terminate(ctx context.Context) error {
    // stop the service
    return s.ctr.Terminate(ctx)
}

func main() {
    cfg := &fiber.Config{}

    // Initialize service.
    redisSrv := &redisService{}
    cfg.Services = append(cfg.Services, redisSrv)

    // Define a context provider for the services startup.
    // This is useful to cancel the startup of the services if the context is canceled.
    // Default is context.Background().
    startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cfg.ServicesStartupContextProvider = func() context.Context {
        return startupCtx
    }

    // Define a context provider for the services shutdown.
    // This is useful to cancel the shutdown of the services if the context is canceled.
    // Default is context.Background().
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    cfg.ServicesShutdownContextProvider = func() context.Context {
        return shutdownCtx
    }

    app := fiber.New(*cfg)

    // Initialize default config
    app.Use(logger.New())

    ctx := context.Background()

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

        key := fmt.Sprintf("user:%s", id)
        user, err := store.Get(key)
        if err == redis.Nil {
            return c.Status(fiber.StatusNotFound).SendString("User not found")
        } else if err != nil {
            return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
        }

        return c.JSON(string(user))
    })

    app.Listen(":3000")
}

```
