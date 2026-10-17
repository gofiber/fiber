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
// validateUpstream call; doActionWithPolicy has already set it on req. Passing
// it in lets the loop skip a redundant re-validation (one url.Parse + one DNS
// lookup) on entry while still guaranteeing every SetRequestURI is fed a string
// from a validateUpstream output — the property CodeQL's go/request-forgery
// query needs to see.
//
// A hop that must differ from the request the caller sent, with another method
// and no body or without credentials, goes out as a private copy. req keeps its
// method, body and credentials for the rest of the handler chain.
func followRedirects(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int, initialURL *url.URL, policy SecurityPolicy) error {
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	hop := req
	defer func() {
		if hop != req {
			fasthttp.ReleaseRequest(hop)
		}
	}()
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
		nextURL, err := resolveRedirect(currentURLStr, location, policy)
		if err != nil {
			return err
		}
		// Strip credentials when the redirect crosses to another origin so
		// secrets bound to the original origin are not leaked to a third
		// party (RFC 9110 §15.4 advisory; matches browser behavior).
		crossOrigin := !sameOrigin(currentURL, nextURL)
		if crossOrigin || redirectChangesMethod(hop, status) {
			if hop == req {
				hop = fasthttp.AcquireRequest()
				req.CopyTo(hop)
			}
			redirectRequest(hop, status)
			if crossOrigin {
				stripCrossHostHeaders(hop)
			}
		}
		currentURL = nextURL
		// Rendered once per hop: SetRequestURI copies the bytes and the next
		// resolveRedirect needs the same string as its base.
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

// sameOrigin reports whether a and b share scheme, host and port (RFC 6454).
// A default port counts as the port of a URL that leaves it out.
func sameOrigin(a, b *url.URL) bool {
	return utils.EqualFold(a.Scheme, b.Scheme) &&
		utils.EqualFold(a.Hostname(), b.Hostname()) &&
		originPort(a) == originPort(b)
}

func originPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if utils.EqualFold(u.Scheme, schemeHTTPS) {
		return "443"
	}
	return "80"
}

// redirectChangesMethod reports whether a redirect with this status turns req
// into a GET (RFC 9110 §15.4): a 303 does so for every method but GET and HEAD,
// a 301 or 302 for POST, and a 307 or 308 never.
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

// redirectRequest rewrites req for the next hop when the method changes: it
// becomes a GET without the body and the fields that framed it.
func redirectRequest(req *fasthttp.Request, status int) {
	if !redirectChangesMethod(req, status) {
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
