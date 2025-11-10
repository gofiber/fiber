package tlstest

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"
)

// GetTLSConfigs generates TLS configurations for a test server and client that
// trust each other using an in-memory certificate authority.
func GetTLSConfigs() (serverTLSConf, clientTLSConf *tls.Config, err error) {
	// set up our CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2021),
		Subject: pkix.Name{
			Organization:  []string{"Fiber"},
			Country:       []string{"NL"},
			Province:      []string{""},
			Locality:      []string{"Amsterdam"},
			StreetAddress: []string{"Huidenstraat"},
			PostalCode:    []string{"1011 AA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("generate CA key: %w", err)
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create CA certificate: %w", err)
	}

	// pem encode
	var caPEM bytes.Buffer
	if err = pem.Encode(&caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	}); err != nil {
		return nil, nil, fmt.Errorf("encode CA cert: %w", err)
	}

	var caPrivKeyPEM bytes.Buffer
	if err = pem.Encode(&caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey),
	}); err != nil {
		return nil, nil, fmt.Errorf("encode CA private key: %w", err)
	}

	// set up our server certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2021),
		Subject: pkix.Name{
			Organization:  []string{"Fiber"},
			Country:       []string{"NL"},
			Province:      []string{""},
			Locality:      []string{"Amsterdam"},
			StreetAddress: []string{"Huidenstraat"},
			PostalCode:    []string{"1011 AA"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("generate server key: %w", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create server certificate: %w", err)
	}

	var certPEM bytes.Buffer
	if err = pem.Encode(&certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return nil, nil, fmt.Errorf("encode server cert: %w", err)
	}

	var certPrivateKeyPEM bytes.Buffer
	if err = pem.Encode(&certPrivateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivateKey),
	}); err != nil {
		return nil, nil, fmt.Errorf("encode server private key: %w", err)
	}

	serverCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivateKeyPEM.Bytes())
	if err != nil {
		return nil, nil, fmt.Errorf("load server key pair: %w", err)
	}

	serverTLSConf = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS12,
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caPEM.Bytes()); !ok {
		return nil, nil, errors.New("append CA cert to cert pool")
	}
	clientTLSConf = &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	return serverTLSConf, clientTLSConf, nil
}
