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
func (d *Service) Start(ctx context.Context) error
```

### String

Returns a string representation of the service, used to print the service in the startup message.

```go
func (d *Service) String() string
```

### State

Returns the current state of the service, used to print the service in the startup message.

```go
func (d *Service) State(ctx context.Context) (string, error)
```

### Terminate

Terminate terminates the service after the application shuts down using a post shutdown hook, returning an error if it fails.

```go
func (d *Service) Terminate(ctx context.Context) error
```

## Comprehensive Examples

### Example: Adding a service

This example demonstrates how to add a Redis store as a service to the application, backed by the Testcontainers Redis Go module.

```go
package main

import (
    "context"
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/redis/go-redis/v9"
    tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

type redisStore struct {
    ctr *tcredis.RedisContainer
}

// Start initializes and starts the service. It implements the [fiber.Service] interface.
func (s *redisStore) Start(ctx context.Context) error {
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
func (s *redisStore) String() string {
    return "redis-store"
}

// State returns the current state of the service.
// It implements the [fiber.Service] interface.
func (s *redisStore) State(ctx context.Context) (string, error) {
    state, err := s.ctr.State(ctx)
    if err != nil {
        return "", fmt.Errorf("container state: %w", err)
    }

    return state.Status, nil
}

// Terminate stops and removes the service. It implements the [fiber.Service] interface.
func (s *redisStore) Terminate(ctx context.Context) error {
    // stop the service
    return s.ctr.Terminate(ctx)
}

func main() {
    cfg := &fiber.Config{}

    // Initialize service.
    store := &redisStore{}
    cfg.Services = append(cfg.Services, store)

    app := fiber.New(*cfg)

    ctx := context.Background()

    // Obtain the connection string from the service.
    connString, err := store.ctr.ConnectionString(ctx)
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

### Example: Adding a service with State Management

This example demonstrates how to use Services with the State for dependency injection in a Fiber application.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/redis/go-redis/v9"
    tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

type User struct {
    ID    int    `query:"id"`
    Name  string `query:"name"`
    Email string `query:"email"`
}

type redisStore struct {
    ctr *tcredis.RedisContainer
}

// Start initializes and starts the service. It implements the [fiber.Service] interface.
func (s *redisStore) Start(ctx context.Context) error {
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
func (s *redisStore) String() string {
    return "redis-store"
}

// State returns the current state of the service.
// It implements the [fiber.Service] interface.
func (s *redisStore) State(ctx context.Context) (string, error) {
    state, err := s.ctr.State(ctx)
    if err != nil {
        return "", fmt.Errorf("container state: %w", err)
    }

    return state.Status, nil
}

// Terminate stops and removes the service. It implements the [fiber.Service] interface.
func (s *redisStore) Terminate(ctx context.Context) error {
    // stop the service
    return s.ctr.Terminate(ctx)
}

func main() {
    cfg := &fiber.Config{}

    // Initialize service.
    store := &redisStore{}
    cfg.Services = append(cfg.Services, store)

    app := fiber.New(*cfg)

    ctx := context.Background()

    // Obtain the connection string from the service.
    connString, err := store.ctr.ConnectionString(ctx)
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

    // Inject the Redis client into Fiber's State for dependency injection.
    app.State().Set("redis", rdb)

    app.Post("/user/create", func(c fiber.Ctx) error {
        var user User
        if err := c.Bind().Query(&user); err != nil {
            return c.Status(fiber.StatusBadRequest).SendString(err.Error())
        }

        // Save the user to the database.
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
