---
id: hooks
title: 🎣 Hooks
sidebar_position: 7
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

With Fiber you can execute custom user functions at specific method execution points. Here is a list of these hooks:

- [OnRoute](#onroute)
- [OnName](#onname)
- [OnGroup](#ongroup)
- [OnGroupName](#ongroupname)
- [OnListen](#onlisten)
- [OnFork](#onfork)
- [OnShutdown](#onshutdown)
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

`OnRoute` is a hook to execute user functions on each route registration. You can access route properties via the **route** parameter.

```go title="Signature"
func (h *Hooks) OnRoute(handler ...OnRouteHandler)
```

## OnName

`OnName` is a hook to execute user functions on each route naming. You can access route properties via the **route** parameter.

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

`OnGroup` is a hook to execute user functions on each group registration. You can access group properties via the **group** parameter.

```go title="Signature"
func (h *Hooks) OnGroup(handler ...OnGroupHandler)
```

## OnGroupName

`OnGroupName` is a hook to execute user functions on each group naming. You can access group properties via the **group** parameter.

:::caution
`OnGroupName` only works with named groups, not routes.
:::

```go title="Signature"
func (h *Hooks) OnGroupName(handler ...OnGroupNameHandler)
```

## OnListen

`OnListen` is a hook to execute user functions on `Listen`, `ListenTLS`, and `Listener`.

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

## OnFork

`OnFork` is a hook to execute user functions on fork.

```go title="Signature"
func (h *Hooks) OnFork(handler ...OnForkHandler)
```

## OnShutdown

in v3, `OnShutdown` is split into `OnPreShutdown` and `OnPostShutdown`.

`OnShutdown` is a hook to execute user functions after shutdown.

```go title="Signature"
func (h *Hooks) OnShutdown(handler ...OnShutdownHandler)
```

### OnPreShutdown

`OnPreShutdown` is a hook to execute user functions before shutdown.

```go title="Signature"
func (h *Hooks) OnPreShutdown(handler ...OnPreShutdownHandler)
```

### OnPostShutdown

`OnPostShutdown` is a hook to execute user functions after shutdown.

```go title="Signature"
func (h *Hooks) OnPostShutdown(handler ...OnPostShutdownHandler)
```

## OnMount

`OnMount` is a hook to execute user functions after the mounting process. The mount event is fired when a sub-app is mounted on a parent app. The parent app is passed as a parameter. It works for both app and group mounting.

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
OnName/OnRoute/OnGroup/OnGroupName hooks are mount-sensitive. If you use one of these routes on sub app, and you mount it; paths of routes and groups will start with mount prefix.
