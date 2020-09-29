package timeout

// // go test -run Test_Middleware_Timeout
// func Test_Middleware_Timeout(t *testing.T) {
// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	h := New(func(c *fiber.Ctx) error {
// 		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
// 		time.Sleep(sleepTime)
// 		return c.SendString("After " + c.Params("sleepTime") + "ms sleeping")
// 	}, 5*time.Millisecond)
// 	app.Get("/test/:sleepTime", h)

// 	testTimeout := func(timeoutStr string) {
// 		resp, err := app.Test(httptest.NewRequest("GET", "/test/"+timeoutStr, nil))
// 		utils.AssertEqual(t, nil, err, "app.Test(req)")
// 		utils.AssertEqual(t, fiber.StatusRequestTimeout, resp.StatusCode, "Status code")

// 		body, err := ioutil.ReadAll(resp.Body)
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, "Request Timeout", string(body))
// 	}
// 	testSucces := func(timeoutStr string) {
// 		resp, err := app.Test(httptest.NewRequest("GET", "/test/"+timeoutStr, nil))
// 		utils.AssertEqual(t, nil, err, "app.Test(req)")
// 		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")

// 		body, err := ioutil.ReadAll(resp.Body)
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, "After "+timeoutStr+"ms sleeping", string(body))
// 	}

// 	testTimeout("15")
// 	testSucces("2")
// 	testTimeout("30")
// 	testSucces("3")
// }

// // go test -run -v Test_Timeout_Panic
// func Test_Timeout_Panic(t *testing.T) {
// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/panic", recover.New(), New(func(c *fiber.Ctx) error {
// 		c.Set("dummy", "this should not be here")
// 		panic("panic in timeout handler")
// 	}, 5*time.Millisecond))

// 	resp, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
// 	utils.AssertEqual(t, nil, err, "app.Test(req)")
// 	utils.AssertEqual(t, fiber.StatusRequestTimeout, resp.StatusCode, "Status code")

// 	body, err := ioutil.ReadAll(resp.Body)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, "Request Timeout", string(body))
// }
