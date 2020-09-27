package proxy

// // go test -run Test_Proxy_Empty_Host
// func Test_Proxy_Empty_Host(t *testing.T) {
// 	app := fiber.New()
// 	app.Use(New(
// 		Config{Hosts: ""},
// 	))

// 	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
// }

// // go test -run Test_Proxy_Next
// func Test_Proxy_Next(t *testing.T) {
// 	app := fiber.New()
// 	app.Use(New(Config{
// 		Hosts: "next",
// 		Next: func(_ *fiber.Ctx) bool {
// 			return true
// 		},
// 	}))

// 	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
// }

// // go test -run Test_Proxy
// func Test_Proxy(t *testing.T) {
// 	target := fiber.New(fiber.Config{
// 		DisableStartupMessage: true,
// 	})

// 	target.Get("/", func(c *fiber.Ctx) error {
// 		return c.SendStatus(fiber.StatusTeapot)
// 	})

// 	go func() {
// 		utils.AssertEqual(t, nil, target.Listen(":3001"))
// 	}()

// 	time.Sleep(time.Second)

// 	resp, err := target.Test(httptest.NewRequest("GET", "/", nil), 2000)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)

// 	app := fiber.New()

// 	host := "localhost:3001"

// 	app.Use(New(Config{
// 		Hosts: host,
// 	}))

// 	req := httptest.NewRequest("GET", "/", nil)
// 	req.Host = host
// 	resp, err = app.Test(req)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
// }

// // go test -run Test_Proxy_Before_With_Error
// func Test_Proxy_Before_With_Error(t *testing.T) {
// 	app := fiber.New()

// 	errStr := "error after Before"

// 	app.Use(
// 		New(Config{
// 			Hosts: "host",
// 			Before: func(c *fiber.Ctx) error {
// 				return fmt.Errorf(errStr)
// 			},
// 		}))

// 	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)

// 	b, err := ioutil.ReadAll(resp.Body)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, errStr, string(b))
// }

// // go test -run Test_Proxy_After_With_Error
// func Test_Proxy_After_With_Error(t *testing.T) {
// 	target := fiber.New(fiber.Config{
// 		DisableStartupMessage: true,
// 	})

// 	target.Get("/", func(c *fiber.Ctx) error {
// 		return c.SendStatus(fiber.StatusTeapot)
// 	})

// 	go func() {
// 		utils.AssertEqual(t, nil, target.Listen(":3002"))
// 	}()

// 	time.Sleep(time.Second)

// 	resp, err := target.Test(httptest.NewRequest("GET", "/", nil), 2000)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)

// 	app := fiber.New()

// 	host := "localhost:3001"
// 	errStr := "error after After"

// 	app.Use(New(Config{
// 		Hosts: host,
// 		After: func(ctx *fiber.Ctx) error {
// 			utils.AssertEqual(t, fiber.StatusTeapot, ctx.Response().StatusCode())
// 			return fmt.Errorf(errStr)
// 		},
// 	}))

// 	req := httptest.NewRequest("GET", "/", nil)
// 	req.Host = host
// 	resp, err = app.Test(req)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)

// 	b, err := ioutil.ReadAll(resp.Body)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, errStr, string(b))
// }

// // go test -run Test_Proxy_Do_With_Error
// func Test_Proxy_Do_With_Error(t *testing.T) {
// 	app := fiber.New()

// 	app.Use(
// 		New(Config{
// 			Hosts: "localhost:90000",
// 		}))

// 	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)

// 	b, err := ioutil.ReadAll(resp.Body)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, true, strings.Contains(string(b), "127.0.0.1:90000"))
// }
