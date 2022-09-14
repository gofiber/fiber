package timeoutcontext

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_TimeoutContext
func Test_TimeoutContext(t *testing.T) {
	// fiber instance
	app := fiber.New()
	h := New(func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContext(c.UserContext(), sleepTime); err != nil {
			return fmt.Errorf("%w: execution error", err)
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

// go test -run Test_TimeoutContextWithCustomError
func Test_TimeoutContextWithCustomError(t *testing.T) {
	// fiber instance
	app := fiber.New()
	h := New(func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
		if err := sleepWithContextWithCustomError(c.UserContext(), sleepTime); err != nil {
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

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return context.DeadlineExceeded
	case <-timer.C:
	}
	return nil
}

func sleepWithContextWithCustomError(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return ErrFooTimeOut
	case <-timer.C:
	}
	return nil
}
