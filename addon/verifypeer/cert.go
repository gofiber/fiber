package verifypeer

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
)

// MutualTLSCustomizer is a struct implementing [fiber.ServerCertificateCustomizer].
type MutualTLSCustomizer struct {
	// Certificate is either a path to a file or the content of the file instead.
	// It must be a PEM encoded CA certificate.
	//
	// Default: ""
	Certificate string `json:"certificate"`
}

var _ fiber.ServerTLSCustomizer = &MutualTLSCustomizer{}

// CustomizeServerTLS implements [fiber.ServerTLSCustomizer].
//
// It parse the ClientCertificate, either a file path or its content, a PEM encoded CA certificate.
// It will add the certiticate to config.ClientCAs and set config.ClientAuth to tls.RequireAndVerifyClientCert
func (c *MutualTLSCustomizer) CustomizeServerTLS(cfg *tls.Config) error {
	certificate, err := c.ClientCertificate()
	if err != nil {
		return err
	} else if certificate == nil {
		return nil
	}
	if cfg.ClientCAs == nil {
		cfg.ClientCAs = x509.NewCertPool()
	}
	cfg.ClientAuth = tls.RequireAndVerifyClientCert
	cfg.ClientCAs.AddCert(certificate)
	return nil
}

// ClientCertificate provide a way to retrieve the configured Client CA certificate.
func (c *MutualTLSCustomizer) ClientCertificate() (*x509.Certificate, error) {
	var certificate *x509.Certificate

	var block *pem.Block

	if c.Certificate != "" {
		if b, _ := pem.Decode([]byte(c.Certificate)); b != nil {
			block = b
		} else if file, err := os.ReadFile(filepath.Clean(c.Certificate)); err != nil {
			return nil, fmt.Errorf("tls: failed to read file from path: %w", err)
		} else if b, _ := pem.Decode(file); b != nil {
			block = b
		}
	}

	if block != nil && block.Type == "CERTIFICATE" {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("tls: cannot load client CA certificate: %w", err)
		}
		certificate = cert
	}

	return certificate, nil
}
