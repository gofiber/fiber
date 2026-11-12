package proxy

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/crosshost"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/redirectbody"
	"github.com/gofiber/fiber/v3/internal/schemehost"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// followRedirects implements a redirect loop that re-validates each
// target against policy before issuing the next request. It replaces
// fasthttp.Client.DoRedirects so we can reject HTTPS→HTTP downgrades and
// reapply SSRF checks to caller-controlled Location headers.
//
// initialURL is the validated *url.URL produced by doActionWithPolicy's
// validateUpstream call; doActionWithPolicy has already set it on req. Passing
// it in lets the loop skip a redundant re-validation (one url.Parse + one DNS
// lookup) on entry while still guaranteeing every SetRequestURI is fed a string
// from a validateUpstream output — the property CodeQL's go/request-forgery
// query needs to see.
//
// A hop that needs another method or no credentials goes out as a copy, so req keeps its method,
// body and credentials. A redirect that keeps a streamed body (StreamRequestBody) is followed only
// if the first hop left all of it in memory; otherwise it fails with fasthttp.ErrRedirectBodyStream.
func followRedirects(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int, initialURL *url.URL, policy SecurityPolicy) error {
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	hop := req
	// Asked before the first hop: sending the request consumes the stream.
	streamed, declared := req.IsBodyStream(), req.Header.ContentLength()
	lookups := lookupsFor(cli)
	currentURL := initialURL
	currentURLStr := "" // rendered when a redirect needs it as the base of Location
	for redirects := 0; ; redirects++ {
		if err := cli.Do(hop, resp); err != nil {
			return err
		}
		status := resp.Header.StatusCode()
		if !fasthttp.StatusCodeIsRedirect(status) {
			return nil
		}
		if redirects >= maxRedirects {
			return fasthttp.ErrTooManyRedirects
		}
		location := resp.Header.Peek(fiber.HeaderLocation)
		if len(location) == 0 {
			return fasthttp.ErrMissingLocation
		}
		if currentURLStr == "" {
			currentURLStr = currentURL.String()
		}
		nextURL, err := resolveRedirect(currentURLStr, location, policy, lookups)
		if err != nil {
			return err
		}
		// Strip credentials when the redirect crosses to another origin so
		// secrets bound to the original origin are not leaked to a third
		// party (RFC 9110 §15.4 advisory; matches browser behavior).
		crossOrigin := !schemehost.Match(currentURL.Scheme, currentURL.Host, nextURL.Scheme, nextURL.Host)
		changesMethod := redirectChangesMethod(hop, status)
		if streamed {
			if changesMethod {
				// The body is gone for the rest of the chain.
				streamed = false
			} else if !holdsBody(hop, declared) {
				return fasthttp.ErrRedirectBodyStream
			}
		}
		if crossOrigin || changesMethod {
			if hop == req {
				hop = copyRequest(req)
				if !changesMethod {
					carryFormBody(hop, req)
				}
			}
			if changesMethod && !redirectRequest(hop) {
				return fiber.ErrRequestHeaderFieldsTooLarge
			}
			if crossOrigin && !crosshost.StripWithin(&hop.Header, crosshost.NewHeaderNames(policy.RedirectSensitiveHeaders), maxConnectionOptions) {
				return fiber.ErrRequestHeaderFieldsTooLarge
			}
			if !keepForwardingOrder(hop) {
				return fiber.ErrRequestHeaderFieldsTooLarge
			}
		}
		currentURL = nextURL
		// Rendered once per hop: SetRequestURI copies it and the next resolveRedirect uses it as base.
		currentURLStr = nextURL.String()
		hop.SetRequestURI(currentURLStr)
		// Re-apply the scheme each hop: SetRequestURI keeps the previous
		// scheme when req.isTLS is set, so a redirect that changes scheme
		// (HTTP→HTTPS upgrade, or HTTPS→HTTP when AllowHTTPSDowngrade is
		// enabled) would otherwise be dispatched with the wrong scheme.
		// See https://github.com/gofiber/fiber/issues/1762.
		hop.URI().SetScheme(currentURL.Scheme)
	}
}

// redirectChangesMethod reports whether a redirect with this status makes req a GET (RFC 9110
// §15.4): a 303 for every method but GET and HEAD, a 301 or 302 for POST, a 307 or 308 never.
func redirectChangesMethod(req *fasthttp.Request, status int) bool {
	switch status {
	case fasthttp.StatusSeeOther:
		return !req.Header.IsGet() && !req.Header.IsHead()
	case fasthttp.StatusMovedPermanently, fasthttp.StatusFound:
		return req.Header.IsPost()
	default:
		return false
	}
}

// copyRequest returns a copy of req for a hop that is to differ from it, with the timeout and user
// values CopyTo leaves out. It is not pooled: releasing it would Close the values it shares with req.
func copyRequest(req *fasthttp.Request) *fasthttp.Request {
	hop := new(fasthttp.Request)
	req.CopyTo(hop)
	hop.SetTimeout(req.GetTimeOut())
	req.VisitUserValuesAll(hop.SetUserValue)
	return hop
}

// holdsBody reports whether req still has all declared bytes of a body it was sent as a stream.
// Sending leaves only what the server read ahead: all of a body of known length that fits, none of a
// chunked one (declared as -1). A stream no transport read is not held: Body would buffer it.
func holdsBody(req *fasthttp.Request, declared int) bool {
	return !req.IsBodyStream() && len(req.Body()) == declared
}

// carryFormBody gives hop the body CopyTo leaves out: a multipart form the server parsed ahead of
// the handler is held as a form, and Body marshals it again. Only a hop that keeps the body needs it.
func carryFormBody(hop, req *fasthttp.Request) {
	if len(hop.Body()) > 0 {
		return
	}
	if body := req.Body(); len(body) > 0 {
		hop.SetBody(body)
	}
}

// contentFields describe or expect content, which a request without any must not carry (RFC 9110
// §10.1.1); redirectRequest drops them in every spelling, once redirectbody.Drop took the body and
// the fields framing it.
var contentFields = []string{
	fasthttp.HeaderContentEncoding,
	fasthttp.HeaderContentLanguage,
	fasthttp.HeaderContentLocation,
	fasthttp.HeaderExpect,
	// fasthttp has no constants for these two.
	"Digest",
	"Content-MD5",
}

// redirectRequest rewrites req, a hop's private copy, as a GET without the body and the fields that
// framed or described it. It reports false when those are spelled in more than maxConnectionOptions
// further ways, as fieldname.DelSetWithin counts them, or when the trailer section announces more
// names than fieldname.MaxTrailers.
func redirectRequest(req *fasthttp.Request) bool {
	req.Header.SetMethod(fasthttp.MethodGet)
	return redirectbody.Drop(req) && fieldname.DelSetWithin(&req.Header, contentFields, maxConnectionOptions)
}

// resolveRedirect parses a redirect target relative to the current URL
// and applies the SecurityPolicy. CRLF and other control bytes are
// rejected to prevent header injection via Location. The returned value
// is the validated *url.URL produced by validateUpstream, so callers
// pass it straight into network sinks without re-parsing user-controlled
// strings.
func resolveRedirect(currentURL string, location []byte, policy SecurityPolicy, lookups *lookupCache) (*url.URL, error) {
	// Control bytes (< 0x20, DEL) are rejected; bytes >= 0x80 are left to URI parsing.
	if utils.IndexControl(location) != -1 {
		return nil, fasthttp.ErrorInvalidURI
	}
	// Resolved on the escaped path (RFC 3986 Section 5.2), not through fasthttp.URI, which decodes
	// "%2F" and resolves the ".." it uncovers: a Location that repeats the request URI, as a
	// canonical-host redirect does, would otherwise name another path than the one requested.
	base, err := url.Parse(currentURL)
	if err != nil {
		return nil, fasthttp.ErrorInvalidURI
	}
	ref, err := url.Parse(escapeStrayPercents(utils.UnsafeString(location)))
	if err != nil {
		return nil, fasthttp.ErrorInvalidURI
	}
	if !rawPathValid(ref.RawPath) {
		ref.RawPath = repairRawPath(ref.RawPath, ref.Path)
	}
	next := base.ResolveReference(ref)
	// A hop has never carried the user info of a Location: it would write an Authorization the
	// request did not have.
	next.User = nil
	if next.Host == "" {
		return nil, fasthttp.ErrorInvalidURI
	}
	target, err := validateUpstream(next.String(), policy, lookups)
	if err != nil {
		return nil, err
	}
	if !policy.AllowHTTPSDowngrade && base.Scheme == schemeHTTPS && target.Scheme == schemeHTTP {
		return nil, ErrRedirectDowngrade
	}
	return target, nil
}

// escapeStrayPercents returns location with every "%" of its path that does not begin an escape written
// as "%25", which is how browsers, curl and fasthttp.URI take "/discount/100%"; net/url refuses it. Only the
// path is touched: the scheme and the authority are cut off as net/url does, so the host a Location names
// does not depend on this. It returns location itself, without allocating, when there is nothing to write.
func escapeStrayPercents(location string) string {
	end := len(location)
	if i := strings.IndexAny(location, "?#"); i >= 0 {
		end = i
	}
	var b []byte
	last := 0
	for i := pathStart(location[:end]); i < end; i++ {
		if location[i] != '%' || i+2 < end && isHexDigit(location[i+1]) && isHexDigit(location[i+2]) {
			continue
		}
		if b == nil {
			b = make([]byte, 0, len(location)+8)
		}
		b = append(b, location[last:i+1]...)
		b = append(b, "25"...)
		last = i + 1
	}
	if b == nil {
		return location
	}
	return string(append(b, location[last:]...))
}

// pathStart returns where the path of ref, which ends where its query or fragment begins, starts, as net/url cuts
// it: after the "scheme:" and the "//authority" it opens with.
func pathStart(ref string) int {
	start := schemeEnd(ref)
	if rest := ref[start:]; strings.HasPrefix(rest, "//") && (start > 0 || !strings.HasPrefix(rest, "///")) {
		if i := strings.IndexByte(rest[2:], '/'); i >= 0 {
			return start + 2 + i
		}
		return len(ref)
	}
	return start
}

// schemeEnd returns the index after the "scheme:" ref opens with, or 0 when it opens with none, as net/url's getScheme does.
func schemeEnd(ref string) int {
	for i := 0; i < len(ref); i++ {
		switch c := ref[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9', c == '+', c == '-', c == '.':
			if i == 0 {
				return 0
			}
		case c == ':' && i > 0:
			return i + 1
		default:
			return 0
		}
	}
	return 0
}
