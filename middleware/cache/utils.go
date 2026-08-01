package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
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
func responseSetsCookie(h *fasthttp.ResponseHeader) bool {
	for range h.Cookies() {
		return true
	}
	// Not PeekAll(Set-Cookie): fasthttp answers that from its cookie store and
	// returns a single empty entry when there is none, so a length test on it
	// reports a cookie that was never set.
	for _, v := range h.PeekAll(setCookie2) {
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
func keyFieldLines(h *fasthttp.RequestHeader, name string) [][]byte {
	if utils.EqualFold(name, fiber.HeaderCookie) {
		h.Cookie("") // collectCookies; the lookup itself is expected to miss
	}
	return h.PeekAll(name)
}

// headerPeeker is the part of fasthttp's request and response headers this
// package needs to read every field line of a name.
type headerPeeker interface {
	PeekAll(key string) [][]byte
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
func joinedHeader(h headerPeeker, key string) []byte {
	values := h.PeekAll(key)
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
