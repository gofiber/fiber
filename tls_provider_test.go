package fiber

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// go test -run Test_ServerTLSProvider_Version
func Test_ServerTLSProvider_Version(t *testing.T) {
	prevPreforkMaster := testPreforkMaster
	testPreforkMaster = true
	t.Cleanup(func() { testPreforkMaster = prevPreforkMaster })

	app := New()

	// Invalid TLSMinVersion
	invalidTLSVersions := []uint16{tls.VersionTLS10, tls.VersionTLS11}
	for _, invalidVersion := range invalidTLSVersions {
		require.Error(t, app.Listen(":0", ListenConfig{
			DisableStartupMessage: true,
			TLSProvider: &ServerCertificateProvider{
				Certificate:   "./.github/testdata/pki/intermediate/server/fullchain.pem",
				TLSMinVersion: invalidVersion,
			},
		}))
		// Prefork
		require.Error(t, app.Listen(":0", ListenConfig{
			DisableStartupMessage: true,
			EnablePrefork:         true,
			TLSProvider: &ServerCertificateProvider{
				Certificate:   "./.github/testdata/pki/intermediate/server/fullchain.pem",
				TLSMinVersion: invalidVersion,
			},
		}))
	}

	// Valid TLSMinVersion
	for _, prefork := range []bool{false, true} {
		go func() {
			time.Sleep(1000 * time.Millisecond)
			assert.NoError(t, app.Shutdown())
		}()
		require.NoError(t, app.Listen(":0", ListenConfig{
			DisableStartupMessage: true,
			EnablePrefork:         prefork,
			TLSProvider: &ServerCertificateProvider{
				Certificate:   "./.github/testdata/pki/intermediate/server/fullchain.pem",
				TLSMinVersion: tls.VersionTLS13,
			},
		}))
	}
}

// go test -run Test_ServerTLSProvider
func Test_ServerTLSProvider(t *testing.T) {
	app := New()

	// invalid port
	require.Error(t, app.Listen(":99999", ListenConfig{
		TLSProvider: &ServerCertificateProvider{
			Certificate: "./.github/testdata/pki/intermediate/server/fullchain.pem",
		},
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		TLSProvider: &ServerCertificateProvider{
			Certificate: "./.github/testdata/pki/intermediate/server/fullchain.pem",
		},
	}))
}

// go test -run Test_ServerTLSProvider_Deprecated
func Test_ServerTLSProvider_Deprecated(t *testing.T) {
	app := New()

	// invalid port
	require.Error(t, app.Listen(":99999", ListenConfig{
		TLSProvider: &ServerCertificateProvider{
			CertFile:    "./.github/testdata/pki/intermediate/server/cert.pem",
			CertKeyFile: "./.github/testdata/pki/intermediate/server/key.pem",
		},
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		TLSProvider: &ServerCertificateProvider{
			CertFile:    "./.github/testdata/pki/intermediate/server/cert.pem",
			CertKeyFile: "./.github/testdata/pki/intermediate/server/key.pem",
		},
	}))
}

// go test -run Test_ServerTLSProvider_Prefork
func Test_ServerTLSProvider_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	// invalid key file content
	require.Error(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		TLSProvider: &ServerCertificateProvider{
			Certificate: "./.github/testdata/template.tmpl",
		},
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		TLSProvider: &ServerCertificateProvider{
			Certificate: "./.github/testdata/pki/intermediate/server/fullchain.pem",
		},
	}))
}

// go test -run Test_ServerTLSProvider_Prefork_Deprecated
func Test_ServerTLSProvider_Prefork_Deprecated(t *testing.T) {
	testPreforkMaster = true

	app := New()

	// invalid key file content
	require.Error(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		TLSProvider: &ServerCertificateProvider{
			CertFile:    "./.github/testdata/pki/intermediate/server/cert.pem",
			CertKeyFile: "./.github/testdata/template.tmpl",
		},
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		TLSProvider: &ServerCertificateProvider{
			CertFile:    "./.github/testdata/pki/intermediate/server/cert.pem",
			CertKeyFile: "./.github/testdata/pki/intermediate/server/key.pem",
		},
	}))
}
