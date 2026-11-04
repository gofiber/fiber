# Contributing

Before making any changes to this repository, we kindly request you to initiate discussions for proposed changes that do not yet have an associated [issue](https://github.com/gofiber/fiber/issues). Please use our [Discord](https://gofiber.io/discord) server to initiate these discussions. For [issue](https://github.com/gofiber/fiber/issues) that already exist, you may proceed with discussions using our [issue](https://github.com/gofiber/fiber/issues) tracker or any other suitable method, in consultation with the repository owners. Your collaboration is greatly appreciated.

Please note: we have a [code of conduct](https://github.com/gofiber/fiber/blob/main/.github/CODE_OF_CONDUCT.md), please follow it in all your interactions with the `Fiber` project.

## Pull Requests or Commits

Titles always we must use prefix according to below:

> 🐛 bug:, 🔥 feat:, 📒 docs:, 🧹 chore:

- 🐛 bug: Improve flow
- 🔥 feat: Add flow to add person
- 📒 docs: Translate to Portuguese middleware redirect
- 🧹 chore: Rename file X to Y

All pull requests that contain a feature or fix are mandatory to have unit tests. Your PR is only to be merged if you respect this flow.

Please do not copy the request/response plumbing from one test into the next, and do not write another fake storage or clock: the helpers in [`internal/apptest`](https://github.com/gofiber/fiber/tree/main/internal/apptest), [`internal/storagetest`](https://github.com/gofiber/fiber/tree/main/internal/storagetest) and [`internal/clocktest`](https://github.com/gofiber/fiber/tree/main/internal/clocktest) cover them, and tests that differ only in their data belong in one table-driven test with a subtest per row. Every test and subtest starts with `t.Parallel()`, except a test that calls `testing.AllocsPerRun` and one that installs package-level state other tests read: those stay sequential and say why in a comment. The helpers, these exceptions in detail and the compact layout the test files use (single-statement closures and short table rows on one line, no blank line after `t.Parallel()`) are described in [`AGENTS.md`](https://github.com/gofiber/fiber/blob/main/AGENTS.md), and `go test ./internal/testconventions` checks them: a justified exception takes a `// conventions:ignore <rule>: <reason>` comment above the test, and a marker that silences nothing fails the check.

## 👍 Contribute

If you want to say **thank you** and/or support the active development of `Fiber`:

1. Add a [GitHub Star](https://github.com/gofiber/fiber/stargazers) to the project.
2. Tweet about the project [on your 𝕏 (Twitter)](https://x.com/intent/tweet?text=%F0%9F%9A%80%20Fiber%20%E2%80%94%20is%20an%20Express.js%20inspired%20web%20framework%20build%20on%20Fasthttp%20for%20%23Go%20https%3A%2F%2Fgithub.com%2Fgofiber%2Ffiber).
3. Write a review or tutorial on [Medium](https://medium.com/), [Dev.to](https://dev.to/) or personal blog.
4. Support the project by donating a [cup of coffee](https://buymeacoff.ee/fenny).

## Generated API documentation

Some Markdown sections are generated from Go declarations and executable examples.
The Go source is canonical: edit it and run `make generate` to update the page.

- A page is generated only when its package declares
  `//go:generate go run ../../internal/genapidocs -out <page>` (see `addon/retry/doc.go`).
- Each block is a `<!-- go:doc Config -->` line and a `<!-- /go:doc -->` line, both on
  lines of their own; the body between them may be empty. Names are exported
  declarations (`Type`, `Func`, `Type.Method`, `Type.Field`, a const or var name) or example
  functions (`ExampleType_Method`); examples must be standalone programs.
- Prose outside the blocks is preserved, so do not restate generated facts there.
- The `Generated API docs` workflow fails when a page drifts from its source.

Markers inside fenced or indented code blocks and inline examples are ignored.
Generated text must not contain marker strings; the page's LF or CRLF line ending is preserved.
A page with `<!-- go:doc-all -->` must reference every exported declaration and method;
fields and interface methods are included by their containing type, and const/var groups count once.
The retry page enables this check, so new exported API requires a documentation update.
Its README and the release notes link to `docs/addon/retry.md` rather than maintaining copies.
Declarations retain source expressions, including references to private constants; only examples are standalone programs.
