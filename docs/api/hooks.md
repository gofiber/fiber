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
- [OnPreStartupMessage/OnPostStartupMessage](#onprestartupmessageonpoststartupmessage)
  - [ListenData](#listendata)
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
type OnPreStartupMessageHandler  = func(*PreStartupMessageData) error
type OnPostStartupMessageHandler = func(*PostStartupMessageData) error
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

## OnPreStartupMessage/OnPostStartupMessage

Use `OnPreStartupMessage` to tweak the banner before Fiber prints it, and `OnPostStartupMessage` to run logic after the banner is printed (or skipped). You can use some helper functions to customize the banner inside the `OnPreStartupMessage` hook.

```go title="Signatures"
// AddInfo adds an informational entry to the startup message with "INFO" label.
func (sm *PreStartupMessageData) AddInfo(key, title, value string, priority ...int)

// AddWarning adds a warning entry to the startup message with "WARNING" label.
func (sm *PreStartupMessageData) AddWarning(key, title, value string, priority ...int)

// AddError adds an error entry to the startup message with "ERROR" label.
func (sm *PreStartupMessageData) AddError(key, title, value string, priority ...int)

// EntryKeys returns all entry keys currently present in the startup message.
func (sm *PreStartupMessageData) EntryKeys() []string

// ResetEntries removes all existing entries from the startup message.
func (sm *PreStartupMessageData) ResetEntries()

// DeleteEntry removes a specific entry from the startup message by its key.
func (sm *PreStartupMessageData) DeleteEntry(key string)
```

- Assign `sm.BannerHeader` to override the ASCII art banner. Leave it empty to use the default banner provided by Fiber.
- Set `sm.PreventDefault = true` to suppress the built-in banner without affecting other hooks.
- `PostStartupMessageData` reports whether the banner was skipped via the `Disabled`, `IsChild`, and `Prevented` flags.

```go title="Customize the startup message"
package main

import (
    "fmt"
    "os"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Hooks().OnPreStartupMessage(func(sm *fiber.PreStartupMessageData) error {
        sm.BannerHeader = "FOOBER " + sm.Version + "\n-------"

        // Optional: you can also remove old entries
        // sm.ResetEntries()

        sm.AddInfo("git-hash", "Git hash", os.Getenv("GIT_HASH"))
        sm.AddInfo("prefork", "Prefork", fmt.Sprintf("%v", sm.Prefork), 15)
        return nil
    })

    app.Hooks().OnPostStartupMessage(func(sm fiber.PostStartupMessageData) error {
        if !sm.Disabled && !sm.IsChild && !sm.Prevented {
            fmt.Println("startup completed")
        }
        return nil
    })

    app.Listen(":5000")
}
```

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
