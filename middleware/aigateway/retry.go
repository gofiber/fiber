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
// response to relay: the first non-retryable one, or — when every attempt
// failed — the last upstream response verbatim. resp is nil only when no
// upstream produced a response at all. jsonBody is the decoded JSON request
// body from sniffModel (nil for non-JSON bodies), used for per-upstream
// ModelMap rewriting.
func sendWithRetry(c fiber.Ctx, cfg *Config, strippedPath, key string, ev *UsageEvent, jsonBody []byte) (*client.Response, error) {
	// lastResp is the most recent retryable response across all upstreams,
	// kept so the client sees a real provider error when every attempt fails.
	var lastResp *client.Response
	var lastErr error

	candidates := candidateUpstreams(cfg, ev)
	orderCandidates(cfg, candidates)

	for _, i := range candidates {
		up := &cfg.Upstreams[i]
		var brk *upstreamBreaker
		if cfg.breakers != nil {
			brk = cfg.breakers[i]
		}

		// The ModelMap rewrite is per-upstream and attempt-invariant, so it is
		// computed once here. A nil body relays the original bytes. An error
		// means a mapping applies but could not be encoded; relaying the
		// unmapped body would request a model this upstream does not serve, so
		// move on to the next upstream instead.
		body, rerr := rewriteForUpstream(c, up, ev.Model, jsonBody)
		if rerr != nil {
			lastErr = rerr
			continue
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
					return nil, fmt.Errorf("aigateway: canceled while waiting to retry: %w", c.Context().Err())
				}
			}

			ev.Attempts++
			ev.Provider = up.Name

			injectKey := up.Key
			if cfg.ForwardClientKey {
				injectKey = key
			}
			resp, err := buildRequest(c, cfg, up, strippedPath, injectKey, body).Send()
			if err != nil {
				// A network error carries no Retry-After, so it must not seed
				// this upstream's backoff basis.
				lastErr = err
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
				return resp, nil
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
			curResp = resp
		}
	}

	if lastResp != nil {
		return lastResp, nil
	}
	if lastErr == nil {
		lastErr = errAllUpstreamsFailed
	}
	return nil, lastErr
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
