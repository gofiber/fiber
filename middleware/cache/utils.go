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

// cachedHeadersSetCookie reports whether a stored entry carries a cookie. Only
// an entry written before Set-Cookie joined ignoreHeaders can, so this asks
// about a persisted store an older version filled, not about anything this one
// would write.
func cachedHeadersSetCookie(headers []cachedHeader) bool {
	if _, ok := lookupCachedHeader(headers, fiber.HeaderSetCookie); ok {
		return true
	}
	_, ok := lookupCachedHeader(headers, setCookie2)
	return ok
}

// responseSetsCookie reports whether the response hands a cookie to the client
// that caused this miss.
func responseSetsCookie(h *fasthttp.ResponseHeader, normalized bool) bool { //nolint:revive // flag-parameter: normalized is a property of the header store
	for range h.Cookies() {
		return true
	}
	// Not PeekAll(Set-Cookie): fasthttp answers that from its cookie store and
	// returns one empty entry when there is none. Set-Cookie2 has no store, so it
	// is found only under the spelling the handler wrote.
	for _, v := range fieldname.Lines(h, setCookie2, normalized) {
		if len(v) > 0 {
			return true
		}
	}
	return false
}

// keyFieldLines returns the field lines of name to key a cache entry on.
//
// Cookie is answered from fasthttp's store, which reports raw lines before
// collection and one merged entry after — so force the collection, the only
// representation both states agree on. Collecting is idempotent.
func keyFieldLines(h *fasthttp.RequestHeader, name string, normalized bool) [][]byte {
	switch {
	case utils.EqualFold(name, fiber.HeaderCookie):
		h.Cookie("") // collectCookies; the lookup itself is expected to miss

	case utils.EqualFold(name, fiber.HeaderContentType) && mediatype.IsForm(h.ContentType()):
		// Fold here rather than read whatever arrived: the key is built once before
		// the handler and once after, and the form accessors lowercase this header
		// in between, so the entry landed under a key no lookup would produce.
		mediatype.NormalizeRequestContentType(h)
	}

	return fieldname.Lines(h, name, normalized)
}

// setFieldLine writes value as the field line for name, replacing whichever
// spelling the response carries. Set is byte-exact, so it otherwise left a
// differently-spelled line and nothing reconciles two Age or two Date lines.
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
// between requests. Matched case-insensitively (RFC 9110 §5.1): a byte-exact
// lookup missed a handler's lower-case "cache-control" and replayed it.
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

// joinedHeader returns every field line for key, comma-joined, since a recipient
// may combine them into that form (RFC 9110 §5.2). Peek returns only the first,
// so a second "Vary:" line was cached as if it had never been sent.
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
