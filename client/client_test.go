package client

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils"
	"github.com/stretchr/testify/require"
)

func Test_Client_Invalid_URL(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	_, err := AcquireClient().
		R().
		SetDial(dial).
		Get("http://example.com\r\n\r\nGET /\r\n\r\n")

	require.ErrorIs(t, err, ErrURLForamt)
}

func Test_Client_Unsupported_Protocol(t *testing.T) {
	t.Parallel()

	_, err := AcquireClient().
		R().
		Get("ftp://example.com")

	require.ErrorIs(t, err, ErrURLForamt)
}

func Test_Get(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	t.Run("global get function", func(t *testing.T) {
		resp, err := Get("http://example.com", Config{
			Dial: dial,
		})
		require.NoError(t, err)
		require.Equal(t, "example.com", utils.UnsafeString(resp.RawResponse.Body()))
	})

	t.Run("client get", func(t *testing.T) {
		resp, err := AcquireClient().Get("http://example.com", Config{
			Dial: dial,
		})
		require.NoError(t, err)
		require.Equal(t, "example.com", utils.UnsafeString(resp.RawResponse.Body()))
	})
}

func Test_Head(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Head("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	t.Run("global head function", func(t *testing.T) {
		resp, err := Head("http://example.com", Config{
			Dial: dial,
		})
		require.NoError(t, err)
		require.Equal(t, "", utils.UnsafeString(resp.RawResponse.Body()))
	})

	t.Run("client head", func(t *testing.T) {
		resp, err := AcquireClient().Head("http://example.com", Config{
			Dial: dial,
		})
		require.NoError(t, err)
		require.Equal(t, "", utils.UnsafeString(resp.RawResponse.Body()))
	})
}

func Test_Post(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Post("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).
			SendString(c.FormValue("foo"))
	})

	go start()

	t.Run("global post function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Post("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusCreated, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client post", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Post("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusCreated, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Put(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Put("/", func(c fiber.Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	go start()

	t.Run("global put function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Put("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client put", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Put("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Delete(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Delete("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNoContent).
			SendString("deleted")
	})

	go start()

	t.Run("global delete function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Delete("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})

	t.Run("client delete", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Delete("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})
}

func Test_Options(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Options("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNoContent).SendString("")
	})

	go start()

	t.Run("global options function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Options("http://example.com", Config{
				Dial: dial,
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})

	t.Run("client options", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Options("http://example.com", Config{
				Dial: dial,
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})
}
func Test_Patch(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Patch("/", func(c fiber.Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	go start()

	t.Run("global patch function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Patch("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client patch", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Patch("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Client_UserAgent(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.UserAgent())
	})

	go start()

	t.Run("default", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Get("http://example.com", Config{
				Dial: dial,
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, defaultUserAgent, resp.String())
		}
	})

	t.Run("custom", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			c := AcquireClient().
				SetUserAgent("ua")

			resp, err := c.Get("http://example.com", Config{
				Dial: dial,
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "ua", resp.String())
			ReleaseClient(c)
		}
	})
}

func Test_Client_Headers(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		c.Request().Header.VisitAll(func(key, value []byte) {
			if k := string(key); k == "K1" || k == "K2" {
				_, _ = c.Write(key)
				_, _ = c.Write(value)
			}
		})
		return nil
	}

	wrapAgent := func(c *Client) {
		c.SetHeader("k1", "v1").
			AddHeader("k1", "v2").
			SetHeaders(map[string]string{
				"k2": "v2",
			}).
			AddHeaders(map[string][]string{
				"k2": {"v22"},
			})
	}

	testClient(t, handler, wrapAgent, "K1v1K1v2K2v2K2v22")
}

func Test_Client_Cookie(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		return c.SendString(
			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3"))
	}

	wrapAgent := func(c *Client) {
		c.SetCookie("k1", "v1").
			SetCookies(map[string]string{
				"k2": "v2",
				"k3": "v3",
			})
	}

	testClient(t, handler, wrapAgent, "v1v2v3")
}

func Test_Client_Referer(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.Referer())
	}

	wrapAgent := func(c *Client) {
		c.SetReferer("http://referer.com")
	}

	testClient(t, handler, wrapAgent, "http://referer.com")
}

// func Test_Client_Agent_Host(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString(c.Hostname())
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	a := Get("http://1.1.1.1:8080").
// 		Host("example.com").
// 		HostBytes([]byte("example.com"))

// 	utils.AssertEqual(t, "1.1.1.1:8080", a.HostClient.Addr)

// 	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 	code, body, errs := a.String()

// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "example.com", body)
// 	utils.AssertEqual(t, 0, len(errs))
// }

// func Test_Client_Agent_QueryString(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().URI().QueryString())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.QueryString("foo=bar&bar=baz").
// 			QueryStringBytes([]byte("foo=bar&bar=baz"))
// 	}

// 	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
// }

// func Test_Client_Agent_BasicAuth(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		// Get authorization header
// 		auth := c.Get(fiber.HeaderAuthorization)
// 		// Decode the header contents
// 		raw, err := base64.StdEncoding.DecodeString(auth[6:])
// 		utils.AssertEqual(t, nil, err)

// 		return c.Send(raw)
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.BasicAuth("foo", "bar").
// 			BasicAuthBytes([]byte("foo"), []byte("bar"))
// 	}

// 	testAgent(t, handler, wrapAgent, "foo:bar")
// }

// func Test_Client_Agent_BodyString(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.BodyString("foo=bar&bar=baz")
// 	}

// 	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
// }

// func Test_Client_Agent_Body(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.Body([]byte("foo=bar&bar=baz"))
// 	}

// 	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
// }

// func Test_Client_Agent_BodyStream(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.BodyStream(strings.NewReader("body stream"), -1)
// 	}

// 	testAgent(t, handler, wrapAgent, "body stream")
// }

// func Test_Client_Agent_Custom_Response(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("custom")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	for i := 0; i < 5; i++ {
// 		a := AcquireAgent()
// 		resp := AcquireResponse()

// 		req := a.Request()
// 		req.Header.SetMethod(fiber.MethodGet)
// 		req.SetRequestURI("http://example.com")

// 		utils.AssertEqual(t, nil, a.Parse())

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.SetResponse(resp).
// 			String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, "custom", body)
// 		utils.AssertEqual(t, "custom", string(resp.Body()))
// 		utils.AssertEqual(t, 0, len(errs))

// 		ReleaseResponse(resp)
// 	}
// }

// func Test_Client_Agent_Dest(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("dest")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	t.Run("small dest", func(t *testing.T) {
// 		dest := []byte("de")

// 		a := Get("http://example.com")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.Dest(dest[:0]).String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, "dest", body)
// 		utils.AssertEqual(t, "de", string(dest))
// 		utils.AssertEqual(t, 0, len(errs))
// 	})

// 	t.Run("enough dest", func(t *testing.T) {
// 		dest := []byte("foobar")

// 		a := Get("http://example.com")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.Dest(dest[:0]).String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, "dest", body)
// 		utils.AssertEqual(t, "destar", string(dest))
// 		utils.AssertEqual(t, 0, len(errs))
// 	})
// }

// // readErrorConn is a struct for testing retryIf
// type readErrorConn struct {
// 	net.Conn
// }

// func (r *readErrorConn) Read(p []byte) (int, error) {
// 	return 0, fmt.Errorf("error")
// }

// func (r *readErrorConn) Write(p []byte) (int, error) {
// 	return len(p), nil
// }

// func (r *readErrorConn) Close() error {
// 	return nil
// }

// func (r *readErrorConn) LocalAddr() net.Addr {
// 	return nil
// }

// func (r *readErrorConn) RemoteAddr() net.Addr {
// 	return nil
// }
// func Test_Client_Agent_RetryIf(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	a := Post("http://example.com").
// 		RetryIf(func(req *Request) bool {
// 			return true
// 		})
// 	dialsCount := 0
// 	a.HostClient.Dial = func(addr string) (net.Conn, error) {
// 		dialsCount++
// 		switch dialsCount {
// 		case 1:
// 			return &readErrorConn{}, nil
// 		case 2:
// 			return &readErrorConn{}, nil
// 		case 3:
// 			return &readErrorConn{}, nil
// 		case 4:
// 			return ln.Dial()
// 		default:
// 			t.Fatalf("unexpected number of dials: %d", dialsCount)
// 		}
// 		panic("unreachable")
// 	}

// 	_, _, errs := a.String()
// 	utils.AssertEqual(t, dialsCount, 4)
// 	utils.AssertEqual(t, 0, len(errs))
// }

// func Test_Client_Agent_Json(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, fiber.MIMEApplicationJSON, string(c.Request().Header.ContentType()))

// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.JSON(data{Success: true})
// 	}

// 	testAgent(t, handler, wrapAgent, `{"success":true}`)
// }

// func Test_Client_Agent_Json_Error(t *testing.T) {
// 	a := Get("http://example.com").
// 		JSONEncoder(json.Marshal).
// 		JSON(complex(1, 1))

// 	_, body, errs := a.String()

// 	utils.AssertEqual(t, "", body)
// 	utils.AssertEqual(t, 1, len(errs))
// 	utils.AssertEqual(t, "json: unsupported type: complex128", errs[0].Error())
// }

// func Test_Client_Agent_XML(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, fiber.MIMEApplicationXML, string(c.Request().Header.ContentType()))

// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.XML(data{Success: true})
// 	}

// 	testAgent(t, handler, wrapAgent, "<data><success>true</success></data>")
// }

// func Test_Client_Agent_XML_Error(t *testing.T) {
// 	a := Get("http://example.com").
// 		XML(complex(1, 1))

// 	_, body, errs := a.String()

// 	utils.AssertEqual(t, "", body)
// 	utils.AssertEqual(t, 1, len(errs))
// 	utils.AssertEqual(t, "xml: unsupported type: complex128", errs[0].Error())
// }

// func Test_Client_Agent_Form(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, fiber.MIMEApplicationForm, string(c.Request().Header.ContentType()))

// 		return c.Send(c.Request().Body())
// 	}

// 	args := AcquireArgs()

// 	args.Set("foo", "bar")

// 	wrapAgent := func(a *Agent) {
// 		a.Form(args)
// 	}

// 	testAgent(t, handler, wrapAgent, "foo=bar")

// 	ReleaseArgs(args)
// }

// func Test_Client_Agent_MultipartForm(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Post("/", func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

// 		mf, err := c.MultipartForm()
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, "bar", mf.Value["foo"][0])

// 		return c.Send(c.Request().Body())
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	args := AcquireArgs()

// 	args.Set("foo", "bar")

// 	a := Post("http://example.com").
// 		Boundary("myBoundary").
// 		MultipartForm(args)

// 	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 	code, body, errs := a.String()

// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "--myBoundary\r\nContent-Disposition: form-data; name=\"foo\"\r\n\r\nbar\r\n--myBoundary--\r\n", body)
// 	utils.AssertEqual(t, 0, len(errs))
// 	ReleaseArgs(args)
// }

// func Test_Client_Agent_MultipartForm_Errors(t *testing.T) {
// 	t.Parallel()

// 	a := AcquireAgent()
// 	a.mw = &errorMultipartWriter{}

// 	args := AcquireArgs()
// 	args.Set("foo", "bar")

// 	ff1 := &FormFile{"", "name1", []byte("content"), false}
// 	ff2 := &FormFile{"", "name2", []byte("content"), false}
// 	a.FileData(ff1, ff2).
// 		MultipartForm(args)

// 	utils.AssertEqual(t, 4, len(a.errs))
// 	ReleaseArgs(args)
// }

// func Test_Client_Agent_MultipartForm_SendFiles(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Post("/", func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

// 		fh1, err := c.FormFile("field1")
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, fh1.Filename, "name")
// 		buf := make([]byte, fh1.Size)
// 		f, err := fh1.Open()
// 		utils.AssertEqual(t, nil, err)
// 		defer func() { _ = f.Close() }()
// 		_, err = f.Read(buf)
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, "form file", string(buf))

// 		fh2, err := c.FormFile("index")
// 		utils.AssertEqual(t, nil, err)
// 		checkFormFile(t, fh2, ".github/testdata/index.html")

// 		fh3, err := c.FormFile("file3")
// 		utils.AssertEqual(t, nil, err)
// 		checkFormFile(t, fh3, ".github/testdata/index.tmpl")

// 		return c.SendString("multipart form files")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	for i := 0; i < 5; i++ {
// 		ff := AcquireFormFile()
// 		ff.Fieldname = "field1"
// 		ff.Name = "name"
// 		ff.Content = []byte("form file")

// 		a := Post("http://example.com").
// 			Boundary("myBoundary").
// 			FileData(ff).
// 			SendFiles(".github/testdata/index.html", "index", ".github/testdata/index.tmpl").
// 			MultipartForm(nil)

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, "multipart form files", body)
// 		utils.AssertEqual(t, 0, len(errs))

// 		ReleaseFormFile(ff)
// 	}
// }

// func checkFormFile(t *testing.T, fh *multipart.FileHeader, filename string) {
// 	t.Helper()

// 	basename := filepath.Base(filename)
// 	utils.AssertEqual(t, fh.Filename, basename)

// 	b1, err := os.ReadFile(filename)
// 	utils.AssertEqual(t, nil, err)

// 	b2 := make([]byte, fh.Size)
// 	f, err := fh.Open()
// 	utils.AssertEqual(t, nil, err)
// 	defer func() { _ = f.Close() }()
// 	_, err = f.Read(b2)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, b1, b2)
// }

// func Test_Client_Agent_Multipart_Random_Boundary(t *testing.T) {
// 	t.Parallel()

// 	a := Post("http://example.com").
// 		MultipartForm(nil)

// 	reg := regexp.MustCompile(`multipart/form-data; boundary=\w{30}`)

// 	utils.AssertEqual(t, true, reg.Match(a.req.Header.Peek(fiber.HeaderContentType)))
// }

// func Test_Client_Agent_Multipart_Invalid_Boundary(t *testing.T) {
// 	t.Parallel()

// 	a := Post("http://example.com").
// 		Boundary("*").
// 		MultipartForm(nil)

// 	utils.AssertEqual(t, 1, len(a.errs))
// 	utils.AssertEqual(t, "mime: invalid boundary character", a.errs[0].Error())
// }

// func Test_Client_Agent_SendFile_Error(t *testing.T) {
// 	t.Parallel()

// 	a := Post("http://example.com").
// 		SendFile("non-exist-file!", "")

// 	utils.AssertEqual(t, 1, len(a.errs))
// 	utils.AssertEqual(t, true, strings.Contains(a.errs[0].Error(), "open non-exist-file!"))
// }

// func Test_Client_Debug(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.SendString("debug")
// 	}

// 	var output bytes.Buffer

// 	wrapAgent := func(a *Agent) {
// 		a.Debug(&output)
// 	}

// 	testAgent(t, handler, wrapAgent, "debug", 1)

// 	str := output.String()

// 	utils.AssertEqual(t, true, strings.Contains(str, "Connected to example.com(pipe)"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "GET / HTTP/1.1"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "User-Agent: fiber"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "Host: example.com\r\n\r\n"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "HTTP/1.1 200 OK"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "Content-Type: text/plain; charset=utf-8\r\nContent-Length: 5\r\n\r\ndebug"))
// }

// func Test_Client_Agent_Timeout(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		time.Sleep(time.Millisecond * 200)
// 		return c.SendString("timeout")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	a := Get("http://example.com").
// 		Timeout(time.Millisecond * 50)

// 	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 	_, body, errs := a.String()

// 	utils.AssertEqual(t, "", body)
// 	utils.AssertEqual(t, 1, len(errs))
// 	utils.AssertEqual(t, "timeout", errs[0].Error())
// }

// func Test_Client_Agent_Reuse(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("reuse")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	a := Get("http://example.com").
// 		Reuse()

// 	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 	code, body, errs := a.String()

// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "reuse", body)
// 	utils.AssertEqual(t, 0, len(errs))

// 	code, body, errs = a.String()

// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "reuse", body)
// 	utils.AssertEqual(t, 0, len(errs))
// }

// func Test_Client_Agent_InsecureSkipVerify(t *testing.T) {
// 	t.Parallel()

// 	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
// 	utils.AssertEqual(t, nil, err)

// 	serverTLSConf := &tls.Config{
// 		Certificates: []tls.Certificate{cer},
// 	}

// 	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
// 	utils.AssertEqual(t, nil, err)

// 	ln = tls.NewListener(ln, serverTLSConf)

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("ignore tls")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	code, body, errs := Get("https://" + ln.Addr().String()).
// 		InsecureSkipVerify().
// 		InsecureSkipVerify().
// 		String()

// 	utils.AssertEqual(t, 0, len(errs))
// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "ignore tls", body)
// }

// func Test_Client_Agent_TLS(t *testing.T) {
// 	t.Parallel()

// 	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
// 	utils.AssertEqual(t, nil, err)

// 	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
// 	utils.AssertEqual(t, nil, err)

// 	ln = tls.NewListener(ln, serverTLSConf)

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("tls")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	code, body, errs := Get("https://" + ln.Addr().String()).
// 		TLSConfig(clientTLSConf).
// 		String()

// 	utils.AssertEqual(t, 0, len(errs))
// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "tls", body)
// }

// func Test_Client_Agent_MaxRedirectsCount(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		if c.Request().URI().QueryArgs().Has("foo") {
// 			return c.Redirect("/foo")
// 		}
// 		return c.Redirect("/")
// 	})
// 	app.Get("/foo", func(c fiber.Ctx) error {
// 		return c.SendString("redirect")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	t.Run("success", func(t *testing.T) {
// 		a := Get("http://example.com?foo").
// 			MaxRedirectsCount(1)

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.String()

// 		utils.AssertEqual(t, 200, code)
// 		utils.AssertEqual(t, "redirect", body)
// 		utils.AssertEqual(t, 0, len(errs))
// 	})

// 	t.Run("error", func(t *testing.T) {
// 		a := Get("http://example.com").
// 			MaxRedirectsCount(1)

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		_, body, errs := a.String()

// 		utils.AssertEqual(t, "", body)
// 		utils.AssertEqual(t, 1, len(errs))
// 		utils.AssertEqual(t, "too many redirects detected when doing the request", errs[0].Error())
// 	})
// }

// func Test_Client_Agent_Struct(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.JSON(data{true})
// 	})

// 	app.Get("/error", func(c fiber.Ctx) error {
// 		return c.SendString(`{"success"`)
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	t.Run("success", func(t *testing.T) {
// 		t.Parallel()

// 		a := Get("http://example.com")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		var d data

// 		code, body, errs := a.Struct(&d)

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, `{"success":true}`, string(body))
// 		utils.AssertEqual(t, 0, len(errs))
// 		utils.AssertEqual(t, true, d.Success)
// 	})

// 	t.Run("pre error", func(t *testing.T) {
// 		t.Parallel()
// 		a := Get("http://example.com")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
// 		a.errs = append(a.errs, errors.New("pre errors"))

// 		var d data
// 		_, body, errs := a.Struct(&d)

// 		utils.AssertEqual(t, "", string(body))
// 		utils.AssertEqual(t, 1, len(errs))
// 		utils.AssertEqual(t, "pre errors", errs[0].Error())
// 		utils.AssertEqual(t, false, d.Success)
// 	})

// 	t.Run("error", func(t *testing.T) {
// 		a := Get("http://example.com/error")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		var d data

// 		code, body, errs := a.JSONDecoder(json.Unmarshal).Struct(&d)

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, `{"success"`, string(body))
// 		utils.AssertEqual(t, 1, len(errs))
// 		utils.AssertEqual(t, "unexpected end of JSON input", errs[0].Error())
// 	})
// }

// func Test_Client_Agent_Parse(t *testing.T) {
// 	t.Parallel()

// 	a := Get("https://example.com:10443")

// 	utils.AssertEqual(t, nil, a.Parse())
// }

// func Test_AddMissingPort_TLS(t *testing.T) {
// 	addr := addMissingPort("example.com", true)
// 	utils.AssertEqual(t, "example.com:443", addr)
// }

// func testAgent(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Agent), excepted string, count ...int) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", handler)

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	c := 1
// 	if len(count) > 0 {
// 		c = count[0]
// 	}

// 	for i := 0; i < c; i++ {
// 		a := Get("http://example.com")

// 		wrapAgent(a)

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, excepted, body)
// 		utils.AssertEqual(t, 0, len(errs))
// 	}
// }

// type data struct {
// 	Success bool `json:"success" xml:"success"`
// }

// type errorMultipartWriter struct {
// 	count int
// }

// func (e *errorMultipartWriter) Boundary() string           { return "myBoundary" }
// func (e *errorMultipartWriter) SetBoundary(_ string) error { return nil }
// func (e *errorMultipartWriter) CreateFormFile(_, _ string) (io.Writer, error) {
// 	if e.count == 0 {
// 		e.count++
// 		return nil, errors.New("CreateFormFile error")
// 	}
// 	return errorWriter{}, nil
// }
// func (e *errorMultipartWriter) WriteField(_, _ string) error { return errors.New("WriteField error") }
// func (e *errorMultipartWriter) Close() error                 { return errors.New("Close error") }

// type errorWriter struct{}

// func (errorWriter) Write(_ []byte) (int, error) { return 0, errors.New("Write error") }

func Test_Client_R(t *testing.T) {
	t.Parallel()

	client := AcquireClient()
	req := client.R()

	require.Equal(t, "Request", reflect.TypeOf(req).Elem().Name())
	require.Equal(t, client, req.Client())
}

func Test_Client_Add_Hook(t *testing.T) {
	t.Parallel()

	t.Run("add request hooks", func(t *testing.T) {
		client := AcquireClient().AddRequestHook(func(c *Client, r *Request) error {
			return nil
		})

		require.Equal(t, 1, len(client.RequestHook()))

		client.AddRequestHook(func(c *Client, r *Request) error {
			return nil
		}, func(c *Client, r *Request) error {
			return nil
		})

		require.Equal(t, 3, len(client.RequestHook()))
	})

	t.Run("add response hooks", func(t *testing.T) {
		client := AcquireClient().AddResponseHook(func(c *Client, resp *Response, r *Request) error {
			return nil
		})

		require.Equal(t, 1, len(client.ResponseHook()))

		client.AddResponseHook(func(c *Client, resp *Response, r *Request) error {
			return nil
		}, func(c *Client, resp *Response, r *Request) error {
			return nil
		})

		require.Equal(t, 3, len(client.ResponseHook()))
	})
}

func Test_Client_Marshal(t *testing.T) {
	t.Run("set json marshal", func(t *testing.T) {
		client := AcquireClient().
			SetJSONMarshal(func(v any) ([]byte, error) {
				return []byte("hello"), nil
			})
		val, err := client.JSONMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("set json unmarshal", func(t *testing.T) {
		client := AcquireClient().
			SetJSONUnmarshal(func(data []byte, v any) error {
				return fmt.Errorf("empty json")
			})

		err := client.JSONUnmarshal()(nil, nil)
		require.Equal(t, fmt.Errorf("empty json"), err)
	})

	t.Run("set xml marshal", func(t *testing.T) {
		client := AcquireClient().
			SetXMLMarshal(func(v any) ([]byte, error) {
				return []byte("hello"), nil
			})
		val, err := client.XMLMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("set xml unmarshal", func(t *testing.T) {
		client := AcquireClient().
			SetXMLUnmarshal(func(data []byte, v any) error {
				return fmt.Errorf("empty xml")
			})

		err := client.XMLUnmarshal()(nil, nil)
		require.Equal(t, fmt.Errorf("empty xml"), err)
	})
}

func Test_Client_SetBaseURL(t *testing.T) {
	t.Parallel()

	client := AcquireClient().SetBaseURL("http://example.com")

	require.Equal(t, "http://example.com", client.BaseURL())
}

func Test_Client_Header(t *testing.T) {
	t.Parallel()

	t.Run("", func(t *testing.T) {

	})
}
