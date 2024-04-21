---
id: whats_new
title: 🆕 Whats New in v3
sidebar_position: 2
---

:::caution

Its a draft, not finished yet.

:::

[//]: # (https://github.com/gofiber/fiber/releases/tag/v3.0.0-beta.2)

## 🎉 Welcome to Fiber v3

We are excited to announce the release of Fiber v3! 🚀

Fiber v3 is a major release with a lot of new features, improvements, and breaking changes. We have worked hard to make Fiber even faster, more flexible, and easier to use.

## 🚀 Highlights

### Drop for old Go versions

Fiber v3 drops support for Go versions below 1.21. We recommend upgrading to Go 1.21 or higher to use Fiber v3.

### App changes

We have made several changes to the Fiber app, including:

* Listen -> unified with config
* app.Config properties moved to listen config
  * DisableStartupMessage
  * EnablePrefork -> previously Prefork
  * EnablePrintRoutes
  * ListenerNetwork -> previously Network

#### new methods

* RegisterCustomBinder
* RegisterCustomConstraint
* NewCtxFunc

#### removed methods

* Mount -> Use app.Use() instead
* ListenTLS -> Use app.Listen() with tls.Config
* ListenTLSWithCertificate -> Use app.Listen() with tls.Config
* ListenMutualTLS -> Use app.Listen() with tls.Config
* ListenMutualTLSWithCertificate -> Use app.Listen() with tls.Config

#### changed methods

* Routing methods -> Get(), Post(), Put(), Delete(), Patch(), Options(), Trace(), Connect() and All()
* Use -> can be used for app mounting
* Test -> timeout changed to 1 second
* Listen -> has a config parameter
* Listener -> has a config parameter

### Context change
#### interface 
#### customizable

#### new methods

* AutoFormat -> ExpressJs like
* Host -> ExpressJs like
* Port -> ExpressJs like
* IsProxyTrusted
* Reset
* Schema -> ExpressJs like
* SendStream -> ExpressJs like
* SendString -> ExpressJs like
* String -> ExpressJs like
* ViewBind -> instead of Bind

#### removed methods

* AllParams -> c.Bind().URL() ?
* ParamsInt -> Params Generic
* QueryBool -> Query Generic
* QueryFloat -> Query Generic
* QueryInt -> Query Generic
* BodyParser -> c.Bind().Body()
* CookieParser -> c.Bind().Cookie()
* ParamsParser -> c.Bind().URL()
* RedirectToRoute -> c.Redirect().Route()
* RedirectBack -> c.Redirect().Back()
* ReqHeaderParser -> c.Bind().Header()

#### changed methods

* Bind -> for Binding instead of View, us c.ViewBind()
* Format -> Param: body interface{} -> handlers ...ResFmt
* Redirect -> c.Redirect().To()

### Client package


### Binding
### Generic functions

### Middleware refactoring
#### Session middleware
#### Filesystem middleware
### Monitor middleware

Monitor middleware is now in Contrib package.

## Migration guide
...
