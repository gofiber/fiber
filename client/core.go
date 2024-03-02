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

// RequestHook is a function that receives Agent and Request,
// it can change the data in Request and Agent.
//
// Called before a request is sent.
type RequestHook func(*Client, *Request) error

// ResponseHook is a function that receives Agent, Response and Request,
// it can change the data is Response or deal with some effects.
//
// Called after a response has been received.
type ResponseHook func(*Client, *Response, *Request) error

// RetryConfig is an alias for config in the `addon/retry` package.
type RetryConfig = retry.Config

// addMissingPort will add the corresponding port number for host.
func addMissingPort(addr string, isTLS bool) string { //revive:disable-line:flag-parameter // Accepting a bool param named isTLS if fine here
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

// `core` stores middleware and plugin definitions,
// and defines the execution process
type core struct {
	client *Client
	req    *Request
	ctx    context.Context //nolint:containedctx // It's needed to be stored in the core.
}

// getRetryConfig returns the retry configuration of the client.
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

// execFunc is the core function of the client.
// It sends the request and receives the response.
func (c *core) execFunc() (*Response, error) {
	resp := AcquireResponse()
	resp.setClient(c.client)
	resp.setRequest(c.req)

	// To avoid memory allocation reuse of data structures such as errch.
	done := int32(0)
	errCh, reqv := acquireErrChan(), fasthttp.AcquireRequest()
	defer func() {
		releaseErrChan(errCh)
	}()

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
			// When get error should release Response
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

// preHooks Exec request hook
func (c *core) preHooks() error {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

	for _, f := range c.client.userRequestHooks {
		err := f(c.client, c.req)
		if err != nil {
			return err
		}
	}

	for _, f := range c.client.builtinRequestHooks {
		err := f(c.client, c.req)
		if err != nil {
			return err
		}
	}

	return nil
}

// afterHooks Exec response hooks
func (c *core) afterHooks(resp *Response) error {
	c.client.mu.Lock()
	defer c.client.mu.Unlock()

	for _, f := range c.client.builtinResponseHooks {
		err := f(c.client, resp, c.req)
		if err != nil {
			return err
		}
	}

	for _, f := range c.client.userResponseHooks {
		err := f(c.client, resp, c.req)
		if err != nil {
			return err
		}
	}

	return nil
}

// timeout deals with timeout
func (c *core) timeout() context.CancelFunc {
	var cancel context.CancelFunc

	if c.req.timeout > 0 {
		c.ctx, cancel = context.WithTimeout(c.ctx, c.req.timeout)
	} else if c.client.timeout > 0 {
		c.ctx, cancel = context.WithTimeout(c.ctx, c.client.timeout)
	}

	return cancel
}

// execute will exec each hooks and plugins.
func (c *core) execute(ctx context.Context, client *Client, req *Request) (*Response, error) {
	// keep a reference, because pass param is boring
	c.ctx = ctx
	c.client = client
	c.req = req

	// The built-in hooks will be executed only
	// after the user-defined hooks are executed.
	err := c.preHooks()
	if err != nil {
		return nil, err
	}

	cancel := c.timeout()
	if cancel != nil {
		defer cancel()
	}

	// Do http request
	resp, err := c.execFunc()
	if err != nil {
		return nil, err
	}

	// The built-in hooks will be executed only
	// before the user-defined hooks are executed.
	err = c.afterHooks(resp)
	if err != nil {
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

// acquireErrChan returns an empty error chan from the pool.
//
// The returned error chan may be returned to the pool with releaseErrChan when no longer needed.
// This allows reducing GC load.
func acquireErrChan() chan error {
	ch, ok := errChanPool.Get().(chan error)
	if !ok {
		panic(errors.New("failed to type-assert to chan error"))
	}

	return ch
}

// releaseErrChan returns the object acquired via acquireErrChan to the pool.
//
// Do not access the released core object, otherwise data races may occur.
func releaseErrChan(ch chan error) {
	errChanPool.Put(ch)
}

// newCore returns an empty core object.
func newCore() *core {
	c := &core{}

	return c
}

var (
	ErrTimeoutOrCancel      = errors.New("timeout or cancel")
	ErrURLFormat            = errors.New("the url is a mistake")
	ErrNotSupportSchema     = errors.New("the protocol is not support, only http or https")
	ErrFileNoName           = errors.New("the file should have name")
	ErrBodyType             = errors.New("the body type should be []byte")
	ErrNotSupportSaveMethod = errors.New("file path and io.Writer are supported")
)
