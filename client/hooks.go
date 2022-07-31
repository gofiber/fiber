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

// parserURL will set the options for the hostclient
// and normalize the url.
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
