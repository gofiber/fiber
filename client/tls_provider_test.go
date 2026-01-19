package client

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ClientTLSProvider_RootCertificatesErrors(t *testing.T) {
	t.Parallel()

	client := New()
	require.Panics(t, func() {
		client.SetTLSProvider(&ClientCertificateProvider{RootCertificates: "does-not-exist.pem"})
	})

	require.Panics(t, func() {
		client.SetTLSProvider(&ClientCertificateProvider{RootCertificates: "invalid pem data"})
	})

	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "invalid.pem")
	require.NoError(t, os.WriteFile(badPath, []byte("not a pem"), 0o600))

	require.Panics(t, func() {
		client.SetTLSProvider(&ClientCertificateProvider{RootCertificates: badPath})
	})
}

func Test_ClientTLSProvider_Certificate(t *testing.T) {
	t.Parallel()

	client := New().SetTLSProvider(&ClientCertificateProvider{Certificate: "../.github/testdata/pki/intermediate/client/fullchain.pem"})
	require.Len(t, client.TLSConfig().Certificates, 1)
}

func Test_ClientTLSProvider_RootCertificates(t *testing.T) {
	t.Parallel()

	client := New().SetTLSProvider(&ClientCertificateProvider{RootCertificates: "../.github/testdata/pki/cacert.pem"})
	require.NotNil(t, client.TLSConfig().RootCAs)
}

func Test_ClientTLSProvider_RootCertificatesFromString(t *testing.T) {
	t.Parallel()

	file, err := os.Open("../.github/testdata/pki/cacert.pem")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	pem, err := io.ReadAll(file)
	require.NoError(t, err)

	client := New().SetTLSProvider(&ClientCertificateProvider{RootCertificates: string(pem)})
	require.NotNil(t, client.TLSConfig().RootCAs)
}
