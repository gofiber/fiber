package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

func Test_OpenAPI_SlowSpecBuildDoesNotBlockOtherRequests(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	slow := func(v any) ([]byte, error) {
		if _, isSpec := v.(openAPISpec); isSpec {
			once.Do(func() { close(started) })
			<-release
		}
		return json.Marshal(v)
	}

	handler := New()
	slowApp := fiber.New(fiber.Config{JSONEncoder: slow})
	slowApp.Use(handler)
	otherApp := fiber.New()
	otherApp.Use(handler)

	built := make(chan error, 1)
	go func() {
		resp, err := slowApp.Test(httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody), fiber.TestConfig{Timeout: 10 * time.Second})
		if err == nil {
			err = resp.Body.Close()
		}
		built <- err
	}()
	apptest.Recv(t, started, 10*time.Second, "the slow build did not start")

	apptest.Expect(t, otherApp, "/openapi.json", fiber.StatusOK)
	apptest.Expect(t, slowApp, "/swagger", fiber.StatusOK)

	close(release)
	require.NoError(t, apptest.Recv(t, built, 10*time.Second, "the slow build did not finish"))
}
