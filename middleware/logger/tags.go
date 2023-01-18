package logger

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
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
		TagReferer: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Get(fiber.HeaderReferer))
		},
		TagProtocol: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Protocol())
		},
		TagPort: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Port())
		},
		TagIP: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.IP())
		},
		TagIPs: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Get(fiber.HeaderXForwardedFor))
		},
		TagHost: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Hostname())
		},
		TagPath: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Path())
		},
		TagURL: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.OriginalURL())
		},
		TagUA: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Get(fiber.HeaderUserAgent))
		},
		TagBody: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.Write(c.Body())
		},
		TagBytesReceived: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return appendInt(output, len(c.Request().Body()))
		},
		TagBytesSent: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			if c.Response().Header.ContentLength() < 0 {
				return appendInt(output, 0)
			}
			return appendInt(output, len(c.Response().Body()))
		},
		TagRoute: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Route().Path)
		},
		TagResBody: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.Write(c.Response().Body())
		},
		TagReqHeaders: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			reqHeaders := make([]string, 0)
			for k, v := range c.GetReqHeaders() {
				reqHeaders = append(reqHeaders, k+"="+v)
			}
			return output.Write([]byte(strings.Join(reqHeaders, "&")))
		},
		TagQueryStringParams: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Request().URI().QueryArgs().String())
		},

		TagBlack: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Black)
		},
		TagRed: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Red)
		},
		TagGreen: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Green)
		},
		TagYellow: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Yellow)
		},
		TagBlue: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Blue)
		},
		TagMagenta: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Magenta)
		},
		TagCyan: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Cyan)
		},
		TagWhite: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.White)
		},
		TagReset: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Reset)
		},
		TagError: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			if data.ChainErr != nil {
				return output.WriteString(data.ChainErr.Error())
			}
			return output.WriteString("-")
		},
		TagReqHeader: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Get(extraParam))
		},
		TagHeader: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Get(extraParam))
		},
		TagRespHeader: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.GetRespHeader(extraParam))
		},
		TagQuery: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Query(extraParam))
		},
		TagForm: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.FormValue(extraParam))
		},
		TagCookie: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(c.Cookies(extraParam))
		},
		TagLocals: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			switch v := c.Locals(extraParam).(type) {
			case []byte:
				return output.Write(v)
			case string:
				return output.WriteString(v)
			case nil:
				return 0, nil
			default:
				return output.WriteString(fmt.Sprintf("%v", v))
			}
		},
		TagStatus: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			if cfg.enableColors {
				colors := c.App().Config().ColorScheme
				return output.WriteString(fmt.Sprintf("%s %3d %s", statusColor(c.Response().StatusCode(), colors), c.Response().StatusCode(), colors.Reset))
			}
			return appendInt(output, c.Response().StatusCode())
		},
		TagMethod: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			if cfg.enableColors {
				colors := c.App().Config().ColorScheme
				return output.WriteString(fmt.Sprintf("%s %-7s %s", methodColor(c.Method(), colors), c.Method(), colors.Reset))
			}
			return output.WriteString(c.Method())
		},
		TagPid: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(data.Pid)
		},
		TagLatency: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			latency := data.Stop.Sub(data.Start)
			return output.WriteString(fmt.Sprintf("%7v", latency))
		},
		TagTime: func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
			return output.WriteString(data.Timestamp.Load().(string))
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
