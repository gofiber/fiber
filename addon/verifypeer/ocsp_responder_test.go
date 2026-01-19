// This file is based on code from github.com/grimm-co/GOCSP-responder.
// Original Copyright (c) 2016 SMFS Inc. DBA GRIMM https://grimm-co.com
//
// Licensed under the MIT License.
// ---------------------------------------------------------
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package verifypeer_test

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/fiber/v3/log"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/ocsp"
)

const (
	StatusValid   = 'V'
	StatusRevoked = 'R'
	StatusExpired = 'E'
)

type OCSPConfig struct {
	IndexFile    string
	RespKeyFile  string
	RespCertFile string
	CaCertFile   string
	Strict       bool
}

//nolint:govet //Testing
type OCSPResponder struct {
	*fiber.App
	config       *OCSPConfig
	indexEntries []IndexEntry
	indexModTime time.Time
	caCert       *x509.Certificate
	respCert     *x509.Certificate
}

//nolint:govet //Testing
type IndexEntry struct {
	Status byte
	Serial *big.Int // wow I totally called it
	// revocation reason may need to be added
	IssueTime         time.Time
	RevocationTime    time.Time
	DistinguishedName string
}

func NewOCSPResponder() (*OCSPResponder, error) {
	config := &OCSPConfig{
		IndexFile:    "../../.github/testdata/pki/intermediate/index.txt",
		RespKeyFile:  "../../.github/testdata/pki/intermediate/server/key.pem",
		RespCertFile: "../../.github/testdata/pki/intermediate/server/cert.pem",
		CaCertFile:   "../../.github/testdata/pki/intermediate/cacert.pem",
		Strict:       true,
	}
	cacert, err := parseCertFile(config.CaCertFile)
	if err != nil {
		return nil, err
	}
	respcert, err := parseCertFile(config.RespCertFile)
	if err != nil {
		return nil, err
	}

	o := &OCSPResponder{
		App:          fiber.New(),
		config:       config,
		indexModTime: time.Time{},
		caCert:       cacert,
		respCert:     respcert,
	}

	o.Post("/", o.Handle)

	return o, nil
}

func (o *OCSPResponder) Handle(c fiber.Ctx) error {
	if o.config.Strict && c.Get("Content-Type") != "application/ocsp-request" {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	c.Set("Content-Type", "application/ocsp-response")
	resp, err := o.Verify(c.Body())
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	return c.Send(resp)
}

func (o *OCSPResponder) Verify(rawreq []byte) ([]byte, error) {
	var status int
	var revokedAt time.Time

	req, err := ocsp.ParseRequest(rawreq)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// make sure the request is valid
	if err = o.verifyIssuer(req); err != nil {
		log.Errorf("verify issuer: %v", err)
		return nil, err
	}

	// get the index entry, if it exists
	ent, err := o.getIndexEntry(req.SerialNumber)
	if err != nil {
		log.Errorf("get index entry: %v", err)
		status = ocsp.Unknown
	} else {
		log.Info("Found entry %+v", ent)
		switch ent.Status {
		case StatusRevoked:
			log.Warn("This certificate is revoked")
			status = ocsp.Revoked
			revokedAt = ent.RevocationTime
		case StatusValid:
			log.Info("This certificate is valid")
			status = ocsp.Good
		default:
		}
	}

	// parse key file
	// perhaps I should zero this out after use
	keyi, err := parseKeyFile(o.config.RespKeyFile)
	if err != nil {
		return nil, err
	}
	key, ok := keyi.(crypto.Signer)
	if !ok {
		return nil, errors.New("tls: Could not make key a signer")
	}

	// construct response template
	template := ocsp.Response{
		Status:           status,
		SerialNumber:     req.SerialNumber,
		Certificate:      o.respCert,
		RevocationReason: ocsp.Unspecified,
		IssuerHash:       req.HashAlgorithm,
		RevokedAt:        revokedAt,
		ThisUpdate:       time.Now().AddDate(0, 0, -1).UTC(),
		// adding 1 day after the current date. This ocsp library sets the default date to epoch which makes ocsp clients freak out.
		NextUpdate: time.Now().AddDate(0, 0, 1).UTC(),
	}

	// make a response to return
	resp, err := ocsp.CreateResponse(o.caCert, o.respCert, template, key)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return resp, nil
}

func (o *OCSPResponder) verifyIssuer(req *ocsp.Request) error {
	h := req.HashAlgorithm.New()
	if _, err := h.Write(o.caCert.RawSubject); err != nil {
		return fmt.Errorf("%w", err)
	}
	if !bytes.Equal(h.Sum(nil), req.IssuerNameHash) {
		return errors.New("tls: Issuer name does not match")
	}
	h.Reset()
	var publicKeyInfo struct {
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}
	if _, err := asn1.Unmarshal(o.caCert.RawSubjectPublicKeyInfo, &publicKeyInfo); err != nil {
		return fmt.Errorf("%w", err)
	}
	if _, err := h.Write(publicKeyInfo.PublicKey.RightAlign()); err != nil {
		return fmt.Errorf("%w", err)
	}
	if !bytes.Equal(h.Sum(nil), req.IssuerKeyHash) {
		return errors.New("tls: Issuer key hash does not match")
	}
	return nil
}

// function to parse the index file
func (o *OCSPResponder) parseIndex() error {
	t := "060102150405Z"
	finfo, err := os.Stat(o.config.IndexFile)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// if the file modtime has changed, then reload the index file
	if !finfo.ModTime().After(o.indexModTime) {
		// the index has not changed. just return
		return nil
	}

	o.indexModTime = finfo.ModTime()
	// clear index entries
	o.indexEntries = o.indexEntries[:0]

	// open and parse the index file
	file, err := os.Open(o.config.IndexFile)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	//nolint:errcheck //Testing
	defer file.Close()
	s := bufio.NewScanner(file)
	for s.Scan() {
		var ie IndexEntry
		ln := strings.Fields(s.Text())
		ie.Status = []byte(ln[0])[0]
		ie.IssueTime, _ = time.Parse(t, ln[1]) //nolint:errcheck //Testing
		switch ie.Status {
		case StatusValid:
			ie.Serial, _ = new(big.Int).SetString(ln[2], 16)
			ie.DistinguishedName = ln[4]
			ie.RevocationTime = time.Time{} // doesn't matter
		case StatusRevoked:
			ie.Serial, _ = new(big.Int).SetString(ln[3], 16)
			ie.DistinguishedName = ln[5]
			ie.RevocationTime, _ = time.Parse(t, ln[2]) //nolint:errcheck //Testing
		default:
			continue
		}
		o.indexEntries = append(o.indexEntries, ie)
	}
	return nil
}

// updates the index if necessary and then searches for the given index in the
// index list
func (o *OCSPResponder) getIndexEntry(s *big.Int) (*IndexEntry, error) {
	log.Info("Looking for serial 0x%x", s)
	if err := o.parseIndex(); err != nil {
		return nil, err
	}
	for _, entry := range o.indexEntries {
		if entry.Serial.Cmp(s) == 0 {
			return &entry, nil
		}
	}
	return nil, fmt.Errorf("tls: Serial 0x%x not found", s)
}

// parses a pem encoded x509 certificate
func parseCertFile(filename string) (*x509.Certificate, error) {
	ct, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	block, _ := pem.Decode(ct)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return cert, nil
}

// parses a PEM encoded PKCS8 private key (RSA only)
func parseKeyFile(filename string) (any, error) {
	kt, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	block, _ := pem.Decode(kt)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return key, nil
}

func get(tlsProvider client.ClientTLSProvider, url string) error {
	cc := client.NewWithClient(
		&fasthttp.Client{
			MaxIdemponentCallAttempts: 1,
		},
	)
	tlsConfig, err := tlsProvider.ProvideClientTLS()
	if err != nil {
		return err
	}
	cc.SetTLSConfig(tlsConfig)
	_, err = cc.Get(url)
	return err
}
