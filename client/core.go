// Core pipeline scaffolds request execution for Fiber's HTTP client, including
// hook invocation, retry orchestration, and timeout management around fasthttp
// transports.
package client

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/addon/retry"
	"github.com/valyala/fasthttp"
)

const boundary = "FiberFormBoundary"

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
	if strings.ContainsRune(addr, ':') {
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
	// do not close, these will be returned to the pool
	errChan := acquireErrChan()
	respChan := acquireResponseChan()

	cfg := c.getRetryConfig()
	go func() {
		// retain both channels until they are drained
		defer releaseErrChan(errChan)
		defer releaseResponseChan(respChan)

		reqv := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(reqv)

		respv := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(respv)

		c.req.RawRequest.CopyTo(reqv)

		var err error
		if cfg != nil {
			// Use an exponential backoff retry strategy.
			err = retry.NewExponentialBackoff(*cfg).Retry(func() error {
				if c.req.maxRedirects > 0 && (string(reqv.Header.Method()) == fiber.MethodGet || string(reqv.Header.Method()) == fiber.MethodHead) {
					return c.client.DoRedirects(reqv, respv, c.req.maxRedirects)
				}
				return c.client.Do(reqv, respv)
			})
		} else {
			if c.req.maxRedirects > 0 && (string(reqv.Header.Method()) == fiber.MethodGet || string(reqv.Header.Method()) == fiber.MethodHead) {
				err = c.client.DoRedirects(reqv, respv, c.req.maxRedirects)
			} else {
				err = c.client.Do(reqv, respv)
			}
		}

		if err != nil {
			errChan <- err
			return
		}

		resp := AcquireResponse()
		resp.setClient(c.client)
		resp.setRequest(c.req)
		respv.CopyTo(resp.RawResponse)
		respChan <- resp
	}()

	select {
	case err := <-errChan:
		return nil, err
	case resp := <-respChan:
		return resp, nil
	case <-c.ctx.Done():
		go func() { // drain the channels and release the response
			select {
			case resp := <-respChan:
				ReleaseResponse(resp)
			case <-errChan:
			}
		}()
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

var responseChanPool = &sync.Pool{
	New: func() any {
		return make(chan *Response)
	},
}

// acquireResponseChan returns an empty, non-closed *Response channel from the pool.
// The returned channel may be returned to the pool with releaseResponseChan
func acquireResponseChan() chan *Response {
	ch, ok := responseChanPool.Get().(chan *Response)
	if !ok {
		panic(errResponseChanTypeAssertion)
	}
	return ch
}

// releaseResponseChan returns the *Response channel to the pool.
// It's the caller's responsibility to ensure that:
// - the channel is not closed
// - the channel is drained before returning it
// - the channel is not reused after returning it
func releaseResponseChan(ch chan *Response) {
	responseChanPool.Put(ch)
}

var errChanPool = &sync.Pool{
	New: func() any {
		return make(chan error)
	},
}

// acquireErrChan returns an empty, non-closed error channel from the pool.
// The returned channel may be returned to the pool with releaseErrChan
func acquireErrChan() chan error {
	ch, ok := errChanPool.Get().(chan error)
	if !ok {
		panic(errChanErrorTypeAssertion)
	}
	return ch
}

// releaseErrChan returns the error channel to the pool.
// It's caller's responsibility to ensure that:
// - the channel is not closed
// - the channel is drained before returning it
// - the channel is not reused after returning it
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
	ErrBodyTypeNotSupported = errors.New("the body type is not supported")
)
