package proxy

import (
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// followRedirects implements a redirect loop that re-validates each
// target against policy before issuing the next request. It replaces
// fasthttp.Client.DoRedirects so we can reject HTTPS→HTTP downgrades and
// reapply SSRF checks to caller-controlled Location headers.
//
// initialURL is the validated *url.URL produced by doActionWithPolicy's
// validateUpstream call. Passing it in lets the loop skip a redundant
// re-validation (one url.Parse + one DNS lookup) on entry while still
// guaranteeing every SetRequestURI is fed a string from a
// validateUpstream output — the property CodeQL's go/request-forgery
// query needs to see.
//
// Each hop goes through redirectRequest; a redirect to another host or port
// also drops credentials (see stripCrossHostHeaders).
func followRedirects(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int, initialURL *url.URL, policy SecurityPolicy) error {
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	currentURL := initialURL
	currentHost := currentURL.Host
	for redirects := 0; ; redirects++ {
		// Rendered once per hop: SetRequestURI copies the bytes and
		// resolveRedirect below needs the same string, so sharing it saves
		// a URL.String() allocation on every redirect.
		currentURLStr := currentURL.String()
		req.SetRequestURI(currentURLStr)
		// Re-apply the scheme each hop: SetRequestURI keeps the previous
		// scheme when req.isTLS is set, so a redirect that changes scheme
		// (HTTP→HTTPS upgrade, or HTTPS→HTTP when AllowHTTPSDowngrade is
		// enabled) would otherwise be dispatched with the wrong scheme.
		// See https://github.com/gofiber/fiber/issues/1762.
		req.URI().SetScheme(currentURL.Scheme)
		if err := cli.Do(req, resp); err != nil {
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
		nextURL, err := resolveRedirect(currentURLStr, location, policy)
		if err != nil {
			return err
		}
		redirectRequest(req, status)
		// Strip credentials when the redirect crosses to a different host or
		// port so secrets bound to the original origin are not leaked to a
		// third party (RFC 9110 §15.4 advisory; matches browser behavior).
		if !utils.EqualFold(nextURL.Host, currentHost) {
			stripCrossHostHeaders(req)
			currentHost = nextURL.Host
		}
		currentURL = nextURL
	}
}

// redirectRequest rewrites req for the next hop (RFC 9110 §15.4): a 303 turns
// every method but GET and HEAD into GET, a 301 or 302 turns POST into GET, and
// a 307 or 308 repeats the request. A changed method drops the body.
func redirectRequest(req *fasthttp.Request, status int) {
	switch status {
	case fasthttp.StatusSeeOther:
		if req.Header.IsGet() || req.Header.IsHead() {
			return
		}
	case fasthttp.StatusMovedPermanently, fasthttp.StatusFound:
		if !req.Header.IsPost() {
			return
		}
	default:
		return
	}
	req.Header.SetMethod(fasthttp.MethodGet)
	// Not ResetBody: with ReduceMemoryUsage it hands the body buffer back to
	// fasthttp's pool while the handler may still hold c.Body().
	req.SetBody(nil)
	// A form the handler already read lives on in the post args, which fasthttp
	// writes as the body when the body is empty.
	req.PostArgs().Reset()
	req.Header.Del(fasthttp.HeaderContentType)
	req.Header.Del(fasthttp.HeaderContentLength)
	req.Header.Del(fasthttp.HeaderTransferEncoding)
	req.Header.Del(fasthttp.HeaderTrailer)
}

// resolveRedirect parses a redirect target relative to the current URL
// and applies the SecurityPolicy. CRLF and other control bytes are
// rejected to prevent header injection via Location. The returned value
// is the validated *url.URL produced by validateUpstream, so callers
// pass it straight into network sinks without re-parsing user-controlled
// strings.
func resolveRedirect(currentURL string, location []byte, policy SecurityPolicy) (*url.URL, error) {
	// Control bytes (< 0x20, DEL) are rejected; bytes >= 0x80 are left to URI
	// parsing.
	if utils.IndexControl(location) != -1 {
		return nil, fasthttp.ErrorInvalidURI
	}
	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.Update(currentURL)
	// previousScheme is at most "https" (5 bytes). A stack-sized scratch
	// buffer holds it without escaping to the heap, eliminating the
	// per-hop allocation that the previous append([]byte(nil), ...)
	// pattern produced.
	var schemeBuf [8]byte
	previousScheme := append(schemeBuf[:0], uri.Scheme()...)
	uri.UpdateBytes(location)
	if len(uri.Host()) == 0 {
		return nil, fasthttp.ErrorInvalidURI
	}
	target, err := validateUpstream(uri.String(), policy)
	if err != nil {
		return nil, err
	}
	if !policy.AllowHTTPSDowngrade && utils.EqualFold(previousScheme, httpsSchemeBytes) && target.Scheme == schemeHTTP {
		return nil, ErrRedirectDowngrade
	}
	return target, nil
}
