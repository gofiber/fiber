---
id: verifypeer
---

# VerifyPeer Addon

The VerifyPeer addon package aim is to provides additional `fiber.ServerTLSCustomizer` interface implementations to add means
to configure a `*tls.Config` 's `VerifyPeerCertificate` field with additional validation for client mTLS connection.

## MutualTLS

`MutualTLSCustomizer` is a struct implementing `fiber.ServerTLSCustomizer`, providing MutualTLS configuration to a `tls.Config` object.

```go title="Examples"
package main

import (
  "github.com/gofiber/fiber/v3"
  "github.com/gofiber/fiber/v3/addon/verifypeer"
)

func main() {
    app := fiber.New()

    app.Listen(":443",
        fiber.ListenConfig{
            TLSProvider: &fiber.ServerCertificateProvider{
                Certificate: "./certificate.pem",
            },
            TLSCustomizer: &verifypeer.MutualTLSCustomizer{
                Certificate: "./ca-cert.pem",
            },
        },
    )
}
```

## MutualTLS with CRL

`MutualTLSWithCRLCustomizer` is a struct implementing `fiber.ServerTLSCustomizer`, providing MutualTLS configuration
with a Certificate Revocation List additional check to a `tls.Config` object.

CRL can be provided via a URL, a file path or directly. If none is provided, the CRL can be fetched
from the URL defined in the CRL Distribution Endpoints.
See [OpenSSL documentation](https://docs.openssl.org/3.5/man5/x509v3_config/#crl-distribution-points) for more information.

**NOTE**: Only CRL version 2 in PEM format is supported.
See [OpenSSL documentation](https://docs.openssl.org/3.5/man1/openssl-ca/#crl-options) for more information.

```go title="Examples"
package main

import (
  "github.com/gofiber/fiber/v3"
  "github.com/gofiber/fiber/v3/addon/verifypeer"
)

func main() {
    app := fiber.New()

    app.Listen(":443",
        fiber.ListenConfig{
            TLSProvider: &fiber.ServerCertificateProvider{
                Certificate: "./certificate.pem",
            },
            TLSCustomizer: &verifypeer.MutualTLSWithCRLCustomizer{
                Certificate: "./ca-cert.pem",
                RevocationList: "./crl.pem",
            },
        },
    )
}
```

## MutualTLS with OCSP Stapling

`MutualTLSWithOCSPCustomizer` is a struct implementing `fiber.ServerTLSCustomizer`.

It provides MutualTLS configuration with OCSP Stapling to a `tls.Config` object.

The OCSP server URL can be provided or one defined in the CA certificate can be used.
See [OpenSSL documentation](https://docs.openssl.org/3.5/man5/x509v3_config/#authority-info-access) for more information.

```go title="Examples"
package main

import (
  "github.com/gofiber/fiber/v3"
  "github.com/gofiber/fiber/v3/addon/verifypeer"
)

func main() {
    app := fiber.New()

    app.Listen(":443",
        fiber.ListenConfig{
            TLSProvider: &fiber.ServerCertificateProvider{
                CertificateChain: "./certificate.pem",
            },
            TLSCustomizer: &verifypeer.MutualTLSWithOCSPCustomizer{
                Certificate: "./ca-cert.pem",
            },
        },
    )
}
```
