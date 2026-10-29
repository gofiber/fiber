---
id: static
---

# Static

The Static middleware serves assets such as **images**, **CSS**, and **JavaScript**. It answers `GET` and `HEAD` requests and passes every other method to the next handler.

:::info
By default, it serves `index.html` when a directory is requested. Customize this behavior in the [Config](#config) options.
:::

## Signatures

```go
func New(root string, cfg ...Config) fiber.Handler
```

## Examples

Import the package:

```go
import(
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/static"
)
```

### Serving files from a directory

```go
app.Get("/*", static.New("./public"))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/hello.html
curl http://localhost:3000/css/style.css
```

</details>

### Serving files from a directory with `Use`

```go
app.Use("/", static.New("./public"))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/hello.html
curl http://localhost:3000/css/style.css
```

</details>

### Serving a single file

```go
app.Use("/static", static.New("./public/hello.html"))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/static # will show hello.html
curl http://localhost:3000/static/john/doe # will show hello.html
```

</details>

### Serving files using os.DirFS

```go
app.Get("/files*", static.New("", static.Config{
    FS:     os.DirFS("files"),
    Browse: true,
}))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/files/css/style.css
curl http://localhost:3000/files/index.html
```

</details>

### Serving files using embed.FS

```go
//go:embed path/to/files
var myfiles embed.FS

app.Get("/files*", static.New("path/to/files", static.Config{
    FS:     myfiles,
    Browse: true,
}))
```

With `FS` set, the first argument names the directory inside the file system to serve, and `""` serves its root. An `embed.FS` keeps the path it embedded, so pass that path.

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/files/css/style.css
curl http://localhost:3000/files/index.html
```

</details>

### SPA (Single Page Application)

```go
app.Use("/web", static.New("", static.Config{
    FS: os.DirFS("dist"),
}))

app.Get("/web*", func(c fiber.Ctx) error {
    return c.SendFile("dist/index.html")
})
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/web/css/style.css
curl http://localhost:3000/web/index.html
curl http://localhost:3000/web
```

</details>

:::caution
To define static routes using `Get`, append the wildcard (`*`) operator at the end of the route. Under a `/static*` route the rest of the path must start at a segment boundary, so `/static-v2/app.js` or `/staticapp.js` are no longer served; only `/static` and paths below `/static/` are.
:::

:::info
The file server resolves the path the router matched. Fiber normalizes request paths before routing (see [Path normalization](../guide/routing.md#path-normalization)), so middleware mounted on `/static/private` also guards `/static/%70rivate/secret.txt` and `/static/x/../private/secret.txt`.

Escapes the router leaves encoded, such as `%20`, `%40` or `%25`, also remain encoded for file lookup. This keeps routing, including path-scoped authorization middleware, and static-file resolution on the same resource name. Consequently, `/static/hello%20world.txt` does not serve `hello world.txt`; enable [`UnescapePath`](../api/fiber.md#config) when encoded spellings should resolve to decoded file names.

A path that cannot name a file inside the root is answered with `404 Not Found`:

- an escape that represents a slash, a backslash or a control character, so `/static/private%2Fsecret.txt` never reaches `private/secret.txt` unless `UnescapePath` decodes it for the router as well;
- a malformed escape such as `%zz`;
- an escape of an unreserved character. The router decodes every one of those when it normalizes a path, so one still in the routed path can only have been forged by a stray `%`, and it is rejected rather than decoded a second time: `/static/%%370rivate/secret.txt` never opens `private/secret.txt`;
- an empty, `.` or `..` segment, so `/static//private/secret.txt` is not found;
- for a root on the operating system's file system, a first segment that starts with a drive letter such as `C:`.

With `UnescapePath` enabled the router has already decoded every escape, so a malformed escape is an ordinary part of the name: `/static/100%zz.txt` names `100%zz.txt`.

A root mount, such as `app.Use(static.New(dir))` or `app.Get("/*", static.New(dir))`, refuses a path whose first segment is empty too, whether it serves a directory or a single file. The router keeps the empty first segment of `//private/secret.txt` apart from `/private/secret.txt`, so a root mount answers it with `404 Not Found` instead of serving anything for a path that middleware mounted on `/private` never saw. That includes any spelling the router turns into such a path, such as `/.//private/secret.txt` or, with `UnescapePath` enabled, `/%2Fprivate/secret.txt`.

A root that is a single file looks nothing up by the path, so it serves its file for every other path the route matches, whatever its spelling, and no spelling of a path can select another file. An empty segment further in is an ordinary part of a name there, as in `/web/https://example.com`.
:::

:::tip
A client that puts a doubled slash in front of a path, such as a base URL that ends in `/` joined to a path that starts with `/`, gets `404 Not Found` from a root mount. To keep serving it, collapse the slash with the [Rewrite](./rewrite.md) middleware before any middleware that matches a path prefix runs. The middleware registered after it, one guarding `/private` included, then sees `/private/secret.txt`:

```go
app.Use(rewrite.New(rewrite.Config{
    RuleList: []rewrite.Rule{{From: "//*", To: "/$1"}},
}))
```

:::

:::caution
The file server opens the name the router matched, so it is only as strict as the filesystem underneath. On Windows a path component loses its trailing dots and spaces when it is opened, so `/static/private./secret.txt` would reach `private/secret.txt` without passing middleware mounted on `/static/private`; the file server answers such a segment with `404 Not Found` on Windows, whatever the root: an `fs.FS` may be OS-backed (`os.DirFS`) and cannot be told apart from a virtual one such as `embed.FS`, so an embedded name ending in a dot or a space is not served on Windows either. Case is a different matter: on a case-insensitive filesystem (Windows, and macOS by default) `/static/PRIVATE/secret.txt` opens `private/secret.txt`, and with [`CaseSensitive`](../api/fiber.md#config) enabled the router keeps the two apart, so middleware mounted on `/static/private` does not run. Leave `CaseSensitive` off (the default) for a static root with guarded directories, or serve it from a case-sensitive filesystem.
:::

## Config

| Property   | Type                    | Description                                                                                                                | Default                |
|:-----------|:------------------------|:---------------------------------------------------------------------------------------------------------------------------|:-----------------------|
| Next       | `func(fiber.Ctx) bool` | Next defines a function to skip this middleware when it returns true.                                                                              | `nil`                  |
| FS       | `fs.FS` | FS is the file system to serve the static files from.<br /><br />You can use interfaces compatible with fs.FS like embed.FS, os.DirFS etc. With `FS` set, the `root` argument of `New` names a directory inside it, and `""` serves its root.                                                 | `nil`                  |
| Compress       | `bool` | When set to true, the server tries minimizing CPU usage by caching compressed files. The middleware will compress the response using `gzip`, `brotli`, or `zstd` compression depending on the [Accept-Encoding](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept-Encoding) header. <br /><br />This works differently than the github.com/gofiber/compression middleware.                                                                              | `false`                  |
| ByteRange       | `bool` | When set to true, enables byte range requests. A request for several ranges, or whose `If-Range` does not match, gets the whole file. | `false`                  |
| Browse       | `bool` | When set to true, enables directory browsing.                                                                             | `false`                  |
| Download       | `bool` | When set to true, enables direct download.                                                                             | `false`                  |
| AllowSymlinksOutsideRoot       | `bool` | When set to true, symbolic links that leave the root, and absolute ones, are followed. Has no effect with `FS`.                                                                             | `false`                  |
| IndexNames       | `[]string` | The names of the index files for serving a directory.                                                                             | `[]string{"index.html"}`                  |
| CacheDuration       | `time.Duration` | Expiration duration for inactive file handlers.<br /><br />Use a negative time.Duration to disable it.                                                                             | `10 * time.Second`                  |
| MaxAge       | `int` | The value for the Cache-Control HTTP-header that is set on the file response. MaxAge is defined in seconds.                                                                             | `0`                  |
| ModifyResponse       | `fiber.Handler` | ModifyResponse defines a function that allows you to alter the response.                                                                             | `nil`                  |
| NotFoundHandler       | `fiber.Handler` | NotFoundHandler defines a function to handle when the path is not found.                                                                             | `nil`                  |

When **Download** is enabled, the response of a served file includes a `Content-Disposition` header with the requested filename, while the `Content-Type` the file server detected is kept. Non-ASCII names use the `filename*` parameter as defined by [RFC 6266](https://www.rfc-editor.org/rfc/rfc6266) and [RFC 8187](https://www.rfc-editor.org/rfc/rfc8187). A request for a missing file falls through to the next handler without the header.

**MaxAge** only applies to successful responses; an error such as `416 Range Not Satisfiable` carries no `Cache-Control`. A handler registered on several routes serves each under that route's own prefix.

A root on disk is confined: a relative symbolic link that stays inside it is followed, while one that leaves it, and an absolute link even when it points inside, is handled like a missing file. Set **AllowSymlinksOutsideRoot** when such links are intended, for example a shared asset directory or a build tool that writes absolute links. With `Compress`, the compressed copy of a file reached through a linked directory is written into that directory. A file root, and a root given through `FS`, are not confined this way.

:::info
You can set `CacheDuration` config property to `-1` to disable caching.
:::

## Default Config

```go
var ConfigDefault = Config{
    IndexNames:    []string{"index.html"},
    CacheDuration: 10 * time.Second,
}
```
