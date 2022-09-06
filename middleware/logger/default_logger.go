package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasttemplate"
)

// LoggerData is a struct to define some variables to use in custom logger function.
type LoggerData struct {
	mu            sync.Mutex
	Pid           string
	ErrPaddingStr string
	ChainErr      error
	Start         time.Time
	Stop          time.Time
	Timestamp     atomic.Value
}

var tmpl *fasttemplate.Template

// default logger for fiber
func defaultLogger(c fiber.Ctx, data *LoggerData, cfg Config) error {
	// Alias colors
	colors := c.App().Config().ColorScheme

	// Get new buffer
	buf := bytebufferpool.Get()

	// Default output when no custom Format or io.Writer is given
	if cfg.enableColors && cfg.Format == defaultFormat {
		// Format error if exist
		formatErr := ""
		if data.ChainErr != nil {
			formatErr = colors.Red + " | " + data.ChainErr.Error() + colors.Reset
		}

		// Format log to buffer
		_, _ = buf.WriteString(fmt.Sprintf("%s |%s %3d %s| %7v | %15s |%s %-7s %s| %-"+data.ErrPaddingStr+"s %s\n",
			data.Timestamp.Load().(string),
			statusColor(c.Response().StatusCode(), colors), c.Response().StatusCode(), colors.Reset,
			data.Stop.Sub(data.Start).Round(time.Millisecond),
			c.IP(),
			methodColor(c.Method(), colors), c.Method(), colors.Reset,
			c.Path(),
			formatErr,
		))

		// Write buffer to output
		_, _ = cfg.Output.Write(buf.Bytes())

		// Put buffer back to pool
		bytebufferpool.Put(buf)

		// End chain
		return nil
	}

	// Loop over template tags to replace it with the correct value
	_, err := tmpl.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
		switch tag {
		case TagTime:
			return buf.WriteString(data.Timestamp.Load().(string))
		case TagReferer:
			return buf.WriteString(c.Get(fiber.HeaderReferer))
		case TagProtocol:
			return buf.WriteString(c.Protocol())
		case TagScheme:
			return buf.WriteString(c.Scheme())
		case TagPid:
			return buf.WriteString(data.Pid)
		case TagPort:
			return buf.WriteString(c.Port())
		case TagIP:
			return buf.WriteString(c.IP())
		case TagIPs:
			return buf.WriteString(c.Get(fiber.HeaderXForwardedFor))
		case TagHost:
			return buf.WriteString(c.Host())
		case TagPath:
			return buf.WriteString(c.Path())
		case TagURL:
			return buf.WriteString(c.OriginalURL())
		case TagUA:
			return buf.WriteString(c.Get(fiber.HeaderUserAgent))
		case TagLatency:
			return buf.WriteString(fmt.Sprintf("%7v", data.Stop.Sub(data.Start).Round(time.Millisecond)))
		case TagBody:
			return buf.Write(c.Body())
		case TagBytesReceived:
			return appendInt(buf, len(c.Request().Body()))
		case TagBytesSent:
			return appendInt(buf, len(c.Response().Body()))
		case TagRoute:
			return buf.WriteString(c.Route().Path)
		case TagStatus:
			if cfg.enableColors {
				return buf.WriteString(fmt.Sprintf("%s %3d %s", statusColor(c.Response().StatusCode(), colors), c.Response().StatusCode(), colors.Reset))
			}
			return appendInt(buf, c.Response().StatusCode())
		case TagResBody:
			return buf.Write(c.Response().Body())
		case TagReqHeaders:
			out := make(map[string]string, 0)
			if err := c.Bind().Header(&out); err != nil {
				return 0, err
			}

			reqHeaders := make([]string, 0)
			for k, v := range out {
				reqHeaders = append(reqHeaders, k+"="+v)
			}
			return buf.Write([]byte(strings.Join(reqHeaders, "&")))
		case TagQueryStringParams:
			return buf.WriteString(c.Request().URI().QueryArgs().String())
		case TagMethod:
			if cfg.enableColors {
				return buf.WriteString(fmt.Sprintf("%s %-7s %s", methodColor(c.Method(), colors), c.Method(), colors.Reset))
			}
			return buf.WriteString(c.Method())
		case TagBlack:
			return buf.WriteString(colors.Black)
		case TagRed:
			return buf.WriteString(colors.Red)
		case TagGreen:
			return buf.WriteString(colors.Green)
		case TagYellow:
			return buf.WriteString(colors.Yellow)
		case TagBlue:
			return buf.WriteString(colors.Blue)
		case TagMagenta:
			return buf.WriteString(colors.Magenta)
		case TagCyan:
			return buf.WriteString(colors.Cyan)
		case TagWhite:
			return buf.WriteString(colors.White)
		case TagReset:
			return buf.WriteString(colors.Reset)
		case TagError:
			if data.ChainErr != nil {
				return buf.WriteString(data.ChainErr.Error())
			}
			return buf.WriteString("-")
		default:
			// Check if we have a value tag i.e.: "reqHeader:x-key"
			switch {
			case strings.HasPrefix(tag, TagReqHeader):
				return buf.WriteString(c.Get(tag[10:]))
			case strings.HasPrefix(tag, TagRespHeader):
				return buf.WriteString(c.GetRespHeader(tag[11:]))
			case strings.HasPrefix(tag, TagQuery):
				return buf.WriteString(c.Query(tag[6:]))
			case strings.HasPrefix(tag, TagForm):
				return buf.WriteString(c.FormValue(tag[5:]))
			case strings.HasPrefix(tag, TagCookie):
				return buf.WriteString(c.Cookies(tag[7:]))
			case strings.HasPrefix(tag, TagLocals):
				switch v := c.Locals(tag[7:]).(type) {
				case []byte:
					return buf.Write(v)
				case string:
					return buf.WriteString(v)
				case nil:
					return 0, nil
				default:
					return buf.WriteString(fmt.Sprintf("%v", v))
				}
			}
		}
		return 0, nil
	})
	// Also write errors to the buffer
	if err != nil {
		_, _ = buf.WriteString(err.Error())
	}
	data.mu.Lock()
	// Write buffer to output
	if _, err := cfg.Output.Write(buf.Bytes()); err != nil {
		// Write error to output
		if _, err := cfg.Output.Write([]byte(err.Error())); err != nil {
			// There is something wrong with the given io.Writer
			fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err)
		}
	}
	data.mu.Unlock()
	// Put buffer back to pool
	bytebufferpool.Put(buf)

	return nil
}

// run something before returning the handler
func beforeHandlerFunc(cfg Config) {
	// Create template parser
	tmpl = fasttemplate.New(cfg.Format, "${", "}")

	// If colors are enabled, check terminal compatibility
	if cfg.enableColors {
		cfg.Output = colorable.NewColorableStdout()
		if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
			cfg.Output = colorable.NewNonColorable(os.Stdout)
		}
	}
}

func appendInt(buf *bytebufferpool.ByteBuffer, v int) (int, error) {
	old := len(buf.B)
	buf.B = fasthttp.AppendUint(buf.B, v)
	return len(buf.B) - old, nil
}
