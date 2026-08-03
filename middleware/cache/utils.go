package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"iter"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/mediatype"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

func cacheBodyFetchError(mask func(string) string, key string, err error) error {
	if errors.Is(err, errCacheMiss) {
		return fmt.Errorf("cache: no cached body for key %q: %w", mask(key), err)
	}
	return err
}

func cachedResponseAge(e *item, now uint64) uint64 {
	clampedDate := clampDateSeconds(e.date, now)

	resident := uint64(0)
	if e.exp != 0 {
		if e.exp <= now {
			resident = e.ttl + (now - e.exp)
		} else {
			resident = e.ttl - (e.exp - now)
		}
	}

	dateAge := uint64(0)
	if clampedDate != 0 && now > clampedDate {
		dateAge = now - clampedDate
	}

	currentAge := max(dateAge, max(resident, e.age))
	return currentAge
}

func appendWarningHeaders(h *fasthttp.ResponseHeader, servedStale, heuristicFreshness bool) { //nolint:revive // flags are intentional to represent Warning variants
	if servedStale {
		h.Add(fiber.HeaderWarning, `110 - "Response is stale"`)
	}
	if heuristicFreshness {
		h.Add(fiber.HeaderWarning, `113 - "Heuristic expiration"`)
	}
}

func remainingFreshness(e *item, now uint64) uint64 {
	if e == nil || e.exp == 0 || now >= e.exp {
		return 0
	}

	return e.exp - now
}

func isHeuristicFreshness(e *item, cfg *Config, entryAge uint64) bool {
	const heuristicAgeThresholdSeconds = uint64(24 * time.Hour / time.Second)
	if entryAge <= heuristicAgeThresholdSeconds {
		return false
	}

	if len(e.expires) > 0 {
		return false
	}

	cacheControl := utils.UnsafeString(e.cacheControl)
	if parsedCC := parseResponseCacheControl(utils.UnsafeBytes(cacheControl)); parsedCC.maxAgeSet || parsedCC.sMaxAgeSet {
		return false
	}

	return cfg.Expiration > 0
}

func lookupCachedHeader(headers []cachedHeader, name string) ([]byte, bool) {
	for i := range headers {
		if utils.EqualFold(utils.UnsafeString(headers[i].key), name) {
			return headers[i].value, true
		}
	}
	return nil, false
}

// setCookie2 is the obsolete RFC 2965 spelling of Set-Cookie. fasthttp keeps it
// as an ordinary field rather than in its cookie store, so it has to be looked
// up separately.
const setCookie2 = "Set-Cookie2"

// responseSetsCookie reports whether the response hands a cookie to the client
// that caused this miss.
func responseSetsCookie(h *fasthttp.ResponseHeader, normalized bool) bool { //nolint:revive // flag-parameter: normalized is a property of the header store
	for range h.Cookies() {
		return true
	}
	// Not PeekAll(Set-Cookie): fasthttp answers that from its cookie store and
	// returns a single empty entry when there is none, so a length test on it
	// reports a cookie that was never set.
	//
	// Set-Cookie2 has no store of its own, so unlike Set-Cookie it is found
	// only under the spelling the handler wrote: fieldLines rather than
	// PeekAll, or a lower-case one walked past this guard and the personalized
	// response it names was stored and replayed to every later client.
	for _, v := range fieldLines(h, setCookie2, normalized) {
		if len(v) > 0 {
			return true
		}
	}
	return false
}

// keyFieldLines returns the field lines of name to key a cache entry on.
//
// PeekAll is what makes a name arriving on several lines key differently from
// one that arrived on a single line — except for Cookie, which fasthttp answers
// from its own store. What it reports there depends on whether that store has
// been collected yet: before collection the raw field lines, after collection
// one merged, re-serialized entry. Whether collection has happened depends on
// which middleware ran first, so reading either accessor directly lets the same
// request on the wire key two different ways: a Vary: Cookie route, whose
// lookup runs before the handler and whose store runs after, misses on every
// second request and strands a duplicate entry each time.
//
// Force the collection so the merged form is what gets keyed, always. It is the
// only representation both states agree on — Peek is no better than PeekAll
// here, since uncollected it reports just the first field line and would drop a
// "Cookie: session=..." sent on a second one out of the key entirely, letting
// two clients with different sessions share an entry. Collecting is idempotent
// and is what any cookie read in the handler would do anyway.
func keyFieldLines(h *fasthttp.RequestHeader, name string, normalized bool) [][]byte {
	switch {
	case utils.EqualFold(name, fiber.HeaderCookie):
		h.Cookie("") // collectCookies; the lookup itself is expected to miss

	case utils.EqualFold(name, fiber.HeaderContentType) && mediatype.IsForm(h.ContentType()):
		// Fold it here rather than read whatever spelling arrived. The key is
		// built twice — once to look an entry up before the handler runs, once
		// to store it after — and the form accessors lowercase this header in
		// place in between, so the two saw different bytes and the entry landed
		// under a key no lookup would produce.
		//
		// Note this is a write, on a path that otherwise only reads. Wherever a
		// key is built before the handler runs — every request when KeyHeaders
		// names Content-Type, and from the second request on for Vary, once a
		// manifest exists — the handler sees the folded value whether or not it
		// asks for a form value. That is the value it would get from FormValue
		// or MultipartForm anyway, and the boundary keeps its case, so only a
		// handler comparing the media type case-sensitively can tell. IsForm
		// gates it, so nothing else is touched.
		mediatype.NormalizeRequestContentType(h)
	}

	return fieldLines(h, name, normalized)
}

// headerNamesAreCanonical reports whether fasthttp normalized this request's
// header names, which decides whether a byte-for-byte lookup finds all of them.
//
// fasthttp keeps the answer private, so take it from the app config that set
// it. Getting this wrong in the safe direction only costs a walk; wrong the
// other way loses field lines, which for Authorization means reading a request
// as anonymous.
func headerNamesAreCanonical(c fiber.Ctx) bool {
	return !c.App().Config().DisableHeaderNormalizing
}

// fieldLines returns every field line stored under name, matching the name the
// way a recipient must.
//
// PeekAll compares stored keys byte for byte. That is enough while fasthttp
// normalizes them, but under DisableHeaderNormalizing the store keeps the
// spelling the client sent, and the names asked for here are written out in
// canonical or lower case by whoever calls. Every lookup then missed, and a
// header nobody can find is a header that is not there: a Vary dimension
// dropped silently out of the key, and — worse — an "authorization:" sent in
// lower case read as no credential at all, so the response to it was stored as
// anonymous and replayed to everyone.
//
// Field names are case-insensitive (RFC 9110 Section 5.1), so match them that
// way. Only reached when the byte-for-byte lookup found nothing, which for a
// normalizing header means the field is absent and the walk finds nothing
// either.
// header store that only the caller can know, not a mode of operation.
//
//nolint:revive // flag-parameter: normalized is a property of the request's
func fieldLines(h headerPeeker, name string, normalized bool) [][]byte {
	if normalized {
		// fasthttp canonicalized every stored key, so a byte-for-byte lookup
		// finds all of them and PeekAll hands back the header's own slice
		// without allocating.
		return h.PeekAll(name)
	}

	var values [][]byte
	for k, v := range h.All() {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			values = append(values, v)
		}
	}
	return values
}

// headerPeeker is the part of fasthttp's request and response headers this
// package needs to read every field line of a name.
type headerPeeker interface {
	Peek(key string) []byte
	PeekAll(key string) [][]byte
	All() iter.Seq2[[]byte, []byte]
}

// firstFieldLine returns the first field line stored under name, which is what
// Peek reports once fasthttp has canonicalized the stored key.
//
// Under DisableHeaderNormalizing it has not, and Peek falls through to a
// byte-for-byte scan of the general header store, so a handler writing
// "cache-control" or "expires" in lower case stored a field this package then
// could not see. Every consequence of that is the same shape: the value goes
// unread where it should decide something, and a second line of the same field
// is written beside it because nothing reported the first.
//
// The names fasthttp keeps in a slot of their own — Content-Type,
// Content-Encoding, Server, Set-Cookie, Trailer, Content-Length — are not
// affected either way. Peek answers those from the slot, and every spelling is
// routed into it on the way in, so they need no help here.
func firstFieldLine(h headerPeeker, name string, normalized bool) []byte { //nolint:revive // flag-parameter: normalized is a property of the header store
	if normalized {
		return h.Peek(name)
	}

	for k, v := range h.All() {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			return v
		}
	}
	return nil
}

// setFieldLine writes value as the field line for name, replacing whichever
// spelling of it the response already carries.
//
// Set matches the stored key byte for byte, so under DisableHeaderNormalizing
// it leaves a differently-spelled line of the same field untouched and the
// response goes out carrying both. Nothing reconciles two Age or two Date
// lines: a downstream cache reads one of them, and which one it reads decides
// how old it believes the response to be.
func setFieldLine(h *fasthttp.ResponseHeader, name string, value []byte, normalized bool) { //nolint:revive // flag-parameter: normalized is a property of the header store
	if !normalized {
		// Every other spelling, not just the first one found. Writing through
		// the first and stopping there left any remaining spelling untouched,
		// so a response already carrying both "Date" and "date" — one from an
		// outer middleware, one from the handler — went out with the stale one
		// beside the value written here, which is the pair this exists to
		// prevent.
		//
		// Collected before anything is deleted: Del is byte-exact too, and
		// removing an entry while ranging over the store it belongs to is not
		// safe.
		var others []string
		for k := range h.All() {
			if ks := utils.UnsafeString(k); ks != name && utils.EqualFold(ks, name) {
				others = append(others, string(k))
			}
		}
		for _, k := range others {
			h.Del(k)
		}
	}

	h.SetBytesV(name, value)
}

// ignoredHeaderNames is ignoreHeaders keyed the way a field name has to be
// matched, since the spellings reaching isIgnoredHeader are the ones a handler
// chose rather than the ones fasthttp canonicalized.
var ignoredHeaderNames = func() map[string]struct{} {
	folded := make(map[string]struct{}, len(ignoreHeaders))
	for name := range ignoreHeaders {
		folded[strings.ToLower(name)] = struct{}{}
	}
	return folded
}()

// maxIgnoredHeaderLen is the longest name in ignoredHeaderNames. Anything
// longer cannot be one of them, so it never needs folding.
const maxIgnoredHeaderLen = 32

// isIgnoredHeader reports whether key names a field this cache must not carry
// between requests.
//
// Field names are case-insensitive (RFC 9110 Section 5.1) and these are matched
// against keys the response stored, which under DisableHeaderNormalizing are
// whatever spelling the handler used. A byte-for-byte lookup missed those, so a
// handler writing "cache-control" or "age" in lower case had it kept in the
// entry and replayed beside the copy this package writes from the entry's own
// fields — two Cache-Control lines on every hit, one of them the "public" that
// gets synthesized precisely because the other was invisible. A shared cache
// downstream then has explicit permission to store and share a response whose
// origin never granted it.
func isIgnoredHeader(key []byte) bool {
	if len(key) > maxIgnoredHeaderLen {
		return false
	}

	var buf [maxIgnoredHeaderLen]byte
	lower := buf[:len(key)]
	for i := range key {
		c := key[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		lower[i] = c
	}

	_, ok := ignoredHeaderNames[string(lower)]
	return ok
}

// joinedHeader returns every field line for key, comma-joined.
//
// A recipient may combine repeated field lines into exactly that form
// (RFC 9110 Section 5.2), and the decisions built on Cache-Control, Pragma and
// Vary have to see all of them: a "no-store" or a "Vary: *" on a second line
// binds just as much as one on the first, and a Vary naming a header the
// response actually differs by is what keeps one client's response off another
// client's request. Peek returns only the first line, so any of those on a
// later line was silently dropped and the response cached as if it had never
// been sent — which is how a response that declared
//
//	Vary: Accept-Encoding
//	Vary: X-Tenant
//
// came to be served across tenants.
//
// The single-line case — every response that does not go out of its way to use
// Header.Add — returns the header's own bytes and allocates nothing. The
// returned slice stays valid across later PeekAll calls: those reuse the
// header's scratch slice of slice headers, not the value bytes it points at.
func joinedHeader(h headerPeeker, key string, normalized bool) []byte {
	values := fieldLines(h, key, normalized)
	switch len(values) {
	case 0:
		return nil
	case 1:
		return values[0]
	}

	n := 0
	for _, v := range values {
		n += len(v) + 1
	}
	joined := make([]byte, 0, n)
	for i, v := range values {
		if i > 0 {
			// A bare comma, matching package fiber's own field-line joiner, so
			// the two render a combined value identically.
			joined = append(joined, ',')
		}
		joined = append(joined, v...)
	}
	return joined
}

func parseHTTPDate(dateBytes []byte) (uint64, bool) {
	if len(dateBytes) == 0 {
		return 0, false
	}
	// utils.ParseHTTPDate matches net/http.ParseTime semantics: the fast
	// scalar path covers IMF-fixdate and, per RFC 9110 §5.6.7, the obsolete
	// RFC 850 and asctime formats are still accepted via the fallback.
	parsedDate, err := utils.ParseHTTPDate(dateBytes)
	if err != nil {
		return 0, false
	}

	return safeUnixSeconds(parsedDate), true
}

func clampDateSeconds(dateSeconds, fallback uint64) uint64 {
	const maxUnixSeconds = uint64(math.MaxInt64)
	if dateSeconds == 0 || dateSeconds > maxUnixSeconds || dateSeconds > fallback {
		return fallback
	}

	return dateSeconds
}

func safeUnixSeconds(t time.Time) uint64 {
	sec := t.Unix()
	if sec < 0 {
		return 0
	}

	return uint64(sec)
}

func secondsToTime(sec uint64) time.Time {
	var clamped int64
	if sec > uint64(math.MaxInt64) {
		clamped = math.MaxInt64
	} else {
		clamped = int64(sec)
	}

	return time.Unix(clamped, 0).UTC()
}

func secondsToDuration(sec uint64) time.Duration {
	const maxSeconds = uint64(math.MaxInt64) / uint64(time.Second)
	if sec > maxSeconds {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(sec) * time.Second
}

func makeHashAuthFunc(hexBufPool *sync.Pool) func([]byte) string {
	return func(authHeader []byte) string {
		sum := sha256.Sum256(authHeader)

		v := hexBufPool.Get()
		bufPtr, ok := v.(*[]byte)
		if !ok || bufPtr == nil {
			b := make([]byte, hexLen)
			bufPtr = &b
		}

		buf := *bufPtr
		if cap(buf) < hexLen {
			buf = make([]byte, hexLen)
		} else {
			buf = buf[:hexLen]
		}
		*bufPtr = buf

		hex.Encode(buf, sum[:])
		result := string(buf)

		hexBufPool.Put(bufPtr)
		return result
	}
}
