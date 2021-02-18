package fiber

import (
	"bytes"
	"crypto/tls"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp/fasthttputil"
)

func Test_Client_Invalid_URL(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		return c.SendString(c.Hostname())
	})

	go app.Listener(ln) //nolint:errcheck

	a := Get("http://example.com\r\n\r\nGET /\r\n\r\n")

	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

	_, body, errs := a.String()

	utils.AssertEqual(t, "", body)
	utils.AssertEqual(t, 1, len(errs))
	utils.AssertEqual(t, "missing required Host header in request", errs[0].Error())
}

func Test_Client_Unsupported_Protocol(t *testing.T) {
	t.Parallel()

	a := Get("ftp://example.com")

	_, body, errs := a.String()

	utils.AssertEqual(t, "", body)
	utils.AssertEqual(t, 1, len(errs))
	utils.AssertEqual(t, `unsupported protocol "ftp". http and https are supported`,
		errs[0].Error())
}

func Test_Client_Get(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		return c.SendString(c.Hostname())
	})

	go app.Listener(ln) //nolint:errcheck

	for i := 0; i < 5; i++ {
		a := Get("http://example.com")

		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		utils.AssertEqual(t, StatusOK, code)
		utils.AssertEqual(t, "example.com", body)
		utils.AssertEqual(t, 0, len(errs))
	}
}

func Test_Client_Post(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Post("/", func(c *Ctx) error {
		return c.SendString(c.Hostname())
	})

	go app.Listener(ln) //nolint:errcheck

	for i := 0; i < 5; i++ {
		args := AcquireArgs()

		args.Set("foo", "bar")

		a := Post("http://example.com").
			Form(args)

		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		utils.AssertEqual(t, StatusOK, code)
		utils.AssertEqual(t, "example.com", body)
		utils.AssertEqual(t, 0, len(errs))

		ReleaseArgs(args)
	}
}

func Test_Client_UserAgent(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		return c.Send(c.Request().Header.UserAgent())
	})

	go app.Listener(ln) //nolint:errcheck

	t.Run("default", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			a := Get("http://example.com")

			a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

			code, body, errs := a.String()

			utils.AssertEqual(t, StatusOK, code)
			utils.AssertEqual(t, defaultUserAgent, body)
			utils.AssertEqual(t, 0, len(errs))
		}
	})

	t.Run("custom", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			c := AcquireClient()
			c.UserAgent = "ua"

			a := c.Get("http://example.com")

			a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

			code, body, errs := a.String()

			utils.AssertEqual(t, StatusOK, code)
			utils.AssertEqual(t, "ua", body)
			utils.AssertEqual(t, 0, len(errs))
			ReleaseClient(c)
		}
	})
}

func Test_Client_Agent_Specific_Host(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		return c.SendString(c.Hostname())
	})

	go app.Listener(ln) //nolint:errcheck

	a := Get("http://1.1.1.1:8080").
		Host("example.com")

	utils.AssertEqual(t, "1.1.1.1:8080", a.HostClient.Addr)

	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

	code, body, errs := a.String()

	utils.AssertEqual(t, StatusOK, code)
	utils.AssertEqual(t, "example.com", body)
	utils.AssertEqual(t, 0, len(errs))
}

func Test_Client_Agent_Headers(t *testing.T) {
	handler := func(c *Ctx) error {
		c.Request().Header.VisitAll(func(key, value []byte) {
			if k := string(key); k == "K1" || k == "K2" {
				_, _ = c.Write(key)
				_, _ = c.Write(value)
			}
		})
		return nil
	}

	wrapAgent := func(a *Agent) {
		a.Set("k1", "v1").
			Add("k1", "v11").
			Set("k2", "v2")
	}

	testAgent(t, handler, wrapAgent, "K1v1K1v11K2v2")
}

func Test_Client_Agent_UserAgent(t *testing.T) {
	handler := func(c *Ctx) error {
		return c.Send(c.Request().Header.UserAgent())
	}

	wrapAgent := func(a *Agent) {
		a.UserAgent("ua")
	}

	testAgent(t, handler, wrapAgent, "ua")
}

func Test_Client_Agent_Connection_Close(t *testing.T) {
	handler := func(c *Ctx) error {
		if c.Request().Header.ConnectionClose() {
			return c.SendString("close")
		}
		return c.SendString("not close")
	}

	wrapAgent := func(a *Agent) {
		a.ConnectionClose()
	}

	testAgent(t, handler, wrapAgent, "close")
}

func Test_Client_Agent_Referer(t *testing.T) {
	handler := func(c *Ctx) error {
		return c.Send(c.Request().Header.Referer())
	}

	wrapAgent := func(a *Agent) {
		a.Referer("http://referer.com")
	}

	testAgent(t, handler, wrapAgent, "http://referer.com")
}

func Test_Client_Agent_QueryString(t *testing.T) {
	handler := func(c *Ctx) error {
		return c.Send(c.Request().URI().QueryString())
	}

	wrapAgent := func(a *Agent) {
		a.QueryString("foo=bar&bar=baz")
	}

	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
}

func Test_Client_Agent_Cookie(t *testing.T) {
	handler := func(c *Ctx) error {
		return c.SendString(
			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3") + c.Cookies("k4"))
	}

	wrapAgent := func(a *Agent) {
		a.Cookie("k1", "v1").
			Cookie("k2", "v2").
			Cookies("k3", "v3", "k4", "v4")
	}

	testAgent(t, handler, wrapAgent, "v1v2v3v4")
}

func Test_Client_Agent_ContentType(t *testing.T) {
	handler := func(c *Ctx) error {
		return c.Send(c.Request().Header.ContentType())
	}

	wrapAgent := func(a *Agent) {
		a.ContentType("custom-type")
	}

	testAgent(t, handler, wrapAgent, "custom-type")
}

func Test_Client_Agent_BodyStream(t *testing.T) {
	handler := func(c *Ctx) error {
		return c.Send(c.Request().Body())
	}

	wrapAgent := func(a *Agent) {
		a.BodyStream(strings.NewReader("body stream"), -1)
	}

	testAgent(t, handler, wrapAgent, "body stream")
}

func Test_Client_Agent_Form(t *testing.T) {
	handler := func(c *Ctx) error {
		utils.AssertEqual(t, MIMEApplicationForm, string(c.Request().Header.ContentType()))

		return c.Send(c.Request().Body())
	}

	args := AcquireArgs()

	args.Set("a", "b")

	wrapAgent := func(a *Agent) {
		a.Form(args)
	}

	testAgent(t, handler, wrapAgent, "a=b")

	ReleaseArgs(args)
}

type jsonData struct {
	F string `json:"f"`
}

func Test_Client_Agent_Json(t *testing.T) {
	handler := func(c *Ctx) error {
		utils.AssertEqual(t, MIMEApplicationJSON, string(c.Request().Header.ContentType()))

		return c.Send(c.Request().Body())
	}

	wrapAgent := func(a *Agent) {
		a.Json(jsonData{F: "f"})
	}

	testAgent(t, handler, wrapAgent, `{"f":"f"}`)
}

func Test_Client_Agent_Json_Error(t *testing.T) {
	a := Get("http://example.com").
		Json(complex(1, 1))

	_, body, errs := a.String()

	utils.AssertEqual(t, "", body)
	utils.AssertEqual(t, 1, len(errs))
	utils.AssertEqual(t, "json: unsupported type: complex128", errs[0].Error())
}

func Test_Client_Debug(t *testing.T) {
	handler := func(c *Ctx) error {
		return c.SendString("debug")
	}

	var output bytes.Buffer

	wrapAgent := func(a *Agent) {
		a.Debug(&output)
	}

	testAgent(t, handler, wrapAgent, "debug", 1)

	str := output.String()

	utils.AssertEqual(t, true, strings.Contains(str, "Connected to example.com(pipe)"))
	utils.AssertEqual(t, true, strings.Contains(str, "GET / HTTP/1.1"))
	utils.AssertEqual(t, true, strings.Contains(str, "User-Agent: fiber"))
	utils.AssertEqual(t, true, strings.Contains(str, "Host: example.com\r\n\r\n"))
	utils.AssertEqual(t, true, strings.Contains(str, "HTTP/1.1 200 OK"))
	utils.AssertEqual(t, true, strings.Contains(str, "Content-Type: text/plain; charset=utf-8\r\nContent-Length: 5\r\n\r\ndebug"))
}

func testAgent(t *testing.T, handler Handler, wrapAgent func(agent *Agent), excepted string, count ...int) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", handler)

	go app.Listener(ln) //nolint:errcheck

	c := 1
	if len(count) > 0 {
		c = count[0]
	}

	for i := 0; i < c; i++ {
		a := Get("http://example.com")

		wrapAgent(a)

		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		utils.AssertEqual(t, StatusOK, code)
		utils.AssertEqual(t, excepted, body)
		utils.AssertEqual(t, 0, len(errs))
	}
}

func Test_Client_Agent_Timeout(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		time.Sleep(time.Millisecond * 200)
		return c.SendString("timeout")
	})

	go app.Listener(ln) //nolint:errcheck

	a := Get("http://example.com").
		Timeout(time.Millisecond * 100)

	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

	_, body, errs := a.String()

	utils.AssertEqual(t, "", body)
	utils.AssertEqual(t, 1, len(errs))
	utils.AssertEqual(t, "timeout", errs[0].Error())
}

func Test_Client_Agent_MaxRedirectsCount(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		if c.Request().URI().QueryArgs().Has("foo") {
			return c.Redirect("/foo")
		}
		return c.Redirect("/")
	})
	app.Get("/foo", func(c *Ctx) error {
		return c.SendString("redirect")
	})

	go app.Listener(ln) //nolint:errcheck

	t.Run("success", func(t *testing.T) {
		a := Get("http://example.com?foo").
			MaxRedirectsCount(1)

		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String()

		utils.AssertEqual(t, 200, code)
		utils.AssertEqual(t, "redirect", body)
		utils.AssertEqual(t, 0, len(errs))
	})

	t.Run("error", func(t *testing.T) {
		a := Get("http://example.com").
			MaxRedirectsCount(1)

		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

		_, body, errs := a.String()

		utils.AssertEqual(t, "", body)
		utils.AssertEqual(t, 1, len(errs))
		utils.AssertEqual(t, "too many redirects detected when doing the request", errs[0].Error())
	})
}

func Test_Client_Agent_Custom(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		return c.SendString("custom")
	})

	go app.Listener(ln) //nolint:errcheck

	for i := 0; i < 5; i++ {
		a := AcquireAgent()
		req := AcquireRequest()
		resp := AcquireResponse()

		req.Header.SetMethod(MethodGet)
		req.SetRequestURI("http://example.com")
		a.Request(req)

		utils.AssertEqual(t, nil, a.Parse())

		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

		code, body, errs := a.String(resp)

		utils.AssertEqual(t, StatusOK, code)
		utils.AssertEqual(t, "custom", body)
		utils.AssertEqual(t, "custom", string(resp.Body()))
		utils.AssertEqual(t, 0, len(errs))

		ReleaseRequest(req)
		ReleaseResponse(resp)
	}
}

func Test_Client_Agent_Reuse(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		return c.SendString("reuse")
	})

	go app.Listener(ln) //nolint:errcheck

	a := Get("http://example.com").
		Reuse()

	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

	code, body, errs := a.String()

	utils.AssertEqual(t, StatusOK, code)
	utils.AssertEqual(t, "reuse", body)
	utils.AssertEqual(t, 0, len(errs))

	code, body, errs = a.String()

	utils.AssertEqual(t, StatusOK, code)
	utils.AssertEqual(t, "reuse", body)
	utils.AssertEqual(t, 0, len(errs))
}

func Test_Client_Agent_Parse(t *testing.T) {
	t.Parallel()

	a := Get("https://example.com:10443")

	utils.AssertEqual(t, nil, a.Parse())
}

func Test_Client_Agent_TLS(t *testing.T) {
	t.Parallel()

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	utils.AssertEqual(t, nil, err)

	config := &tls.Config{
		Certificates: []tls.Certificate{cer},
	}

	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)

	ln = tls.NewListener(ln, config)

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		return c.SendString("tls")
	})

	go app.Listener(ln) //nolint:errcheck

	code, body, errs := Get("https://" + ln.Addr().String()).
		InsecureSkipVerify().
		TLSConfig(config).
		InsecureSkipVerify().
		String()

	utils.AssertEqual(t, 0, len(errs))
	utils.AssertEqual(t, StatusOK, code)
	utils.AssertEqual(t, "tls", body)
}

type data struct {
	Success bool `json:"success"`
}

func Test_Client_Agent_Struct(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New(Config{DisableStartupMessage: true})

	app.Get("/", func(c *Ctx) error {
		return c.JSON(data{true})
	})

	app.Get("/error", func(c *Ctx) error {
		return c.SendString(`{"success"`)
	})

	go app.Listener(ln) //nolint:errcheck

	t.Run("success", func(t *testing.T) {
		a := Get("http://example.com")

		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

		var d data

		code, body, errs := a.Struct(&d)

		utils.AssertEqual(t, StatusOK, code)
		utils.AssertEqual(t, `{"success":true}`, string(body))
		utils.AssertEqual(t, 0, len(errs))
		utils.AssertEqual(t, true, d.Success)
	})

	t.Run("error", func(t *testing.T) {
		a := Get("http://example.com/error")

		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

		var d data

		code, body, errs := a.Struct(&d)

		utils.AssertEqual(t, StatusOK, code)
		utils.AssertEqual(t, `{"success"`, string(body))
		utils.AssertEqual(t, 1, len(errs))
		utils.AssertEqual(t, "json: unexpected end of JSON input after object field key: ", errs[0].Error())
	})
}

func Test_AddMissingPort_TLS(t *testing.T) {
	addr := addMissingPort("example.com", true)
	utils.AssertEqual(t, "example.com:443", addr)
}
