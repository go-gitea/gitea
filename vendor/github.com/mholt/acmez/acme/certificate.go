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

package acme

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
)

// Certificate represents a certificate chain, which we usually refer
// to as "a certificate" because in practice an end-entity certificate
// is seldom useful/practical without a chain.
type Certificate struct {
	// The certificate resource URL as provisioned by
	// the ACME server. Some ACME servers may split
	// the chain into multiple URLs that are Linked
	// together, in which case this URL represents the
	// starting point.
	URL string `json:"url"`

	// The PEM-encoded certificate chain, end-entity first.
	ChainPEM []byte `json:"-"`
}

// GetCertificateChain downloads all available certificate chains originating from
// the given certURL. This is to be done after an order is finalized.
//
// "To download the issued certificate, the client simply sends a POST-
// as-GET request to the certificate URL."
//
// "The server MAY provide one or more link relation header fields
// [RFC8288] with relation 'alternate'.  Each such field SHOULD express
// an alternative certificate chain starting with the same end-entity
// certificate.  This can be used to express paths to various trust
// anchors.  Clients can fetch these alternates and use their own
// heuristics to decide which is optimal." §7.4.2
func (c *Client) GetCertificateChain(ctx context.Context, account Account, certURL string) ([]Certificate, error) {
	if err := c.provision(ctx); err != nil {
		return nil, err
	}

	var chains []Certificate

	addChain := func(certURL string) (*http.Response, error) {
		// can't pool this buffer; bytes escape scope
		buf := new(bytes.Buffer)

		// TODO: set the Accept header? ("application/pem-certificate-chain") See end of §7.4.2
		resp, err := c.httpPostJWS(ctx, account.PrivateKey, account.Location, certURL, nil, buf)
		if err != nil {
			return resp, err
		}
		contentType := parseMediaType(resp)

		switch contentType {
		case "application/pem-certificate-chain":
			chains = append(chains, Certificate{
				URL:      certURL,
				ChainPEM: buf.Bytes(),
			})
		default:
			return resp, fmt.Errorf("unrecognized Content-Type from server: %s", contentType)
		}

		// "For formats that can only express a single certificate, the server SHOULD
		// provide one or more "Link: rel="up"" header fields pointing to an
		// issuer or issuers so that ACME clients can build a certificate chain
		// as defined in TLS (see Section 4.4.2 of [RFC8446])." (end of §7.4.2)
		allUp := extractLinks(resp, "up")
		for _, upURL := range allUp {
			upCerts, err := c.GetCertificateChain(ctx, account, upURL)
			if err != nil {
				return resp, fmt.Errorf("retrieving next certificate in chain: %s: %w", upURL, err)
			}
			for _, upCert := range upCerts {
				chains[len(chains)-1].ChainPEM = append(chains[len(chains)-1].ChainPEM, upCert.ChainPEM...)
			}
		}

		return resp, nil
	}

	// always add preferred/first certificate chain
	resp, err := addChain(certURL)
	if err != nil {
		return chains, err
	}

	// "The server MAY provide one or more link relation header fields
	// [RFC8288] with relation 'alternate'.  Each such field SHOULD express
	// an alternative certificate chain starting with the same end-entity
	// certificate.  This can be used to express paths to various trust
	// anchors.  Clients can fetch these alternates and use their own
	// heuristics to decide which is optimal." §7.4.2
	alternates := extractLinks(resp, "alternate")
	for _, altURL := range alternates {
		_, err = addChain(altURL)
		if err != nil {
			return nil, fmt.Errorf("retrieving alternate certificate chain at %s: %w", altURL, err)
		}
	}

	return chains, nil
}

// RevokeCertificate revokes the given certificate. If the certificate key is not
// provided, then the account key is used instead. See §7.6.
func (c *Client) RevokeCertificate(ctx context.Context, account Account, cert *x509.Certificate, certKey crypto.Signer, reason int) error {
	if err := c.provision(ctx); err != nil {
		return err
	}

	body := struct {
		Certificate string `json:"certificate"`
		Reason      int    `json:"reason"`
	}{
		Certificate: base64.RawURLEncoding.EncodeToString(cert.Raw),
		Reason:      reason,
	}

	// "Revocation requests are different from other ACME requests in that
	// they can be signed with either an account key pair or the key pair in
	// the certificate." §7.6
	kid := ""
	if certKey == account.PrivateKey {
		kid = account.Location
	}

	_, err := c.httpPostJWS(ctx, certKey, kid, c.dir.RevokeCert, body, nil)
	return err
}

// Reasons for revoking a certificate, as defined
// by RFC 5280 §5.3.1.
// https://tools.ietf.org/html/rfc5280#section-5.3.1
const (
	ReasonUnspecified          = iota // 0
	ReasonKeyCompromise               // 1
	ReasonCACompromise                // 2
	ReasonAffiliationChanged          // 3
	ReasonSuperseded                  // 4
	ReasonCessationOfOperation        // 5
	ReasonCertificateHold             // 6
	_                                 // 7 (unused)
	ReasonRemoveFromCRL               // 8
	ReasonPrivilegeWithdrawn          // 9
	ReasonAACompromise                // 10
)
