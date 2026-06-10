package logger

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

// Logger variables
const (
	TagPid               = "pid"
	TagTime              = "time"
	TagReferer           = "referer"
	TagProtocol          = "protocol"
	TagScheme            = "scheme"
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
	TagReqHeader         = "reqHeader:"
	TagRespHeader        = "respHeader:"
	TagLocals            = "locals:"
	TagQuery             = "query:"
	TagForm              = "form:"
	TagCookie            = "cookie:"
	TagBlack             = "black"
	TagRed               = "red"
	TagGreen             = "green"
	TagYellow            = "yellow"
	TagBlue              = "blue"
	TagMagenta           = "magenta"
	TagCyan              = "cyan"
	TagWhite             = "white"
	TagReset             = "reset"
)

// ErrTagInvalid is returned by RegisterTag and panicked from MustRegisterTag
// when the supplied tag name or renderer is empty.
var ErrTagInvalid = errors.New("logger: tag name and function are required")

// hasControlByte reports whether s contains an ASCII control byte (< 0x20 or
// 0x7f). These bytes — most importantly CR (\r) and LF (\n) — can appear in
// request-derived values such as the percent-decoded path or query and would
// otherwise let an attacker forge log records ("log injection").
func hasControlByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

// writeSanitized writes s to output, escaping ASCII control bytes so that
// attacker-controlled request values cannot inject newlines (or other control
// characters) into the log output. Values without control bytes — the common
// case — are written verbatim with no extra allocation.
func writeSanitized(output Buffer, s string) (int, error) {
	if !hasControlByte(s) {
		return output.WriteString(s)
	}

	written := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\n':
			n, err := output.WriteString(`\n`)
			written += n
			if err != nil {
				return written, err
			}
		case c == '\r':
			n, err := output.WriteString(`\r`)
			written += n
			if err != nil {
				return written, err
			}
		case c == '\t':
			n, err := output.WriteString(`\t`)
			written += n
			if err != nil {
				return written, err
			}
		case c < 0x20 || c == 0x7f:
			n, err := fmt.Fprintf(output, `\x%02x`, c)
			written += n
			if err != nil {
				return written, err
			}
		default:
			if err := output.WriteByte(c); err != nil {
				return written, err
			}
			written++
		}
	}
	return written, nil
}

var registeredTags = struct {
	m map[string]LogFunc
	sync.RWMutex
}{
	m: make(map[string]LogFunc),
}

// RegisterTag registers a global logger middleware tag.
// Registered tags are available to logger middleware instances created after
// registration and can be overridden per instance with Config.CustomTags.
// Re-registering a tag replaces the existing tag function.
func RegisterTag(tag string, fn LogFunc) error {
	if tag == "" || fn == nil {
		return ErrTagInvalid
	}

	registeredTags.Lock()
	defer registeredTags.Unlock()

	registeredTags.m[tag] = fn
	return nil
}

// MustRegisterTag registers a global logger middleware tag and panics on failure.
func MustRegisterTag(tag string, fn LogFunc) {
	if err := RegisterTag(tag, fn); err != nil {
		panic(err)
	}
}

// createTagMap function merged the default with the custom tags
func createTagMap(cfg *Config) map[string]LogFunc {
	// Set default tags
	tagFunctions := map[string]LogFunc{
		TagReferer: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return writeSanitized(output, c.Get(fiber.HeaderReferer))
		},
		TagProtocol: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.Protocol())
		},
		TagScheme: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.Scheme())
		},
		TagPort: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.Port())
		},
		TagIP: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.IP())
		},
		TagIPs: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return writeSanitized(output, c.Get(fiber.HeaderXForwardedFor))
		},
		TagHost: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return writeSanitized(output, c.Hostname())
		},
		TagPath: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return writeSanitized(output, c.Path())
		},
		TagURL: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return writeSanitized(output, c.OriginalURL())
		},
		TagUA: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return writeSanitized(output, c.Get(fiber.HeaderUserAgent))
		},
		TagBody: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.Write(c.Body())
		},
		TagBytesReceived: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return appendInt(output, c.Request().Header.ContentLength())
		},
		TagBytesSent: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return appendInt(output, c.Response().Header.ContentLength())
		},
		TagRoute: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.Route().Path)
		},
		TagResBody: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.Write(c.Response().Body())
		},
		TagReqHeaders: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			out := make(map[string][]string)
			if err := c.Bind().Header(&out); err != nil {
				return 0, err
			}

			reqHeaders := make([]string, 0, len(out))
			for k, v := range out {
				reqHeaders = append(reqHeaders, k+"="+strings.Join(v, ","))
			}
			return writeSanitized(output, strings.Join(reqHeaders, "&"))
		},
		TagQueryStringParams: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return writeSanitized(output, c.Request().URI().QueryArgs().String())
		},

		TagBlack: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Black)
		},
		TagRed: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Red)
		},
		TagGreen: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Green)
		},
		TagYellow: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Yellow)
		},
		TagBlue: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Blue)
		},
		TagMagenta: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Magenta)
		},
		TagCyan: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Cyan)
		},
		TagWhite: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.White)
		},
		TagReset: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString(c.App().Config().ColorScheme.Reset)
		},
		TagError: func(output Buffer, c fiber.Ctx, data *Data, _ string) (int, error) {
			if data.ChainErr != nil {
				if cfg.areColorsEnabled {
					colors := c.App().Config().ColorScheme
					n, err := output.WriteString(colors.Red)
					if err != nil {
						return n, err
					}
					m, err := writeSanitized(output, data.ChainErr.Error())
					n += m
					if err != nil {
						return n, err
					}
					m, err = output.WriteString(colors.Reset)
					return n + m, err
				}
				return writeSanitized(output, data.ChainErr.Error())
			}
			return output.WriteString("-")
		},
		TagReqHeader: func(output Buffer, c fiber.Ctx, _ *Data, extraParam string) (int, error) {
			return writeSanitized(output, c.Get(extraParam))
		},
		TagRespHeader: func(output Buffer, c fiber.Ctx, _ *Data, extraParam string) (int, error) {
			return writeSanitized(output, c.GetRespHeader(extraParam))
		},
		TagQuery: func(output Buffer, c fiber.Ctx, _ *Data, extraParam string) (int, error) {
			return writeSanitized(output, fiber.Query[string](c, extraParam))
		},
		TagForm: func(output Buffer, c fiber.Ctx, _ *Data, extraParam string) (int, error) {
			return writeSanitized(output, c.FormValue(extraParam))
		},
		TagCookie: func(output Buffer, c fiber.Ctx, _ *Data, extraParam string) (int, error) {
			return writeSanitized(output, c.Cookies(extraParam))
		},
		TagLocals: func(output Buffer, c fiber.Ctx, _ *Data, extraParam string) (int, error) {
			switch v := c.Locals(extraParam).(type) {
			case []byte:
				return output.Write(v)
			case string:
				return output.WriteString(v)
			case nil:
				return 0, nil
			default:
				return fmt.Fprintf(output, "%v", v)
			}
		},
		TagStatus: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			if cfg.areColorsEnabled {
				colors := c.App().Config().ColorScheme
				return fmt.Fprintf(output, "%s%3d%s", statusColor(c.Response().StatusCode(), &colors), c.Response().StatusCode(), colors.Reset)
			}
			return appendInt(output, c.Response().StatusCode())
		},
		TagMethod: func(output Buffer, c fiber.Ctx, _ *Data, _ string) (int, error) {
			if cfg.areColorsEnabled {
				colors := c.App().Config().ColorScheme
				return fmt.Fprintf(output, "%s%s%s", methodColor(c.Method(), &colors), c.Method(), colors.Reset)
			}
			return output.WriteString(c.Method())
		},
		TagPid: func(output Buffer, _ fiber.Ctx, data *Data, _ string) (int, error) {
			return output.WriteString(data.Pid)
		},
		TagLatency: func(output Buffer, _ fiber.Ctx, data *Data, _ string) (int, error) {
			latency := data.Stop.Sub(data.Start)
			return fmt.Fprintf(output, "%13v", latency)
		},
		TagTime: func(output Buffer, _ fiber.Ctx, data *Data, _ string) (int, error) {
			return output.WriteString(data.Timestamp)
		},
	}
	registeredTags.RLock()
	maps.Copy(tagFunctions, registeredTags.m)
	registeredTags.RUnlock()

	// merge with custom tags from user
	maps.Copy(tagFunctions, cfg.CustomTags)

	return tagFunctions
}
