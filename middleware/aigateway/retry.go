package aigateway

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
)

var errAllUpstreamsFailed = errors.New("aigateway: all upstreams failed")

// relayTarget is what sendWithRetry hands back for relaying: the upstream
// response, the upstream that produced it (its dialect governs response
// translation), and the streaming intent recorded while translating the
// request for that upstream (zero for pass-through).
type relayTarget struct {
	resp *client.Response
	up   *Upstream
	opts streamOpts
}

// isRetryableStatus reports whether an upstream status code should trigger a
// retry or failover instead of being relayed.
func isRetryableStatus(status int) bool {
	switch status {
	case fiber.StatusTooManyRequests,
		fiber.StatusInternalServerError,
		fiber.StatusBadGateway,
		fiber.StatusServiceUnavailable,
		fiber.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// retryAfter parses a Retry-After header value, either delta-seconds or an
// HTTP-date in any of the three RFC 9110 formats (RFC1123, RFC850, asctime).
func retryAfter(resp *client.Response) (time.Duration, bool) {
	val := string(resp.RawResponse.Header.Peek(fiber.HeaderRetryAfter))
	if val == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(val); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(val); err == nil {
		if d := time.Until(t); d > 0 {
			return d, true
		}
		return 0, true
	}
	return 0, false
}

// sendWithRetry walks the upstream chain (skipping upstreams whose circuit
// breaker is open), retrying each upstream up to cfg.Retry.Attempts times on
// retryable failures before failing over to the next one. It returns the
// response to relay — the first non-retryable one, or, when every attempt
// failed, the last upstream response verbatim — together with the upstream
// that produced it (the caller needs its dialect to translate the response)
// and the streaming intent recorded while translating the request for that
// upstream (zero for pass-through). target.resp is nil only when no upstream
// produced a response at all. jsonBody is the decoded JSON request body from
// sniffModel (nil for non-JSON bodies), used for translation and per-upstream
// ModelMap rewriting.
func sendWithRetry(c fiber.Ctx, cfg *Config, strippedPath, key string, clientD Dialect, ev *UsageEvent, jsonBody []byte) (relayTarget, error) {
	// lastResp is the most recent retryable response across all upstreams,
	// kept so the client sees a real provider error when every attempt fails;
	// lastUp/lastOpts belong to the upstream that produced it (its dialect
	// governs how that error body is translated).
	var lastResp *client.Response
	var lastUp *Upstream
	var lastOpts streamOpts
	var lastErr error

	// A translation error must never mask a real upstream failure: the
	// 400-vs-502 classification in New() keys on errUntranslatable, so a
	// network error from one candidate keeps precedence over a later
	// candidate's translation failure.
	recordErr := func(err error, translation bool) {
		if translation && lastErr != nil && !errors.Is(lastErr, errUntranslatable) {
			return
		}
		lastErr = err
	}

	candidates := candidateUpstreams(cfg, ev)
	orderCandidates(cfg, candidates)

	for _, i := range candidates {
		up := &cfg.Upstreams[i]
		var brk *upstreamBreaker
		if cfg.breakers != nil {
			brk = cfg.breakers[i]
		}

		// Translation and the ModelMap rewrite are per-upstream and
		// attempt-invariant, so both are computed once here. An untranslatable
		// request does not abort the chain: a same-dialect fallback can still
		// serve it verbatim.
		upPath := strippedPath
		var body []byte // nil relays the original raw bytes
		var opts streamOpts
		translating := false
		if needsTranslation(clientD, up.Dialect) {
			tb, topts, terr := translateRequest(clientD, up.Dialect, jsonBody,
				c.App().Config().JSONDecoder, c.App().Config().JSONEncoder, cfg.MaxTokensCap)
			if terr != nil {
				recordErr(terr, true)
				continue
			}
			body = tb
			opts = topts
			upPath = chatPathForDialect(up.Dialect)
			translating = true
		}

		// ModelMap runs on the (possibly translated) body — the "model" field
		// is top-level in both dialects, and translation preserves the
		// client-requested model that keys the map. An error means a mapping
		// applies but could not be encoded; relaying the unmapped body would
		// request a model this upstream does not serve, so move on to the
		// next upstream instead.
		mapSrc := jsonBody
		if translating {
			mapSrc = body
		}
		if mapped, rerr := rewriteForUpstream(c, up, ev.Model, mapSrc); rerr != nil {
			recordErr(rerr, true)
			continue
		} else if mapped != nil {
			body = mapped
		}

		// curResp is this upstream's most recent retryable response; it is the
		// only basis for this upstream's backoff, so a Retry-After from a
		// previous upstream can never govern the current one.
		var curResp *client.Response

		for attempt := 1; attempt <= cfg.Retry.Attempts; attempt++ {
			if attempt > 1 {
				// Same-upstream retry: back off first. Failover to the next
				// upstream is always immediate.
				if !waitBeforeRetry(c, cfg, attempt, curResp) {
					// Client gave up while we were waiting. An upstream did produce
					// a (retryable) response, so record its status before dropping
					// it: ev.StatusCode == 0 is reserved for "no upstream response
					// at all", matching the buffered/streaming relay paths.
					if lastResp != nil {
						ev.StatusCode = lastResp.StatusCode()
						abortUpstreamResponse(lastResp)
					}
					return relayTarget{}, fmt.Errorf("aigateway: canceled while waiting to retry: %w", c.Context().Err())
				}
			}

			ev.Attempts++
			ev.Provider = up.Name

			injectKey := up.Key
			if cfg.ForwardClientKey {
				injectKey = key
			}
			translateTo := DialectUnspecified
			if translating {
				translateTo = up.Dialect
			}
			resp, err := buildRequest(c, cfg, up, upPath, injectKey, body, translateTo).Send()
			if err != nil {
				// A network error carries no Retry-After, so it must not seed
				// this upstream's backoff basis.
				recordErr(err, false)
				curResp = nil
				if brk != nil {
					brk.recordFailure(cfg.BreakerThreshold, cfg.BreakerCooldown)
				}
				continue
			}
			if !isRetryableStatus(resp.StatusCode()) {
				// Any received non-retryable response — success or a client
				// error relayed verbatim — proves the upstream healthy.
				if brk != nil {
					brk.recordSuccess()
				}
				if lastResp != nil {
					abortUpstreamResponse(lastResp)
				}
				return relayTarget{resp: resp, up: up, opts: opts}, nil
			}
			if brk != nil {
				brk.recordFailure(cfg.BreakerThreshold, cfg.BreakerCooldown)
			}
			// Retryable response: it becomes both the backoff basis and the
			// candidate for verbatim relay on exhaustion. Free any older held
			// response first.
			if lastResp != nil && lastResp != resp {
				abortUpstreamResponse(lastResp)
			}
			lastResp = resp
			lastUp = up
			lastOpts = opts
			curResp = resp
		}
	}

	if lastResp != nil {
		return relayTarget{resp: lastResp, up: lastUp, opts: lastOpts}, nil
	}
	if lastErr == nil {
		lastErr = errAllUpstreamsFailed
	}
	return relayTarget{}, lastErr
}

// waitBeforeRetry sleeps for the backoff computed from the previous failure.
// A Retry-After above cfg.Retry.MaxBackoff skips the wait entirely. It
// returns false when the client disconnected while waiting.
func waitBeforeRetry(c fiber.Ctx, cfg *Config, attempt int, curResp *client.Response) bool {
	// attempt is the upcoming try (2..N): the first retry waits Backoff,
	// doubling on each further one.
	delay := cfg.Retry.Backoff << (attempt - 2)
	if delay > cfg.Retry.MaxBackoff || delay <= 0 {
		delay = cfg.Retry.MaxBackoff
	}
	if curResp != nil {
		if ra, ok := retryAfter(curResp); ok {
			if ra > cfg.Retry.MaxBackoff {
				return true
			}
			delay = ra
		}
	}
	if delay <= 0 {
		return true
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-c.Context().Done():
		return false
	}
}
