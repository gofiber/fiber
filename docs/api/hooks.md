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

Hooks of one kind run in the order you registered them. At startup, `OnListen` runs first, then `OnPreStartupMessage`, the banner and `OnPostStartupMessage`. At shutdown, `OnPreShutdown` runs before the server drains its connections and `OnPostShutdown` after. What a returned error does depends on the hook; see [Errors returned by hooks](#errors-returned-by-hooks).

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

A registration that covers several methods runs the hook once per method: `Add` runs it once for each method it lists, and `Use` and `All` once for each entry of `RequestMethods`. Fiber also runs it for every `HEAD` route it generates for a `GET` route, which happens when the app starts or when [`RebuildTree`](./app.md#rebuildtree) runs.

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

Runs when the app starts listening via `Listen` or `Listener`, before the startup message is printed. With prefork enabled it runs once, in the master process after the child processes have started; the children do not run it.

```go title="Signature"
func (h *Hooks) OnListen(handler ...OnListenHandler)
```

<Tabs>
<TabItem value="onlisten-example" label="OnListen Example">

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/log"
)

func main() {
    app := fiber.New()

    app.Hooks().OnListen(func(listenData fiber.ListenData) error {
        scheme := "http"
        if listenData.TLS {
            scheme = "https"
        }
        log.Info(scheme + "://" + listenData.Host + ":" + listenData.Port)
        return nil
    })

    app.Listen(":5000", fiber.ListenConfig{DisableStartupMessage: true})
}
```

</TabItem>
</Tabs>

## OnPreStartupMessage/OnPostStartupMessage

Use `OnPreStartupMessage` to tweak the banner before Fiber prints it, and `OnPostStartupMessage` to run logic after the banner is printed (or skipped). You can use some helper functions to customize the banner inside the `OnPreStartupMessage` hook. Both hooks run even when `ListenConfig.DisableStartupMessage` is set, and with prefork they run in the master process only.

```go title="Signatures"
// AddInfo adds an informational entry to the startup message with "INFO" label.
func (sm *PreStartupMessageData) AddInfo(key, title, value string, priority ...int)

// AddWarning adds a warning entry to the startup message with "WARN" label.
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
- Entries print from the highest `priority` to the lowest. An entry added without a priority gets `-1`, so it prints after the built-in ones.
- Adding an entry with a `key` that already exists replaces that entry, so you can change a built-in one.

The built-in entries use these keys and priorities:

| Key | Entry | Priority |
| --- | --- | --- |
| `server_address` | Server started on | `10` |
| `app_name` | Application name, only when `AppName` is set | `9` |
| `total_handlers` | Total handlers | `8` |
| `prefork` | Prefork | `7` when enabled, `6` when disabled |
| `pid` | PID | `5` |
| `process_count` | Total process count | `4` |

### Startup Message Customization

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

    app.Hooks().OnPostStartupMessage(func(sm *fiber.PostStartupMessageData) error {
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

Runs in the prefork master process each time it starts a child process, including a child it starts again after a crash. The callback receives the PID of that child. The children themselves do not run it.

```go title="Signature"
func (h *Hooks) OnFork(handler ...OnForkHandler)
```

## OnPreShutdown

Runs when a shutdown begins, before the server stops accepting connections. It runs once per shutdown, whether `Shutdown`, `ShutdownWithTimeout`, `ShutdownWithContext` or a `GracefulContext` started it.

```go title="Signature"
func (h *Hooks) OnPreShutdown(handler ...OnPreShutdownHandler)
```

## OnPostShutdown

Runs once the shutdown has finished, even when it failed, and exactly once per shutdown. The callback receives the error the shutdown returned: `nil` after a clean shutdown, or for example `context.DeadlineExceeded` when the timeout expired while connections were still active.

```go title="Signature"
func (h *Hooks) OnPostShutdown(handler ...OnPostShutdownHandler)
```

:::note
Fiber registers its own `OnPostShutdown` hook before yours. It terminates the configured [services](./services.md) and then closes the `SharedStorage`, so both are already gone when your hooks run.
:::

## OnMount

Fires after a sub-app is mounted on a parent. Register it on the sub-app: the parent app is passed to the callback, and it works for app, group and `Domain` mounts.

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
        fmt.Print("Mount path of sub-app: " + subApp.MountPath())
        // Additional custom logic...
        return nil
    })

    app.Use("/sub", subApp)
}

func testSimpleHandler(c fiber.Ctx) error {
    return c.SendString("Hello, Fiber!")
}

// Result:
// Mount path of sub-app: /sub
```

</TabItem>
</Tabs>

:::caution
OnName, OnRoute, OnGroup, and OnGroupName are mount-sensitive. When you mount a sub-app that registers these hooks, the route and group paths it reports include the mount prefix, but only for routes and groups registered after the mount. The parent app's hooks do not run for the routes of its sub-apps.
:::

## Errors returned by hooks

Every hook handler returns an `error`. What Fiber does with it depends on the hook:

| Hooks | When a handler returns an error |
| --- | --- |
| `OnRoute`, `OnName`, `OnGroup`, `OnGroupName`, `OnMount`, `OnListen` | Fiber panics with the error, so the route registration, the mount or the start fails loudly. |
| `OnPreStartupMessage`, `OnPostStartupMessage` | Fiber logs the error and skips the remaining hooks of that kind. The app keeps starting. |
| `OnPreShutdown`, `OnPostShutdown`, `OnFork` | Fiber logs the error and still runs the remaining hooks. |
