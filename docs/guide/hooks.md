---
id: hooks
title: 🎣 Hooks
sidebar_position: 6
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

With Fiber v2.30.0, you can execute custom user functions when to run some methods. Here is a list of this hooks:
- [OnRoute](#onroute)
- [OnName](#onname)
- [OnGroup](#ongroup)
- [OnGroupName](#ongroupname)
- [OnListen](#onlisten)
- [OnFork](#onfork)
- [OnShutdown](#onshutdown)
- [OnMount](#onmount)

## Constants
```go
// Handlers define a function to create hooks for Fiber.
type OnRouteHandler = func(Route) error
type OnNameHandler = OnRouteHandler
type OnGroupHandler = func(Group) error
type OnGroupNameHandler = OnGroupHandler
type OnListenHandler = func(ListenData) error
type OnForkHandler = func(int) error
type OnShutdownHandler = func() error
type OnMountHandler = func(*App) error
```

## OnRoute

OnRoute is a hook to execute user functions on each route registeration. Also you can get route properties by **route** parameter.

```go title="Signature"
func (h *Hooks) OnRoute(handler ...OnRouteHandler)
```

## OnName

OnName is a hook to execute user functions on each route naming. Also you can get route properties by **route** parameter.

:::caution
OnName only works with naming routes, not groups.
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

	"github.com/gofiber/fiber/v2"
)

func main() {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
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

	app.Get("/add/user", func(c *fiber.Ctx) error {
		return c.SendString(c.Route().Name)
	}).Name("addUser")

	app.Delete("/destroy/user", func(c *fiber.Ctx) error {
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

OnGroup is a hook to execute user functions on each group registeration. Also you can get group properties by **group** parameter.

```go title="Signature"
func (h *Hooks) OnGroup(handler ...OnGroupHandler)
```

## OnGroupName

OnGroupName is a hook to execute user functions on each group naming. Also you can get group properties by **group** parameter.

:::caution
OnGroupName only works with naming groups, not routes.
:::

```go title="Signature"
func (h *Hooks) OnGroupName(handler ...OnGroupNameHandler)
```

## OnListen

OnListen is a hook to execute user functions on Listen, ListenTLS, Listener.

```go title="Signature"
func (h *Hooks) OnListen(handler ...OnListenHandler)
```

<Tabs>
<TabItem value="onlisten-example" label="OnListen Example">

```go
app := fiber.New(fiber.Config{
  DisableStartupMessage: true,
})

app.Hooks().OnListen(func(listenData fiber.ListenData) error {
  if fiber.IsChild() {
	  return nil
  }
  scheme := "http"
  if data.TLS {
    scheme = "https"
  }
  log.Println(scheme + "://" + listenData.Host + ":" + listenData.Port)
  return nil
})

app.Listen(":5000")
```

</TabItem>
</Tabs>

## OnFork

OnFork is a hook to execute user functions on Fork.

```go title="Signature"
func (h *Hooks) OnFork(handler ...OnForkHandler)
```

## OnShutdown

OnShutdown is a hook to execute user functions after Shutdown.

```go title="Signature"
func (h *Hooks) OnShutdown(handler ...OnShutdownHandler)
```

## OnMount

OnMount is a hook to execute user function after mounting process. The mount event is fired when sub-app is mounted on a parent app. The parent app is passed as a parameter. It works for app and group mounting.

```go title="Signature"
func (h *Hooks) OnMount(handler ...OnMountHandler) 
```

<Tabs>
<TabItem value="onmount-example" label="OnMount Example">

```go
package main

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func main() {
	app := New()
	app.Get("/", testSimpleHandler).Name("x")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	subApp.Hooks().OnMount(func(parent *fiber.App) error {
		fmt.Print("Mount path of parent app: "+parent.MountPath())
		// ...

		return nil
	})

	app.Mount("/sub", subApp)
}

// Result:
// Mount path of parent app: 
```

</TabItem>
</Tabs>


:::caution
OnName/OnRoute/OnGroup/OnGroupName hooks are mount-sensitive. If you use one of these routes on sub app and you mount it; paths of routes and groups will start with mount prefix.
