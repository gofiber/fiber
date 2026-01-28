---
id: shutdown
title: Graceful Shutdown
description: How to configure, monitor, and troubleshoot graceful shutdowns in Fiber — including Kubernetes integration, load balancer draining, hooks, and the shutdown telemetry debug endpoint.
sidebar_position: 13
---

# Graceful Shutdown

Fiber provides a full-lifecycle graceful shutdown system that drains active connections, notifies protocol-specific clients (WebSocket / SSE), executes user-defined hooks, and exposes per-phase timing through a telemetry snapshot and a JSON debug endpoint.

---

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    // Register the shutdown debug endpoint
    app.Get("/debug/shutdown", app.ShutdownDebugHandler())

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello World")
    })

    // Listen for OS termination signals
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

    go func() {
        <-quit

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        tel := app.LastShutdownTelemetry()
        if tel != nil {
            log.Printf("Shutdown completed in %s (drained=%d, forced=%d)",
                tel.TotalDuration, tel.DrainedConns, tel.ForcedConns)
        }

        if err := app.ShutdownWithConfig(ctx, fiber.ShutdownConfig{
            OnShutdownStart: func(active int) {
                log.Printf("Shutdown started with %d active connections", active)
            },
            OnDrainProgress: func(remaining int, elapsed time.Duration) {
                log.Printf("Draining: %d connections remaining (%.1fs elapsed)", remaining, elapsed.Seconds())
            },
            OnForceClose: func(forceClosed int) {
                log.Printf("Force-closed %d connections after deadline", forceClosed)
            },
        }); err != nil {
            log.Fatalf("Shutdown error: %v", err)
        }

        // Read telemetry after shutdown completes
        tel = app.LastShutdownTelemetry()
        if tel != nil {
            log.Printf("Shutdown completed in %s (drained=%d, forced=%d)",
                tel.TotalDuration, tel.DrainedConns, tel.ForcedConns)
        }
    }()

    log.Fatal(app.Listen(":3000"))
}
```

---

## Shutdown Methods

Fiber exposes three shutdown entry points. All share the same underlying lifecycle.

| Method | Context Source | Force-Close? | Telemetry? |
|--------|---------------|--------------|------------|
| `app.Shutdown()` | `context.Background()` (no deadline) | No | No |
| `app.ShutdownWithTimeout(d)` | `context.WithTimeout(…, d)` | Yes, after `d` | No |
| `app.ShutdownWithConfig(ctx, cfg)` | Caller-supplied `ctx` | Yes, when `ctx` expires | Yes |

Only `ShutdownWithConfig` populates `ShutdownTelemetry` and stores it for retrieval via `app.LastShutdownTelemetry()` or the debug handler.

---

## ShutdownConfig Reference

```go
type ShutdownConfig struct {
    // OnShutdownStart is called once at the start of shutdown with the
    // number of currently active connections.
    OnShutdownStart func(activeConns int)

    // OnDrainProgress is called every DrainInterval while connections
    // are still open.  remaining is the live count; elapsed is wall-clock
    // time since shutdown began.
    OnDrainProgress func(remaining int, elapsed time.Duration)

    // OnForceClose is called after the context deadline is reached and
    // all remaining connections are force-closed.
    OnForceClose func(forceClosed int)

    // RequestDeadline adds a per-request deadline on top of the caller's
    // context.  The effective deadline is whichever expires first.
    //   Default: 0 (no additional deadline)
    RequestDeadline time.Duration

    // DrainInterval controls how often OnDrainProgress fires.
    //   Default: 500ms
    DrainInterval time.Duration

    // RequestContext, when non-nil, replaces the app's internal shutdown
    // context as the parent for every in-flight request.  Handlers can
    // then observe a concrete deadline via c.Deadline().
    RequestContext context.Context

    // WebSocketCloseTimeout is how long to wait for a WebSocket client's
    // close-frame acknowledgement.
    //   Default: 5s
    WebSocketCloseTimeout time.Duration

    // SSECloseTimeout is how long to wait after writing the final SSE event
    // for the client to disconnect.
    //   Default: 2s
    SSECloseTimeout time.Duration

    // SSECloseEvent is the raw SSE payload sent to each tracked SSE
    // connection.  Must follow SSE wire format.
    //   Default: "event: shutdown\ndata: server shutting down\n\n"
    SSECloseEvent string

    // OnWebSocketClose is called after each WebSocket close handshake.
    OnWebSocketClose func(connID int64, err error)

    // OnSSEClose is called after each SSE shutdown event is written.
    OnSSEClose func(connID int64, err error)
}
```

---

## ShutdownTelemetry

`ShutdownTelemetry` is populated automatically by `ShutdownWithConfig` and stored atomically so the debug handler or your application code can read the last snapshot without synchronization.

```go
type ShutdownTelemetry struct {
    StartedAt     time.Time     // Wall-clock start of shutdown
    CompletedAt   time.Time     // Wall-clock end of shutdown
    TotalDuration time.Duration // CompletedAt − StartedAt

    PreHooksDuration      time.Duration // Time spent in pre-shutdown hooks
    GracefulCloseDuration time.Duration // Time spent closing WS/SSE connections
    DrainDuration         time.Duration // Time spent polling for zero active conns
    PostHooksDuration     time.Duration // Time spent in post-shutdown hooks

    InitialConns     int  // Active connections when shutdown began
    DrainedConns     int  // Connections that closed naturally (InitialConns − ForcedConns)
    ForcedConns      int  // Connections force-closed after the deadline
    WebSocketsClosed int  // WebSocket connections processed in phase 5b
    SSEsClosed       int  // SSE connections processed in phase 5b
    TimedOut         bool // true when the context deadline was exceeded
}
```

### Accessor

```go
func (app *App) LastShutdownTelemetry() *ShutdownTelemetry
```

Returns `nil` before any `ShutdownWithConfig` call has completed.

---

## Debug Endpoint

`ShutdownDebugHandler()` returns a handler you register at any path.  It responds with JSON containing the current status and, if available, the last telemetry snapshot with human-readable duration strings.

```go
app.Get("/debug/shutdown", app.ShutdownDebugHandler())
```

### Response Shape

**Before shutdown (status `"running"`):**

```json
{
  "status": "running",
  "activeConnections": 12,
  "lastShutdown": null
}
```

**During shutdown, before telemetry is stored (status `"shutting_down"`):**

```json
{
  "status": "shutting_down",
  "activeConnections": 5,
  "lastShutdown": null
}
```

**After shutdown completes (status `"shutdown"`):**

```json
{
  "status": "shutdown",
  "activeConnections": 0,
  "lastShutdown": {
    "startedAt": "2025-06-10T12:00:00Z",
    "completedAt": "2025-06-10T12:00:03.45Z",
    "totalDuration": "3.45s",
    "drainDuration": "2.1s",
    "preHooksDuration": "50.2ms",
    "gracefulCloseDuration": "800ms",
    "postHooksDuration": "10.5ms",
    "initialConns": 15,
    "drainedConns": 12,
    "forcedConns": 3,
    "webSocketsClosed": 4,
    "sseClosed": 1,
    "timedOut": true
  }
}
```

All duration values are formatted using Go's `time.Duration.String()` (e.g. `"1.23s"`, `"450ms"`) and are directly parseable with `time.ParseDuration`.

---

## Shutdown Lifecycle — Phase Order

Understanding the phase order is critical when configuring hooks and interpreting telemetry.

| Phase | What Happens | Telemetry Field Populated |
|-------|--------------|---------------------------|
| 1 | Mark `IsShuttingDown() == true`; cancel in-flight request contexts | `StartedAt`, `InitialConns` |
| 2 | Set `IdleTimeout` to 1 ns — evicts idle keepalive connections immediately | — |
| 3 | Close the listener — no new connections are accepted | — |
| 3b | Apply `RequestDeadline` (if set) and swap `RequestContext` (if set) | — |
| 4 | Call `OnShutdownStart(activeConns)` | — |
| 5 | Execute `OnPreShutdown` hooks | `PreHooksDuration` |
| 5b | Graceful close of WebSocket and SSE connections | `GracefulCloseDuration`, `WebSocketsClosed`, `SSEsClosed` |
| 6 | Start drain-monitor goroutine (fires `OnDrainProgress` every `DrainInterval`) | — |
| 7 | Poll `activeConns` until zero or context deadline | `DrainDuration` |
| 8 | If deadline exceeded: force-close all remaining connections; call `OnForceClose` | `ForcedConns`, `TimedOut` |
| 9 | Execute `OnPostShutdown` hooks | `PostHooksDuration`, `CompletedAt`, `TotalDuration`, `DrainedConns` |

Hooks execute in **registration order** (FIFO) within each phase.  The strict guarantee is:

```
OnShutdownStart → pre-hooks → GracefulCloseTyped → drain → force-close → post-hooks → return
```

---

## Hooks

### Pre-Shutdown Hook

Runs before any connections are drained. Useful for flushing caches, notifying downstream services, or stopping background workers.

```go
app.Hooks().OnPreShutdown(func() error {
    log.Println("Flushing cache before shutdown")
    cache.Flush()
    return nil
})
```

### Post-Shutdown Hook

Runs after all connections are drained (or force-closed). Receives the shutdown error (`nil` on clean drain, `context.DeadlineExceeded` or `context.Canceled` on timeout/cancel).

```go
app.Hooks().OnPostShutdown(func(err error) error {
    if err != nil {
        log.Printf("Shutdown ended with error: %v", err)
    }
    // Close database connections, release resources, etc.
    db.Close()
    return nil
})
```

### Connection Cleanup Hook

Register a per-connection hook for protocol-specific teardown.  The hook replaces the framework's default close-frame / SSE-event writer when set.

```go
app.Get("/ws", func(c fiber.Ctx) error {
    tc := c.TrackedConn()
    if tc != nil {
        tc.SetConnType(fiber.ConnTypeWebSocket)
        tc.SetCleanupHook(func() error {
            // Custom WebSocket close handshake
            return writeCloseFrame(tc)
        })
    }
    // ... WebSocket handler logic
    return nil
})
```

Only the **first** call to `SetCleanupHook` takes effect; subsequent calls are ignored.

### Multiple Hooks

Register multiple hooks; they execute in registration order.

```go
app.Hooks().OnPreShutdown(func() error { log.Println("pre-A"); return nil })
app.Hooks().OnPreShutdown(func() error { log.Println("pre-B"); return nil })
app.Hooks().OnPostShutdown(func(_ error) error { log.Println("post-A"); return nil })
app.Hooks().OnPostShutdown(func(_ error) error { log.Println("post-B"); return nil })
// Output order: pre-A → pre-B → post-A → post-B
```

---

## Kubernetes Integration

### Deployment Manifest

The following example wires up a Fiber app with Kubernetes-compatible health probes, a graceful-shutdown debug endpoint, and a pre-stop hook that lets the kubelet drain the pod before sending SIGTERM.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fiber-app
spec:
  replicas: 3
  template:
    spec:
      terminationGracePeriodSeconds: 30   # Must be ≥ preStop sleep + ShutdownTimeout
      containers:
      - name: app
        image: myregistry/fiber-app:latest
        ports:
        - containerPort: 3000
        lifecycle:
          preStop:
            exec:
              command: ["sleep", "5"]     # Allow load balancer to finish draining
        livenessProbe:
          httpGet:
            path: /healthz
            port: 3000
          initialDelaySeconds: 2
          periodSeconds: 5
          timeoutSeconds: 2
        readinessProbe:
          httpGet:
            path: /readyz
            port: 3000
          initialDelaySeconds: 1
          periodSeconds: 2
          timeoutSeconds: 2
        env:
        - name: SHUTDOWN_TIMEOUT
          value: "20s"
```

### Application Code

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/healthcheck"
)

func main() {
    app := fiber.New()

    // Health probes (Kubernetes liveness / readiness)
    app.Get("/healthz", healthcheck.New())
    app.Get("/readyz", healthcheck.New(healthcheck.Config{
        Probe: func(c fiber.Ctx) bool {
            // Return false once shutdown has begun so the pod is
            // removed from the load balancer's target set.
            return !app.IsShuttingDown()
        },
    }))

    // Shutdown telemetry endpoint (internal / ops only)
    app.Get("/debug/shutdown", app.ShutdownDebugHandler())

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello")
    })

    // Signal handling
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

    go func() {
        <-quit
        log.Println("Received termination signal")

        timeout, _ := time.ParseDuration(
            getEnvOrDefault("SHUTDOWN_TIMEOUT", "20s"),
        )
        ctx, cancel := context.WithTimeout(context.Background(), timeout)
        defer cancel()

        err := app.ShutdownWithConfig(ctx, fiber.ShutdownConfig{
            OnShutdownStart: func(active int) {
                log.Printf("Shutting down with %d active connections", active)
            },
            OnForceClose: func(n int) {
                log.Printf("Force-closed %d connections after timeout", n)
            },
        })
        if err != nil {
            log.Printf("Shutdown completed with: %v", err)
        }

        tel := app.LastShutdownTelemetry()
        if tel != nil {
            log.Printf("Total shutdown duration: %s", tel.TotalDuration)
        }
    }()

    log.Fatal(app.Listen(":3000"))
}

func getEnvOrDefault(key, fallback string) string {
    if v, ok := os.LookupEnv(key); ok {
        return v
    }
    return fallback
}
```

### Key Kubernetes Considerations

| Concern | Recommendation |
|---------|----------------|
| **`terminationGracePeriodSeconds`** | Must be **strictly greater** than `preStop` sleep + your `ShutdownTimeout`.  Kubernetes sends SIGKILL after this window — your post-shutdown hooks will not run if exceeded. |
| **`preStop` sleep** | Gives the load balancer (or kube-proxy / iptables rules) time to stop routing new traffic to the pod.  A 3–5 second sleep is typical. |
| **Readiness probe during shutdown** | Return `false` from your readiness probe once `app.IsShuttingDown()` is `true`.  This removes the pod from the Endpoints object so no new traffic is routed to it. |
| **Liveness probe during shutdown** | Keep returning `true` until shutdown finishes — a failing liveness probe causes an immediate restart, losing the graceful window. |
| **Drain timeout alignment** | Set `ShutdownWithConfig`'s context timeout to `terminationGracePeriodSeconds − preStop − small_buffer` so you have a margin to run post-shutdown hooks before SIGKILL arrives. |

---

## Load Balancer Integration

### AWS ALB / NLB

- Configure your target group's **deregistration delay** (default 300 s for ALB, 30 s for NLB).  Traffic stops flowing to a target once it is marked unhealthy, but existing connections are kept alive until the delay expires or the target closes them.
- Set your readiness probe to fail immediately on shutdown (`!app.IsShuttingDown()`) so the ALB marks the target unhealthy.
- Set your Fiber `ShutdownTimeout` shorter than the deregistration delay so your drain completes cleanly before the ALB forcibly tears down connections.

### GCP Cloud Load Balancing

- Use a **connection draining timeout** (up to 300 s) on your backend service.  The load balancer stops sending new requests and waits for in-flight ones to finish.
- Align your Fiber drain context with this timeout minus a safety margin.

### Generic Pattern

```
┌─────────────┐   stop routing   ┌────────────────┐  drain timeout  ┌──────────┐
│ Load Balancer│ ─────────────>   │  Fiber Instance │ ──────────────> │  Done    │
└─────────────┘                   │ (draining conns)│                 └──────────┘
                                  └────────────────┘
        ↑                                  ↑
  Readiness probe                  ShutdownWithConfig ctx deadline
  returns false                    ≤ LB drain timeout − buffer
```

---

## Troubleshooting

### Connections Never Drain (Shutdown Hangs)

**Symptom:** `DrainDuration` in telemetry is very large; `activeConns` stays above zero.

| Cause | Fix |
|-------|-----|
| Handler sleeps or blocks indefinitely (e.g., polling a channel) | Select on `c.Done()` inside long-running handlers and return early when shutdown is detected. |
| Keep-alive connections sit idle but the counter is not decremented | Fiber sets `IdleTimeout = 1 ns` at shutdown start. If you override `IdleTimeout` after `ShutdownWithConfig` is called, idle conns will not be evicted. |
| Handler holds a database lock that never releases | Use `RequestDeadline` or `RequestContext` to bound per-request lifetime. |

**Example: handler that respects shutdown**

```go
app.Get("/long-poll", func(c fiber.Ctx) error {
    select {
    case result := <-expensiveWork():
        return c.JSON(result)
    case <-c.Done():
        return c.Status(fiber.StatusServiceUnavailable).SendString("shutting down")
    }
})
```

### Hooks Fire Zero Times or Twice

| Symptom | Cause | Fix |
|---------|-------|-----|
| `OnPostShutdown` fires twice | `ShutdownWithContext` (the legacy path) runs hooks internally; if `gracefulShutdown()` also calls them the count doubles. | Use `ShutdownWithConfig` exclusively, or rely on the `GracefulContext` + `ShutdownTimeout` path which delegates to `ShutdownWithContext` (hooks fire once). |
| Hooks never execute | `app.server` is `nil` — `ShutdownWithConfig` returns `ErrNotRunning` before reaching the hook phases. | Ensure the app has been started with `Listen` or `Listener` before calling shutdown. |

### Telemetry Is `nil` After Shutdown

`LastShutdownTelemetry()` returns `nil` when:

1. Shutdown was performed via `Shutdown()` or `ShutdownWithTimeout()` — these do **not** populate telemetry.
2. `ShutdownWithConfig` returned `ErrNotRunning` before reaching the telemetry-store phase.

Always use `ShutdownWithConfig` if you need telemetry.

### WebSocket / SSE Counters Are Zero

- Connections must be explicitly marked with `SetConnType(ConnTypeWebSocket)` or `SetConnType(ConnTypeSSE)` inside the handler.  Unmarked connections are treated as plain HTTP and are not counted.
- The mark must happen **before** the handler blocks or sleeps — otherwise the connection is still `ConnTypeHTTP` when `GracefulCloseTyped` iterates.

```go
app.Get("/ws", func(c fiber.Ctx) error {
    tc := c.TrackedConn()
    if tc != nil {
        tc.SetConnType(fiber.ConnTypeWebSocket)  // Must happen early
    }
    // ... rest of handler
    return nil
})
```

### Timeout Mismatch Between Kubernetes and Application

If your pod is killed (SIGKILL) before post-shutdown hooks run:

```
terminationGracePeriodSeconds  <  preStop + ShutdownTimeout + hook_time
```

Increase `terminationGracePeriodSeconds` or reduce `ShutdownTimeout`.  Use telemetry's `TotalDuration` and `PostHooksDuration` to size this correctly based on observed behaviour.

### Debug Endpoint Returns `"shutting_down"` Indefinitely

The status transitions to `"shutdown"` only after `app.lastTelemetry.Store(tel)` executes at the very end of `ShutdownWithConfig`. If the app is stuck in the drain phase (see "Connections Never Drain" above), the telemetry is never written. Monitor `activeConnections` in the JSON response — a non-zero value confirms the drain is the bottleneck.

---

## Concurrency Safety

| Resource | Access Pattern | Why It's Safe |
|----------|----------------|---------------|
| `ShutdownTelemetry` allocation | Single goroutine (the `ShutdownWithConfig` caller) writes all fields sequentially | No concurrent writes; only one shutdown runs at a time (mutex-guarded) |
| `app.lastTelemetry` | Written once via `atomic.Pointer.Store`; read by `ShutdownDebugHandler` via `atomic.Pointer.Load` | Atomic pointer swap guarantees the reader sees either `nil` or a fully-populated snapshot — no torn reads |
| `WebSocketsClosed` / `SSEsClosed` counters | Incremented inside `GracefulCloseTyped`'s sequential `sync.Map.Range` | Only one goroutine executes the Range; no atomics needed |
| `activeConns` | Atomic int64 on the App | Incremented on Accept, decremented on Close, read by `ActiveConnections()` and `drainConnections` |
