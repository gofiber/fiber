package client

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
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

// `Core` stores middleware and plugin definitions,
// and defines the execution process
type Core struct {
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

// execute will exec each hooks and plugins.
func (c *Core) execute(ctx context.Context, agent *Client, req *Request) (*Response, error) {
	execFunc := func(ctx context.Context, a *Client, r *Request) (*Response, error) {
		resp := AcquireResponse()
		resp.setClient(a)
		resp.setRequest(r)

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
			return nil, fmt.Errorf("timeout or cancel error")
		}
	}

	// The built-in hooks will be executed only
	// after the user-defined hooks are executed。
	for _, f := range c.userRequestHooks {
		err := f(agent, req)
		if err != nil {
			return nil, err
		}
	}

	for _, f := range c.buildinRequestHooks {
		err := f(agent, req)
		if err != nil {
			return nil, err
		}
	}

	// Do http request
	resp, err := execFunc(ctx, agent, req)
	if err != nil {
		return nil, err
	}

	// The built-in hooks will be executed only
	// before the user-defined hooks are executed.
	for _, f := range c.buildinResposeHooks {
		err := f(agent, resp, req)
		if err != nil {
			return nil, err
		}
	}

	for _, f := range c.userResponseHooks {
		err := f(agent, resp, req)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// reset clears core object.
// It will not clear buildin hooks.
func (c *Core) reset() {
	c.userRequestHooks = c.userRequestHooks[:0]
	c.userResponseHooks = c.userResponseHooks[:0]
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

var corePool sync.Pool

// AcquireCore returns an empty core object from the pool.
//
// The returned core may be returned to the pool with ReleaseCore when no longer needed.
// This allows reducing GC load.
func AcquireCore() (c *Core) {
	cv := corePool.Get()
	if cv != nil {
		c = cv.(*Core)
		return
	}
	c = &Core{
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

// ReleaseCore returns the object acquired via AcquireCore to the pool.
//
// Do not access the released core object, otherwise data races may occur.
func ReleaseCore(c *Core) {
	c.reset()
	corePool.Put(c)
}
