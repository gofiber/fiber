package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
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
	// only under the spelling the handler wrote: fieldname.Lines rather than
	// PeekAll, or a lower-case one walked past this guard and the personalized
	// response it names was stored and replayed to every later client.
	for _, v := range fieldname.Lines(h, setCookie2, normalized) {
		if len(v) > 0 {
			return true
		}
	}
	return false
}

// keyFieldLines returns the field lines of name to key a cache entry on.
//
// Cookie is answered from fasthttp's own store, which reports raw field lines
// before collection and one merged entry after — and which of those happens
// depends on middleware order. So a Vary: Cookie route, keyed before the handler
// and stored after, keyed the same request two ways: a miss every second request
// and a stranded duplicate each time.
//
// Force the collection so the merged form is always what gets keyed. It is the
// only representation both states agree on, and collecting is idempotent.
func keyFieldLines(h *fasthttp.RequestHeader, name string, normalized bool) [][]byte {
	switch {
	case utils.EqualFold(name, fiber.HeaderCookie):
		h.Cookie("") // collectCookies; the lookup itself is expected to miss

	case utils.EqualFold(name, fiber.HeaderContentType) && mediatype.IsForm(h.ContentType()):
		// Fold here rather than read whatever arrived: the key is built once
		// before the handler and once after, and the form accessors lowercase
		// this header in place between them, so the entry landed under a key no
		// lookup would produce.
		//
		// This is a write on an otherwise read-only path. The handler then sees
		// the folded value — which is what FormValue would give it anyway, with
		// the boundary's case intact — and IsForm gates it, so nothing else is
		// touched.
		mediatype.NormalizeRequestContentType(h)
	}

	return fieldname.Lines(h, name, normalized)
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
		fieldname.DelOthers(h, name)
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
// A recipient may combine repeated field lines into that form (RFC 9110
// Section 5.2), and Cache-Control, Pragma and Vary have to be read across all
// of them — Peek returns only the first, so a response declaring
//
//	Vary: Accept-Encoding
//	Vary: X-Tenant
//
// was cached as if the second line had never been sent, and served across
// tenants.
//
// The single-line case allocates nothing, and the returned slice survives later
// PeekAll calls: those reuse the scratch slice of headers, not the value bytes.
func joinedHeader(h fieldname.Peeker, key string, normalized bool) []byte {
	values := fieldname.Lines(h, key, normalized)
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
