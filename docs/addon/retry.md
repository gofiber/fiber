---
id: retry
---

# Retry Addon

Retry addon for [Fiber](https://github.com/gofiber/fiber) designed to apply retry mechanism for unsuccessful network
operations. This addon uses an exponential backoff algorithm with jitter. It calls the function multiple times and tries
to make it successful. If all calls are failed, then, it returns an error. It adds a jitter at each retry step because adding
a jitter is a way to break synchronization across the client and avoid collision.

## Table of Contents

- [Retry Addon](#retry-addon)
- [Table of Contents](#table-of-contents)
- [Signatures](#signatures)
- [Examples](#examples)
- [Default Config](#default-config)
- [Custom Config](#custom-config)
- [Config](#config)
- [Default Config Example](#default-config-example)

## Signatures

```go
func NewExponentialBackoff(config ...retry.Config) *retry.ExponentialBackoff
```

## Examples

```go
package main

import (
    "fmt"

    "github.com/gofiber/fiber/v3/addon/retry"
    "github.com/gofiber/fiber/v3/client"
)

func main() {
    expBackoff := retry.NewExponentialBackoff(retry.Config{})

    // Local variables that will be used inside of Retry
    var resp *client.Response
    var err error

    // Retry a network request and return an error to signify to try again
    err = expBackoff.Retry(func() error {
        client := client.New()
        resp, err = client.Get("https://gofiber.io")
        if err != nil {
            return fmt.Errorf("GET gofiber.io failed: %w", err)
        }
        if resp.StatusCode() != 200 {
            return fmt.Errorf("GET gofiber.io did not return OK 200")
        }
        return nil
    })

    // If all retries failed, panic
    if err != nil {
        panic(err)
    }
    fmt.Printf("GET gofiber.io succeeded with status code %d\n", resp.StatusCode())
}
```

## Default Config

```go
retry.NewExponentialBackoff()
```

## Custom Config

```go
retry.NewExponentialBackoff(retry.Config{
    InitialInterval: 2 * time.Second,
    MaxBackoffTime:  64 * time.Second,
    Multiplier:      2.0,
    MaxRetryCount:   15,
})
```

## Config

```go
// Config defines the config for addon.
type Config struct {
    // InitialInterval defines the initial time interval for backoff algorithm.
    //
    // Optional. Default: 1 * time.Second
    InitialInterval time.Duration

    // MaxBackoffTime defines maximum time duration for backoff algorithm. When
    // the algorithm is reached this time, rest of the retries will be maximum
    // 32 seconds.
    //
    // Optional. Default: 32 * time.Second
    MaxBackoffTime time.Duration

    // Multiplier defines multiplier number of the backoff algorithm.
    //
    // Optional. Default: 2.0
    Multiplier float64

    // MaxRetryCount defines maximum retry count for the backoff algorithm.
    //
    // Optional. Default: 10
    MaxRetryCount int
}
```

## Default Config Example

```go
// DefaultConfig is the default config for retry.
var DefaultConfig = Config{
    InitialInterval: 1 * time.Second,
    MaxBackoffTime:  32 * time.Second,
    Multiplier:      2.0,
    MaxRetryCount:   10,
    currentInterval: 1 * time.Second,
}
```
