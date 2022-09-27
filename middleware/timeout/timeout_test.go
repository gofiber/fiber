package timeout

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_Timeout
func Test_Timeout(t *testing.T) {
	// fiber instance
	app := fiber.New()
	h := New(func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContext(c.UserContext(), sleepTime, context.DeadlineExceeded); err != nil {
			return fmt.Errorf("%w: l2 wrap", fmt.Errorf("%w: l1 wrap ", err))
		}
		return nil
	}, 100*time.Millisecond)
	app.Get("/test/:sleepTime", h)
	testTimeout := func(timeoutStr string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/test/"+timeoutStr, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusRequestTimeout, resp.StatusCode, "Status code")
	}
	testSucces := func(timeoutStr string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/test/"+timeoutStr, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	}
	testTimeout("300")
	testTimeout("500")
	testSucces("50")
	testSucces("30")
}

var ErrFooTimeOut = errors.New("foo context canceled")

// go test -run Test_TimeoutWithCustomError
func Test_TimeoutWithCustomError(t *testing.T) {
	// fiber instance
	app := fiber.New()
	h := New(func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContext(c.UserContext(), sleepTime, ErrFooTimeOut); err != nil {
			return fmt.Errorf("%w: execution error", err)
		}
		return nil
	}, 100*time.Millisecond, ErrFooTimeOut)
	app.Get("/test/:sleepTime", h)
	testTimeout := func(timeoutStr string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/test/"+timeoutStr, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusRequestTimeout, resp.StatusCode, "Status code")
	}
	testSucces := func(timeoutStr string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/test/"+timeoutStr, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	}
	testTimeout("300")
	testTimeout("500")
	testSucces("50")
	testSucces("30")
}

// go test -run Test_TimeoutUseWithCustomError
func Test_TimeoutUseWithCustomError(t *testing.T) {
	// fiber instance
	app := fiber.New()
	app.Use(Use(200*time.Millisecond, ErrFooTimeOut))
	h := func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContext(c.UserContext(), sleepTime, context.DeadlineExceeded); err != nil {
			return fmt.Errorf("%w: l2 wrap", fmt.Errorf("%w: l1 wrap ", err))
		}
		return nil
	}
	group := app.Group("/group", Use(100*time.Millisecond, ErrFooTimeOut))
	{
		group.Get("/:sleepTime", h)
	}
	app.Get("/test/:sleepTime", New(h, 100*time.Millisecond, ErrFooTimeOut))
	app.Get("/:sleepTime", h)
	testTimeout := func(traget string) {
		resp, err := app.Test(httptest.NewRequest("GET", traget, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusRequestTimeout, resp.StatusCode, "Status code")
	}
	testSucces := func(traget string) {
		resp, err := app.Test(httptest.NewRequest("GET", traget, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	}
	testTimeout("/300")
	testTimeout("/group/150")
	testTimeout("/test/150")
	testSucces("/150")
	testSucces("/group/30")
	testSucces("/test/30")
}

// go test -run Test_WithTimeoutSkipTimeoutStatus
func Test_WithTimeoutSkipTimeoutStatus(t *testing.T) {
	// fiber instance
	app := fiber.New()
	app.Use(Use(200*time.Millisecond, ErrFooTimeOut))
	h := func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContext(c.UserContext(), sleepTime, context.DeadlineExceeded); err != nil {
			return c.SendString("Error: " + err.Error())
		}
		return c.SendString("OK")
	}
	group := app.Group("/group", Use(100*time.Millisecond, ErrFooTimeOut))
	{
		group.Get("/:sleepTime", h)
	}
	app.Get("/test/:sleepTime", New(h, 100*time.Millisecond, ErrFooTimeOut))
	app.Get("/:sleepTime", h)
	testTimeout := func(traget string) {
		resp, err := app.Test(httptest.NewRequest("GET", traget, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
		body, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Error: "+context.DeadlineExceeded.Error(), string(body))
	}
	testSucces := func(traget string) {
		resp, err := app.Test(httptest.NewRequest("GET", traget, nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
		body, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "OK", string(body))
	}
	testTimeout("/300")
	testTimeout("/group/150")
	testTimeout("/test/150")
	testSucces("/150")
	testSucces("/group/30")
	testSucces("/test/30")
}

func sleepWithContext(ctx context.Context, d time.Duration, te error) error {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return te
	case <-timer.C:
	}
	return nil
}
