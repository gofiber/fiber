package aigateway

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/utils/v2"
)

// alwaysSkipHeaders are request headers never relayed upstream: hop-by-hop
// headers (RFC 9110 section 7.6.1) plus framing/routing headers that the
// upstream URL and body determine.
var alwaysSkipHeaders = []string{
	fiber.HeaderHost,
	fiber.HeaderContentLength,
	fiber.HeaderConnection,
	fiber.HeaderKeepAlive,
	fiber.HeaderProxyAuthenticate,
	fiber.HeaderProxyAuthorization,
	fiber.HeaderTE,
	fiber.HeaderTrailer,
	fiber.HeaderTransferEncoding,
	fiber.HeaderUpgrade,
}

// buildRequest constructs a fresh upstream request for one attempt. key is
// the credential to inject: the client's own key in pass-through mode or
// Upstream.Key in unified-key mode.
func buildRequest(c fiber.Ctx, cfg *Config, up *Upstream, strippedPath, key string) *client.Request {
	req := cfg.Client.R()
	req.SetMethod(c.Method())
	req.SetTimeout(cfg.HeaderTimeout)

	if qs := c.RequestCtx().URI().QueryString(); len(qs) > 0 {
		req.SetURL(up.URL + strippedPath + "?" + utils.UnsafeString(qs))
	} else {
		req.SetURL(up.URL + strippedPath)
	}

	// Copy the incoming headers directly onto the raw request; the client's
	// built-in hooks add builder-level headers on top without clearing.
	connectionTokens := connectionHeaderTokens(c)
	for k, v := range c.Request().Header.All() {
		if skipRequestHeader(cfg, utils.UnsafeString(k), connectionTokens) {
			continue
		}
		req.RawRequest.Header.AddBytesKV(k, v)
	}

	// Inject the upstream credential after every configured credential header
	// was dropped by skipRequestHeader.
	val := key
	if up.Auth.Scheme != "" {
		val = up.Auth.Scheme + " " + key
	}
	req.RawRequest.Header.Set(up.Auth.Header, val)

	for k, v := range up.Headers {
		req.RawRequest.Header.Set(k, v)
	}

	// The client force-sets User-Agent; forward the caller's one when present.
	if ua := c.Get(fiber.HeaderUserAgent); ua != "" {
		req.SetUserAgent(ua)
	}

	if body := c.BodyRaw(); len(body) > 0 {
		req.SetRawBody(body)
	}

	return req
}

// connectionHeaderTokens returns the header names listed in the incoming
// Connection header, which are hop-by-hop by declaration. The near-universal
// "keep-alive"/"close" values name no extra headers, so they short-circuit
// without allocating a token slice.
func connectionHeaderTokens(c fiber.Ctx) []string {
	conn := c.Get(fiber.HeaderConnection)
	if conn == "" || utils.EqualFold(conn, "keep-alive") || utils.EqualFold(conn, "close") {
		return nil
	}
	tokens := strings.Split(conn, ",")
	for i := range tokens {
		tokens[i] = utils.TrimSpace(tokens[i])
	}
	return tokens
}

func skipRequestHeader(cfg *Config, name string, connectionTokens []string) bool {
	for _, h := range alwaysSkipHeaders {
		if utils.EqualFold(name, h) {
			return true
		}
	}
	// Credential headers (well-known + every Upstream.Auth.Header + the
	// extractor's headers) are always dropped so the injected upstream key is
	// the only credential and a client cannot smuggle a second one.
	for h := range cfg.stripHeaders {
		if utils.EqualFold(name, h) {
			return true
		}
	}
	for _, h := range connectionTokens {
		if h != "" && utils.EqualFold(name, h) {
			return true
		}
	}
	return false
}

// copyResponseHeaders relays upstream response headers to the client,
// dropping hop-by-hop headers and framing headers managed by fasthttp.
func copyResponseHeaders(c fiber.Ctx, resp *client.Response) {
	for k, v := range resp.RawResponse.Header.All() {
		if skipResponseHeader(utils.UnsafeString(k)) {
			continue
		}
		c.Response().Header.AddBytesKV(k, v)
	}
}

// responseSkipHeaders are response headers fasthttp manages for the outgoing
// connection and must not be copied from upstream.
var responseSkipHeaders = []string{
	fiber.HeaderContentLength,
	fiber.HeaderConnection,
	fiber.HeaderKeepAlive,
	fiber.HeaderTransferEncoding,
	fiber.HeaderTrailer,
	fiber.HeaderUpgrade,
}

func skipResponseHeader(name string) bool {
	for _, h := range responseSkipHeaders {
		if utils.EqualFold(name, h) {
			return true
		}
	}
	return false
}
