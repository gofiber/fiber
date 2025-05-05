---
id: devtime_dependencies
title: 🥡 Development-Time Dependencies
sidebar_position: 9
---

Development-time services provide external dependencies needed to run the application while developing it. They are only supposed to be used while developing and are disabled when the application is deployed.

## DevTimeDependency Interface

`DevTimeDependency` is an interface that defines the methods for a development-time dependency.

### Definition

```go
// DevTimeDependency is an interface that defines the methods for a development-time dependency.
type DevTimeDependency interface {
    // Start starts the dependency, returning an error if it fails.
    Start(ctx context.Context) error

    // String returns a string representation of the dependency.
    // It is used to print the dependency in the startup message.
    String() string

    // Terminate terminates the dependency, returning an error if it fails.
    Terminate(ctx context.Context) error
}
```

## Methods on the DevTimeDependency

### Start

Start starts the dependency, returning an error if it fails. This method is automatically called when the application starts.

```go
// Start starts the dependency, returning an error if it fails.
func (d *DevTimeDependency) Start(ctx context.Context) error
```

### String

String returns a string representation of the dependency, used to print the dependency in the startup message.

```go
// String returns a string representation of the dependency.
func (d *DevTimeDependency) String() string
```

### Terminate

Terminate terminates the dependency after the application shuts down using a post shutdown hook, returning an error if it fails.

```go
// Terminate terminates the dependency, returning an error if it fails.
func (d *DevTimeDependency) Terminate(ctx context.Context) error
```

## Comprehensive Examples

### Example: Adding a development-time dependency

This example demonstrates how to add a Redis store as a development-time dependency to the application, backed by the Testcontainers Redis Go module.

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

// Start initializes and starts the dependency. It implements the fiber.RuntimeDependency interface.
func (s *redisStore) Start(ctx context.Context) error {
    // start the dependency
    c, err := tcredis.Run(ctx, "redis:latest")
    if err != nil {
        return err
    }

    s.ctr = c
    return nil
}

// String returns a human-readable representation of the dependency's state.
// It implements the fiber.RuntimeDependency interface.
func (s *redisStore) String() string {
    return "redis-store"
}

// Terminate stops and removes the dependency. It implements the fiber.RuntimeDependency interface.
func (s *redisStore) Terminate(ctx context.Context) error {
    // stop the dependency
    return s.ctr.Terminate(ctx)
}

func main() {
    cfg := &fiber.Config{}

    // Initialize development-time dependency.
    store := &redisStore{}
    cfg.DevTimeDependencies = append(cfg.DevTimeDependencies, store)

    app := fiber.New(*cfg)

    ctx := context.Background()

    // Obtain the connection string from the dependency.
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

### Example: Adding a development-time dependency with State Management

This example demonstrates how to use DevTimeDependencies with the State for dependency injection in a Fiber application.

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

// Start initializes and starts the dependency. It implements the fiber.RuntimeDependency interface.
func (s *redisStore) Start(ctx context.Context) error {
    // start the dependency
    c, err := tcredis.Run(ctx, "redis:latest")
    if err != nil {
        return err
    }

    s.ctr = c
    return nil
}

// String returns a human-readable representation of the dependency's state.
// It implements the fiber.RuntimeDependency interface.
func (s *redisStore) String() string {
    return "redis-store"
}

// Terminate stops and removes the dependency. It implements the fiber.RuntimeDependency interface.
func (s *redisStore) Terminate(ctx context.Context) error {
    // stop the dependency
    return s.ctr.Terminate(ctx)
}

func main() {
    cfg := &fiber.Config{}

    // Initialize development-time dependency.
    store := &redisStore{}
    cfg.DevTimeDependencies = append(cfg.DevTimeDependencies, store)

    app := fiber.New(*cfg)

    ctx := context.Background()

    // Obtain the connection string from the dependency.
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
