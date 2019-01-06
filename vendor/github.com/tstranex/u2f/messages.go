// Go FIDO U2F Library
// Copyright 2015 The Go FIDO U2F Library Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package u2f

import (
	"encoding/json"
)

// JwkKey represents a public key used by a browser for the Channel ID TLS
// extension.
type JwkKey struct {
	KTy string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// ClientData as defined by the FIDO U2F Raw Message Formats specification.
type ClientData struct {
	Typ       string          `json:"typ"`
	Challenge string          `json:"challenge"`
	Origin    string          `json:"origin"`
	CIDPubKey json.RawMessage `json:"cid_pubkey"`
}

// RegisterRequest as defined by the FIDO U2F Javascript API 1.1.
type RegisterRequest struct {
	Version   string `json:"version"`
	Challenge string `json:"challenge"`
}

// WebRegisterRequest contains the parameters needed for the u2f.register()
// high-level Javascript API function as defined by the
// FIDO U2F Javascript API 1.1.
type WebRegisterRequest struct {
	AppID            string            `json:"appId"`
	RegisterRequests []RegisterRequest `json:"registerRequests"`
	RegisteredKeys   []RegisteredKey   `json:"registeredKeys"`
}

// RegisterResponse as defined by the FIDO U2F Javascript API 1.1.
type RegisterResponse struct {
	Version          string `json:"version"`
	RegistrationData string `json:"registrationData"`
	ClientData       string `json:"clientData"`
}

// RegisteredKey as defined by the FIDO U2F Javascript API 1.1.
type RegisteredKey struct {
	Version   string `json:"version"`
	KeyHandle string `json:"keyHandle"`
	AppID     string `json:"appId"`
}

// WebSignRequest contains the parameters needed for the u2f.sign()
// high-level Javascript API function as defined by the
// FIDO U2F Javascript API 1.1.
type WebSignRequest struct {
	AppID          string          `json:"appId"`
	Challenge      string          `json:"challenge"`
	RegisteredKeys []RegisteredKey `json:"registeredKeys"`
}

// SignResponse as defined by the FIDO U2F Javascript API 1.1.
type SignResponse struct {
	KeyHandle     string `json:"keyHandle"`
	SignatureData string `json:"signatureData"`
	ClientData    string `json:"clientData"`
}

// TrustedFacets as defined by the FIDO AppID and Facet Specification.
type TrustedFacets struct {
	Version struct {
		Major int `json:"major"`
		Minor int `json:"minor"`
	} `json:"version"`
	Ids []string `json:"ids"`
}

// TrustedFacetsEndpoint is a container of TrustedFacets.
// It is used as the response for an appId URL endpoint.
type TrustedFacetsEndpoint struct {
	TrustedFacets []TrustedFacets `json:"trustedFacets"`
}
