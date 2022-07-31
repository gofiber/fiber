package client

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

func TestParserURL(t *testing.T) {
	type args struct {
		c   *Client
		req *Request
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parserURL(tt.args.c, tt.args.req); (err != nil) != tt.wantErr {
				t.Errorf("parserURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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
