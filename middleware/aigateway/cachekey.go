package aigateway

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
)

// CacheKeyGenerator returns a KeyGenerator for the cache middleware that
// makes LLM POST requests cacheable: the key is a SHA-256 over the request
// path, query string, body, and the client credential, so identical prompts
// hit the cache while different clients, bodies, or endpoints never share an
// entry. Mount cache.New *before* the gateway:
//
//	app.Use(cache.New(cache.Config{
//	    Methods:      []string{fiber.MethodPost},
//	    KeyGenerator: aigateway.CacheKeyGenerator(),
//	    Next:         aigateway.CacheSkipStreaming(),
//	}))
//	app.Use("/openai", aigateway.New(...))
//
// The credential is folded in because a cache hit is served before the
// gateway runs — without it, one client's cached completion could be replayed
// to another. It is read with the gateway's default extractor chain
// (Authorization: Bearer, x-api-key, api-key); pass a custom extractor when
// the gateway uses one. Note the remaining sharp edges, which are inherent to
// fronting the gateway with a cache: a hit bypasses key validation, quotas,
// hooks, and usage accounting entirely (a revoked key can replay its own
// cached entries until they expire), and the cache middleware only stores
// responses to requests carrying an Authorization header when the response
// has explicit shared-cache directives (Cache-Control: public/s-maxage).
func CacheKeyGenerator(extractor ...extractors.Extractor) func(fiber.Ctx) string {
	ext := defaultKeyExtractor()
	if len(extractor) > 0 {
		ext = extractor[0]
	}
	return func(c fiber.Ctx) string {
		key, err := ext.Extract(c)
		if err != nil && !errors.Is(err, extractors.ErrNotFound) {
			key = ""
		}
		h := sha256.New()
		_, _ = h.Write([]byte(c.Path()))                   //nolint:errcheck // sha256 never errors
		_, _ = h.Write([]byte{0})                          //nolint:errcheck // sha256 never errors
		_, _ = h.Write(c.RequestCtx().URI().QueryString()) //nolint:errcheck // sha256 never errors
		_, _ = h.Write([]byte{0})                          //nolint:errcheck // sha256 never errors
		_, _ = h.Write(c.BodyRaw())                        //nolint:errcheck // sha256 never errors
		_, _ = h.Write([]byte{0})                          //nolint:errcheck // sha256 never errors
		_, _ = h.Write([]byte(key))                        //nolint:errcheck // sha256 never errors
		return hex.EncodeToString(h.Sum(nil))
	}
}

// CacheSkipStreaming returns a Next predicate for the cache middleware that
// prevents storing responses that must not be cached: streaming responses
// (SSE / NDJSON — the cache's store path would buffer the whole stream),
// requests that asked for one ("stream": true), and bodies the gateway
// cannot inspect. The cache middleware evaluates Next after the handler ran,
// so the response Content-Type is available here.
func CacheSkipStreaming() func(fiber.Ctx) bool {
	return func(c fiber.Ctx) bool {
		// Response side: never store a streaming Content-Type.
		ct := string(c.Response().Header.ContentType())
		for _, prefix := range streamingContentTypes {
			if strings.HasPrefix(strings.ToLower(ct), prefix) {
				return true
			}
		}

		// Request side: skip bodies that requested a stream or that cannot
		// be verified not to have.
		body := c.BodyRaw()
		if len(body) == 0 {
			return false
		}
		if enc := c.Get(fiber.HeaderContentEncoding); enc != "" && !strings.EqualFold(strings.TrimSpace(enc), "identity") {
			return true // encoded body: cannot cheaply verify, do not cache
		}
		var m struct {
			Stream bool `json:"stream"`
		}
		if err := c.App().Config().JSONDecoder(body, &m); err != nil {
			return true // undecodable body: play it safe
		}
		return m.Stream
	}
}
