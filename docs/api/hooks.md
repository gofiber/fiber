---
id: hooks
title: 🎣 Hooks
sidebar_position: 7
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Fiber lets you run custom callbacks at specific points in the routing lifecycle. Available hooks include:

- [OnRoute](#onroute)
- [OnName](#onname)
- [OnGroup](#ongroup)
- [OnGroupName](#ongroupname)
- [OnListen](#onlisten)
- [OnFork](#onfork)
- [OnPreShutdown](#onpreshutdown)
- [OnPostShutdown](#onpostshutdown)
- [OnMount](#onmount)

## Constants

```go
// Handlers define functions to create hooks for Fiber.
type OnRouteHandler = func(Route) error
type OnNameHandler = OnRouteHandler
type OnGroupHandler = func(Group) error
type OnGroupNameHandler = OnGroupHandler
type OnListenHandler = func(ListenData) error
type OnForkHandler = func(int) error
type OnPreShutdownHandler  = func() error
type OnPostShutdownHandler = func(error) error
type OnMountHandler = func(*App) error
```

## OnRoute

Runs after each route is registered. The callback receives the route so you can inspect its properties.

```go title="Signature"
func (h *Hooks) OnRoute(handler ...OnRouteHandler)
```

## OnName

Runs when a route is named. The callback receives the route.

:::caution
`OnName` only works with named routes, not groups.
:::

```go title="Signature"
func (h *Hooks) OnName(handler ...OnNameHandler)
```

<Tabs>
<TabItem value="onname-example" label="OnName Example">

```go
package main

import (
    "fmt"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString(c.Route().Name)
    }).Name("index")

    app.Hooks().OnName(func(r fiber.Route) error {
        fmt.Print("Name: " + r.Name + ", ")
        return nil
    })

    app.Hooks().OnName(func(r fiber.Route) error {
        fmt.Print("Method: " + r.Method + "\n")
        return nil
    })

    app.Get("/add/user", func(c fiber.Ctx) error {
        return c.SendString(c.Route().Name)
    }).Name("addUser")

    app.Delete("/destroy/user", func(c fiber.Ctx) error {
        return c.SendString(c.Route().Name)
    }).Name("destroyUser")

    app.Listen(":5000")
}

// Results:
// Name: addUser, Method: GET
// Name: destroyUser, Method: DELETE
```

</TabItem>
</Tabs>

## OnGroup

Runs after each group is registered. The callback receives the group.

```go title="Signature"
func (h *Hooks) OnGroup(handler ...OnGroupHandler)
```

## OnGroupName

Runs when a group is named. The callback receives the group.

:::caution
`OnGroupName` only works with named groups, not routes.
:::

```go title="Signature"
func (h *Hooks) OnGroupName(handler ...OnGroupNameHandler)
```

## OnListen

Runs when the app starts listening via `Listen`, `ListenTLS`, or `Listener`.

```go title="Signature"
func (h *Hooks) OnListen(handler ...OnListenHandler)
```

<Tabs>
<TabItem value="onlisten-example" label="OnListen Example">

```go
package main

import (
    "log"
    "os"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/log"
)

func main() {
    app := fiber.New(fiber.Config{
        DisableStartupMessage: true,
    })

    app.Hooks().OnListen(func(listenData fiber.ListenData) error {
        if fiber.IsChild() {
            return nil
        }
        scheme := "http"
        if listenData.TLS {
            scheme = "https"
        }
        log.Println(scheme + "://" + listenData.Host + ":" + listenData.Port)
        return nil
    })

    app.Listen(":5000")
}
```

</TabItem>
</Tabs>

### ListenData

`ListenData` exposes runtime metadata about the listener:

| Field | Type | Description |
| --- | --- | --- |
| `Host` | `string` | Resolved hostname or IP address. |
| `Port` | `string` | The bound port. |
| `TLS` | `bool` | Indicates whether TLS is enabled. |
| `Version` | `string` | Fiber version reported in the startup banner. |
| `AppName` | `string` | Application name from the configuration. |
| `HandlerCount` | `int` | Total registered handler count. |
| `ProcessCount` | `int` | Number of processes Fiber will use. |
| `PID` | `int` | Current process identifier. |
| `Prefork` | `bool` | Whether prefork is enabled. |
| `ChildPIDs` | `[]int` | Child process identifiers when preforking. |
| `ColorScheme` | [`Colors`](https://github.com/gofiber/fiber/blob/main/color.go) | Active color scheme for the startup message. |

You can customize the default startup output with the helper methods:

- `PreventDefault()` stops Fiber from printing the built-in startup message.
- `UseHeader(header string)` overrides the ASCII art banner.
- `UsePrimaryInfoMap(fiber.Map)` replaces the primary info section (server URL, handler counts, etc.).
- `UseSecondaryInfoMap(fiber.Map)` replaces the secondary info section (prefork status, PID, process count).
- `AfterPrint() <-chan struct{}` returns a channel that closes once Fiber has printed (or skipped) the startup message. Use this from a goroutine to log follow-up information.

```go title="Customize the startup message"
package main

import (
    "log"
    "os"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New(fiber.Config{DisableStartupMessage: true})

    app.Hooks().OnListen(func(listenData fiber.ListenData) error {
        listenData.UseHeader("FOOBER " + listenData.Version + "\n-------")
        listenData.UsePrimaryInfoMap(fiber.Map{"Git hash": os.Getenv("GIT_HASH")})
        listenData.UseSecondaryInfoMap(fiber.Map{"Prefork": listenData.Prefork})

        go func() {
            <-listenData.AfterPrint()
            log.Println("startup completed")
        }()

        return nil
    })

    app.Listen(":5000")
}
```

## OnFork

Runs in the child process after a fork.

```go title="Signature"
func (h *Hooks) OnFork(handler ...OnForkHandler)
```

## OnPreShutdown

Runs before the server shuts down.

```go title="Signature"
func (h *Hooks) OnPreShutdown(handler ...OnPreShutdownHandler)
```

## OnPostShutdown

Runs after the server shuts down.

```go title="Signature"
func (h *Hooks) OnPostShutdown(handler ...OnPostShutdownHandler)
```

## OnMount

Fires after a sub-app is mounted on a parent. The parent app is passed to the callback and it works for both app and group mounts.

```go title="Signature"
func (h *Hooks) OnMount(handler ...OnMountHandler)
```

<Tabs>
<TabItem value="onmount-example" label="OnMount Example">

```go
package main

import (
    "fmt"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()
    app.Get("/", testSimpleHandler).Name("x")

    subApp := fiber.New()
    subApp.Get("/test", testSimpleHandler)

    subApp.Hooks().OnMount(func(parent *fiber.App) error {
        fmt.Print("Mount path of parent app: " + parent.MountPath())
        // Additional custom logic...
        return nil
    })

    app.Mount("/sub", subApp)
}

func testSimpleHandler(c fiber.Ctx) error {
    return c.SendString("Hello, Fiber!")
}

// Result:
// Mount path of parent app: /sub
```

</TabItem>
</Tabs>

:::caution
OnName, OnRoute, OnGroup, and OnGroupName are mount-sensitive. When you mount a sub-app that registers these hooks, route and group paths include the mount prefix.
:::
