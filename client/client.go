package client

import (
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

type Client struct {
	core

	baseUrl string
	header  map[string][]string
}

// Add user-defined request hooks.
func (c *Client) AddRequestHook(h ...RequestHook) *Client {
	c.userRequestHooks = append(c.userRequestHooks, h...)
	return c
}

// Add user-defined response hooks.
func (c *Client) AddResponseHook(h ...ResponseHook) *Client {
	c.userResponseHooks = append(c.userResponseHooks, h...)
	return c
}

func (c *Client) SetDial(f fasthttp.DialFunc) *Client {
	c.client.Dial = f
	return c
}

// Set json encoder.
func (c *Client) SetJSONMarshal(f utils.JSONMarshal) *Client {
	c.jsonMarshal = f
	return c
}

// Set json decoder.
func (c *Client) SetJSONUnmarshal(f utils.JSONUnmarshal) *Client {
	c.jsonUnmarshal = f
	return c
}

// Set xml encoder.
func (c *Client) SetXMLMarshal(f utils.XMLMarshal) *Client {
	c.xmlMarshal = f
	return c
}

// Set xml decoder.
func (c *Client) SetXMLUnmarshal(f utils.XMLUnmarshal) *Client {
	c.xmlUnmarshal = f
	return c
}

func (c *Client) Get(url string) (*Response, error) {
	req := AcquireRequest().
		SetURL(url).
		SetMethod(fiber.MethodGet)

	return c.execute(req.Context(), c, req)
}

var (
	defaultClient *Client
	clientPool    sync.Pool
)

func init() {
	defaultClient = AcquireClient()
}

func AcquireClient() *Client {
	return &Client{
		core:   *acquireCore(),
		header: map[string][]string{},
	}
}

// Get default client.
func C() *Client {
	return defaultClient
}

func Get(url string) (*Response, error) {
	return defaultClient.Get(url)
}
