package proxy

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// hopByHopHeaders are the RFC 9110 §7.6.1 connection-level headers that
// intermediaries must not forward. Stripping these prevents request
// smuggling (Transfer-Encoding/TE), proxy-credential forwarding
// (Proxy-Authorization), and protocol-upgrade leaks.
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

// forwardedPrefix opens the X-Forwarded-* fields (For, Host, Proto, Port and
// the rest), which an app behind a trusted proxy reads for the client's
// address, host and scheme, as Fiber's own Ctx does.
const forwardedPrefix = "X-Forwarded-"

// isForwardingHeader reports whether name is a field that tells the upstream
// about the client as this hop saw it: X-Real-IP, which the forwarding
// helpers write, Forwarded (RFC 7239) and every X-Forwarded-* field.
//
// A client listing one of these in Connection does not have it removed. RFC
// 9110 §7.6.1 has an intermediary take the listed fields out of the message
// it received and then replace Connection with options of its own; the fields
// it writes for the next hop are its own, not the client's to name. The
// helpers write X-Real-IP once the listing has been applied, but X-Forwarded-*
// set by the application, and the X-Real-IP it sets by hand before Do, are
// written before it and cannot be told from a line the client sent. Leaving
// the family alone keeps the upstream's view of the client with this hop. The
// client gives up nothing: a forwarding field it sends is forwarded whenever
// Connection does not name it, so the listing could only ever take away a
// line this hop or the application wrote.
func isForwardingHeader(name string) bool {
	return utils.HasPrefixFold(name, forwardedPrefix) ||
		utils.EqualFold(name, realIPHeader) ||
		utils.EqualFold(name, fiber.HeaderForwarded)
}

// maxConnectionOptions bounds the names a message may list in Connection plus
// the other spellings of a removed name that a raw store holds, here and in
// setRealIP: each costs a scan of every header and trailer line.
const maxConnectionOptions = 32

// stripHopByHopRequestHeaders removes RFC 9110 §7.6.1 hop-by-hop headers
// from req. Callers can pass header names in except to preserve specific
// headers — used by the legacy KeepConnectionHeader option to retain the
// literal Connection header while still dropping the other hop-by-hop
// headers. It reports false, for the caller to refuse the request, when
// that takes more than maxConnectionOptions names and spellings.
func stripHopByHopRequestHeaders(req *fasthttp.Request, normalized bool, except ...string) bool { //nolint:revive // flag-parameter: normalized is a property of the header store
	// Headers listed in Connection must be removed first so the listing is
	// honored before the Connection field itself is dropped. A walk of its
	// own rather than delConnectionListedHeaders, which the response keeps:
	// the forwarding fields are not the client's to remove (see
	// isForwardingHeader). The names alias the Connection value buffers,
	// which is sound for the reason given there.
	budget := maxConnectionOptions
	for name := range headerlist.AllLines(fieldname.Lines(&req.Header, fiber.HeaderConnection, normalized)) {
		budget--
		if budget < 0 {
			return false
		}
		if isForwardingHeader(name) {
			continue
		}
		if budget = fieldname.DelWithin(&req.Header, name, normalized, budget); budget < 0 {
			return false
		}
	}
	for _, h := range hopByHopHeaders {
		if fieldname.ContainsFold(except, h) {
			continue
		}
		if budget = fieldname.DelWithin(&req.Header, h, normalized, budget); budget < 0 {
			return false
		}
	}
	return true
}

// stripHopByHopResponseHeaders applies the same filtering on the way back.
// It reports false, having reset res, when the same budget runs out.
//
// It takes no normalized argument: a proxied response is parsed by the outbound
// fasthttp.Client, whose own setting is independent of the app's, so ask the
// header. One pass finds every spelling, cheaper than the per-name scan.
func stripHopByHopResponseHeaders(res *fasthttp.Response, except ...string) bool {
	// Headers listed in Connection must be removed first so the listing is
	// honored before the Connection field itself is dropped.
	budget := delConnectionListedHeaders(&res.Header, fieldname.Lines(&res.Header, fiber.HeaderConnection, false))

	names := hopByHopHeaders
	if len(except) > 0 {
		names = make([]string, 0, len(hopByHopHeaders))
		for _, h := range hopByHopHeaders {
			if !fieldname.ContainsFold(except, h) {
				names = append(names, h)
			}
		}
	}
	// Each name's own spelling is free, as it is in DelWithin.
	if budget < 0 || !fieldname.DelSetWithin(&res.Header, names, len(names)+budget) {
		// Partly stripped, so none of the upstream's fields go out with the 502.
		res.Reset()
		return false
	}
	return true
}

// delConnectionListedHeaders deletes every comma-separated header name
// listed inside one or more Connection header fields, per RFC 9110 §7.6.1,
// without copying the values or collecting the names into a slice. The
// names handed to Del alias the Connection value buffers; that is sound
// because fasthttp's Del only re-slices the header's entry list — it never
// rewrites other entries' key/value buffers — and Del does not retain the
// name after returning. It returns what is left of maxConnectionOptions, or
// -1 once the names and their spellings need more.
//
// This is the response's walk. The request has one of its own in
// stripHopByHopRequestHeaders, which leaves the forwarding fields alone.
func delConnectionListedHeaders(h fieldname.Deleter, values [][]byte) int {
	budget := maxConnectionOptions
	for name := range headerlist.AllLines(values) {
		budget--
		if budget < 0 {
			return -1
		}
		if budget = fieldname.DelWithin(h, name, false, budget); budget < 0 {
			return -1
		}
	}
	return budget
}

// realIPHeader is the field the proxy overwrites with the peer address Fiber
// derived, so an upstream reading it sees this hop's view rather than the
// client's claim.
const realIPHeader = "X-Real-IP"

// setRealIP replaces every inbound X-Real-IP field line with ip, the peer
// address Fiber derived. Set alone overwrites the first and leaves the rest, so
// a client sending it twice kept a value of its own on the wire.
//
// The caller reads ip with realIP, from the request as received and before
// any field is removed: with ProxyHeader set to "X-Real-IP", c.IP() reads the
// very header being replaced, and deleting first handed the upstream an empty
// value; set to another name, it may read one the client listed in Connection.
// It reports false, for the caller to refuse the request, when the field is
// spelled in more than maxConnectionOptions other ways.
func setRealIP(c fiber.Ctx, ip []byte) bool {
	// Add, not Set: nothing is left to replace, and Add says what is meant.
	// fieldname.DelWithin rather than Del so a differently-spelled line does not
	// survive beside it.
	if fieldname.DelWithin(&c.Request().Header, realIPHeader, headerlookup.Canonical(c), maxConnectionOptions) < 0 {
		return false
	}
	c.Request().Header.AddBytesV(realIPHeader, ip)
	return true
}

// realIPScratch is the stack room for an address copied out of the request:
// the longest IPv6 text form is 45 bytes, and a zone may follow it. A longer
// value, such as a whole forwarding chain read without validation, spills to
// the heap and is still correct.
const realIPScratch = 64

// realIP copies the peer address Fiber derived into buf, which the caller
// keeps for as long as the address is needed. A copy rather than the string
// itself: with ProxyHeader set, c.IP() aliases that header's own buffer, and
// removing Trailer afterwards returns the received trailer fields to the
// header section, where fasthttp reuses the buffers of the lines deleted
// before them in place.
func realIP(c fiber.Ctx, buf []byte) []byte {
	return append(buf[:0], c.IP()...)
}
