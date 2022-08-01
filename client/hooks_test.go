package client

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

func TestParserURL(t *testing.T) {
	t.Parallel()

	t.Run("client baseurl should be set", func(t *testing.T) {
		client := AcquireClient().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("")

		err := parserURL(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "http://example.com/api", req.rawRequest.URI().String())
	})

	t.Run("request url should be set", func(t *testing.T) {
		client := AcquireClient()
		req := AcquireRequest().SetURL("http://example.com/api")

		err := parserURL(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "http://example.com/api", req.rawRequest.URI().String())
	})

	t.Run("the request url will override baseurl with protocol", func(t *testing.T) {
		client := AcquireClient().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("http://example.com/api/v1")

		err := parserURL(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "http://example.com/api/v1", req.rawRequest.URI().String())
	})

	t.Run("the request url should be append after baseurl without protocol", func(t *testing.T) {
		client := AcquireClient().SetBaseURL("http://example.com/api")
		req := AcquireRequest().SetURL("/v1")

		err := parserURL(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "http://example.com/api/v1", req.rawRequest.URI().String())
	})

	t.Run("the url is error", func(t *testing.T) {
		client := AcquireClient().SetBaseURL("example.com/api")
		req := AcquireRequest().SetURL("/v1")

		err := parserURL(client, req)
		utils.AssertEqual(t, fmt.Errorf("url format error"), err)
	})

	t.Run("query params from client should be set", func(t *testing.T) {
		client := AcquireClient().
			SetParam("foo", "bar")
		req := AcquireRequest().SetURL("http://example.com/api/v1")

		err := parserURL(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, []byte("foo=bar"), req.rawRequest.URI().QueryString())
	})

	t.Run("query params from request should be set", func(t *testing.T) {
		client := AcquireClient()
		req := AcquireRequest().
			SetURL("http://example.com/api/v1").
			SetParam("bar", "foo")

		err := parserURL(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, []byte("bar=foo"), req.rawRequest.URI().QueryString())
	})

	t.Run("query params should be merged", func(t *testing.T) {
		client := AcquireClient().
			SetParam("bar", "foo1")
		req := AcquireRequest().
			SetURL("http://example.com/api/v1?bar=foo2").
			SetParam("bar", "foo")

		err := parserURL(client, req)
		utils.AssertEqual(t, nil, err)

		values, _ := url.ParseQuery(string(req.rawRequest.URI().QueryString()))

		flag1, flag2, flag3 := false, false, false
		for _, v := range values["bar"] {
			if v == "foo1" {
				flag1 = true
			} else if v == "foo2" {
				flag2 = true
			} else if v == "foo" {
				flag3 = true
			}
		}
		utils.AssertEqual(t, true, flag1)
		utils.AssertEqual(t, true, flag2)
		utils.AssertEqual(t, true, flag3)
	})
}

func TestParserHeader(t *testing.T) {
	t.Parallel()

	t.Run("client header should be set", func(t *testing.T) {
		client := &Client{
			header: &Header{
				Header: map[string][]string{
					fiber.HeaderContentType: {"application/json"},
				},
			},
		}

		req := &Request{
			header: &Header{
				Header: make(http.Header),
			},
			rawRequest: fasthttp.AcquireRequest(),
		}

		err := parserHeader(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, []byte("application/json"), req.rawRequest.Header.ContentType())
	})

	t.Run("request header should be set", func(t *testing.T) {
		client := &Client{
			header: &Header{
				Header: make(http.Header),
			},
		}

		req := &Request{
			header: &Header{
				Header: map[string][]string{
					fiber.HeaderContentType: {"application/json", "utf-8"},
				},
			},
			rawRequest: fasthttp.AcquireRequest(),
		}

		err := parserHeader(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, []byte("application/json, utf-8"), req.rawRequest.Header.ContentType())
	})

	t.Run("request header should override client header", func(t *testing.T) {
		client := &Client{
			header: &Header{
				Header: map[string][]string{
					fiber.HeaderContentType: {"application/xml"},
				},
			},
		}

		req := &Request{
			header: &Header{
				Header: map[string][]string{
					fiber.HeaderContentType: {"application/json", "utf-8"},
				},
			},
			rawRequest: fasthttp.AcquireRequest(),
		}

		err := parserHeader(client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, []byte("application/json, utf-8"), req.rawRequest.Header.ContentType())
	})
}
