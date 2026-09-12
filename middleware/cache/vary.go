package cache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/fasthttp"
)

// maxVaryHeaders caps the number of Vary headers processed to prevent DoS.
const maxVaryHeaders = 32

// defaultVaryNames is the capacity parseVary starts its name slice at, sized to
// hold what a real Vary field carries without regrowing.
const defaultVaryNames = 8

func parseVary(vary string) ([]string, bool) {
	// Asked of every response, and almost none carry the field. The scan below
	// would answer the same, after allocating the slice it never fills.
	if utils.TrimSpace(vary) == "" {
		return nil, false
	}

	// Given a capacity up front, because appending into a nil slice regrows the
	// backing array as it goes — three allocations for a three-name manifest —
	// and this runs per lookup for every entry that varies.
	//
	// A fixed size rather than one counted off the separators: counting them
	// reads the whole field, while the walk below stops at the cap. That gave a
	// long list one full pass it is about to be rejected for, which is the work
	// maxVaryHeaders exists to refuse. Real fields carry a name or three, so the
	// exact size bought one allocation over this in a case that does not arise.
	names := make([]string, 0, defaultVaryNames)
	count := 0
	for part := range headerlist.All(vary) {
		name := utilsstrings.ToLower(part)
		if name == "*" {
			return nil, true
		}

		// Protect against DoS via excessive Vary headers
		count++
		if count > maxVaryHeaders {
			// Too many Vary headers, treat as uncacheable (same as Vary: *)
			return nil, true
		}

		names = append(names, name)
	}

	if len(names) == 0 {
		return nil, false
	}

	sort.Strings(names)
	return names, false
}

// appendVaryKey appends the variant suffix — "|vary|" and the digest of the
// Vary'd request headers — to dst.
//
// Appended rather than returned as a string so the whole cache key is built in
// one buffer: returning the suffix cost an allocation for it and another for the
// concatenation, on every request whose entry varies.
func appendVaryKey(dst []byte, names []string, hdr *fasthttp.RequestHeader, normalized bool) []byte { //nolint:revive // flag-parameter: normalized is a property of the header store
	sum := sha256.New()
	// Written inline rather than through a closure: capturing lenBuf would
	// force the array to the heap, adding two allocations to every request
	// that has a Vary manifest.
	var lenBuf [binary.MaxVarintLen64]byte
	for _, name := range names {
		_, _ = sum.Write(binary.AppendUvarint(lenBuf[:0], uint64(len(name)))) //nolint:errcheck // hash.Hash.Write for std hashes never errors
		_, _ = sum.Write(utils.UnsafeBytes(name))                             //nolint:errcheck // hash.Hash.Write for std hashes never errors

		// Every field line, not just the first: the split and comma-joined forms
		// are equivalent (RFC 9110 §5.2), so a request sending X-Tenant twice
		// hashed like one sending just the first value.
		values := keyFieldLines(hdr, name, normalized)
		_, _ = sum.Write(binary.AppendUvarint(lenBuf[:0], uint64(len(values)))) //nolint:errcheck // hash.Hash.Write for std hashes never errors
		for _, v := range values {
			// Length-prefixed so the framing stays injective: without it
			// two lines "a" and "b" would hash like a single line "a\x00b".
			_, _ = sum.Write(binary.AppendUvarint(lenBuf[:0], uint64(len(v)))) //nolint:errcheck // hash.Hash.Write for std hashes never errors
			_, _ = sum.Write(v)                                                //nolint:errcheck // hash.Hash.Write for std hashes never errors
		}
	}

	var hashBytes [sha256.Size]byte
	sum.Sum(hashBytes[:0])

	dst = append(dst, "|vary|"...)
	return hex.AppendEncode(dst, hashBytes[:])
}

// varyKey returns base with the variant suffix appended, assembled in a pooled
// buffer so the result costs one allocation rather than one for the suffix and
// another for the join.
func varyKey(base string, names []string, hdr *fasthttp.RequestHeader, normalized bool) string { //nolint:revive // flag-parameter: normalized is a property of the header store
	bufPtr := acquireKeyBuffer()
	buf := (*bufPtr)[:0]
	buf = append(buf, base...)
	buf = appendVaryKey(buf, names, hdr, normalized)
	key := string(buf)
	releaseKeyBuffer(bufPtr, buf)
	return key
}

func storeVaryManifest(ctx context.Context, manager *manager, manifestKey string, names []string, exp time.Duration) error {
	if len(names) == 0 {
		return nil
	}
	data := strings.Join(names, ",")
	return manager.setRaw(ctx, manifestKey, utils.UnsafeBytes(data), exp)
}

//nolint:gocritic // returning explicit values keeps the signature concise while avoiding unnecessary named results
func loadVaryManifest(ctx context.Context, manager *manager, manifestKey string) ([]string, bool, error) {
	raw, err := manager.getRaw(ctx, manifestKey)
	if err != nil {
		if errors.Is(err, errCacheMiss) {
			return nil, false, nil
		}
		return nil, false, err
	}
	manifest := utils.UnsafeString(raw)
	names, hasStar := parseVary(manifest)
	if hasStar {
		return nil, false, nil
	}
	return names, len(names) > 0, nil
}
