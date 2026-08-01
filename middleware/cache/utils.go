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

// headerPeeker is the part of fasthttp's request and response headers this
// package needs to read every field line of a name.
type headerPeeker interface {
	PeekAll(key string) [][]byte
}

// joinedHeader returns every field line for key, joined with ", ".
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
		n += len(v) + 2
	}
	joined := make([]byte, 0, n)
	for i, v := range values {
		if i > 0 {
			joined = append(joined, ',', ' ')
		}
		joined = append(joined, v...)
	}
	return joined
}

// headerSeenEarlier reports whether key already appears among the entries
// preceding it, so the restore loop can tell a name's first stored field line
// from its duplicates. Field names are case-insensitive (RFC 9110 Section 5.1)
// and a response header can reach the store unnormalized, so compare folded.
func headerSeenEarlier(earlier []cachedHeader, key []byte) bool {
	name := utils.UnsafeString(key)
	for i := range earlier {
		if utils.EqualFold(utils.UnsafeString(earlier[i].key), name) {
			return true
		}
	}
	return false
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
