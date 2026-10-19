package proxy

import (
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
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
// A hop that needs another method and no body, or no credentials, goes out as a private
// copy: req keeps its method, body and credentials for the rest of the handler chain.
//
// A redirect that keeps the body of a streamed request (StreamRequestBody) is followed only if the
// first hop left the whole body in memory: one of known length that fits what the server reads
// ahead of the handler, 8 KiB. Any other fails with fasthttp.ErrRedirectBodyStream, as fasthttp's
// own loop does: the first hop consumed the stream, and what it left cannot be sent again.
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
			if changesMethod {
				redirectRequest(hop)
			}
			if crossOrigin {
				stripCrossOriginHeaders(hop)
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

// copyRequest returns a request of its own that holds what req holds, for a hop that is to differ
// from it. CopyTo leaves out the request timeout and the user values, and the copy takes them
// along. It is not taken from the pool: releasing it would Close the user values it shares with req.
func copyRequest(req *fasthttp.Request) *fasthttp.Request {
	hop := new(fasthttp.Request)
	req.CopyTo(hop)
	hop.SetTimeout(req.GetTimeOut())
	req.VisitUserValuesAll(hop.SetUserValue)
	return hop
}

// holdsBody reports whether req still has all declared bytes of the body it was sent as a stream.
// Sending reads a stream to its end and leaves in the request only what the server had read ahead
// of the handler: all of a body of known length that fits, none of a chunked one, declared as -1.
// A stream no transport read is not held either: Body would read it into memory.
func holdsBody(req *fasthttp.Request, declared int) bool {
	return !req.IsBodyStream() && len(req.Body()) == declared
}

// carryFormBody gives hop the body CopyTo left out of it: a multipart form the server parsed ahead
// of the handler is held as a form rather than as a body, and Body marshals it again. Only a hop
// that keeps the body of req needs it, and it can be as large as an upload.
func carryFormBody(hop, req *fasthttp.Request) {
	if len(hop.Body()) > 0 {
		return
	}
	if body := req.Body(); len(body) > 0 {
		hop.SetBody(body)
	}
}

// contentFields describe the content a request carried, or expect it. A request without content
// has none of them: a 100-continue expectation may not be generated without content (RFC 9110
// §10.1.1), and a target that decodes by Content-Encoding chokes on the empty body it is left.
// The fields that frame a body are dropped one by one in redirectRequest, as fasthttp keeps them
// in slots of their own.
var contentFields = []string{
	fasthttp.HeaderContentEncoding,
	fasthttp.HeaderContentLanguage,
	fasthttp.HeaderContentLocation,
	fasthttp.HeaderExpect,
	// fasthttp has no constants for these two.
	"Digest",
	"Content-MD5",
}

// redirectRequest rewrites req as a GET without the body and the fields that framed or described it.
func redirectRequest(req *fasthttp.Request) {
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
	fieldname.DelSet(&req.Header, contentFields)
}

// resolveRedirect parses a redirect target relative to the current URL
// and applies the SecurityPolicy. CRLF and other control bytes are
// rejected to prevent header injection via Location. The returned value
// is the validated *url.URL produced by validateUpstream, so callers
// pass it straight into network sinks without re-parsing user-controlled
// strings.
func resolveRedirect(currentURL string, location []byte, policy SecurityPolicy, lookups *lookupCache) (*url.URL, error) {
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
	target, err := validateUpstream(uri.String(), policy, lookups)
	if err != nil {
		return nil, err
	}
	if !policy.AllowHTTPSDowngrade && utils.EqualFold(previousScheme, httpsSchemeBytes) && target.Scheme == schemeHTTP {
		return nil, ErrRedirectDowngrade
	}
	return target, nil
}
