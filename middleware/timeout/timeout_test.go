package timeout

import (
	"errors"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

/*
	NOTE: You have to reasonable while setting timeout values.
	Some tests will fail in subsequent runs if execution time
	and timeout value are same OR close enough.
*/

func TestNewWithDefaultTimeout(t *testing.T) {
	const defaultTimeout = 10 * time.Millisecond

	th := New(func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")

		time.Sleep(sleepTime)

		return nil
	}, defaultTimeout)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test/:sleepTime", th)

	type args struct {
		sleepTime string
	}

	type expected struct {
		reqErr     error
		bdyErr     error
		statusCode int
	}

	tests := []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name: "Execution time > timeout",
			args: args{sleepTime: "30"},
			expected: expected{
				reqErr:     nil,
				bdyErr:     nil,
				statusCode: fiber.StatusRequestTimeout,
			},
		},
		{
			name: "Execution time < timeout",
			args: args{sleepTime: "0"},
			expected: expected{
				reqErr:     nil,
				bdyErr:     nil,
				statusCode: fiber.StatusOK,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", "/test/"+tt.args.sleepTime, nil), -1)
			utils.AssertEqual(t, tt.expected.reqErr, err, "app.Test(req)")
			utils.AssertEqual(t, tt.expected.statusCode, resp.StatusCode, "Status code")

			_, err = ioutil.ReadAll(resp.Body)
			utils.AssertEqual(t, tt.expected.bdyErr, err)
		})
	}
}

func TestNewWithDefaultTimeoutAndError(t *testing.T) {
	const defaultTimeout = 30 * time.Millisecond
	funcErr := errors.New("something went wrong")

	th := New(func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")

		time.Sleep(sleepTime)

		return funcErr
	}, defaultTimeout)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test/:sleepTime", th)

	type args struct {
		sleepTime string
	}

	type expected struct {
		reqErr     error
		bdyErr     error
		body       string
		statusCode int
	}

	tests := []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name: "Error in actual handler",
			args: args{sleepTime: "1"},
			expected: expected{
				reqErr:     nil,
				bdyErr:     nil,
				statusCode: fiber.StatusInternalServerError,
				body:       funcErr.Error(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", "/test/"+tt.args.sleepTime, nil), -1)
			utils.AssertEqual(t, tt.expected.reqErr, err, "app.Test(req)")
			utils.AssertEqual(t, tt.expected.statusCode, resp.StatusCode, "Status code")

			body, err := ioutil.ReadAll(resp.Body)
			utils.AssertEqual(t, tt.expected.bdyErr, err)
			utils.AssertEqual(t, tt.expected.body, string(body))
		})
	}
}

func TestNewWithNoTimeout(t *testing.T) {
	th := New(func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")

		time.Sleep(sleepTime)

		return nil
	}, 0)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test/:sleepTime", th)

	type args struct {
		sleepTime string
	}

	type expected struct {
		reqErr     error
		bdyErr     error
		statusCode int
	}

	tests := []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name: "No Timeout",
			args: args{sleepTime: "15"},
			expected: expected{
				reqErr:     nil,
				bdyErr:     nil,
				statusCode: fiber.StatusOK,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", "/test/"+tt.args.sleepTime, nil), -1)
			utils.AssertEqual(t, tt.expected.reqErr, err, "app.Test(req)")
			utils.AssertEqual(t, tt.expected.statusCode, resp.StatusCode, "Status code")

			_, err = ioutil.ReadAll(resp.Body)
			utils.AssertEqual(t, tt.expected.bdyErr, err)
		})
	}
}
