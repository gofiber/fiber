package cache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/fasthttp"
)

// maxVaryHeaders caps the number of Vary headers processed to prevent DoS.
const maxVaryHeaders = 32

func parseVary(vary string) ([]string, bool) {
	names := make([]string, 0, 8)
	count := 0
	for part := range strings.SplitSeq(vary, ",") {
		name := utils.TrimSpace(utilsstrings.ToLower(part))
		if name == "" {
			continue
		}
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

func makeBuildVaryKeyFunc(hexBufPool *sync.Pool) func([]string, *fasthttp.RequestHeader) string {
	return func(names []string, hdr *fasthttp.RequestHeader) string {
		sum := sha256.New()
		// Written inline rather than through a closure: capturing lenBuf would
		// force the array to the heap, adding two allocations to every request
		// that has a Vary manifest.
		var lenBuf [binary.MaxVarintLen64]byte
		for _, name := range names {
			_, _ = sum.Write(binary.AppendUvarint(lenBuf[:0], uint64(len(name)))) //nolint:errcheck // hash.Hash.Write for std hashes never errors
			_, _ = sum.Write(utils.UnsafeBytes(name))                             //nolint:errcheck // hash.Hash.Write for std hashes never errors

			// Every field line, not just the first. A name may arrive on more
			// than one line, and the two forms are equivalent on the wire
			// (RFC 9110 Section 5.2) — but Peek returns only the first, so a
			// request carrying
			//
			//	X-Tenant: public
			//	X-Tenant: acme-private
			//
			// hashed to the same key as one carrying just "public". The entry
			// stored for the first is then served to the second, which is the
			// exact cross-request mixing Vary exists to prevent.
			//
			// PeekAll reuses the header's own scratch slice for ordinary names,
			// so those cost no allocation; the values are consumed before the
			// next call to it. Cookie and Trailer are the exception — fasthttp
			// re-serializes those from its own stores into a fresh buffer per
			// call — so a "Vary: Cookie" response pays for that twice per
			// request, once at lookup and once at store.
			values := stableFieldLines(name, hdr.PeekAll(name))
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

		v := hexBufPool.Get()
		bufPtr, ok := v.(*[]byte)
		if !ok || bufPtr == nil {
			b := make([]byte, hexLen)
			bufPtr = &b
		}

		buf := *bufPtr
		// Defensive in case someone changed Pool.New or Put a different sized buffer.
		if cap(buf) < hexLen {
			buf = make([]byte, hexLen)
		} else {
			buf = buf[:hexLen]
		}
		*bufPtr = buf

		hex.Encode(buf, hashBytes[:])
		result := "|vary|" + string(buf)

		hexBufPool.Put(bufPtr)
		return result
	}
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
