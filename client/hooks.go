package client

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

var (
	protocolCheck = regexp.MustCompile(`^https?://.*$`)

	headerAccept = "Accept"

	applicationJSON   = "application/json"
	applicationXML    = "application/xml"
	applicationForm   = "application/x-www-form-urlencoded"
	multipartFormData = "multipart/form-data"

	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// randString returns a random string with n length
func randString(n int) string {
	b := make([]byte, n)
	length := len(letterBytes)
	src := rand.NewSource(time.Now().UnixNano())

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
	splitURL := strings.Split(req.url, "?")
	// I don't want to judge splitURL length.
	splitURL = append(splitURL, "")

	// Determine whether to superimpose baseurl based on
	// whether the URL starts with the protocol
	uri := splitURL[0]
	if !protocolCheck.MatchString(uri) {
		uri = c.baseURL + uri
		if !protocolCheck.MatchString(uri) {
			return ErrURLFormat
		}
	}

	// set path params
	req.path.VisitAll(func(key, val string) {
		uri = strings.ReplaceAll(uri, ":"+key, val)
	})
	c.path.VisitAll(func(key, val string) {
		uri = strings.ReplaceAll(uri, ":"+key, val)
	})

	// set uri to request and other related setting
	req.RawRequest.SetRequestURI(uri)

	// merge query params
	hashSplit := strings.Split(splitURL[1], "#")
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
	req.RawRequest.URI().SetQueryStringBytes(utils.CopyBytes(args.QueryString()))
	req.RawRequest.URI().SetHash(hashSplit[1])

	return nil
}

// parserRequestHeader will make request header up.
// It will merge headers from client and request.
// Header should be set automatically based on data.
// User-Agent should be set.
func parserRequestHeader(c *Client, req *Request) error {
	// set method
	req.RawRequest.Header.SetMethod(req.Method())
	// merge header
	c.header.VisitAll(func(key, value []byte) {
		req.RawRequest.Header.AddBytesKV(key, value)
	})

	req.header.VisitAll(func(key, value []byte) {
		req.RawRequest.Header.AddBytesKV(key, value)
	})

	// according to data set content-type
	switch req.bodyType {
	case jsonBody:
		req.RawRequest.Header.SetContentType(applicationJSON)
		req.RawRequest.Header.Set(headerAccept, applicationJSON)
	case xmlBody:
		req.RawRequest.Header.SetContentType(applicationXML)
	case formBody:
		req.RawRequest.Header.SetContentType(applicationForm)
	case filesBody:
		req.RawRequest.Header.SetContentType(multipartFormData)
		// set boundary
		if req.boundary == boundary {
			req.boundary += randString(16)
		}
		req.RawRequest.Header.SetMultipartFormBoundary(req.boundary)
	default:
	}

	// set useragent
	req.RawRequest.Header.SetUserAgent(defaultUserAgent)
	if c.userAgent != "" {
		req.RawRequest.Header.SetUserAgent(c.userAgent)
	}
	if req.userAgent != "" {
		req.RawRequest.Header.SetUserAgent(req.userAgent)
	}

	// set referer
	req.RawRequest.Header.SetReferer(c.referer)
	if req.referer != "" {
		req.RawRequest.Header.SetReferer(req.referer)
	}

	// set cookie
	// add cookie form jar to req
	if c.cookieJar != nil {
		c.cookieJar.dumpCookiesToReq(req.RawRequest)
	}

	c.cookies.VisitAll(func(key, val string) {
		req.RawRequest.Header.SetCookie(key, val)
	})

	req.cookies.VisitAll(func(key, val string) {
		req.RawRequest.Header.SetCookie(key, val)
	})

	return nil
}

// parserRequestBody automatically serializes the data according to
// the data type and stores it in the body of the rawRequest
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
	case formBody:
		req.RawRequest.SetBody(req.formData.QueryString())
	case filesBody:
		mw := multipart.NewWriter(req.RawRequest.BodyWriter())
		err := mw.SetBoundary(req.boundary)
		if err != nil {
			return fmt.Errorf("set boundary error: %w", err)
		}
		defer func() {
			err := mw.Close()
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
			return fmt.Errorf("write formdata error: %w", err)
		}

		// add file
		b := make([]byte, 512)
		for i, v := range req.files {
			if v.name == "" && v.path == "" {
				return ErrFileNoName
			}

			// if name is not exist, set name
			if v.name == "" && v.path != "" {
				v.path = filepath.Clean(v.path)
				v.name = filepath.Base(v.path)
			}

			// if field name is not exist, set it
			if v.fieldName == "" {
				v.fieldName = "file" + strconv.Itoa(i+1)
			}

			// check the reader
			if v.reader == nil {
				v.reader, err = os.Open(v.path)
				if err != nil {
					return fmt.Errorf("open file error: %w", err)
				}
			}

			// write file
			w, err := mw.CreateFormFile(v.fieldName, v.name)
			if err != nil {
				return fmt.Errorf("create file error: %w", err)
			}

			for {
				n, err := v.reader.Read(b)
				if err != nil && !errors.Is(err, io.EOF) {
					return fmt.Errorf("read file error: %w", err)
				}

				if errors.Is(err, io.EOF) {
					break
				}

				_, err = w.Write(b[:n])
				if err != nil {
					return fmt.Errorf("write file error: %w", err)
				}
			}

			err = v.reader.Close()
			if err != nil {
				return fmt.Errorf("close file error: %w", err)
			}
		}
	case rawBody:
		if body, ok := req.body.([]byte); ok {
			req.RawRequest.SetBody(body)
		} else {
			return ErrBodyType
		}
	case noBody:
		return nil
	}

	return nil
}

// parserResponseHeader will parse the response header and store it in the response
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

	// store cookies to jar
	if c.cookieJar != nil {
		c.cookieJar.parseCookiesFromResp(req.RawRequest.URI().Host(), req.RawRequest.URI().Path(), resp.RawResponse)
	}

	return nil
}

// logger is a response hook that logs the request and response
func logger(c *Client, resp *Response, req *Request) error {
	if !c.debug {
		return nil
	}

	c.logger.Debugf("%s\n", req.RawRequest.String())
	c.logger.Debugf("%s\n", resp.RawResponse.String())

	return nil
}
