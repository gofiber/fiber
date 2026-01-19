package verifypeer_test

import (
	"sync"
	"testing"
	"time"

	_ "unsafe"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/addon/verifypeer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:linkname testPreforkMaster github.com/gofiber/fiber/v3.testPreforkMaster
var testPreforkMaster = false

var preforkMu sync.Mutex

// go test -run Test_Listen_MutualTLS
func Test_Listen_MutualTLS(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// invalid port
	require.Error(t, app.Listen(":99999", fiber.ListenConfig{
		TLSProvider: &fiber.ServerCertificateProvider{
			Certificate: "../../.github/testdata/pki/intermediate/server/fullchain.pem",
		},
		TLSCustomizer: &verifypeer.MutualTLSCustomizer{
			Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
		},
	}))

	shutdownErr := make(chan error, 1)
	go func() {
		time.Sleep(1000 * time.Millisecond)
		shutdownErr <- app.Shutdown()
	}()

	require.NoError(t, app.Listen(":0", fiber.ListenConfig{
		DisableStartupMessage: true,
		TLSProvider: &fiber.ServerCertificateProvider{
			Certificate: "../../.github/testdata/pki/intermediate/server/fullchain.pem",
		},
		TLSCustomizer: &verifypeer.MutualTLSCustomizer{
			Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
		},
	}))
	require.NoError(t, <-shutdownErr)
}

// go test -run Test_Listen_MutualTLS_Deprecated
func Test_Listen_MutualTLS_Deprecated(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// invalid port
	require.Error(t, app.Listen(":99999", fiber.ListenConfig{
		TLSProvider: &fiber.ServerCertificateProvider{
			CertFile:    "../../.github/testdata/pki/intermediate/server/cert.pem",
			CertKeyFile: "../../.github/testdata/pki/intermediate/server/key.pem",
		},
		TLSCustomizer: &verifypeer.MutualTLSCustomizer{
			Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
		},
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", fiber.ListenConfig{
		DisableStartupMessage: true,
		TLSProvider: &fiber.ServerCertificateProvider{
			CertFile:    "../../.github/testdata/pki/intermediate/server/cert.pem",
			CertKeyFile: "../../.github/testdata/pki/intermediate/server/key.pem",
		},
		TLSCustomizer: &verifypeer.MutualTLSCustomizer{
			Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
		},
	}))
}

// go test -run Test_Listen_MutualTLS_Prefork
func Test_Listen_MutualTLS_Prefork(t *testing.T) {
	t.Parallel()
	preforkMu.Lock()
	t.Cleanup(preforkMu.Unlock)
	testPreforkMaster = true
	t.Cleanup(func() { testPreforkMaster = false })

	app := fiber.New()

	// invalid key file content
	require.Error(t, app.Listen(":0", fiber.ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		TLSProvider: &fiber.ServerCertificateProvider{
			Certificate: "../../.github/testdata/template.html",
		},
		TLSCustomizer: &verifypeer.MutualTLSCustomizer{
			Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
		},
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", fiber.ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		TLSProvider: &fiber.ServerCertificateProvider{
			Certificate: "../../.github/testdata/pki/intermediate/server/fullchain.pem",
		},
		TLSCustomizer: &verifypeer.MutualTLSCustomizer{
			Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
		},
	}))
}

// go test -run Test_Listen_MutualTLS_Prefork_Deprecated
func Test_Listen_MutualTLS_Prefork_Deprecated(t *testing.T) {
	t.Parallel()
	preforkMu.Lock()
	t.Cleanup(preforkMu.Unlock)
	testPreforkMaster = true
	t.Cleanup(func() { testPreforkMaster = false })

	app := fiber.New()

	// invalid key file content
	require.Error(t, app.Listen(":0", fiber.ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		TLSProvider: &fiber.ServerCertificateProvider{
			CertFile:    "../../.github/testdata/pki/intermediate/server/cert.pem",
			CertKeyFile: "../../.github/testdata/template.html",
		},
		TLSCustomizer: &verifypeer.MutualTLSCustomizer{
			Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
		},
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", fiber.ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		TLSProvider: &fiber.ServerCertificateProvider{
			CertFile:    "../../.github/testdata/pki/intermediate/server/cert.pem",
			CertKeyFile: "../../.github/testdata/pki/intermediate/server/key.pem",
		},
		TLSCustomizer: &verifypeer.MutualTLSCustomizer{
			Certificate: "../../.github/testdata/pki/intermediate/cacert.pem",
		},
	}))
}
