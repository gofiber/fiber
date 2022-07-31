package client

import (
	"net/http"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

type Client struct {
	core *Core

	baseUrl string
	header  *Header
}

// Add user-defined request hooks.
func (c *Client) AddRequestHook(h ...RequestHook) *Client {
	c.core.userRequestHooks = append(c.core.userRequestHooks, h...)
	return c
}

// Add user-defined response hooks.
func (c *Client) AddResponseHook(h ...ResponseHook) *Client {
	c.core.userResponseHooks = append(c.core.userResponseHooks, h...)
	return c
}

// Set HostClient dial, this method for unit test,
// maybe don't use it.
func (c *Client) SetDial(f fasthttp.DialFunc) *Client {
	c.core.client.Dial = f
	return c
}

// Set json encoder.
func (c *Client) SetJSONMarshal(f utils.JSONMarshal) *Client {
	c.core.jsonMarshal = f
	return c
}

// Set json decoder.
func (c *Client) SetJSONUnmarshal(f utils.JSONUnmarshal) *Client {
	c.core.jsonUnmarshal = f
	return c
}

// Set xml encoder.
func (c *Client) SetXMLMarshal(f utils.XMLMarshal) *Client {
	c.core.xmlMarshal = f
	return c
}

// Set xml decoder.
func (c *Client) SetXMLUnmarshal(f utils.XMLUnmarshal) *Client {
	c.core.xmlUnmarshal = f
	return c
}

// Set baseUrl which is prefix of real url.
func (c *Client) SetBaseURL(url string) *Client {
	c.baseUrl = url
	return c
}

// AddHeader method adds a single header field and its value in the client instance.
// These headers will be applied to all requests raised from this client instance.
// Also it can be overridden at request level header options.
func (c *Client) AddHeader(key, val string) *Client {
	c.header.Add(key, val)
	return c
}

// SetHeader method sets a single header field and its value in the client instance.
// These headers will be applied to all requests raised from this client instance.
// Also it can be overridden at request level header options.
func (c *Client) SetHeader(key, val string) *Client {
	c.header.Set(key, val)
	return c
}

// AddHeaders method adds multiple headers field and its values at one go in the client instance.
// These headers will be applied to all requests raised from this client instance. Also it can be
// overridden at request level headers options.
func (c *Client) AddHeaders(h map[string][]string) *Client {
	c.header.AddHeaders(h)
	return c
}

// SetHeaders method sets multiple headers field and its values at one go in the client instance.
// These headers will be applied to all requests raised from this client instance. Also it can be
// overridden at request level headers options.
func (c *Client) SetHeaders(h map[string]string) *Client {
	c.header.SetHeaders(h)
	return c
}

// Reset clear Client object.
func (c *Client) Reset() {
	c.baseUrl = ""

	for k := range c.header.Header {
		delete(c.header.Header, k)
	}

	c.core.reset()
}

// Get provide a API like axios which send get request.
func (c *Client) Get(url string) (*Response, error) {
	req := AcquireRequest().
		setMethod(fiber.MethodGet).
		SetURL(url)

	return c.core.execute(req.Context(), c, req)
}

var (
	defaultClient *Client
	clientPool    sync.Pool
)

func init() {
	defaultClient = AcquireClient()
}

// AcquireClient returns an empty Client object from the pool.
//
// The returned Client object may be returned to the pool with ReleaseClient when no longer needed.
// This allows reducing GC load.
func AcquireClient() (c *Client) {
	cv := clientPool.Get()
	if cv != nil {
		c = cv.(*Client)
		return
	}
	c = &Client{
		core: AcquireCore(),
		header: &Header{
			Header: make(http.Header),
		},
	}
	return
}

// ReleaseClient returns the object acquired via AcquireClient to the pool.
//
// Do not access the released Client object, otherwise data races may occur.
func ReleaseClient(c *Client) {
	c.Reset()
	clientPool.Put(c)
}

// Get default client.
func C() *Client {
	return defaultClient
}

// Get send a get request use defaultClient, a convenient method.
func Get(url string) (*Response, error) {
	return defaultClient.Get(url)
}
