package cache

import (
	"context"
	"crypto/sha256"
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
		for _, name := range names {
			_, _ = sum.Write(utils.UnsafeBytes(name)) //nolint:errcheck // hash.Hash.Write for std hashes never errors
			_, _ = sum.Write([]byte{0})               //nolint:errcheck // hash.Hash.Write for std hashes never errors
			_, _ = sum.Write(hdr.Peek(name))          //nolint:errcheck // hash.Hash.Write for std hashes never errors
			_, _ = sum.Write([]byte{0})               //nolint:errcheck // hash.Hash.Write for std hashes never errors
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
