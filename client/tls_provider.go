package client

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ClientTLSProvider provides an interface for creating a [*tls.Config] to be used with by a client.
type ClientTLSProvider interface {
	// ProvideClientTLS provides possibly a *tls.Config object to be used by a client or
	// an error if it fails to do so.
	//
	// NOTE: Implementations may return (nil, nil) if no tls.Config can be provided and no error occurred.
	// For example when no path to a certificate is provided (empty string), instead of a wrong
	// path which will generate an error.
	ProvideClientTLS() (*tls.Config, error)
}

// ClientCertificateProvider is a struct implementing ClientTLSProvider.
type ClientCertificateProvider struct {
	// Certificate is either a path to a file or the content of the file instead.
	//
	// It must contain, in order, in PEM format the certificate, key and any additional
	// intermediate CA certificate signing this certificate.
	//
	// Default: ""
	Certificate string `json:"certificate"`

	// RootCertificates is either a path to a file or the content of the file instead.
	//
	// It must contain in PEM format a list of CA certificates to add to the certificate pool,
	// permitting to authenticate a remote server.
	//
	// Default: ""
	RootCertificates string `json:"root_certificates"`

	// IncludeSystemCertificates allows to inherit CA certificates from the system.
	//
	// Default: false
	IncludeSystemCertificates bool `json:"include_system_certificates"`

	// TLSMinVersion allows to set TLS minimum version.
	//
	// Default: tls.VersionTLS12
	TLSMinVersion uint16 `json:"tls_min_version"`
}

var _ ClientTLSProvider = &ClientCertificateProvider{}

// ProvideClientTLS implements [ClientTLSProvider].
func (p *ClientCertificateProvider) ProvideClientTLS() (*tls.Config, error) {
	if p.TLSMinVersion == 0 {
		p.TLSMinVersion = tls.VersionTLS12
	}

	var tlsConfig *tls.Config

	initTLS := func() error {
		if tlsConfig == nil {
			tlsConfig = &tls.Config{ //nolint:gosec // This is a user input
				MinVersion: p.TLSMinVersion,
			}
			if p.IncludeSystemCertificates {
				root, err := x509.SystemCertPool()
				if err != nil {
					return fmt.Errorf("clienttls: cannot load system root certificates: %w", err)
				}
				tlsConfig.RootCAs = root
			} else {
				tlsConfig.RootCAs = x509.NewCertPool()
			}
		}
		return nil
	}

	if p.IncludeSystemCertificates {
		if err := initTLS(); err != nil {
			return nil, err
		}
	}

	setCertificate := func(cert tls.Certificate) error {
		if err := initTLS(); err != nil {
			return err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		return nil
	}

	if p.Certificate != "" {
		if block, _ := pem.Decode([]byte(p.Certificate)); block != nil {
			if cert, err := tls.X509KeyPair([]byte(p.Certificate), []byte(p.Certificate)); err != nil {
				return nil, fmt.Errorf("clienttls: cannot load TLS key pair from Certificate: %w", err)
			} else if err := setCertificate(cert); err != nil {
				return nil, err
			}
		} else if cert, err := tls.LoadX509KeyPair(p.Certificate, p.Certificate); err != nil {
			return nil, fmt.Errorf("clienttls: cannot load TLS key pair from Certificate: %w", err)
		} else if err := setCertificate(cert); err != nil {
			return nil, err
		}
	}

	appendRootAndReturn := func(cas []byte) (*tls.Config, error) {
		if err := initTLS(); err != nil {
			return nil, err
		}
		if !tlsConfig.RootCAs.AppendCertsFromPEM(cas) {
			return nil, errors.New("clienttls: failed to append root certificates")
		}
		return tlsConfig, nil
	}

	if p.RootCertificates != "" {
		if block, _ := pem.Decode([]byte(p.RootCertificates)); block != nil {
			return appendRootAndReturn([]byte(p.RootCertificates))
		} else if file, err := os.ReadFile(filepath.Clean(p.RootCertificates)); err != nil {
			return nil, fmt.Errorf("clienttls: cannot load file for root certificates: %w", err)
		} else if block, _ := pem.Decode(file); block != nil {
			return appendRootAndReturn(file)
		}
		return nil, errors.New("clienttls: unable to load root certificate")
	}

	return tlsConfig, nil
}
