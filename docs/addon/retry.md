---
id: retry
---

# Retry Addon

The Retry addon for [Fiber](https://github.com/gofiber/fiber) retries failed network operations using exponential
backoff with jitter. It repeatedly invokes a function until it succeeds or the maximum number of attempts is
exhausted. Jitter at each step breaks client synchronization and helps avoid collisions. If all attempts fail, the
addon returns an error.

## Table of Contents

- [Signatures](#signatures)
- [Examples](#examples)
- [Default Config](#default-config)
- [Custom Config](#custom-config)
- [Config](#config)
- [Default Config Example](#default-config-example)

## Signatures

<!-- go:doc NewExponentialBackoff ExponentialBackoff.Retry -->

```go
// NewExponentialBackoff creates a ExponentialBackoff with default values.
func NewExponentialBackoff(config ...Config) *ExponentialBackoff
```

```go
// Retry is the core logic of the retry mechanism. If the calling function returns
// nil as an error, then the Retry method is terminated with returning nil. Otherwise,
// if all function calls are returned error, then the method returns this error.
//
// f is called at least once: a field left at its zero value takes its default, so
// even a zero ExponentialBackoff makes up to DefaultConfig.MaxRetryCount attempts.
//
// Each call starts from InitialInterval with its own state, so one ExponentialBackoff can be shared.
func (e *ExponentialBackoff) Retry(f func() error) error
```

<!-- /go:doc -->

`Retry` calls `f` up to `MaxRetryCount` times in total and returns `nil` as soon as `f` returns `nil`. If every attempt fails, it returns the error of the last attempt. Each call to `Retry` starts from `InitialInterval` with its own state, so one `ExponentialBackoff` can be shared.

A field left at zero or below takes its default from the [Default Config](#default-config-example), also on a `retry.ExponentialBackoff` you build as a struct literal instead of with `NewExponentialBackoff`. Even `retry.ExponentialBackoff{}` therefore calls `f` at least once, and up to 10 times when it keeps failing, waiting between the attempts as the default does: about 160 seconds in all. A literal that sets only some fields waits as well, so `retry.ExponentialBackoff{MaxRetryCount: 2}` no longer retries at once. To retry without waiting, set `MaxBackoffTime` to a short duration such as `time.Nanosecond`.

## Examples

<!-- go:doc ExampleExponentialBackoff_Retry -->

```go
// ExampleExponentialBackoff_Retry demonstrates retrying a transient failure.
package main

import (
    "errors"
    "fmt"
    "os"
    "time"

    "github.com/gofiber/fiber/v3/addon/retry"
)

func main() {
    backoff := retry.NewExponentialBackoff(retry.Config{MaxBackoffTime: time.Nanosecond})
    attempts := 0
    err := backoff.Retry(func() error {
        attempts++
        if attempts < 3 {
            return errors.New("temporarily unavailable")
        }
        return nil
    })
    fmt.Fprintf(os.Stdout, "attempts: %d\nerror: %v\n", attempts, err)
}
```

Output:

```text
attempts: 3
error: <nil>
```

<!-- /go:doc -->

## Default Config

<!-- go:doc ExampleNewExponentialBackoff_defaults -->

```go
// ExampleNewExponentialBackoff_defaults shows the default retry settings.
package main

import (
    "fmt"
    "os"

    "github.com/gofiber/fiber/v3/addon/retry"
)

func main() {
    backoff := retry.NewExponentialBackoff()
    fmt.Fprintln(os.Stdout, backoff.InitialInterval, backoff.MaxBackoffTime, backoff.Multiplier, backoff.MaxRetryCount)
}
```

Output:

```text
1s 32s 2 10
```

<!-- /go:doc -->

## Custom Config

<!-- go:doc ExampleNewExponentialBackoff -->

```go
// ExampleNewExponentialBackoff shows a custom backoff configuration.
package main

import (
    "fmt"
    "os"
    "time"

    "github.com/gofiber/fiber/v3/addon/retry"
)

func main() {
    backoff := retry.NewExponentialBackoff(retry.Config{
        InitialInterval: 2 * time.Second,
        MaxBackoffTime:  64 * time.Second,
        Multiplier:      2,
        MaxRetryCount:   15,
    })
    fmt.Fprintln(os.Stdout, backoff.InitialInterval, backoff.MaxBackoffTime, backoff.Multiplier, backoff.MaxRetryCount)
}
```

Output:

```text
2s 1m4s 2 15
```

<!-- /go:doc -->

## Config

<!-- go:doc Config -->

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
    // contains filtered or unexported fields
}
```

<!-- /go:doc -->

## Default Config Example

<!-- go:doc DefaultConfig -->

```go
// DefaultConfig is the default config for retry.
var DefaultConfig = Config{
    InitialInterval: 1 * time.Second,
    MaxBackoffTime:  32 * time.Second,
    Multiplier:      2.0,
    MaxRetryCount:   10,
    // contains filtered or unexported fields
}
```

<!-- /go:doc -->
