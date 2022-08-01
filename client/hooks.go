package client

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	httpBytes  = []byte("http")
	httpsBytes = []byte("https")

	protocolCheck = regexp.MustCompile(`^https?://.*$`)
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
// The baseUrl will be merge with request uri.
// TODO: Query params and path params should be deal in this function.
func parserURL(c *Client, req *Request) error {
	splitUrl := strings.Split(req.url, "?")
	// I don't want to judege splitUrl length.
	splitUrl = append(splitUrl, "")

	// Determine whether to superimpose baseurl based on
	// whether the URL starts with the protocol
	uri := splitUrl[0]
	if !protocolCheck.MatchString(uri) {
		uri = c.baseUrl + uri
		if !protocolCheck.MatchString(uri) {
			return fmt.Errorf("url format error")
		}
	}

	// set uri to request and orther related setting
	req.rawRequest.SetRequestURI(uri)
	rawUri := req.rawRequest.URI()
	isTLS, scheme := false, rawUri.Scheme()
	if bytes.Equal(httpsBytes, scheme) {
		isTLS = true
	} else if !bytes.Equal(httpBytes, scheme) {
		return fmt.Errorf("unsupported protocol %q. http and https are supported", scheme)
	}

	c.core.client.Addr = addMissingPort(string(rawUri.Host()), isTLS)
	c.core.client.IsTLS = isTLS

	// merge query params
	hashSplit := strings.Split(splitUrl[1], "#")
	hashSplit = append(hashSplit, "")
	queryParams, err := url.ParseQuery(hashSplit[0])
	if err != nil {
		return err
	}
	for k, v := range c.params.Values {
		for _, vv := range v {
			queryParams.Add(k, vv)
		}
	}

	for k, v := range req.params.Values {
		for _, vv := range v {
			queryParams.Add(k, vv)
		}
	}

	req.rawRequest.URI().SetQueryString(queryParams.Encode())
	req.rawRequest.URI().SetHash(hashSplit[1])

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
