package fiber

import (
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"

	"golang.org/x/crypto/acme/autocert"
)

// ServerTLSProvider provides an interface for creating a tls.Config pointer to be used with by a server.
type ServerTLSProvider interface {
	// ProvideServerTLS provides possibly a *tls.Config object to be used by Listen or
	// an error if it fails to do so.
	//
	// NOTE: Implementations may return (nil, nil) if no tls.Config can be provided and no error occurred.
	// For example when no path to a certificate is provided (empty string), instead of a wrong
	// path which will generate an error.
	ProvideServerTLS() (*tls.Config, error)
}

// ACMECertificateProvider is a struct implementing the [ServerTLSProvider] interface to be used with ListenConfig
type ACMECertificateProvider struct {
	// AutoCertManager manages TLS certificates automatically using the ACME protocol,
	// Enables integration with Let's Encrypt or other ACME-compatible providers.
	//
	// Default: nil
	AutoCertManager *autocert.Manager `json:"auto_cert_manager"`

	// TLSMinVersion allows to set TLS minimum version.
	//
	// Default: tls.VersionTLS12
	// WARNING: TLS1.0 and TLS1.1 versions are not supported.
	TLSMinVersion uint16 `json:"tls_min_version"`
}

// ProvideServerTLS implements [ServerTLSProvider].
func (p *ACMECertificateProvider) ProvideServerTLS() (*tls.Config, error) {
	var tlsConfig *tls.Config

	if p.TLSMinVersion == 0 {
		p.TLSMinVersion = tls.VersionTLS12
	}

	if p.TLSMinVersion != tls.VersionTLS12 && p.TLSMinVersion != tls.VersionTLS13 {
		return nil, errors.New("tls: unsupported TLS version, please use tls.VersionTLS12 or tls.VersionTLS13")
	}
	if p.AutoCertManager != nil {
		tlsConfig = &tls.Config{ //nolint:gosec // This is a user input
			MinVersion:     p.TLSMinVersion,
			GetCertificate: p.AutoCertManager.GetCertificate,
			NextProtos:     []string{"http/1.1", "acme-tls/1"},
		}
	}

	return tlsConfig, nil
}

var _ ServerTLSProvider = &ACMECertificateProvider{}

// ServerCertificateProvider is a struct implementing [ServerTLSProvider], to be used with ListenConfig.
type ServerCertificateProvider struct {
	// Certificate is either a path to a file or the content of the file instead.
	//
	// It must contain, in order, in PEM format the certificate, key and any additional
	// intermediate CA certificate signing this certificate.
	//
	// Default: ""
	Certificate string `json:"certificate"`

	// CertFile is a path of certificate file.
	// If you want to use TLS, you should enter this field or use "Certificate".
	//
	// Default : ""
	// NOTE : Deprecated. Use "Certificate" instead
	CertFile string `json:"cert_file"`

	// KeyFile is a path of certificate's private key.
	// If you want to use TLS, you should enter this field or use "Certificate".
	//
	// Default : ""
	// NOTE : Deprecated. Use "Certificate" instead
	CertKeyFile string `json:"cert_key_file"`

	// TLSMinVersion allows to set TLS minimum version.
	//
	// Default: tls.VersionTLS12
	// WARNING: TLS1.0 and TLS1.1 versions are not supported.
	TLSMinVersion uint16 `json:"tls_min_version"`
}

var _ ServerTLSProvider = &ServerCertificateProvider{}

// ProvideServerTLS implements [ServerTLSProvider]
func (p *ServerCertificateProvider) ProvideServerTLS() (*tls.Config, error) {
	var tlsConfig *tls.Config

	if p.TLSMinVersion == 0 {
		p.TLSMinVersion = tls.VersionTLS12
	}

	if p.TLSMinVersion != tls.VersionTLS12 && p.TLSMinVersion != tls.VersionTLS13 {
		return nil, errors.New("tls: unsupported TLS version, please use tls.VersionTLS12 or tls.VersionTLS13")
	}

	setCert := func(cert tls.Certificate) {
		tlsConfig = &tls.Config{ //nolint:gosec // This is a user input
			MinVersion:   p.TLSMinVersion,
			Certificates: []tls.Certificate{cert},
		}
	}

	if p.Certificate != "" {
		if block, _ := pem.Decode([]byte(p.Certificate)); block != nil {
			cert, err := tls.X509KeyPair([]byte(p.Certificate), []byte(p.Certificate))
			if err != nil {
				return nil, fmt.Errorf("tls: cannot load TLS key pair from CertificateChain: %w", err)
			}
			setCert(cert)
		} else {
			cert, err := tls.LoadX509KeyPair(p.Certificate, p.Certificate)
			if err != nil {
				return nil, fmt.Errorf("tls: cannot load TLS key pair from CertificateChain: %w", err)
			}
			setCert(cert)
		}
	} else if p.CertFile != "" && p.CertKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(p.CertFile, p.CertKeyFile)
		if err != nil {
			return nil, fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %w", p.CertFile, p.CertKeyFile, err)
		}
		setCert(cert)
	}

	if tlsConfig != nil {
		tlsHandler := &TLSHandler{}
		tlsConfig.GetCertificate = tlsHandler.GetClientInfo
		return tlsConfig, nil
	}

	return tlsConfig, nil
}

// ServerTLSCustomizer provides an interface for customizing an existing [tls.Config] object configured for a server.
type ServerTLSCustomizer interface {
	CustomizeServerTLS(config *tls.Config) error
}
