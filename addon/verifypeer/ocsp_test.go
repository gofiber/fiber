package verifypeer_test

import (
	"net"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/addon/verifypeer"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/fiber/v3/middleware/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_OCSP_Server(t *testing.T) {
	t.Parallel()
	staplerAddrCh := make(chan string, 1)
	appAddrCh := make(chan string, 1)

	stapler, err := NewOCSPResponder()
	require.NoError(t, err)

	go func() {
		assert.NoError(t, stapler.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerNetwork:       fiber.NetworkTCP4,
			ListenerAddrFunc: func(addr net.Addr) {
				staplerAddrCh <- addr.String()
			},
		}))
	}()

	ocspAddr := <-staplerAddrCh
	ocspPort, found := strings.CutPrefix(ocspAddr, "0.0.0.0:")
	require.True(t, found)

	// App
	app := fiber.New()
	app.Use(logger.New())
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello")
	})
	go func() {
		assert.NoError(t, app.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerNetwork:       fiber.NetworkTCP4,
			ListenerAddrFunc: func(addr net.Addr) {
				appAddrCh <- addr.String()
			},
			TLSProvider: &fiber.ServerCertificateProvider{
				Certificate: "../../.github/testdata/pki/intermediate/server/fullchain.pem",
			},
			TLSCustomizer: &verifypeer.MutualTLSWithOCSPCustomizer{
				Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
				OCSPServer:  "http://localhost:" + ocspPort,
			},
		}))
	}()

	appAddr := <-appAddrCh
	appPort, found := strings.CutPrefix(appAddr, "0.0.0.0:")
	require.True(t, found)

	// Authorized
	require.NoError(t, get(&client.ClientCertificateProvider{
		Certificate:      "../../.github/testdata/pki/intermediate/client/fullchain.pem",
		RootCertificates: "../../.github/testdata/pki/cacert.pem",
	}, "https://localhost:"+appPort))

	// Revoke
	require.Error(t, get(&client.ClientCertificateProvider{
		Certificate:      "../../.github/testdata/pki/intermediate/revoked/fullchain.pem",
		RootCertificates: "../../.github/testdata/pki/cacert.pem",
	}, "https://localhost:"+appPort))

	require.NoError(t, stapler.Shutdown())
	require.NoError(t, app.Shutdown())
}
