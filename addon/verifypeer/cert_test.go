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

func testCert(t *testing.T, cert string) {
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
			TLSCustomizer: &verifypeer.MutualTLSCustomizer{
				Certificate: cert,
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

	// Expired
	require.Error(t, get(&client.ClientCertificateProvider{
		Certificate:      "../../.github/testdata/pki/intermediate/expired/fullchain.pem",
		RootCertificates: "../../.github/testdata/pki/cacert.pem",
	}, "https://localhost:"+appPort))

	require.NoError(t, app.Shutdown())
}

func Test_Cert_File(t *testing.T) {
	t.Parallel()
	testCert(t, "../../.github/testdata/pki/intermediate/cacert.pem")
}

func Test_Cert_Content(t *testing.T) {
	t.Parallel()
	cert, err := os.ReadFile(filepath.Clean("../../.github/testdata/pki/intermediate/cacert.pem"))
	require.NoError(t, err)
	testCert(t, string(cert))
}
