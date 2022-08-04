package client

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

var (
	httpBytes  = []byte("http")
	httpsBytes = []byte("https")

	protocolCheck = regexp.MustCompile(`^https?://.*$`)

	headerAccept = "Accept"

	applicationJSON   = "application/json"
	applicationXML    = "application/xml"
	applicationForm   = "application/x-www-form-urlencoded"
	multipartFormData = "multipart/form-data"

	src           = rand.NewSource(time.Now().UnixNano())
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
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

func randString(n int) string {
	b := make([]byte, n)
	length := len(letterBytes)
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}

		if idx := int(cache & int64(letterIdxMask)); idx < length {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= int64(letterIdxBits)
		remain--
	}

	return utils.UnsafeString(b)
}

// parserRequestURL will set the options for the hostclient
// and normalize the url.
// The baseUrl will be merge with request uri.
// Query params and path params deal in this function.
func parserRequestURL(c *Client, req *Request) error {
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

	// set path params
	req.path.VisitAll(func(key, val string) {
		uri = strings.Replace(uri, "{"+key+"}", val, -1)
	})
	c.path.VisitAll(func(key, val string) {
		uri = strings.Replace(uri, "{"+key+"}", val, -1)
	})

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
	args := fasthttp.AcquireArgs()
	defer func() {
		fasthttp.ReleaseArgs(args)
	}()

	args.Parse(hashSplit[0])
	c.params.VisitAll(func(key, value []byte) {
		args.AddBytesKV(key, value)
	})
	req.params.VisitAll(func(key, value []byte) {
		args.AddBytesKV(key, value)
	})
	req.rawRequest.URI().SetQueryStringBytes(utils.CopyBytes(args.QueryString()))
	req.rawRequest.URI().SetHash(hashSplit[1])

	return nil
}

// parserRequestHeader will make request header up.
// It will merge headers from client and request.
// Header should be set automatically based on data.
// User-Agent should be set.
func parserRequestHeader(c *Client, req *Request) error {
	// merge header
	c.header.VisitAll(func(key, value []byte) {
		req.rawRequest.Header.SetBytesKV(key, value)
	})

	req.header.VisitAll(func(key, value []byte) {
		req.rawRequest.Header.SetBytesKV(key, value)
	})

	// according to data set content-type
	switch req.bodyType {
	case jsonBody:
		req.rawRequest.Header.SetContentType(applicationJSON)
		req.rawRequest.Header.Set(headerAccept, applicationJSON)
	case xmlBody:
		req.rawRequest.Header.SetContentType(applicationXML)
	case formBody:
		req.rawRequest.Header.SetContentType(applicationForm)
	case filesBody:
		req.rawRequest.Header.SetContentType(multipartFormData)
		// set boundary
		req.rawRequest.Header.SetMultipartFormBoundary(req.boundary)
	default:
	}

	// set useragent
	req.rawRequest.Header.SetUserAgent(defaultUserAgent)
	if c.userAgent != "" {
		req.rawRequest.Header.SetUserAgent(c.userAgent)
	}
	if req.userAgent != "" {
		req.rawRequest.Header.SetUserAgent(req.userAgent)
	}

	// set referer
	req.rawRequest.Header.SetReferer(c.referer)
	if req.referer != "" {
		req.rawRequest.Header.SetReferer(req.referer)
	}

	// set cookie
	c.cookies.VisitAll(func(key, val string) {
		req.rawRequest.Header.SetCookie(key, val)
	})

	req.cookies.VisitAll(func(key, val string) {
		req.rawRequest.Header.SetCookie(key, val)
	})

	return nil
}

// parserRequestBody automatically serializes the data according to
// the data type and stores it in the body of the rawRequest
func parserRequestBody(c *Client, req *Request) (err error) {
	switch req.bodyType {
	case jsonBody:
		body, err := c.core.jsonMarshal(req.body)
		if err != nil {
			return err
		}
		req.rawRequest.SetBody(body)
	case xmlBody:
		body, err := c.core.xmlMarshal(req.body)
		if err != nil {
			return err
		}
		req.rawRequest.SetBody(body)
	case formBody:
		req.rawRequest.SetBody(req.formData.QueryString())
	case filesBody:
		mw := multipart.NewWriter(req.rawRequest.BodyWriter())
		mw.SetBoundary(req.boundary)
		defer func() {
			err = mw.Close()
			if err != nil {
				return
			}
		}()

		// add formdata
		req.formData.VisitAll(func(key, value []byte) {
			if err != nil {
				return
			}
			err = mw.WriteField(utils.UnsafeString(key), utils.UnsafeString(value))
		})
		if err != nil {
			return
		}

		// add file
		b := make([]byte, 512)
		for i, v := range req.files {
			if v.name == "" && v.path == "" {
				return fmt.Errorf("the file should have a name")
			}

			// if name is not exist, set name
			if v.name == "" && v.path != "" {
				v.path = filepath.Clean(v.path)
				v.name = filepath.Base(v.name)
			}

			// if param is not exist, set it
			if v.paramName == "" {
				v.paramName = "file" + fmt.Sprint(i)
			}

			// check the reader
			if v.reader == nil {
				v.reader, err = os.Open(v.path)
				if err != nil {
					return
				}
			}

			// wirte file
			w, err := mw.CreateFormFile(v.paramName, v.name)
			if err != nil {
				return err
			}

			for {
				_, err := v.reader.Read(b)
				if err != nil && err != io.EOF {
					return err
				}

				if err == io.EOF {
					break
				}

				w.Write(b)
			}

			// ignore err
			v.reader.Close()
		}
	case rawBody:
		if body, ok := req.body.([]byte); ok {
			req.rawRequest.SetBody(body)
		} else {
			return fmt.Errorf("the raw body should be []byte, but we receive %s", reflect.TypeOf(req.body).Kind().String())
		}
	}
	return nil
}

func parserResponseCookie(c *Client, resp *Response, req *Request) (err error) {
	resp.rawResponse.Header.VisitAllCookie(func(key, value []byte) {
		cookie := fasthttp.AcquireCookie()
		err = cookie.ParseBytes(value)
		if err != nil {
			return
		}
		cookie.SetKeyBytes(key)
	})

	return
}
