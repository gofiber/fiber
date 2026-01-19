package verifypeer

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
)

// MutualTLSWithCRLCustomizer is a struct implementing [fiber.ServerTLSCustomizer].
//
// It provides MutualTLS configuration with Certificate Revocation List check to a [tls.Config] object.
//
// CRL can be provided via a URL, a file path or directly. If none is provided, the CRL can be fetched
// from the URL defined in the CRL Distribution Endpoints.
// See: https://docs.openssl.org/3.5/man5/x509v3_config/#crl-distribution-points
//
// NOTE: Only CRL version 2 in PEM format is supported.
// See: https://docs.openssl.org/3.5/man1/openssl-ca/#crl-options
type MutualTLSWithCRLCustomizer struct {
	// Certificate is either a path to a file or the content of the file instead.
	// It must be a PEM encoded CA certificate.
	//
	// Default: ""
	Certificate string `json:"certificate"`

	// TLSProvider is an optional ClientTLSProvider to configure the Fiber Client that will fetch a CRL.
	//
	// Default: nil
	TLSProvider client.ClientTLSProvider `json:"tls_provider"`

	// RevocationList is either a URL to, a path to or a CRL directly.
	//
	// Default: ""
	RevocationList string `json:"revocation_list"`

	// Default: 10 * time.Second
	Timeout time.Duration `json:"timeout"`
}

var _ fiber.ServerTLSCustomizer = &MutualTLSWithCRLCustomizer{}

// CustomizeServerTLS implements [fiber.ServerCertificateCustomizer]
func (m *MutualTLSWithCRLCustomizer) CustomizeServerTLS(config *tls.Config) error {
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

	var crlBytes []byte

	if m.RevocationList != "" {
		if bytes, ok := m.decodePemCrl([]byte(m.RevocationList)); ok {
			crlBytes = bytes
		} else if distURL, err := m.validateFetchURL(m.RevocationList); err == nil {
			file, err := m.fetchCRL(distURL)
			if err != nil {
				return err
			}
			crlBytes = file
		} else if file, err := os.ReadFile(filepath.Clean(m.RevocationList)); err != nil {
			return fmt.Errorf("tls: failed to read CRL file from path=%q: %w", m.RevocationList, err)
		} else if bytes, ok := m.decodePemCrl(file); ok {
			crlBytes = bytes
		} else {
			return fmt.Errorf("tls: failed to parse CRL from path=%q", m.RevocationList)
		}
	} else {
		for _, dist := range clientCACert.CRLDistributionPoints {
			if distURL, err := m.validateFetchURL(dist); err == nil {
				if crl, err := m.fetchCRL(distURL); err == nil {
					crlBytes = crl
					break
				}
			}
		}
	}

	var clientCRL *x509.RevocationList

	if len(crlBytes) > 0 {
		crl, err := x509.ParseRevocationList(crlBytes)
		if err != nil {
			return fmt.Errorf("tls: unable to load CRL: %w", err)
		}
		if err := crl.CheckSignatureFrom(clientCACert); err != nil {
			return fmt.Errorf("tls: invalid CRL signature: %w", err)
		}
		clientCRL = crl
	}

	if clientCRL != nil {
		config.VerifyPeerCertificate = func(_ [][]byte, verifiedChains [][]*x509.Certificate) error {
			cert := verifiedChains[0][0]
			for _, revokedCertificate := range clientCRL.RevokedCertificateEntries {
				if revokedCertificate.SerialNumber.Cmp(cert.SerialNumber) == 0 {
					return errors.New("tls: the certificate was revoked")
				}
			}
			return nil
		}
	}

	return nil
}

func (*MutualTLSWithCRLCustomizer) decodePemCrl(crl []byte) ([]byte, bool) {
	if block, _ := pem.Decode(crl); block != nil && block.Type == "X509 CRL" {
		return block.Bytes, true
	}
	return []byte{}, false
}

func (*MutualTLSWithCRLCustomizer) validateFetchURL(dist string) (*url.URL, error) {
	distURL, err := url.Parse(dist)
	if err != nil {
		return nil, fmt.Errorf("tls: %w", err)
	}
	if !slices.Contains([]string{"http", "https"}, distURL.Scheme) {
		return nil, fmt.Errorf("tls: CRL fetching client: wrong scheme=%q, only 'http' or 'https' is supported", distURL.Scheme)
	}
	return distURL, nil
}

func (m *MutualTLSWithCRLCustomizer) fetchCRL(distURL *url.URL) ([]byte, error) {
	cc := client.New()
	cc.SetTimeout(m.Timeout)

	if distURL.Scheme == "https" && m.TLSProvider != nil {
		tlsConfig, err := m.TLSProvider.ProvideClientTLS()
		if err != nil {
			return []byte{}, err
		}
		cc.SetTLSConfig(tlsConfig)
	}

	resp, err := cc.Get(distURL.String())
	if err != nil {
		return []byte{}, fmt.Errorf("tls: unable to fetch certificate revocation list from url=%q: %w", distURL.String(), err)
	}
	defer resp.Close()
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return []byte{}, fmt.Errorf("tls: failed to fetch CRL from url=%q: HTTP status %d", distURL.String(), resp.StatusCode())
	}

	if bytes, ok := m.decodePemCrl(resp.Body()); ok {
		return bytes, nil
	}
	return []byte{}, fmt.Errorf("tls: unable to parse PEM CRL from url=%q", distURL.String())
}
