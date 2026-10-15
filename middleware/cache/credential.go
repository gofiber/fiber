package cache

import (
	"context"
	"errors"
	"slices"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

// credentialHeaders carry nothing but a credential. No other name counts: nothing in a
// name tells an API key from a tenant or a language.
var credentialHeaders = []string{fiber.HeaderAuthorization, fiber.HeaderCookie, fiber.HeaderProxyAuthorization, "X-API-Key", "X-Auth-Token", "X-Access-Token"}

// carriesCredential reports whether the header is one of credentialHeaders or declared
// in KeyCredentialHeaders: the one rule for the key, the Vary names and the stored Vary.
func (cfg *Config) carriesCredential(name string) bool {
	return fieldname.ContainsFold(credentialHeaders, name) || fieldname.ContainsFold(cfg.KeyCredentialHeaders, name)
}

// appendCredentials appends to dst those of names that carry a credential.
func (cfg *Config) appendCredentials(dst, names []string) []string {
	for _, name := range names {
		if cfg.carriesCredential(name) {
			dst = append(dst, name)
		}
	}
	return dst
}

// varyCarriesCredential reports whether any of the Vary names carries a credential.
func (cfg *Config) varyCarriesCredential(names []string) bool {
	return slices.ContainsFunc(names, cfg.carriesCredential)
}

// varyListsCredential is true when a Vary field value names a header that carries
// a credential: the entry then holds the response to one client's credential.
// Walked rather than parsed, so a hit allocates nothing for it.
func (cfg *Config) varyListsCredential(vary []byte) bool {
	for name := range headerlist.All(utils.UnsafeString(vary)) {
		if cfg.carriesCredential(name) {
			return true
		}
	}
	return false
}

// holdsCredentialResponse reports whether an entry stored with this Vary can hold one
// client's response: the key is partitioned by a credential or the Vary names one.
func (cfg *Config) holdsCredentialResponse(vary []byte) bool {
	return len(cfg.KeyCookies) > 0 || len(cfg.credentialKeyHeaders) > 0 || cfg.varyListsCredential(vary)
}

// refusesCredential reports whether status is the origin refusing the request's
// credential (407 for a proxy's): a verdict on it, unlike a 5xx, which RFC 9111
// §4.3.3 lets a cache ride out.
func refusesCredential(status int) bool {
	return status == fiber.StatusUnauthorized ||
		status == fiber.StatusForbidden ||
		status == fiber.StatusProxyAuthRequired
}

// errorRefusesCredential is refusesCredential for a refusal returned as a *fiber.Error,
// wrapped or not; the error handler that sets its status runs above this middleware.
func errorRefusesCredential(err error) bool {
	var fiberErr *fiber.Error
	// A nil *fiber.Error held in an error matches errors.As and has no Code.
	return errors.As(err, &fiberErr) && fiberErr != nil && refusesCredential(fiberErr.Code)
}

// credentialScope is how far a refusal of the request's credential reaches.
type credentialScope struct {
	record string // where to record the refusal, when the Vary manifest is shared
	entry  bool   // the entry is the credential's own
	keyed  bool   // the key holds the credential, so the Vary manifest is its own too
}

// scopeOf says what a refusal reaches in the entry the request found. It is the
// credential's own when the key holds it, the responses vary on one (refusal is set) or
// a custom KeyGenerator built the key (opaque, so assumed). Read before the handler runs.
func (cfg *Config) scopeOf(c fiber.Ctx, hasAuthorization bool, refusal string) credentialScope { //nolint:revive // flag-parameter: a property of the request, not a mode of operation
	keyed := hasAuthorization || cfg.keyHoldsRequestCredential(c)
	scope := credentialScope{
		keyed: keyed,
		entry: keyed || refusal != "" || !cfg.usesDefaultKeyGenerator,
	}
	if !keyed {
		// The manifest is shared by every credential, so the refusal gets a record.
		scope.record = refusal
	}
	return scope
}

// refusalKey is where the refusal of the request's credential is recorded: a digest of
// its values, not of a variant, so one record reaches every variant.
func (cfg *Config) refusalKey(c fiber.Ctx, base string, varyNames []string) string {
	var named [defaultVaryNames]string
	credentials := cfg.appendCredentials(named[:0], varyNames)

	bufPtr := acquireKeyBuffer()
	buf := (*bufPtr)[:0]
	buf = append(buf, base...)
	buf = append(buf, "|refused"...)
	buf = appendVaryKey(buf, credentials, &c.Request().Header, headerlookup.Canonical(c))
	key := string(buf)
	releaseKeyBuffer(bufPtr, buf)
	return key
}

// refusedBefore reports whether a refusal was recorded under key. A refusal ends only the
// entry it revalidated; the record keeps the credential's other variants, under a shared
// manifest, from being served. A storage that cannot answer counts as one.
func refusedBefore(ctx context.Context, m *manager, key string) bool {
	_, err := m.getRaw(ctx, key)
	if err == nil {
		return true
	}
	if errors.Is(err, errCacheMiss) {
		return false
	}
	log.Warnf("cache: failed to read the record of a refused credential: %v", err)
	return true
}

// recordRefusal keeps the refusal under key for at least Expiration, and as long as the
// refused entry had left (entryExp, unix seconds, zero when unknown).
func (cfg *Config) recordRefusal(ctx context.Context, m *manager, key string, entryExp uint64) {
	retention := cfg.Expiration
	if now := safeUnixSeconds(cfg.now()); entryExp > now {
		retention = max(retention, secondsToDuration(entryExp-now))
	}
	if err := m.setRaw(ctx, key, []byte{1}, retention); err != nil {
		log.Warnf("cache: failed to record the refusal of a credential: %v", err)
	}
}

// keyHoldsRequestCredential reports whether the default key holds a credential the
// request carries (a KeyCookies cookie or a credential header). A custom KeyGenerator
// is opaque, so it holds none.
func (cfg *Config) keyHoldsRequestCredential(c fiber.Ctx) bool {
	if !cfg.usesDefaultKeyGenerator {
		return false
	}
	for _, name := range cfg.KeyCookies {
		if c.Cookies(name) != "" {
			return true
		}
	}
	for _, name := range cfg.credentialKeyHeaders {
		for _, line := range keyFieldLines(&c.Request().Header, name, headerlookup.Canonical(c)) {
			if len(line) > 0 {
				return true
			}
		}
	}
	return false
}
