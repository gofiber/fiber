package logger

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
)

// Logger variables
const (
	TagPid               = "pid"
	TagTime              = "time"
	TagReferer           = "referer"
	TagProtocol          = "protocol"
	TagPort              = "port"
	TagIP                = "ip"
	TagIPs               = "ips"
	TagHost              = "host"
	TagMethod            = "method"
	TagPath              = "path"
	TagURL               = "url"
	TagUA                = "ua"
	TagLatency           = "latency"
	TagStatus            = "status"
	TagResBody           = "resBody"
	TagReqHeaders        = "reqHeaders"
	TagQueryStringParams = "queryParams"
	TagBody              = "body"
	TagBytesSent         = "bytesSent"
	TagBytesReceived     = "bytesReceived"
	TagRoute             = "route"
	TagError             = "error"
	// DEPRECATED: Use TagReqHeader instead
	TagHeader     = "header:"
	TagReqHeader  = "reqHeader:"
	TagRespHeader = "respHeader:"
	TagLocals     = "locals:"
	TagQuery      = "query:"
	TagForm       = "form:"
	TagCookie     = "cookie:"
	TagBlack      = "black"
	TagRed        = "red"
	TagGreen      = "green"
	TagYellow     = "yellow"
	TagBlue       = "blue"
	TagMagenta    = "magenta"
	TagCyan       = "cyan"
	TagWhite      = "white"
	TagReset      = "reset"
)

// createTagMap function merged the default with the custom tags
func createTagMap(cfg *Config) map[string]LogFunc {
	// Set default tags
	tagFunctions := map[string]LogFunc{
		TagReferer: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Get(fiber.HeaderReferer))
		},
		TagProtocol: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Protocol())
		},
		TagPort: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Port())
		},
		TagIP: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.IP())
		},
		TagIPs: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Get(fiber.HeaderXForwardedFor))
		},
		TagHost: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Hostname())
		},
		TagPath: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Path())
		},
		TagURL: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.OriginalURL())
		},
		TagUA: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Get(fiber.HeaderUserAgent))
		},
		TagBody: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.Write(c.Body())
		},
		TagBytesReceived: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return appendInt(buf, len(c.Request().Body()))
		},
		TagBytesSent: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return appendInt(buf, len(c.Response().Body()))
		},
		TagRoute: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Route().Path)
		},
		TagResBody: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.Write(c.Response().Body())
		},
		TagReqHeaders: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			reqHeaders := make([]string, 0)
			for k, v := range c.GetReqHeaders() {
				reqHeaders = append(reqHeaders, k+"="+v)
			}
			return buf.Write([]byte(strings.Join(reqHeaders, "&")))
		},
		TagQueryStringParams: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Request().URI().QueryArgs().String())
		},

		TagBlack: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Black)
		},
		TagRed: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Red)
		},
		TagGreen: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Green)
		},
		TagYellow: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Yellow)
		},
		TagBlue: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Blue)
		},
		TagMagenta: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Magenta)
		},
		TagCyan: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Cyan)
		},
		TagWhite: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.White)
		},
		TagReset: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.App().Config().ColorScheme.Reset)
		},
		TagError: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			if data.ChainErr != nil {
				return buf.WriteString(data.ChainErr.Error())
			}
			return buf.WriteString("-")
		},
		TagReqHeader: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Get(extraParam))
		},
		TagHeader: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Get(extraParam))
		},
		TagRespHeader: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.GetRespHeader(extraParam))
		},
		TagQuery: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Query(extraParam))
		},
		TagForm: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.FormValue(extraParam))
		},
		TagCookie: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(c.Cookies(extraParam))
		},
		TagLocals: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			switch v := c.Locals(extraParam).(type) {
			case []byte:
				return buf.Write(v)
			case string:
				return buf.WriteString(v)
			case nil:
				return 0, nil
			default:
				return buf.WriteString(fmt.Sprintf("%v", v))
			}
		},
		TagStatus: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			if cfg.enableColors {
				colors := c.App().Config().ColorScheme
				return buf.WriteString(fmt.Sprintf("%s %3d %s", statusColor(c.Response().StatusCode(), colors), c.Response().StatusCode(), colors.Reset))
			}
			return appendInt(buf, c.Response().StatusCode())
		},
		TagMethod: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			if cfg.enableColors {
				colors := c.App().Config().ColorScheme
				return buf.WriteString(fmt.Sprintf("%s %-7s %s", methodColor(c.Method(), colors), c.Method(), colors.Reset))
			}
			return buf.WriteString(c.Method())
		},
		TagPid: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(data.Pid)
		},
		TagLatency: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			latency := data.Stop.Sub(data.Start).Round(time.Millisecond)
			return buf.WriteString(fmt.Sprintf("%7v", latency))
		},
		TagTime: func(buf *bytebufferpool.ByteBuffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return buf.WriteString(data.Timestamp.Load().(string))
		},
	}
	// merge with custom tags from user
	if cfg.CustomTags != nil {
		for k, v := range cfg.CustomTags {
			tagFunctions[k] = v
		}
	}

	return tagFunctions
}
