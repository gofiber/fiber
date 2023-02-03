package proxy

import (
	"crypto/tls"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/tlstest"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
)

func createProxyTestServer(t *testing.T, handler fiber.Handler) (*fiber.App, string) {
	t.Helper()

	target := fiber.New(fiber.Config{DisableStartupMessage: true})
	target.Get("/", handler)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)

	go func() {
		utils.AssertEqual(t, nil, target.Listener(ln))
	}()

	time.Sleep(2 * time.Second)
	addr := ln.Addr().String()

	return target, addr
}

// go test -run Test_Proxy_Empty_Host
func Test_Proxy_Empty_Upstream_Servers(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			utils.AssertEqual(t, "Servers cannot be empty", r)
		}
	}()
	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{}}))
}

// go test -run Test_Proxy_Next
func Test_Proxy_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{"127.0.0.1"},
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_Proxy
func Test_Proxy(t *testing.T) {
	t.Parallel()

	target, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := target.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), 2000)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(Balancer(Config{Servers: []string{addr}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Host = addr
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
}

// go test -run Test_Proxy_Balancer_WithTlsConfig
func Test_Proxy_Balancer_WithTlsConfig(t *testing.T) {
	t.Parallel()

	serverTLSConf, _, err := tlstest.GetTLSConfigs()
	utils.AssertEqual(t, nil, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/tlsbalaner", func(c *fiber.Ctx) error {
		return c.SendString("tls balancer")
	})

	addr := ln.Addr().String()
	clientTLSConf := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // We're in a test func, so this is fine

	// disable certificate verification in Balancer
	app.Use(Balancer(Config{
		Servers:   []string{addr},
		TlsConfig: clientTLSConf,
	}))

	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

	code, body, errs := fiber.Get("https://" + addr + "/tlsbalaner").TLSConfig(clientTLSConf).String()

	utils.AssertEqual(t, 0, len(errs))
	utils.AssertEqual(t, fiber.StatusOK, code)
	utils.AssertEqual(t, "tls balancer", body)
}

// go test -run Test_Proxy_Forward_WithTlsConfig_To_Http
func Test_Proxy_Forward_WithTlsConfig_To_Http(t *testing.T) {
	t.Parallel()

	_, targetAddr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		return c.SendString("hello from target")
	})

	proxyServerTLSConf, _, err := tlstest.GetTLSConfigs()
	utils.AssertEqual(t, nil, err)

	proxyServerLn, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)

	proxyServerLn = tls.NewListener(proxyServerLn, proxyServerTLSConf)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	proxyAddr := proxyServerLn.Addr().String()

	app.Use(Forward("http://" + targetAddr))

	go func() { utils.AssertEqual(t, nil, app.Listener(proxyServerLn)) }()

	code, body, errs := fiber.Get("https://" + proxyAddr).
		InsecureSkipVerify().
		Timeout(5 * time.Second).
		String()

	utils.AssertEqual(t, 0, len(errs))
	utils.AssertEqual(t, fiber.StatusOK, code)
	utils.AssertEqual(t, "hello from target", body)
}

// go test -run Test_Proxy_Forward
func Test_Proxy_Forward(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	_, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		return c.SendString("forwarded")
	})

	app.Use(Forward("http://" + addr))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "forwarded", string(b))
}

// go test -run Test_Proxy_Forward_WithTlsConfig
func Test_Proxy_Forward_WithTlsConfig(t *testing.T) {
	t.Parallel()

	serverTLSConf, _, err := tlstest.GetTLSConfigs()
	utils.AssertEqual(t, nil, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/tlsfwd", func(c *fiber.Ctx) error {
		return c.SendString("tls forward")
	})

	addr := ln.Addr().String()
	clientTLSConf := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // We're in a test func, so this is fine

	// disable certificate verification
	WithTlsConfig(clientTLSConf)
	app.Use(Forward("https://" + addr + "/tlsfwd"))

	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

	code, body, errs := fiber.Get("https://" + addr).TLSConfig(clientTLSConf).String()

	utils.AssertEqual(t, 0, len(errs))
	utils.AssertEqual(t, fiber.StatusOK, code)
	utils.AssertEqual(t, "tls forward", body)
}

// go test -run Test_Proxy_Modify_Response
func Test_Proxy_Modify_Response(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		return c.Status(500).SendString("not modified")
	})

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		ModifyResponse: func(c *fiber.Ctx) error {
			c.Response().SetStatusCode(fiber.StatusOK)
			return c.SendString("modified response")
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "modified response", string(b))
}

// go test -run Test_Proxy_Modify_Request
func Test_Proxy_Modify_Request(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		b := c.Request().Body()
		return c.SendString(string(b))
	})

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		ModifyRequest: func(c *fiber.Ctx) error {
			c.Request().SetBody([]byte("modified request"))
			return nil
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "modified request", string(b))
}

// go test -run Test_Proxy_Timeout_Slow_Server
func Test_Proxy_Timeout_Slow_Server(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		time.Sleep(2 * time.Second)
		return c.SendString("fiber is awesome")
	})

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		Timeout: 3 * time.Second,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), 5000)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "fiber is awesome", string(b))
}

// go test -run Test_Proxy_With_Timeout
func Test_Proxy_With_Timeout(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		time.Sleep(1 * time.Second)
		return c.SendString("fiber is awesome")
	})

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		Timeout: 100 * time.Millisecond,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), 2000)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "timeout", string(b))
}

// go test -run Test_Proxy_Buffer_Size_Response
func Test_Proxy_Buffer_Size_Response(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		long := strings.Join(make([]string, 5000), "-")
		c.Set("Very-Long-Header", long)
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)

	app = fiber.New()
	app.Use(Balancer(Config{
		Servers:        []string{addr},
		ReadBufferSize: 1024 * 8,
	}))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
}

// go test -race -run Test_Proxy_Do_RestoreOriginalURL
func Test_Proxy_Do_RestoreOriginalURL(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/proxy", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		originalURL := utils.CopyString(c.OriginalURL())
		if err := Do(c, "/proxy"); err != nil {
			return err
		}
		utils.AssertEqual(t, originalURL, c.OriginalURL())
		return c.SendString("ok")
	})
	_, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", nil))
	// This test requires multiple requests due to zero allocation used in fiber
	_, err2 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", nil))

	utils.AssertEqual(t, nil, err1)
	utils.AssertEqual(t, nil, err2)
}

// go test -race -run Test_Proxy_Do_HTTP_Prefix_URL
func Test_Proxy_Do_HTTP_Prefix_URL(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		return c.SendString("hello world")
	})

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/*", func(c *fiber.Ctx) error {
		path := c.OriginalURL()
		url := strings.TrimPrefix(path, "/")

		utils.AssertEqual(t, "http://"+addr, url)
		if err := Do(c, url); err != nil {
			return err
		}
		c.Response().Header.Del(fiber.HeaderServer)
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/http://"+addr, nil))
	utils.AssertEqual(t, nil, err)
	s, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "hello world", string(s))
}

// go test -race -run Test_Proxy_Forward_Global_Client
func Test_Proxy_Forward_Global_Client(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)
	WithClient(&fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test_global_client", func(c *fiber.Ctx) error {
		return c.SendString("test_global_client")
	})

	addr := ln.Addr().String()
	app.Use(Forward("http://" + addr + "/test_global_client"))
	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

	code, body, errs := fiber.Get("http://" + addr).String()
	utils.AssertEqual(t, 0, len(errs))
	utils.AssertEqual(t, fiber.StatusOK, code)
	utils.AssertEqual(t, "test_global_client", body)
}

// go test -race -run Test_Proxy_Forward_Local_Client
func Test_Proxy_Forward_Local_Client(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test_local_client", func(c *fiber.Ctx) error {
		return c.SendString("test_local_client")
	})

	addr := ln.Addr().String()
	app.Use(Forward("http://"+addr+"/test_local_client", &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,

		Dial: fasthttp.Dial,
	}))
	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

	code, body, errs := fiber.Get("http://" + addr).String()
	utils.AssertEqual(t, 0, len(errs))
	utils.AssertEqual(t, fiber.StatusOK, code)
	utils.AssertEqual(t, "test_local_client", body)
}

// go test -run Test_ProxyBalancer_Custom_Client
func Test_ProxyBalancer_Custom_Client(t *testing.T) {
	t.Parallel()

	target, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := target.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), 2000)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(Balancer(Config{Client: &fasthttp.LBClient{
		Clients: []fasthttp.BalancingClient{
			&fasthttp.HostClient{
				NoDefaultUserAgentHeader: true,
				DisablePathNormalizing:   true,
				Addr:                     addr,
			},
		},
		Timeout: time.Second,
	}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Host = addr
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
}

// go test -run Test_Proxy_Domain_Forward_Local
func Test_Proxy_Domain_Forward_Local(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	// target server
	ln1, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	utils.AssertEqual(t, nil, err)
	app1 := fiber.New(fiber.Config{DisableStartupMessage: true})

	app1.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("test_local_client:" + c.Query("query_test"))
	})

	proxyAddr := ln.Addr().String()
	targetAddr := ln1.Addr().String()
	localDomain := strings.Replace(proxyAddr, "127.0.0.1", "localhost", 1)
	app.Use(DomainForward(localDomain, "http://"+targetAddr, &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,

		Dial: fasthttp.Dial,
	}))

	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()
	go func() { utils.AssertEqual(t, nil, app1.Listener(ln1)) }()

	code, body, errs := fiber.Get("http://" + localDomain + "/test?query_test=true").String()
	utils.AssertEqual(t, 0, len(errs))
	utils.AssertEqual(t, fiber.StatusOK, code)
	utils.AssertEqual(t, "test_local_client:true", body)
}

// go test -run Test_Proxy_Balancer_Forward_Local
func Test_Proxy_Balancer_Forward_Local(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	_, addr := createProxyTestServer(t, func(c *fiber.Ctx) error {
		return c.SendString("forwarded")
	})

	app.Use(BalancerForward([]string{addr}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, string(b), "forwarded")
}
