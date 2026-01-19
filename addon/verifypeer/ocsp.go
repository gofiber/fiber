package verifypeer

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"golang.org/x/crypto/ocsp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
)

// MutualTLSWithOCSPCustomizer is a struct implementing [fiber.ServerTLSCustomizer].
//
// It provides MutualTLS configuration with OCSP Stapling to a [tls.Config] object.
//
// The OCSP server URL can be provided or one defined in the CA certificate can be used.
// See: https://docs.openssl.org/3.5/man5/x509v3_config/#authority-info-access
type MutualTLSWithOCSPCustomizer struct {
	// Certificate is either a path to a file or the content of the file instead.
	// It must be a PEM encoded CA certificate.
	//
	// Default: ""
	Certificate string `json:"certificate"`

	// TLSProvider adds an optional ClientTLSProvider to configure the OCSP fetch client.
	//
	// Default: nil
	TLSProvider client.ClientTLSProvider `json:"tls_provider"`

	OCSPServer string `json:"ocsp_server"`

	// Default: 10 * time.Second
	Timeout time.Duration `json:"timeout"`
}

var _ fiber.ServerTLSCustomizer = &MutualTLSWithOCSPCustomizer{}

// CustomizeServerTLS implements [fiber.ServerTLSCustomizer].
func (m *MutualTLSWithOCSPCustomizer) CustomizeServerTLS(config *tls.Config) error {
	var clientCACert *x509.Certificate

	mtls := &MutualTLSCustomizer{
		Certificate: m.Certificate,
	}

	clientCACert, err := mtls.ClientCertificate()
	if err != nil {
		return err
	} else if clientCACert == nil {
		return nil
	}
	if config.ClientCAs == nil {
		config.ClientCAs = x509.NewCertPool()
	}
	config.ClientAuth = tls.RequireAndVerifyClientCert
	config.ClientCAs.AddCert(clientCACert)
	if m.Timeout == 0 {
		m.Timeout = 10 * time.Second
	}

	var ocspServer string

	if m.OCSPServer != "" {
		if err := m.validateOSCPServer(m.OCSPServer); err != nil {
			return err
		}
		ocspServer = m.OCSPServer
	} else {
		for _, server := range clientCACert.OCSPServer {
			if err := m.validateOSCPServer(server); err == nil {
				ocspServer = server
				break
			}
		}
	}

	if ocspServer != "" {
		cc := client.New()
		cc.SetTimeout(m.Timeout)

		//nolint:errcheck //input already verified
		ocspURL, _ := url.Parse(ocspServer)
		if ocspURL.Scheme == "https" {
			if m.TLSProvider == nil {
				return errors.New("mtls: TLSProvider is required for https OCSP server")
			}
			tlsConfig, err := m.TLSProvider.ProvideClientTLS()
			if err != nil {
				return err
			}
			cc.SetTLSConfig(tlsConfig)
		}

		config.VerifyPeerCertificate = func(_ [][]byte, verifiedChains [][]*x509.Certificate) error {
			cert := verifiedChains[0][0]
			opts := &ocsp.RequestOptions{Hash: crypto.SHA256}
			buffer, err := ocsp.CreateRequest(cert, clientCACert, opts)
			if err != nil {
				return fmt.Errorf("mtls: %w", err)
			}
			req := client.AcquireRequest()
			defer client.ReleaseRequest(req)

			req.SetClient(cc)
			req.AddHeader("Content-Type", "application/ocsp-request")
			req.AddHeader("Accept", "application/ocsp-response")
			req.SetRawBody(buffer)

			resp, err := req.Post(ocspServer)
			if err != nil {
				return err
			}
			defer resp.Close()
			ocspResponse, err := ocsp.ParseResponseForCert(resp.Body(), cert, clientCACert)
			if err != nil {
				return fmt.Errorf("mtls: %w", err)
			}

			switch ocspResponse.Status {
			case ocsp.Good:
				return nil
			case ocsp.Revoked:
				return errors.New("tls: the certificate was revoked")
			case ocsp.Unknown:
				return errors.New("tls: the certificate is unknown to OCSP server")
			}

			return nil
		}
	}

	return nil
}

func (*MutualTLSWithOCSPCustomizer) validateOSCPServer(ocspServer string) error {
	if ocspURL, err := url.Parse(ocspServer); err != nil {
		return fmt.Errorf("mtls: %w", err)
	} else if !slices.Contains([]string{"http", "https"}, ocspURL.Scheme) {
		return fmt.Errorf("tls: OCSP Stapling: wrong scheme=%q, only 'http' or 'https' is supported", ocspURL.Scheme)
	}
	return nil
}
