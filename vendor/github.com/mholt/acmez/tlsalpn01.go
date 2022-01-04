// Copyright 2020 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package acmez

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/mholt/acmez/acme"
)

// TLSALPN01ChallengeCert creates a certificate that can be used for
// handshakes while solving the tls-alpn-01 challenge. See RFC 8737 §3.
func TLSALPN01ChallengeCert(challenge acme.Challenge) (*tls.Certificate, error) {
	keyAuthSum := sha256.Sum256([]byte(challenge.KeyAuthorization))
	keyAuthSumASN1, err := asn1.Marshal(keyAuthSum[:sha256.Size])
	if err != nil {
		return nil, err
	}

	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	challengeKeyASN1, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "ACME challenge"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour * 365),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{challenge.Identifier.Value},

		// add key authentication digest as the acmeValidation-v1 extension
		// (marked as critical such that it won't be used by non-ACME software).
		// Reference: https://www.rfc-editor.org/rfc/rfc8737.html#section-3
		ExtraExtensions: []pkix.Extension{
			{
				Id:       idPEACMEIdentifierV1,
				Critical: true,
				Value:    keyAuthSumASN1,
			},
		},
	}
	challengeCertDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &certKey.PublicKey, certKey)
	if err != nil {
		return nil, err
	}

	challengeCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: challengeCertDER})
	challengeKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: challengeKeyASN1})

	cert, err := tls.X509KeyPair(challengeCertPEM, challengeKeyPEM)
	if err != nil {
		return nil, err
	}

	return &cert, nil
}

// ACMETLS1Protocol is the ALPN value for the TLS-ALPN challenge
// handshake. See RFC 8737 §6.2.
const ACMETLS1Protocol = "acme-tls/1"

// idPEACMEIdentifierV1 is the SMI Security for PKIX Certification Extension OID referencing the ACME extension.
// See RFC 8737 §6.1. https://www.rfc-editor.org/rfc/rfc8737.html#section-6.1
var idPEACMEIdentifierV1 = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 31}
