---
id: redirect
---

# Redirect

Redirect middleware maps old URLs to new ones using simple rules.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/redirect"
)

func main() {
    app := fiber.New()

    app.Use(redirect.New(redirect.Config{
      Rules: map[string]string{
        "/old":   "/new",
        "/old/*": "/new/$1",
      },
      StatusCode: fiber.StatusMovedPermanently,
    }))

    app.Get("/new", func(c fiber.Ctx) error {
      return c.SendString("Hello, World!")
    })
    app.Get("/new/*", func(c fiber.Ctx) error {
      return c.SendString("Wildcard: " + c.Params("*"))
    })

    app.Listen(":3000")
}
```

## Test

```bash
curl http://localhost:3000/old
curl http://localhost:3000/old/hello
```

## Config

| Property   | Type                | Description                               | Default                |
|:-----------|:--------------------|:------------------------------------------|:-----------------------|
| Next       | `func(fiber.Ctx) bool` | Skip when function returns true.          | nil                    |
| Rules      | `map[string]string`   | Map paths to new ones; `$1`, `$2` insert params. | Required               |
| StatusCode | `int`                 | HTTP code for redirects.                  | 302 Temporary Redirect |

## How captures are placed

The values a rule captures come from the request path, so where a `$N` sits in
the target decides what it is allowed to contribute.

**A target with no host of its own** — `"/api/*": "/$1"` — always redirects
within this application. The composed location is read the way a browser reads
it, then held to this origin: a leading run of slashes collapses to one, and a
capture that turned the location into an absolute URL is rooted as a path.
Without that, `/api//evil.com` would send `Location: //evil.com`, which the
browser follows to evil.com.

**A target naming its own host** — `"/ext/*": "https://cdn.example.com/$1"` —
redirects wherever it says. A `$N` after the host may hold anything, including
slashes; it can only add to the path.

**A target with a `$N` inside its host** — `"/cdn/*": "https://$1.cdn.example.com/"` —
requires the capture to be a label of that host. The rule is skipped, and the request falls
through to the rest of the stack, whenever the captured value would move the
host instead: one containing `/`, `\`, `?`, `#`, `@` or `:` in that position, or
one that extends the host where the target ends in `$N`
(`"https://cdn.example.com$1"` accepts `/foo.png`, which starts a path, but not
`foo.png` or `@evil.com`).

::::caution

A target that hands the whole host to the request — `"/go/*": "https://$1"`,
`"//$1"`, or `"https:$1"` — is an open redirect: anyone able to shape the path
chooses where the client lands. **Such a rule never fires**, and Fiber logs a
warning naming it at startup, since there is no way to tell the intended host
from an attacker's. Pin the host in the target instead.

A capture hands over the host whenever nothing beside it in the authority is
host text, so these are the same case and are refused too:

| Target | Why it pins no host |
| --- | --- |
| `"https://$1:8080"`, `"https://$1:$2"` | A port is not a host — `evil.com:8080` is still `evil.com`. |
| `"//$1."`, `"//.$1"` | `"///evil.com."` is read host-first by the URL parser; `evil.com.` is `evil.com` with the DNS root spelled out. |
| `"https://$1$2"` | A second capture pins nothing an attacker does not also supply. |
| `"https://example.com@$1"` | Everything before an `@` is a username, so the host is still the capture's. |
| `"https://\t$1"` | A tab, LF or CR is deleted by the URL parser before it reads the host, so it pins nothing that survives. |
| `"https://$1\u00ad"` | Same for the code points UTS #46 mapping deletes — soft hyphen, zero-width space, BOM — and for those it folds onto a plain `.`, such as the ideographic full stop. Percent-escaped spellings such as `%2E` and `%C2%AD` count too, since the parser decodes a host before reading it. |
| `"https://$1.1"`, `"https://$1.0x1"` | A host whose last label reads as a number is parsed as an IPv4 address, so the author's trailing text is the low octets and the request supplies the network — `/r/127.0.0` would reach `127.0.0.1`. A complete address is fine: `"https://127.0.0.1:$1"` pins the host and captures the port. |
| `"https://$1xyz"`, `"https://$1cafe"` | With no dot of its own the text sits inside a label the capture opens, so it is a label's tail and not a label. The request decides what the label becomes: `/r/evil.` reaches `evil.xyz`, and `/r/0x` reaches `0.0.202.254`. Give it a dot — `"https://$1.cdn.example.com"` pins that domain. |

Note that `"https://$1@example.com"` is fine — there the author's host follows
the `@` and the capture is only userinfo — as are `"https://cdn.example.com:$1"`
and `"https://tenant-$1.example.com"`.

:::danger Pin a domain you control, not just a suffix

These checks ask whether the *author* wrote the host, not whether the host is
one you own. `"https://$1.com"` and `"https://$1.xyz"` pass — the capture is a
label under a suffix the target names — but anyone can register a name under
`.com` or `.xyz`, so `/go/evil` still reaches `evil.com`. The same goes for a
suffix shared with other tenants.

Fiber has no list of public suffixes and cannot tell `"$1.com"` from
`"$1.example.com"`. Pin enough of the host that every name below it is yours,
and where the set of destinations is known, prefer matching the capture against
an allowlist in your own handler over interpolating it into a host at all.

:::

A capture inside the brackets of an IPv6 literal is refused separately, and for
its own reason — one side of it genuinely does pin a host, so the rules above do
not decide the case.

A DNS name is written least-significant label first, so either side of a capture
can pin it: `"https://$1.example.com"` stays under `example.com`, and
`"https://cdn.example.com$1"` stays on `cdn.example.com`. A bracketed address is
written the other way round, most-significant group first. There
`"https://[$1::1]"` leaves the routing prefix to the request — enough to reach
loopback or a link-local address — while `"https://[2001:db8::$1]"` really does
pin the network and only lets the request choose the last group.

Deciding those two apart needs a rule that is the reverse of the one used
everywhere else, so rather than keep two opposite readings in step Fiber refuses
both and logs a warning saying which case it is. Write the address in full and
capture the port instead:

```go
"/r/*": "https://[2001:db8::1]:$1",  // fine
```

A refused rule is dropped, so the request continues to the **remaining rules of
this middleware** before reaching the rest of the stack. With
`{"/go/*": "https://$1", "/*": "/home"}`, a request for `/go/evil.com` is
redirected to `/home` by the catch-all.

This applies to every scheme, not only the ones a browser navigates by. A `//`
opens an authority whatever the scheme is, so `"mailto:$1"`, `"myapp:$1"` and
any custom or deep-link scheme hand the host to a captured `//evil.com` exactly
as `"https:$1"` does; for `http`, `https`, `ws`, `wss` and `ftp` the parser does
not even need the slashes, reading `"ws:evil.com"` as `ws://evil.com`.

Because those schemes reach a host without the slashes, a target that writes
one without them names an authority all the same, and every rule above applies
to it unchanged: `"https:cdn.example.com$1"` is read exactly as
`"https://cdn.example.com$1"` — so it too refuses a capture that would turn the
author's host into userinfo (`/r/@evil.com`) or extend it into another domain
(`/r/.evil.com`).

What decides it otherwise is where the capture sits, not which scheme precedes
it. A capture with the author's text in front of it opens no authority, so
`"myapp:fixed/$1"` and `"https:fixed/$1"` are fine. And a capture at the front
is fine where the author wrote host text after it — in
`"mailto:$1@example.com"` the parser reads `example.com` as the host and the
capture as userinfo. `mailto` is not one of the special schemes, so without a
`//` it has no authority at all.

::::

## Default Config

```go
var ConfigDefault = Config{
    StatusCode: fiber.StatusFound,
}
```
