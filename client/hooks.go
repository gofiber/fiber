package client

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
)

var (
	httpBytes  = []byte("http")
	httpsBytes = []byte("https")
)

// addMissingPort will add the corresponding port number for host.
func addMissingPort(addr string, isTLS bool) string {
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

// parserURL will set the options for the hostclient
// and normalize the url.
// TODO: The baseUrl should be merge with request uri.
// TODO: Query params and path params should be deal in this function.
func parserURL(c *Client, req *Request) error {
	req.rawRequest.SetRequestURI(req.url)

	uri := req.rawRequest.URI()

	isTLS, scheme := false, uri.Scheme()
	if bytes.Equal(httpsBytes, scheme) {
		isTLS = true
	} else if !bytes.Equal(httpBytes, scheme) {
		return fmt.Errorf("unsupported protocol %q. http and https are supported", scheme)
	}

	c.core.client.Addr = addMissingPort(string(uri.Host()), isTLS)
	c.core.client.IsTLS = isTLS

	return nil
}

// parserHeader will make request header up.
// It will merge headers from client and request.
// TODO: Header should be set automatically based on data.
// TODO: User-Agent should be set?
func parserHeader(c *Client, req *Request) error {
	for k, v := range c.header.Header {
		req.rawRequest.Header.Set(k, strings.Join(v, ", "))
	}

	for k, v := range req.header.Header {
		req.rawRequest.Header.Set(k, strings.Join(v, ", "))
	}

	return nil
}
