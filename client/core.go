package client

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"sync"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

// RequestHook is a function that receives Agent and Request,
// it can change the data in Request and Agent.
//
// Called before a request is sent.
type RequestHook func(*Client, *Request) error

// ResponseHook is a function that receives Agent, Respose and Request,
// it can change the data is Respose or deal with some effects.
//
// Called after a respose has been received.
type ResponseHook func(*Client, *Response, *Request) error

// `core` stores middleware and plugin definitions,
// and defines the execution process
type core struct {
	client *fasthttp.HostClient

	// user defined request hooks
	userRequestHooks []RequestHook

	// client package defined request hooks
	buildinRequestHooks []RequestHook

	// user defined response hooks
	userResponseHooks []ResponseHook

	// client package defined respose hooks
	buildinResposeHooks []ResponseHook

	jsonMarshal   utils.JSONMarshal
	jsonUnmarshal utils.JSONUnmarshal
	xmlMarshal    utils.XMLMarshal
	xmlUnmarshal  utils.XMLUnmarshal
}

func (c *core) execFunc(ctx context.Context, client *Client, req *Request) (*Response, error) {
	resp := AcquireResponse()
	resp.setClient(client)
	resp.setRequest(req)

	// To avoid memory allocation reuse of data structures such as errch.
	errCh, reqv, respv := acquireErrChan(), fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer func() {
		releaseErrChan(errCh)
		fasthttp.ReleaseRequest(reqv)
		fasthttp.ReleaseResponse(respv)
	}()

	req.rawRequest.CopyTo(reqv)
	go func() {
		err := c.client.Do(reqv, respv)
		if err != nil {
			errCh <- err
			return
		}
		respv.CopyTo(resp.rawResponse)
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			// When get error should release Response
			ReleaseResponse(resp)
			return nil, err
		}
		return resp, nil
	case <-ctx.Done():
		ReleaseResponse(resp)
		return nil, ErrTimeoutOrCancel
	}
}

// execute will exec each hooks and plugins.
func (c *core) execute(ctx context.Context, client *Client, req *Request) (*Response, error) {
	// The built-in hooks will be executed only
	// after the user-defined hooks are executed。
	for _, f := range c.userRequestHooks {
		err := f(client, req)
		if err != nil {
			return nil, err
		}
	}

	for _, f := range c.buildinRequestHooks {
		err := f(client, req)
		if err != nil {
			return nil, err
		}
	}

	// deal with timeout
	if req.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.timeout)
		defer func() {
			cancel()
		}()
	} else {
		if client.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, client.timeout)
			defer func() {
				cancel()
			}()
		}
	}

	// Do http request
	resp, err := c.execFunc(ctx, client, req)
	if err != nil {
		return nil, err
	}

	// The built-in hooks will be executed only
	// before the user-defined hooks are executed.
	for _, f := range c.buildinResposeHooks {
		err := f(client, resp, req)
		if err != nil {
			return nil, err
		}
	}

	for _, f := range c.userResponseHooks {
		err := f(client, resp, req)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

var errChanPool sync.Pool

// acquireErrChan returns an empty error chan from the pool.
//
// The returned error chan may be returned to the pool with releaseErrChan when no longer needed.
// This allows reducing GC load.
func acquireErrChan() (ch chan error) {
	chv := errChanPool.Get()
	if chv != nil {
		ch = chv.(chan error)
		return
	}
	ch = make(chan error, 1)
	return
}

// releaseErrChan returns the object acquired via acquireErrChan to the pool.
//
// Do not access the released core object, otherwise data races may occur.
func releaseErrChan(ch chan error) {
	errChanPool.Put(ch)
}

// newCore returns an empty core object.
func newCore() (c *core) {
	c = &core{
		client:              &fasthttp.HostClient{},
		userRequestHooks:    []RequestHook{},
		buildinRequestHooks: []RequestHook{parserRequestURL, parserRequestHeader, parserRequestBody},
		userResponseHooks:   []ResponseHook{},
		buildinResposeHooks: []ResponseHook{parserResponseCookie},
		jsonMarshal:         json.Marshal,
		jsonUnmarshal:       json.Unmarshal,
		xmlMarshal:          xml.Marshal,
		xmlUnmarshal:        xml.Unmarshal,
	}

	return
}

var (
	ErrTimeoutOrCancel  = errors.New("timeout or cancel")
	ErrURLForamt        = errors.New("the url is a mistake")
	ErrNotSupportSchema = errors.New("the protocol is not support, only http or https")
	ErrFileNoName       = errors.New("the file should have name")
	ErrBodyType         = errors.New("the body type should be []byte")
)
