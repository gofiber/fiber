package client

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/addon/retry"
	"github.com/valyala/fasthttp"
)

var boundary = "--FiberFormBoundary"

// RequestHook is a function invoked before the request is sent.
// It receives a Client and a Request, allowing you to modify the Request or Client data.
type RequestHook func(*Client, *Request) error

// ResponseHook is a function invoked after a response is received.
// It receives a Client, Response, and Request, allowing you to modify the Response data
// or perform actions based on the response.
type ResponseHook func(*Client, *Response, *Request) error

// RetryConfig is an alias for the `retry.Config` type from the `addon/retry` package.
type RetryConfig = retry.Config

// addMissingPort appends the appropriate port number to the given address if it doesn't have one.
// If isTLS is true, it uses port 443; otherwise, it uses port 80.
func addMissingPort(addr string, isTLS bool) string { //revive:disable-line:flag-parameter
	n := strings.Index(addr, ":")
	if n >= 0 {
		return addr
	}
	port := 80
	if isTLS {
		port = 443
	}
	return net.JoinHostPort(addr, strconv.Itoa(port))
}

// core stores middleware and plugin definitions and defines the request execution process.
type core struct {
	client *Client
	req    *Request
	ctx    context.Context //nolint:containedctx // Context is needed here.
}

// getRetryConfig returns a copy of the client's retry configuration.
func (c *core) getRetryConfig() *RetryConfig {
	c.client.mu.RLock()
	defer c.client.mu.RUnlock()

	cfg := c.client.RetryConfig()
	if cfg == nil {
		return nil
	}

	return &RetryConfig{
		InitialInterval: cfg.InitialInterval,
		MaxBackoffTime:  cfg.MaxBackoffTime,
		Multiplier:      cfg.Multiplier,
		MaxRetryCount:   cfg.MaxRetryCount,
	}
}

// execFunc is the core logic to send the request and receive the response.
// It leverages the fasthttp client, optionally with retries or redirects.
func (c *core) execFunc() (*Response, error) {
	resp := AcquireResponse()
	resp.setClient(c.client)
	resp.setRequest(c.req)

	done := int32(0)
	errCh, reqv := acquireErrChan(), fasthttp.AcquireRequest()
	defer releaseErrChan(errCh)

	c.req.RawRequest.CopyTo(reqv)
	cfg := c.getRetryConfig()

	var err error
	go func() {
		respv := fasthttp.AcquireResponse()
		defer func() {
			fasthttp.ReleaseRequest(reqv)
			fasthttp.ReleaseResponse(respv)
		}()

		if cfg != nil {
			// Use an exponential backoff retry strategy.
			err = retry.NewExponentialBackoff(*cfg).Retry(func() error {
				if c.req.maxRedirects > 0 && (string(reqv.Header.Method()) == fiber.MethodGet || string(reqv.Header.Method()) == fiber.MethodHead) {
					return c.client.fasthttp.DoRedirects(reqv, respv, c.req.maxRedirects)
				}
				return c.client.fasthttp.Do(reqv, respv)
			})
		} else {
			if c.req.maxRedirects > 0 && (string(reqv.Header.Method()) == fiber.MethodGet || string(reqv.Header.Method()) == fiber.MethodHead) {
				err = c.client.fasthttp.DoRedirects(reqv, respv, c.req.maxRedirects)
			} else {
				err = c.client.fasthttp.Do(reqv, respv)
			}
		}

		if atomic.CompareAndSwapInt32(&done, 0, 1) {
			if err != nil {
				errCh <- err
				return
			}
			respv.CopyTo(resp.RawResponse)
			errCh <- nil
		}
	}()

	select {
	case err := <-errCh:
		if err != nil {
			// Release the response if an error occurs.
			ReleaseResponse(resp)
			return nil, err
		}
		return resp, nil
	case <-c.ctx.Done():
		atomic.SwapInt32(&done, 1)
		ReleaseResponse(resp)
		return nil, ErrTimeoutOrCancel
	}
}

// preHooks runs all request hooks before sending the request.
func (c *core) preHooks() error {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

	for _, f := range c.client.userRequestHooks {
		if err := f(c.client, c.req); err != nil {
			return err
		}
	}

	for _, f := range c.client.builtinRequestHooks {
		if err := f(c.client, c.req); err != nil {
			return err
		}
	}

	return nil
}

// afterHooks runs all response hooks after receiving the response.
func (c *core) afterHooks(resp *Response) error {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

	for _, f := range c.client.builtinResponseHooks {
		if err := f(c.client, resp, c.req); err != nil {
			return err
		}
	}

	for _, f := range c.client.userResponseHooks {
		if err := f(c.client, resp, c.req); err != nil {
			return err
		}
	}

	return nil
}

// timeout applies the configured timeout to the request, if any.
func (c *core) timeout() context.CancelFunc {
	var cancel context.CancelFunc

	if c.req.timeout > 0 {
		c.ctx, cancel = context.WithTimeout(c.ctx, c.req.timeout)
	} else if c.client.timeout > 0 {
		c.ctx, cancel = context.WithTimeout(c.ctx, c.client.timeout)
	}

	return cancel
}

// execute runs all hooks, applies timeouts, sends the request, and runs response hooks.
func (c *core) execute(ctx context.Context, client *Client, req *Request) (*Response, error) {
	// Store references locally.
	c.ctx = ctx
	c.client = client
	c.req = req

	// Execute pre request hooks (user-defined and built-in).
	if err := c.preHooks(); err != nil {
		return nil, err
	}

	// Apply timeout if specified.
	cancel := c.timeout()
	if cancel != nil {
		defer cancel()
	}

	// Perform the actual HTTP request.
	resp, err := c.execFunc()
	if err != nil {
		return nil, err
	}

	// Execute after response hooks (built-in and then user-defined).
	if err := c.afterHooks(resp); err != nil {
		resp.Close()
		return nil, err
	}

	return resp, nil
}

var errChanPool = &sync.Pool{
	New: func() any {
		return make(chan error, 1)
	},
}

// acquireErrChan returns an empty error channel from the pool.
//
// The returned channel may be returned to the pool with releaseErrChan when no longer needed,
// reducing GC load.
func acquireErrChan() chan error {
	ch, ok := errChanPool.Get().(chan error)
	if !ok {
		panic(errors.New("failed to type-assert to chan error"))
	}
	return ch
}

// releaseErrChan returns the error channel to the pool.
//
// Do not use the released channel afterward to avoid data races.
func releaseErrChan(ch chan error) {
	errChanPool.Put(ch)
}

// newCore returns a new core object.
func newCore() *core {
	return &core{}
}

var (
	ErrTimeoutOrCancel      = errors.New("timeout or cancel")
	ErrURLFormat            = errors.New("the URL is incorrect")
	ErrNotSupportSchema     = errors.New("protocol not supported; only http or https are allowed")
	ErrFileNoName           = errors.New("the file should have a name")
	ErrBodyType             = errors.New("the body type should be []byte")
	ErrNotSupportSaveMethod = errors.New("only file paths and io.Writer are supported")
)
