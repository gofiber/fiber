package middleware

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	fiber "github.com/gofiber/fiber"
	utils "github.com/gofiber/utils"
	colorable "github.com/mattn/go-colorable"
	isatty "github.com/mattn/go-isatty"
	bytebufferpool "github.com/valyala/bytebufferpool"
)

// Middleware types
type (
	// LoggerConfig defines the config for Logger middleware.
	LoggerConfig struct {
		// Next defines a function to skip this middleware if returned true.
		Next func(*fiber.Ctx) bool

		// Format defines the logging tags
		//
		// - pid
		// - time
		// - ip
		// - ips
		// - url
		// - host
		// - method
		// - methodColored
		// - path
		// - protocol
		// - route
		// - referer
		// - ua
		// - latency
		// - status
		// - statusColored
		// - body
		// - error
		// - bytesSent
		// - bytesReceived
		// - header:<key>
		// - query:<key>
		// - form:<key>
		// - cookie:<key>
		//
		// Optional. Default: #${pid} - ${time} ${status} - ${latency} ${method} ${path}\n
		Format string

		// TimeFormat https://programming.guide/go/format-parse-string-time-date-example.html
		//
		// Optional. Default: 2006/01/02 15:04:05
		TimeFormat string

		// TimeZone can be specified, such as "UTC" and "America/New_York" and "Asia/Chongqing", etc
		//
		// Optional. Default: Local
		TimeZone string

		// Output is a writter where logs are written
		//
		// Default: os.Stderr
		Output io.Writer

		// Colors are only supported if no custom Output is given
		enableColors bool

		// timeZoneLocation holds the compiled timezone
		timeZoneLocation *time.Location
	}
)

// Logger variables
const (
	LoggerTagPid           = "pid"
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
	LoggerTagColorBlack    = "black"
	LoggerTagColorRed      = "red"
	LoggerTagColorGreen    = "green"
	LoggerTagColorYellow   = "yellow"
	LoggerTagColorBlue     = "blue"
	LoggerTagColorMagenta  = "magenta"
	LoggerTagColorCyan     = "cyan"
	LoggerTagColorWhite    = "white"
	LoggerTagColorReset    = "resetColor"
	// LoggerTagStatusColor   = "statusColor"
	// LoggerTagMethodColor   = "methodColor"
)

// NEW : Color variables
const (
	cBlack   = "\u001b[90m"
	cRed     = "\u001b[91m"
	cGreen   = "\u001b[92m"
	cYellow  = "\u001b[93m"
	cBlue    = "\u001b[94m"
	cMagenta = "\u001b[95m"
	cCyan    = "\u001b[96m"
	cWhite   = "\u001b[97m"
	cReset   = "\u001b[0m"
)

// for colorizing response status and request method
var (
	statusColor    string
	responseStatus int
	methodColor    string
	requestMethod  string
)

// LoggerConfigDefault is the default config
var LoggerConfigDefault = LoggerConfig{
	Next:       nil,
	Format:     "#${pid} - ${time} ${status} - ${latency} ${method} ${path}\n",
	TimeFormat: "2006/01/02 15:04:05",
	TimeZone:   "Local",
	Output:     os.Stderr,
}

/*
Logger allows the following config arguments in any order:
	- Logger()
	- Logger(next func(*fiber.Ctx) bool)
	- Logger(output io.Writer)
	- Logger(format string)
	- Logger(timeZone string)
	- Logger(timeFormat string)
	- Logger(config LoggerConfig)
*/
func Logger(options ...interface{}) fiber.Handler {
	// Create default config
	var config = LoggerConfig{}
	// Assert options if provided to adjust the config
	if len(options) > 0 {
		for i := range options {
			switch opt := options[i].(type) {
			case func(*fiber.Ctx) bool:
				config.Next = opt
			case string:
				if strings.Contains(opt, "${") {
					config.Format = opt
				} else if tzl := getTimeZoneLocation(opt); tzl != nil {
					config.TimeZone = opt
					config.timeZoneLocation = tzl
				} else {
					config.TimeFormat = opt
				}
			case io.Writer:
				config.Output = opt
			case LoggerConfig:
				config = opt
			default:
				panic("Logger: the following option types are allowed: string, io.Writer, LoggerConfig")
			}
		}
	}
	// Return logger
	return logger(config)
}

func logger(config LoggerConfig) fiber.Handler {
	// Set config default values
	if config.Format == "" {
		config.Format = LoggerConfigDefault.Format
	}
	if config.TimeZone == "" {
		config.TimeZone = LoggerConfigDefault.TimeZone
	}
	if config.TimeFormat == "" {
		config.TimeFormat = LoggerConfigDefault.TimeFormat
	}
	if config.Output == nil {
		// Check if colors should be disabled
		if os.Getenv("TERM") == "dumb" ||
			(!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
			config.Output = LoggerConfigDefault.Output
		} else {
			config.enableColors = true
			config.Output = colorable.NewColorableStderr()

		}
	}

	var tmpl loggerTemplate
	tmpl.new(config.Format, "${", "}")

	var timestamp atomic.Value
	timestamp.Store(nowTimeString(config.timeZoneLocation, config.TimeFormat))
	// Update date/time every 750 milliseconds in a separate go routine
	if strings.Contains(config.Format, "${time}") {
		go func() {
			for {
				time.Sleep(750 * time.Millisecond)
				timestamp.Store(nowTimeString(config.timeZoneLocation, config.TimeFormat))
			}
		}()
	}
	pid := fmt.Sprintf("%-5s", strconv.Itoa(os.Getpid()))
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
				return buf.WriteString(timestamp.Load().(string))
			case LoggerTagReferer:
				return buf.WriteString(c.Get(fiber.HeaderReferer))
			case LoggerTagProtocol:
				return buf.WriteString(c.Protocol())
			case LoggerTagPid:
				return buf.WriteString(pid)
			case LoggerTagIP:
				return buf.WriteString(c.IP())
			case LoggerTagIPs:
				return buf.WriteString(c.Get(fiber.HeaderXForwardedFor))
			case LoggerTagHost:
				return buf.WriteString(c.Hostname())
			case LoggerTagPath:
				return buf.WriteString(c.Path())
			case LoggerTagURL:
				return buf.WriteString(c.OriginalURL())
			case LoggerTagUA:
				return buf.WriteString(c.Get(fiber.HeaderUserAgent))
			case LoggerTagLatency:
				return buf.WriteString(fmt.Sprintf("%-6s", stop.Sub(start).Round(1*time.Millisecond)))
				// return buf.WriteString(stop.Sub(start).String())
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
			case LoggerTagColorBlack:
				return buf.WriteString(cBlack)
			case LoggerTagColorRed:
				return buf.WriteString(cRed)
			case LoggerTagColorGreen:
				return buf.WriteString(cGreen)
			case LoggerTagColorYellow:
				return buf.WriteString(cYellow)
			case LoggerTagColorBlue:
				return buf.WriteString(cBlue)
			case LoggerTagColorMagenta:
				return buf.WriteString(cMagenta)
			case LoggerTagColorCyan:
				return buf.WriteString(cCyan)
			case LoggerTagColorWhite:
				return buf.WriteString(cWhite)
			case LoggerTagColorReset:
				return buf.WriteString(cReset)
			case LoggerTagStatus:
				responseStatus = c.Fasthttp.Response.StatusCode()
				if !config.enableColors {
					return buf.WriteString(strconv.Itoa(responseStatus))
				}
				switch {
				case responseStatus >= 200 && responseStatus < 300:
					statusColor = cGreen
				case responseStatus >= 300 && responseStatus < 400:
					statusColor = cBlue
				case responseStatus >= 400 && responseStatus < 500:
					statusColor = cYellow
				default:
					statusColor = cRed
				}
				return buf.WriteString(statusColor + strconv.Itoa(responseStatus) + cReset)
			case LoggerTagMethod:
				requestMethod = c.Method()
				if !config.enableColors {
					return buf.WriteString(requestMethod)
				}
				switch requestMethod {
				case fiber.MethodGet:
					methodColor = cGreen
				case fiber.MethodPost:
					methodColor = cCyan
				case fiber.MethodPut:
					methodColor = cYellow
				case fiber.MethodDelete:
					methodColor = cRed
				case fiber.MethodPatch:
					methodColor = cBlue
				case fiber.MethodHead:
					methodColor = cMagenta
				case fiber.MethodOptions:
					methodColor = cBlack
				default:
					methodColor = cReset
				}
				return buf.WriteString(fmt.Sprintf("%s%7s%s", methodColor, requestMethod, cReset))
				//return buf.WriteString(methodColor + requestMethod + cReset)
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

func nowTimeString(tzl *time.Location, layout string) string {
	// This is different from Golang's time package which returns UTC, and Local is better than it
	if tzl == nil {
		return time.Now().Format(layout)
	}
	return time.Now().In(tzl).Format(layout)
}

// Use Golang's time package to determine whether the TimeZone is available
func getTimeZoneLocation(name string) *time.Location {
	tz, _ := time.LoadLocation(name)
	return tz
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
