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

func makeBuildVaryKeyFunc(hexBufPool *sync.Pool) func([]string, *fasthttp.RequestHeader, bool) string {
	return func(names []string, hdr *fasthttp.RequestHeader, normalized bool) string {
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
