# Review findings: tracking (PR #3702, `origin/main...HEAD`)

Status: `[x]` done.

## Hard violations (Standards)

- [x] **H1** `middleware/openapi/paths.go:207-209` trims a trailing slash with a hand-written `strings.HasSuffix` loop; use `utils.TrimRight`, keeping `/` as `/`.
  - `paths.go` now uses `utils.TrimRight`; a path of only slashes still becomes `/`.
- [x] **H2** `middleware/openapi/openapi.go:322-325` the `prefixSegmentBounds` doc comment sits above `segmentKind`; the function has none.
  - The comment now sits directly above `prefixSegmentBounds`.
- [x] **H3** Declarations not at the top of the file: `chain.go` (`authMiddleware`, `knownMiddleware`), `schema.go:123` (reflect-type `var` block).
  - `authMiddleware`, the middleware tables, `fiberRoot` (chain.go) and the reflect-type vars (schema.go) are at the top of their files.
- [x] **H3b** `router.go` `skipSlashFilters` / `routeIDs` sat mid-file (they predate this branch, but it is a one-line move).
  - Both are now at the top of `router.go`.

## Judgement calls (Standards)

- [x] **J1** Route-documentation helpers are written once each on `App`, `Group` and `domainRouter` (about 25 helpers, 75 methods).
  - The 40 Group/domain forwards are now one-liners over a single `document` helper per receiver; the apply-to-last-registration logic lives once in each. The interface entry plus three receivers per helper remains by design (Go has no shared implementation across three distinct types without back-pointers).
- [x] **J2** `paths.go:270-290` `sanitizeOpenAPIWildcardParamName` and `sanitizeOpenAPIParamName` open with the same trim-and-fallback.
  - Both sanitizers now share `trimWildcardMarkers`.
- [x] **J3** Weakly typed helper parameters: the `(schema any, schemaRef, example, examples, mediaTypes...)` clump and string locations.
  - The location argument now has exported constants (`fiber.ParamInPath`, `ParamInQuery`, `ParamInHeader`, `ParamInCookie`, `ParamInQuerystring`) instead of magic strings, documented in `docs/api/app.md`. The schema/ref/example/examples clump stays: it is the documented public signature, and `AddParameter`, `RequestBodyContent` and `ResponseContent` take typed structs for callers who want them.
- [x] **J4** Overlapping variants: `Parameter`/`ParameterWithExample`/`AddParameter`, `RequestBody`/`RequestBodyContent`, `Response`/`ResponseContent`.
  - Doc comments on `Parameter`, `ParameterWithExample`, `RequestBody`, `RequestBodyWithExample`, `Response` and `ResponseWithExample` now say which general form each is the short form of, and `docs/api/app.md` says when to use the per-media-type variants. The variants stay: they are the API.
- [x] **J5** The `equal func(a, b string) bool` parameter does not say it is the router's case/strict comparator.
  - The parameter is the named type `segmentEqual`, documented as the router's case rule.
- [x] **J6** `classifySegment` and `paths.go` lex route tokens separately.
  - Both lexers now sit side by side (`segments.go` explains why there are two, and `paths.go` is the other). `Test_RouteLexers_Agree` fails if they disagree on the parameter count of 12 patterns (escapes, constraints, optional, wildcards, mixed segments). Replacing them with the router's parsed segments needs a public router API, which this PR does not add.

## Spec issues

- [x] **S1** Middleware is detected by a source-path substring (`/middleware/keyauth/`), so a user package at `x/middleware/keyauth` is documented as Fiber's `keyauth` (false `security`, `401` and scheme).
  - Fiber's own middleware is matched against the directory `fiber.New` is compiled from (`fiberRoot`), so `x/middleware/keyauth` no longer matches. Verified with `-trimpath`. contrib JWT is matched on the `/gofiber/contrib/v3/jwt` path.
- [x] **S1b** A recognised `keyauth`/`basicauth` with a custom `Next` is documented as always required; the docs do not say so.
  - The docs now say a recognized middleware is documented as always applying and how to scope it.
- [x] **S2** `docs/middleware/openapi.md:745` says only `GET` and `HEAD` routes are documented; the code documents every method.
  - `docs/middleware/openapi.md` now says every registered method is documented.
- [x] **S3** Routes on different domains with the same path collapse to one operation (first wins); the host parameter is never emitted.
  - An operation on a domain route now carries an operation-level `servers` entry (`//host`, `:param` labels become server variables defaulting to their name); `Test_OpenAPI_DomainRoutesCarryTheirHost` covers it. OpenAPI allows one operation per path and method, so two domains sharing both still describe the first-registered one, now labeled with its host. Documented in `docs/middleware/openapi.md`.

## Scope creep

- [x] **C1** `Name()` and the documentation helpers now target only the latest registration (behaviour change in core routing).
  - The `:::caution` in `docs/whats_new.md` (router section) already states it; kept.
- [x] **C2** The `Router` interface gained about 15 methods, which breaks external implementers; not called out as breaking.
  - `docs/whats_new.md` now lists the 24 added `Router` methods and tells implementers what to do.
- [x] **C3** `GetRoute`/`GetRoutes` are deep copies behind a new name index.
  - Decided by the maintainer ("keep, explain in description"). Kept; the reason belongs in the refreshed PR description (documenting routes made `Route` bigger and URL building slower).
- [x] **C4** Inference beyond the spec (summaries, group tags, validator `400`, middleware headers and `304`, CSRF parameter).
  - Decided by the maintainer ("keep, explain in description"). Kept; every inference has a `Disable*` switch and is documented.
- [x] **C5** Default Swagger SRI hashes need a manual bump with each URL change.
  - A test now checks the three default URLs name one `swagger-ui-dist` version and every default hash is `sha384-`; the config comment says to bump them together.
