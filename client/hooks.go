package client

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

var protocolCheck = regexp.MustCompile(`^https?://.*$`)

const (
	headerAccept      = "Accept"
	applicationJSON   = "application/json"
	applicationCBOR   = "application/cbor"
	applicationXML    = "application/xml"
	applicationForm   = "application/x-www-form-urlencoded"
	multipartFormData = "multipart/form-data"

	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// unsafeRandString returns a random string of length n.
// An error is returned if the random source fails.
func unsafeRandString(n int) (string, error) {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		if err != nil {
			return "", fmt.Errorf("failed to read random index: %w", err)
		}
		b[i] = letterBytes[idx.Int64()]
	}

	return utils.UnsafeString(b), nil
}

// parserRequestURL sets options for the hostclient and normalizes the URL.
// It merges the baseURL with the request URI if needed and applies query and path parameters.
func parserRequestURL(c *Client, req *Request) error {
	splitURL := strings.Split(req.url, "?")
	// Ensure splitURL has at least two elements.
	splitURL = append(splitURL, "")

	// If the URL doesn't start with http/https, prepend the baseURL.
	uri := splitURL[0]
	if !protocolCheck.MatchString(uri) {
		uri = c.baseURL + uri
		if !protocolCheck.MatchString(uri) {
			return ErrURLFormat
		}
	}

	// Set path parameters from the request and client.
	req.path.VisitAll(func(key, val string) {
		uri = strings.ReplaceAll(uri, ":"+key, val)
	})
	c.path.VisitAll(func(key, val string) {
		uri = strings.ReplaceAll(uri, ":"+key, val)
	})

	// Set the URI in the raw request.
	req.RawRequest.SetRequestURI(uri)

	// Merge query parameters.
	hashSplit := strings.Split(splitURL[1], "#")
	hashSplit = append(hashSplit, "")
	args := fasthttp.AcquireArgs()
	defer fasthttp.ReleaseArgs(args)

	args.Parse(hashSplit[0])

	c.params.VisitAll(func(key, value []byte) {
		args.AddBytesKV(key, value)
	})
	req.params.VisitAll(func(key, value []byte) {
		args.AddBytesKV(key, value)
	})

	req.RawRequest.URI().SetQueryStringBytes(utils.CopyBytes(args.QueryString()))
	req.RawRequest.URI().SetHash(hashSplit[1])

	return nil
}

// parserRequestHeader merges client and request headers, and sets headers automatically based on the request data.
// It also sets the User-Agent and Referer headers, and applies any cookies from the cookie jar.
func parserRequestHeader(c *Client, req *Request) error {
	// Set HTTP method.
	req.RawRequest.Header.SetMethod(req.Method())

	// Merge headers from the client.
	c.header.VisitAll(func(key, value []byte) {
		req.RawRequest.Header.AddBytesKV(key, value)
	})

	// Merge headers from the request.
	req.header.VisitAll(func(key, value []byte) {
		req.RawRequest.Header.AddBytesKV(key, value)
	})

	// Set Content-Type and Accept headers based on the request body type.
	switch req.bodyType {
	case jsonBody:
		req.RawRequest.Header.SetContentType(applicationJSON)
		req.RawRequest.Header.Set(headerAccept, applicationJSON)
	case xmlBody:
		req.RawRequest.Header.SetContentType(applicationXML)
	case cborBody:
		req.RawRequest.Header.SetContentType(applicationCBOR)
	case formBody:
		req.RawRequest.Header.SetContentType(applicationForm)
	case filesBody:
		req.RawRequest.Header.SetContentType(multipartFormData)
		// If boundary is default, append a random string to it.
		if req.boundary == boundary {
			randStr, err := unsafeRandString(16)
			if err != nil {
				return fmt.Errorf("boundary generation: %w", err)
			}
			req.boundary += randStr
		}
		req.RawRequest.Header.SetMultipartFormBoundary(req.boundary)
	default:
		// noBody or rawBody do not require special handling here.
	}

	// Set User-Agent header.
	req.RawRequest.Header.SetUserAgent(defaultUserAgent)
	if c.userAgent != "" {
		req.RawRequest.Header.SetUserAgent(c.userAgent)
	}
	if req.userAgent != "" {
		req.RawRequest.Header.SetUserAgent(req.userAgent)
	}

	// Set Referer header.
	req.RawRequest.Header.SetReferer(c.referer)
	if req.referer != "" {
		req.RawRequest.Header.SetReferer(req.referer)
	}

	// Set cookies from the cookie jar if available.
	if c.cookieJar != nil {
		c.cookieJar.dumpCookiesToReq(req.RawRequest)
	}

	// Set cookies from the client.
	c.cookies.VisitAll(func(key, val string) {
		req.RawRequest.Header.SetCookie(key, val)
	})

	// Set cookies from the request.
	req.cookies.VisitAll(func(key, val string) {
		req.RawRequest.Header.SetCookie(key, val)
	})

	return nil
}

// parserRequestBody serializes the request body based on its type and sets it into the RawRequest.
func parserRequestBody(c *Client, req *Request) error {
	switch req.bodyType {
	case jsonBody:
		body, err := c.jsonMarshal(req.body)
		if err != nil {
			return err
		}
		req.RawRequest.SetBody(body)
	case xmlBody:
		body, err := c.xmlMarshal(req.body)
		if err != nil {
			return err
		}
		req.RawRequest.SetBody(body)
	case cborBody:
		body, err := c.cborMarshal(req.body)
		if err != nil {
			return err
		}
		req.RawRequest.SetBody(body)
	case formBody:
		req.RawRequest.SetBody(req.formData.QueryString())
	case filesBody:
		return parserRequestBodyFile(req)
	case rawBody:
		if body, ok := req.body.([]byte); ok { //nolint:revive // ignore simplicity
			req.RawRequest.SetBody(body)
		} else {
			return ErrBodyType
		}
	case noBody:
		// No body to set.
		return nil
	}

	return nil
}

// parserRequestBodyFile handles the case where the request contains files to be uploaded.
func parserRequestBodyFile(req *Request) error {
	mw := multipart.NewWriter(req.RawRequest.BodyWriter())
	err := mw.SetBoundary(req.boundary)
	if err != nil {
		return fmt.Errorf("set boundary error: %w", err)
	}
	defer func() {
		e := mw.Close()
		if e != nil {
			// Close errors are typically ignored.
			return
		}
	}()

	// Add form data.
	req.formData.VisitAll(func(key, value []byte) {
		if err != nil {
			return
		}
		err = mw.WriteField(utils.UnsafeString(key), utils.UnsafeString(value))
	})
	if err != nil {
		return fmt.Errorf("write formdata error: %w", err)
	}

	// Add files.
	fileBuf := make([]byte, 1<<20) // 1MB buffer
	for i, v := range req.files {
		if v.name == "" && v.path == "" {
			return ErrFileNoName
		}

		// Set the file name if not provided.
		if v.name == "" && v.path != "" {
			v.path = filepath.Clean(v.path)
			v.name = filepath.Base(v.path)
		}

		// Set the field name if not provided.
		if v.fieldName == "" {
			v.fieldName = "file" + strconv.Itoa(i+1)
		}

		// If reader is not set, open the file.
		if v.reader == nil {
			v.reader, err = os.Open(v.path)
			if err != nil {
				return fmt.Errorf("open file error: %w", err)
			}
		}

		// Create form file and copy the content.
		w, err := mw.CreateFormFile(v.fieldName, v.name)
		if err != nil {
			return fmt.Errorf("create file error: %w", err)
		}

		if _, err := io.CopyBuffer(w, v.reader, fileBuf); err != nil {
			return fmt.Errorf("failed to copy file data: %w", err)
		}

		if err := v.reader.Close(); err != nil {
			return fmt.Errorf("close file error: %w", err)
		}
	}

	return nil
}

// parserResponseCookie parses the Set-Cookie headers from the response and stores them.
func parserResponseCookie(c *Client, resp *Response, req *Request) error {
	var err error
	resp.RawResponse.Header.VisitAllCookie(func(key, value []byte) {
		cookie := fasthttp.AcquireCookie()
		err = cookie.ParseBytes(value)
		if err != nil {
			return
		}
		cookie.SetKeyBytes(key)
		resp.cookie = append(resp.cookie, cookie)
	})

	if err != nil {
		return err
	}

	// Store cookies in the cookie jar if available.
	if c.cookieJar != nil {
		c.cookieJar.parseCookiesFromResp(req.RawRequest.URI().Host(), req.RawRequest.URI().Path(), resp.RawResponse)
	}

	return nil
}

// logger is a response hook that logs request and response data if debug mode is enabled.
func logger(c *Client, resp *Response, req *Request) error {
	if !c.debug {
		return nil
	}

	c.logger.Debugf("%s\n", req.RawRequest.String())
	c.logger.Debugf("%s\n", resp.RawResponse.String())

	return nil
}
