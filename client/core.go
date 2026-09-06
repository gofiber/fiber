// Core pipeline scaffolds request execution for Fiber's HTTP client, including
// hook invocation, retry orchestration, and timeout management around fasthttp
// transports.
package client

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/addon/retry"
)

const boundary = "FiberFormBoundary"

// RequestHook is a function invoked before the request is sent.
// It receives a Client and a Request. Mutate the Request, not the shared Client
// config: the Client is read concurrently by in-flight requests (see the Client
// type's concurrency contract), so per-request changes belong on the Request.
type RequestHook func(*Client, *Request) error

// ResponseHook is a function invoked after a response is received.
// It receives a Client, Response, and Request. Mutate the Response or act on it;
// do not mutate the shared Client config, which is read concurrently by other
// in-flight requests (see the Client type's concurrency contract).
type ResponseHook func(*Client, *Response, *Request) error

// RetryConfig is an alias for the `retry.Config` type from the `addon/retry` package.
type RetryConfig = retry.Config

// core stores middleware and plugin definitions and defines the request execution process.
type core struct {
	client *Client
	req    *Request
	ctx    context.Context //nolint:containedctx // Context is needed here.
}

// getRetryConfig returns a copy of the client's retry configuration.
func (c *core) getRetryConfig() *RetryConfig {
	return c.client.RetryConfig()
}

// execFunc is the core logic to send the request and receive the response.
// It leverages the fasthttp client, optionally with retries or redirects.
func (c *core) execFunc() (*Response, error) {
	// do not close, these will be returned to the pool
	errChan := acquireErrChan()
	respChan := acquireResponseChan()

	cfg := c.getRetryConfig()

	// Copy the request before the goroutine starts: a caller whose context is
	// already done gets ErrTimeoutOrCancel at once and may release its request.
	reqv := fasthttp.AcquireRequest()
	c.req.RawRequest.CopyTo(reqv)
	if bodyStream := c.req.RawRequest.BodyStream(); bodyStream != nil {
		reqv.SetBodyStream(bodyStream, c.req.RawRequest.Header.ContentLength())
	}
	// Read what the goroutine needs now; only its own copy is safe to touch later.
	maxRedirects := c.req.maxRedirects
	method := string(reqv.Header.Method())
	followRedirects := maxRedirects > 0 && (method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodQuery)

	go func() {
		// retain both channels until they are drained
		defer releaseErrChan(errChan)
		defer releaseResponseChan(respChan)

		defer fasthttp.ReleaseRequest(reqv)

		respv := fasthttp.AcquireResponse()
		defer func() {
			if respv != nil {
				fasthttp.ReleaseResponse(respv)
			}
		}()

		var resp *Response
		defer func() {
			if r := recover(); r != nil {
				if resp != nil {
					ReleaseResponse(resp)
				}
				errChan <- fmt.Errorf("client panic: %v", r)
			}
		}()

		var err error
		if cfg != nil {
			// Use an exponential backoff retry strategy.
			err = retry.NewExponentialBackoff(*cfg).Retry(func() error {
				if followRedirects {
					return c.client.DoRedirects(reqv, respv, maxRedirects)
				}
				return c.client.Do(reqv, respv)
			})
		} else {
			if followRedirects {
				err = c.client.DoRedirects(reqv, respv, maxRedirects)
			} else {
				err = c.client.Do(reqv, respv)
			}
		}

		if err != nil {
			errChan <- err
			return
		}

		resp = AcquireResponse()
		resp.setClient(c.client)
		resp.setRequest(c.req)
		// reqv carries the URI of the hop that produced this response, which after a
		// redirect is not c.req's. Record it before reqv is pooled so the response
		// hooks can attribute cookies to its real origin — and only where a jar
		// will read it, since the copy is otherwise pure cost on every request.
		if c.client != nil && c.client.cookieJar != nil {
			resp.setRespondedURI(reqv.URI())
		}

		// Swap the fasthttp response with the Fiber response's RawResponse field.
		// This is required, as (*fasthttp.Response).CopyTo() explicitly does not
		// copy body streams.
		//
		// See: https://github.com/valyala/fasthttp/blob/v1.69.0/http.go#L909-L923
		//
		// The defer statement above ensures that the original RawResponse
		// (now stored in respv) will be properly released.
		resp.RawResponse, respv = respv, resp.RawResponse
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
	c.client.mu.RLock()
	userHooks := slices.Clone(c.client.userRequestHooks)
	finalHooks := slices.Clone(c.client.finalRequestHooks)
	c.client.mu.RUnlock()

	for _, f := range userHooks {
		if err := f(c.client, c.req); err != nil {
			return err
		}
	}

	if err := c.runBuiltinRequestHooks(); err != nil {
		return err
	}

	for _, f := range finalHooks {
		if err := f(c.client, c.req); err != nil {
			return err
		}
	}

	return nil
}

// runBuiltinRequestHooks serializes the request under the client lock, as the
// built-in hooks read client-level state while they fill in RawRequest.
func (c *core) runBuiltinRequestHooks() error {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

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
	userHooks := slices.Clone(c.client.userResponseHooks)
	for _, f := range c.client.builtinResponseHooks {
		if err := f(c.client, resp, c.req); err != nil {
			c.client.mu.Unlock()
			return err
		}
	}
	c.client.mu.Unlock()

	for _, f := range userHooks {
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
	} else {
		c.client.mu.RLock()
		timeout := c.client.timeout
		c.client.mu.RUnlock()
		if timeout > 0 {
			c.ctx, cancel = context.WithTimeout(c.ctx, timeout)
		}
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
		// Nothing has been dispatched yet, so a request the client created for
		// this call goes back to the pool: no response will carry it there.
		if req.clientOwned {
			ReleaseRequest(req)
		}
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
		// Nothing carries the request back to the pool: no response reaches the
		// caller. A streamed body is the exception — the transport goroutine was
		// handed that reader itself, so a canceled send may still be reading it
		// and releasing here would close it mid-read.
		if req.clientOwned && !req.RawRequest.IsBodyStream() {
			ReleaseRequest(req)
		}
		return nil, err
	}

	// Execute after response hooks (built-in and then user-defined).
	if err := c.afterHooks(resp); err != nil {
		// No response reaches the caller, so only what the client created is released.
		resp.request = nil
		ReleaseResponse(resp)
		if req.clientOwned {
			ReleaseRequest(req)
		}
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
	ErrPathParamInHost      = errors.New("the path parameter value is not valid in a host")
	ErrPathParamInPath      = errors.New("the path parameter value is not a single path segment")
	ErrNotSupportSchema     = errors.New("protocol not supported; only http or https are allowed")
	ErrFileNoName           = errors.New("the file should have a name")
	ErrBodyType             = errors.New("the body type should be []byte")
	ErrNotSupportSaveMethod = errors.New("only file paths and io.Writer are supported")
	ErrBodyTypeNotSupported = errors.New("the body type is not supported")
)
