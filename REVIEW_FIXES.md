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

---

## Round 2 (after the comment trim; `origin/main...HEAD` at `cea1862`)

Delete this file before merging; it is a working checklist, not project documentation.

Status: `[x]` done.

### Hard violations (Standards)

- [x] **R2-H1** utils/v2 helpers exist for: `strconv.Atoi(utils.TrimSpace(..))` (`constraint.go:157`), `strconv.Itoa` (`registry.go:82`), `strings.Split*` + `TrimSpace` (`schema.go:503`, `schema.go:548`).
  - `constraint.go` uses `utils.ParseInt`, `registry.go` `utils.FormatInt`, `schema.go` `utils.SplitTrimSeq` (empty list elements are now skipped).
- [x] **R2-H2** `router.go:120` says to keep `Route` in sync with `app.copyRoute`; the functions that need field sync are `copyRouteBaseInto`, `isDocumented` and `cloneRouteDocInto`.
  - The `Route` comment now names what must stay in sync (`cloneRouteDocInto`, `isDocumented`, `copyRouteBaseInto`) and the test that guards it.
- [x] **R2-H3** `ShutdownWithContext` says twice that `app.mutex` must not be held across the drain.
  - The duplicate comment is gone; one remains.
- [x] **R2-H4** `docs/api/hooks.md` does not say `OnRoute`/`OnName` now run unlocked and receive a snapshot.
  - `docs/api/hooks.md` and `docs/whats_new.md` now say the hooks run unlocked with a snapshot.
- [x] **R2-H5** `GetRoute`/`GetRoutes` are documented as deep copies, but `Handlers`, `Params` and `docHandlers` alias the live route.
  - `GetRoute`/`GetRoutes` comments, `copyRouteInto` and `docs/api/app.md` now say documentation metadata is cloned and `Handlers`/`Params` are shared.
- [x] **R2-H6** The exported `Route` fields `Summary`, `Description`, `Consumes`, `Produces`, `Responses`, `Parameters`, `Tags`, `Deprecated` have no godoc.
  - Every exported `Route` documentation field has a godoc line.

### Judgement calls (Standards)

- [x] **R2-J1** `deepCopyAny*`/`cloneSecurityRequirements` (openapi/config.go) duplicate `copyAny*`/`cloneRouteSecurity` (router_docs.go); `maxCopyDepth` is declared twice.
  - New `internal/deepcopy` (Map, Value, Security) is used by both the core and the middleware; both private copies and the second `maxCopyDepth` are gone, with tests moved over.
- [x] **R2-J2** The case-rule setup (`EqualFold` vs `stringsEqual`) appears twice in `openapi.go`.
  - `segmentEqualFor(caseSensitive)` replaces both copies.
- [x] **R2-J3** `docAddParameter` and `docAddParameterModel` each switch over the allowed locations.
  - `validParamLocation(location, querystring)` replaces both switches.
- [x] **R2-J4** The exported `ParamIn*` constants are ignored by the middleware, which keeps its own location literals.
  - The middleware uses `fiber.ParamIn*` for every location literal; its own constants are removed. The core keeps `openapiRefKey`/`openapiTypeString` for its own schema maps.
- [x] **R2-J5** Adding a documentation field to `Route` means editing three functions; a missed one silently aliases or leaks data. Guard test coverage of `copyRouteBaseInto`/`cloneRouteDocInto`.
  - `Test_Route_DocumentationFieldsStayInSync` now also checks that `copyRouteInto` returns a new container for every populated container field (verified by breaking `Tags` on purpose).
- [x] **R2-J6** `copyRouteValue` is a middle man with one caller.
  - `copyRouteValue` is inlined into `copyRoute`.
- [x] **R2-J7** `maxCachedSwaggerPages` also bounds the per-app cache map; `cloned` is shadowed in `cloneRouteSecurity`.
  - `maxCachedSwaggerPages` is now `maxCachedEntries`; the shadowed `cloned` went with the move to `deepcopy.Security`.
- [x] **R2-J8** `Registering.wrap` and `Registering.domain` are always set together.
  - `Registering` holds one `domain *domainRouter` instead of `wrap` plus `domain`.
- [x] **R2-J9** `app_routedocs.go` still holds the naming and name-index code next to the documentation helpers.
  - The naming and name-index code moved to `app_names.go`; `app_routedocs.go` keeps the documentation helpers.
- [x] **R2-J10** `router.go` sets `Summary`, `Description`, `Consumes`, `Produces` to `""` explicitly.
  - The four empty-string initializers are removed.
- [x] **R2-J11** Swagger UI: `crossorigin` is on the scripts unconditionally but on the stylesheet only with an integrity value; a CORS-less custom CDN breaks the scripts.
  - `crossorigin` is emitted on a script or stylesheet only together with its `integrity` value; the test asserts a custom CDN gets neither.

### Spec issues

- [x] **R2-S1** A sub-app mounted through a domain router (`d.Use("/dm", sub)`) loses its host: no `servers` entry.
  - `domainRoutes` sets `Route.Domain` on the routes of a mounted sub-app (one the sub-app already scoped keeps its own), so the operation carries the `servers` entry; `Test_OpenAPI_DomainMountedSubAppCarriesTheHost`.
- [x] **R2-S2** Route names become `operationId` verbatim; a name with spaces or other odd characters leaks into the spec.
  - `operationIDFromName` keeps letters, digits, `_`, `-`, `.` and turns other runs into `_`; a name with nothing usable falls back to the generated id. Tested and documented.
- [x] **R2-S3** `docs/middleware/openapi.md:47` says every response carries `application/json`; 204 responses carry no content.
  - The docs now say a `204` has no body.
- [x] **R2-S4** Documentation changed on a sub-app after the parent expanded the mount is not reflected in the parent; undocumented.
  - `docs/middleware/openapi.md` says to document a sub-app before the parent starts serving, because the parent copies its routes at startup.
- [x] **R2-S5** `keyauth` is always documented as `bearerAuth` whatever extractor it uses; make sure the override path is documented and tested.
  - Already covered by `explicit security and user schemes win` (a user `bearerAuth` replaces the inferred scheme) and documented; the docs now also say an inferred middleware cannot see `Next`.

### Scope creep

- [x] **R2-C1** Behaviour changes existing apps can notice are not all called out: `RemoveRouteFunc` gets copies; hooks fire unlocked with snapshots; `Name()` after a mount is a no-op; same-path routes on different domains no longer merge or share an auto-HEAD twin; `Register` gained methods.
  - `docs/whats_new.md` has a caution listing each change (`Name()` after a mount, hooks, `RemoveRouteFunc` copies, domain routes, copies from `GetRoute`, `Register`), each checked against the code.
- [x] **R2-C2** Handler-name summaries are on by default and put Go identifiers in a public spec.
  - Kept on by the maintainer's earlier decision; the docs now say it puts Go function names in the public document and how to turn it off.
- [x] **R2-C3** `REVIEW_FIXES.md` is committed at the repo root.
  - The file now opens with "Delete this file before merging"; it stays in the repo because the git hook requires committed files.
