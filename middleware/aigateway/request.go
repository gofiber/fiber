package aigateway

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/utils/v2"
)

// hopByHopHeaders are connection-scoped headers that must not be relayed
// (RFC 9110 section 7.6.1).
var hopByHopHeaders = []string{
	fiber.HeaderConnection,
	fiber.HeaderKeepAlive,
	fiber.HeaderProxyAuthenticate,
	fiber.HeaderProxyAuthorization,
	fiber.HeaderTE,
	fiber.HeaderTrailer,
	fiber.HeaderTransferEncoding,
	fiber.HeaderUpgrade,
}

// credentialHeaders are every auth header the gateway understands. All of
// them are stripped before the upstream credential is injected, so a client
// cannot smuggle a second credential past the gateway.
var credentialHeaders = []string{
	fiber.HeaderAuthorization,
	"x-api-key",
	"api-key",
}

// buildRequest constructs a fresh upstream request for one attempt. key is
// the credential to inject: the client's own key in pass-through mode or
// Upstream.Key in unified-key mode.
func buildRequest(c fiber.Ctx, cfg *Config, up *Upstream, strippedPath, key string) *client.Request {
	req := cfg.Client.R()
	req.SetMethod(c.Method())
	req.SetTimeout(cfg.HeaderTimeout)

	uri := up.URL + strippedPath
	if qs := c.RequestCtx().URI().QueryString(); len(qs) > 0 {
		uri += "?" + string(qs)
	}
	req.SetURL(uri)

	// Copy the incoming headers directly onto the raw request; the client's
	// built-in hooks add builder-level headers on top without clearing.
	connectionTokens := connectionHeaderTokens(c)
	for k, v := range c.Request().Header.All() {
		if skipRequestHeader(string(k), connectionTokens) {
			continue
		}
		req.RawRequest.Header.AddBytesKV(k, v)
	}

	// Inject the upstream credential after every known auth header was
	// dropped by skipRequestHeader.
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
// Connection header, which are hop-by-hop by declaration.
func connectionHeaderTokens(c fiber.Ctx) []string {
	conn := c.Get(fiber.HeaderConnection)
	if conn == "" {
		return nil
	}
	tokens := strings.Split(conn, ",")
	for i := range tokens {
		tokens[i] = utils.TrimSpace(tokens[i])
	}
	return tokens
}

func skipRequestHeader(name string, connectionTokens []string) bool {
	// Host derives from the upstream URL; Content-Length from the body.
	if utils.EqualFold(name, fiber.HeaderHost) || utils.EqualFold(name, fiber.HeaderContentLength) {
		return true
	}
	for _, h := range hopByHopHeaders {
		if utils.EqualFold(name, h) {
			return true
		}
	}
	for _, h := range credentialHeaders {
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
		name := string(k)
		if skipResponseHeader(name) {
			continue
		}
		c.Response().Header.AddBytesKV(k, v)
	}
}

func skipResponseHeader(name string) bool {
	if utils.EqualFold(name, fiber.HeaderContentLength) {
		return true
	}
	for _, h := range hopByHopHeaders {
		if utils.EqualFold(name, h) {
			return true
		}
	}
	return false
}
