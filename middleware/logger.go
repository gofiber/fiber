package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/bytebufferpool"
)

// Middleware types
type (
	// LoggerConfig defines the config for Logger middleware.
	LoggerConfig struct {
		// Next defines a function to skip this middleware if returned true.
		Next func(*fiber.Ctx) bool

		// Format defines the logging tags
		//
		// - time
		// - ip
		// - ips
		// - url
		// - host
		// - method
		// - path
		// - protocol
		// - route
		// - referer
		// - ua
		// - latency
		// - status
		// - body
		// - error
		// - bytesSent
		// - bytesReceived
		// - header:<key>
		// - query:<key>
		// - form:<key>
		// - cookie:<key>
		//
		// Optional. Default: ${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n
		Format string

		// TimeFormat https://programming.guide/go/format-parse-string-time-date-example.html
		//
		// Optional. Default: 15:04:05
		TimeFormat string

		// Output is a writter where logs are written
		//
		// Default: os.Stderr
		Output io.Writer
	}
)

// Logger variables
const (
	LoggerTagTime          = "time"
	LoggerTagReferer       = "referer"
	LoggerTagProtocol      = "protocol"
	LoggerTagIP            = "ip"
	LoggerTagIPs           = "ips"
	LoggerTagHost          = "host"
	LoggerTagMethod        = "method"
	LoggerTagPath          = "path"
	LoggerTagURL           = "url"
	LoggerTagUA            = "ua"
	LoggerTagLatency       = "latency"
	LoggerTagStatus        = "status"
	LoggerTagBody          = "body"
	LoggerTagBytesSent     = "bytesSent"
	LoggerTagBytesReceived = "bytesReceived"
	LoggerTagRoute         = "route"
	LoggerTagError         = "error"
	LoggerTagHeader        = "header:"
	LoggerTagQuery         = "query:"
	LoggerTagForm          = "form:"
	LoggerTagCookie        = "cookie:"
)

// LoggerConfigDefault is the default config
var LoggerConfigDefault = LoggerConfig{
	Next:       nil,
	Format:     "${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n",
	TimeFormat: "15:04:05",
	Output:     os.Stdout,
}

/*
Logger allows the following config arguments in any order:
	- Logger()
	- Logger(next func(*fiber.Ctx) bool)
	- Logger(output io.Writer)
	- Logger(format string)
	- Logger(timeformat string)
	- Logger(config LoggerConfig)
*/
func Logger(options ...interface{}) fiber.Handler {
	// Create default config
	var config = LoggerConfigDefault
	// Assert options if provided to adjust the config
	if len(options) > 0 {
		for i := range options {
			switch opt := options[i].(type) {
			case func(*fiber.Ctx) bool:
				config.Next = opt
			case string:
				if strings.Contains(opt, "${") {
					config.Format = opt
				} else {
					config.TimeFormat = opt
				}
			case io.Writer:
				config.Output = opt
			case LoggerConfig:
				config = opt
			default:
				log.Fatal("Logger: the following option types are allowed: string, io.Writer, LoggerConfig")
			}
		}
	}
	// Return logger
	return logger(config)
}

func logger(config LoggerConfig) fiber.Handler {
	if loggerDeprecated {
		fmt.Println("middleware.LoggerWithConfig is deprecated, please use middleware.Logger")
	}
	// Set config default values
	if config.Format == "" {
		config.Format = LoggerConfigDefault.Format
	}
	if config.TimeFormat == "" {
		config.TimeFormat = LoggerConfigDefault.TimeFormat
	}
	if config.Output == nil {
		config.Output = LoggerConfigDefault.Output
	}
	// Middleware settings
	var mutex sync.RWMutex

	var tmpl loggerTemplate
	tmpl.new(config.Format, "${", "}")

	timestamp := time.Now().Format(config.TimeFormat)
	// Update date/time every second in a separate go routine
	if strings.Contains(config.Format, "${time}") {
		go func() {
			for {
				mutex.Lock()
				timestamp = time.Now().Format(config.TimeFormat)
				mutex.Unlock()
				time.Sleep(500 * time.Millisecond)
			}
		}()
	}
	// Return handler
	return func(c *fiber.Ctx) {
		// Don't execute the middleware if Next returns true
		if config.Next != nil && config.Next(c) {
			c.Next()
			return
		}
		// Middleware logic...
		start := time.Now()
		// handle request
		c.Next()
		// build log
		stop := time.Now()
		// Get new buffer
		buf := bytebufferpool.Get()
		_, err := tmpl.executeFunc(buf, func(w io.Writer, tag string) (int, error) {
			switch tag {
			case LoggerTagTime:
				mutex.RLock()
				defer mutex.RUnlock()
				return buf.WriteString(timestamp)
			case LoggerTagReferer:
				return buf.WriteString(c.Get(fiber.HeaderReferer))
			case LoggerTagProtocol:
				return buf.WriteString(c.Protocol())
			case LoggerTagIP:
				return buf.WriteString(c.IP())
			case LoggerTagIPs:
				return buf.WriteString(c.Get(fiber.HeaderXForwardedFor))
			case LoggerTagHost:
				return buf.WriteString(c.Hostname())
			case LoggerTagMethod:
				return buf.WriteString(c.Method())
			case LoggerTagPath:
				return buf.WriteString(c.Path())
			case LoggerTagURL:
				return buf.WriteString(c.OriginalURL())
			case LoggerTagUA:
				return buf.WriteString(c.Get(fiber.HeaderUserAgent))
			case LoggerTagLatency:
				return buf.WriteString(stop.Sub(start).String())
			case LoggerTagStatus:
				return buf.WriteString(strconv.Itoa(c.Fasthttp.Response.StatusCode()))
			case LoggerTagBody:
				return buf.WriteString(c.Body())
			case LoggerTagBytesReceived:
				return buf.WriteString(strconv.Itoa(len(c.Fasthttp.Request.Body())))
			case LoggerTagBytesSent:
				return buf.WriteString(strconv.Itoa(len(c.Fasthttp.Response.Body())))
			case LoggerTagRoute:
				return buf.WriteString(c.Route().Path)
			case LoggerTagError:
				if c.Error() != nil {
					return buf.WriteString(c.Error().Error())
				}
			default:
				switch {
				case strings.HasPrefix(tag, LoggerTagHeader):
					return buf.WriteString(c.Get(tag[7:]))
				case strings.HasPrefix(tag, LoggerTagQuery):
					return buf.WriteString(c.Query(tag[6:]))
				case strings.HasPrefix(tag, LoggerTagForm):
					return buf.WriteString(c.FormValue(tag[5:]))
				case strings.HasPrefix(tag, LoggerTagCookie):
					return buf.WriteString(c.Cookies(tag[7:]))
				}
			}
			return 0, nil
		})
		if err != nil {
			_, _ = buf.WriteString(err.Error())
		}
		if _, err := config.Output.Write(buf.Bytes()); err != nil {
			fmt.Println(err)
		}
		bytebufferpool.Put(buf)
	}
}

// MIT License fasttemplate
// Copyright (c) 2015 Aliaksandr Valialkin
// https://github.com/valyala/fasttemplate/blob/master/LICENSE

type (
	loggerTemplate struct {
		template string
		startTag string
		endTag   string
		texts    [][]byte
		tags     []string
	}
	loggerTagFunc func(w io.Writer, tag string) (int, error)
)

func (t *loggerTemplate) new(template, startTag, endTag string) {
	t.template = template
	t.startTag = startTag
	t.endTag = endTag
	t.texts = t.texts[:0]
	t.tags = t.tags[:0]

	if len(startTag) == 0 {
		panic("startTag cannot be empty")
	}
	if len(endTag) == 0 {
		panic("endTag cannot be empty")
	}

	s := utils.GetBytes(template)
	a := utils.GetBytes(startTag)
	b := utils.GetBytes(endTag)

	tagsCount := bytes.Count(s, a)
	if tagsCount == 0 {
		return
	}

	if tagsCount+1 > cap(t.texts) {
		t.texts = make([][]byte, 0, tagsCount+1)
	}
	if tagsCount > cap(t.tags) {
		t.tags = make([]string, 0, tagsCount)
	}

	for {
		n := bytes.Index(s, a)
		if n < 0 {
			t.texts = append(t.texts, s)
			break
		}
		t.texts = append(t.texts, s[:n])

		s = s[n+len(a):]
		n = bytes.Index(s, b)
		if n < 0 {
			panic(fmt.Errorf("cannot find end tag=%q in the template=%q starting from %q", endTag, template, s))
		}

		t.tags = append(t.tags, utils.GetString(s[:n]))
		s = s[n+len(b):]
	}
}

func (t *loggerTemplate) executeFunc(w io.Writer, f loggerTagFunc) (int64, error) {
	var nn int64

	n := len(t.texts) - 1
	if n == -1 {
		ni, err := w.Write(utils.GetBytes(t.template))
		return int64(ni), err
	}

	for i := 0; i < n; i++ {
		ni, err := w.Write(t.texts[i])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}

		ni, err = f(w, t.tags[i])
		nn += int64(ni)
		if err != nil {
			return nn, err
		}
	}
	ni, err := w.Write(t.texts[n])
	nn += int64(ni)
	return nn, err
}
