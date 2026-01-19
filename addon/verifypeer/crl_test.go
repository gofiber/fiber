package verifypeer_test

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/addon/verifypeer"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/fiber/v3/middleware/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCRL(t *testing.T, crl string) {
	t.Helper()
	var network string

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
				network = addr.String()
			},
			TLSProvider: &fiber.ServerCertificateProvider{
				Certificate: "../../.github/testdata/pki/intermediate/server/fullchain.pem",
			},
			TLSCustomizer: &verifypeer.MutualTLSWithCRLCustomizer{
				Certificate:    "../../.github/testdata/pki/intermediate/cacert.pem",
				RevocationList: crl,
			},
		}))
	}()

	time.Sleep(500 * time.Millisecond)
	appPort, found := strings.CutPrefix(network, "0.0.0.0:")
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

	require.NoError(t, app.Shutdown())
}

func Test_CRL_File(t *testing.T) {
	testCRL(t, "../../.github/testdata/pki/intermediate/crl.pem")
}

func Test_CRL_Content(t *testing.T) {
	crlFile, err := os.ReadFile(filepath.Clean("../../.github/testdata/pki/intermediate/crl.pem"))
	require.NoError(t, err)

	testCRL(t, string(crlFile))
}

func Test_CRL_Server(t *testing.T) {
	var network string

	crl, err := os.ReadFile(filepath.Clean("../../.github/testdata/pki/intermediate/crl.pem"))
	require.NoError(t, err)

	app := fiber.New()
	app.Get("/crl.pem", func(c fiber.Ctx) error {
		return c.Send(crl)
	})

	go func() {
		assert.NoError(t, app.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerNetwork:       fiber.NetworkTCP4,
			ListenerAddrFunc: func(addr net.Addr) {
				network = addr.String()
			},
		}))
	}()

	time.Sleep(500 * time.Millisecond)
	crlPort, found := strings.CutPrefix(network, "0.0.0.0:")
	require.True(t, found)

	testCRL(t, "http://localhost:"+crlPort+"/crl.pem")

	require.NoError(t, app.Shutdown())
}
