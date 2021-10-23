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

type (
	args struct {
		sleepTime string
	}

	expected struct {
		reqErr     error
		bdyErr     error
		body       string
		statusCode int
	}

	test struct {
		name     string
		args     args
		expected expected
	}
)

func newTimeoutHandler(t time.Duration, err error) fiber.Handler {
	return New(func(c *fiber.Ctx) error {
		sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")

		time.Sleep(sleepTime)

		return err
	}, t)
}

func newApp(th fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test/:sleepTime", th)

	return app
}

func run(t *testing.T, tests []test, app *fiber.App) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", "/test/"+test.args.sleepTime, nil), -1)
			utils.AssertEqual(t, test.expected.reqErr, err, "app.Test(req)")
			utils.AssertEqual(t, test.expected.statusCode, resp.StatusCode, "Status code")

			body, err := ioutil.ReadAll(resp.Body)
			utils.AssertEqual(t, test.expected.bdyErr, err)
			utils.AssertEqual(t, test.expected.body, string(body))
		})
	}
}

func TestNewWithDefaultTimeout(t *testing.T) {
	const defaultTimeout = 10 * time.Millisecond

	th := newTimeoutHandler(defaultTimeout, nil)

	app := newApp(th)

	tests := []test{
		{
			name: "Execution time > timeout",
			args: args{sleepTime: "30"},
			expected: expected{
				reqErr:     nil,
				bdyErr:     nil,
				statusCode: fiber.StatusRequestTimeout,
				body:       fiber.ErrRequestTimeout.Error(),
			},
		},
		{
			name: "Execution time < timeout",
			args: args{sleepTime: "0"},
			expected: expected{
				reqErr:     nil,
				bdyErr:     nil,
				statusCode: fiber.StatusOK,
				body:       "",
			},
		},
	}

	run(t, tests, app)
}

func TestNewWithDefaultTimeoutAndError(t *testing.T) {
	const defaultTimeout = 30 * time.Millisecond

	funcErr := errors.New("something went wrong")

	th := newTimeoutHandler(defaultTimeout, funcErr)

	app := newApp(th)

	tests := []test{
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

	run(t, tests, app)
}

func TestNewWithNoTimeout(t *testing.T) {
	th := newTimeoutHandler(0, nil)

	app := newApp(th)

	tests := []test{
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

	run(t, tests, app)
}
