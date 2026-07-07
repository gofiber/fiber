package aigateway

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valyala/fasthttp"

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

// rewriteForUpstream returns the body to relay to up when its ModelMap maps
// the requested model to a different name, or nil to relay the original bytes
// untouched. jsonBody is the decoded JSON object from sniffModel; the rewrite
// preserves every other top-level value byte-for-byte (only top-level key
// order and whitespace may change). An error means a mapping applies but the
// body could not be re-encoded — the caller must not relay the unmapped body,
// since this upstream does not serve the requested model name.
func rewriteForUpstream(c fiber.Ctx, up *Upstream, model string, jsonBody []byte) ([]byte, error) {
	if model == "" || len(up.ModelMap) == 0 || jsonBody == nil {
		return nil, nil
	}
	mapped, ok := up.ModelMap[model]
	if !ok || mapped == model {
		return nil, nil
	}

	// Decode only the top level, keeping every value as raw bytes, so the
	// rewrite cannot disturb nested payloads or number formatting.
	var obj map[string]json.RawMessage
	if err := c.App().Config().JSONDecoder(jsonBody, &obj); err != nil {
		return nil, fmt.Errorf("aigateway: model rewrite for upstream %q: %w", up.Name, err)
	}
	quoted, err := c.App().Config().JSONEncoder(mapped)
	if err != nil {
		return nil, fmt.Errorf("aigateway: model rewrite for upstream %q: %w", up.Name, err)
	}
	obj["model"] = quoted
	out, err := c.App().Config().JSONEncoder(obj)
	if err != nil {
		return nil, fmt.Errorf("aigateway: model rewrite for upstream %q: %w", up.Name, err)
	}
	return out, nil
}

// buildRequest constructs a fresh upstream request for one attempt. key is
// the credential to inject: the client's own key in pass-through mode or
// Upstream.Key in unified-key mode. A non-nil body replaces the client's raw
// body (a translation or ModelMap rewrite); it is identity-encoded, so the
// original Content-Encoding header is dropped with it. translateTo names the
// upstream's dialect when the request was translated (DialectUnspecified
// otherwise): translated exchanges pin Accept-Encoding: identity — the
// response must be inspected to be translated back — and fill dialect-
// mandatory headers the client's SDK could not have sent.
func buildRequest(c fiber.Ctx, cfg *Config, up *Upstream, strippedPath, key string, body []byte, translateTo Dialect) *client.Request {
	req := cfg.Client.R()
	req.SetMethod(c.Method())
	req.SetTimeout(cfg.HeaderTimeout)

	req.SetURL(up.URL + strippedPath + relayQuery(c, cfg))

	// Copy the incoming headers directly onto the raw request; the client's
	// built-in hooks add builder-level headers on top without clearing.
	connectionTokens := connectionHeaderTokens(c)
	for k, v := range c.Request().Header.All() {
		name := utils.UnsafeString(k)
		if skipRequestHeader(cfg, name, connectionTokens) {
			continue
		}
		if body != nil && utils.EqualFold(name, fiber.HeaderContentEncoding) {
			// The rewritten body is relayed decoded; the original
			// Content-Encoding no longer describes it.
			continue
		}
		if translateTo != DialectUnspecified && utils.EqualFold(name, fiber.HeaderAcceptEncoding) {
			// Dropped wholesale (a client may send several Accept-Encoding
			// lines, and Set below replaces only the first): the identity pin
			// after the loop must be the only value on a translated exchange.
			continue
		}
		req.RawRequest.Header.AddBytesKV(k, v)
	}

	// Remove any cookie the extractor reads the client credential from so it is
	// not forwarded upstream (the header copy above brought the Cookie header).
	for _, name := range cfg.stripCookies {
		req.RawRequest.Header.DelCookie(name)
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

	if translateTo != DialectUnspecified {
		// The upstream response must be readable to translate it back.
		req.RawRequest.Header.Set(fiber.HeaderAcceptEncoding, "identity")
		// An OpenAI-SDK client cannot send Anthropic's mandatory version
		// header; fill a default unless the client or Upstream.Headers did.
		if translateTo == DialectAnthropic && len(req.RawRequest.Header.Peek(headerAnthropicVersion)) == 0 {
			req.RawRequest.Header.Set(headerAnthropicVersion, defaultAnthropicVersion)
		}
	}

	// The client force-sets User-Agent; forward the caller's one when present.
	if ua := c.Get(fiber.HeaderUserAgent); ua != "" {
		req.SetUserAgent(ua)
	}

	if body == nil {
		body = c.BodyRaw()
	}
	if len(body) > 0 {
		req.SetRawBody(body)
	}

	return req
}

// relayQuery returns the "?query" suffix to relay upstream, with any query
// param the extractor reads the client credential from removed. It returns ""
// when there is no query. The common no-strip path avoids parsing.
func relayQuery(c fiber.Ctx, cfg *Config) string {
	qs := c.RequestCtx().URI().QueryString()
	if len(qs) == 0 {
		return ""
	}
	if len(cfg.stripQuery) == 0 {
		return "?" + utils.UnsafeString(qs)
	}
	args := fasthttp.AcquireArgs()
	defer fasthttp.ReleaseArgs(args)
	args.ParseBytes(qs)
	for _, name := range cfg.stripQuery {
		args.Del(name)
	}
	if args.Len() == 0 {
		return ""
	}
	return "?" + string(args.QueryString())
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
