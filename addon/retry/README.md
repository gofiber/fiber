# Retry Addon

Retry addon for [Fiber](https://github.com/gofiber/fiber) designed to apply retry mechanism for unsuccessful network
operations. This addon uses exponential backoff algorithm with jitter. It calls the function multiple times and tries
to make it successful. If all calls are failed, then, it returns error. It adds a jitter at each retry step because adding
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
func NewExponentialBackoff(config ...Config) *ExponentialBackoff
```

## Examples

Firstly, import the addon from Fiber,

```go
import (
    "github.com/gofiber/fiber/v3/addon/retry"
)
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
    
    // currentInterval tracks the current waiting time.
    //
    // Optional. Default: 1 * time.Second
    currentInterval time.Duration
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