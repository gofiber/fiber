## Fiber Core Middleware

- [middleware/compress](compress.md)
Compression middleware for Fiber, it supports `deflate`, `gzip` and `brotli`.
- [middleware/favicon](favicon.md)
This middleware caches the favicon in memory to improve performance
- [middleware/filesystem](filesystem.md)
FileServer middleware to allow embedded http.FileSystem
- [middleware/logger](logger.md)
HTTP request/response logger for Fiber
- [middleware/recover](recover.md)
Recover middleware recovers from panics anywhere in the stack chain
- [middleware/requestid](request_id.md)
Adds an indentifier to the response using the `X-Request-ID` header
- [middleware/timeout](timeout.md)
Wrapper function which provides a handler with a timeout.