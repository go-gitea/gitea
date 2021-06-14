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
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Account represents a set of metadata associated with an account
// as defined by the ACME spec §7.1.2:
// https://tools.ietf.org/html/rfc8555#section-7.1.2
type Account struct {
	// status (required, string):  The status of this account.  Possible
	// values are "valid", "deactivated", and "revoked".  The value
	// "deactivated" should be used to indicate client-initiated
	// deactivation whereas "revoked" should be used to indicate server-
	// initiated deactivation.  See Section 7.1.6.
	Status string `json:"status"`

	// contact (optional, array of string):  An array of URLs that the
	// server can use to contact the client for issues related to this
	// account.  For example, the server may wish to notify the client
	// about server-initiated revocation or certificate expiration.  For
	// information on supported URL schemes, see Section 7.3.
	Contact []string `json:"contact,omitempty"`

	// termsOfServiceAgreed (optional, boolean):  Including this field in a
	// newAccount request, with a value of true, indicates the client's
	// agreement with the terms of service.  This field cannot be updated
	// by the client.
	TermsOfServiceAgreed bool `json:"termsOfServiceAgreed,omitempty"`

	// externalAccountBinding (optional, object):  Including this field in a
	// newAccount request indicates approval by the holder of an existing
	// non-ACME account to bind that account to this ACME account.  This
	// field is not updateable by the client (see Section 7.3.4).
	//
	// Use SetExternalAccountBinding() to set this field's value properly.
	ExternalAccountBinding json.RawMessage `json:"externalAccountBinding,omitempty"`

	// orders (required, string):  A URL from which a list of orders
	// submitted by this account can be fetched via a POST-as-GET
	// request, as described in Section 7.1.2.1.
	Orders string `json:"orders"`

	// In response to new-account, "the server returns this account
	// object in a 201 (Created) response, with the account URL
	// in a Location header field." §7.3
	//
	// We transfer the value from the header to this field for
	// storage and recall purposes.
	Location string `json:"location,omitempty"`

	// The private key to the account. Because it is secret, it is
	// not serialized as JSON and must be stored separately (usually
	// a PEM-encoded file).
	PrivateKey crypto.Signer `json:"-"`
}

// SetExternalAccountBinding sets the ExternalAccountBinding field of the account.
// It only sets the field value; it does not register the account with the CA. (The
// client parameter is necessary because the EAB encoding depends on the directory.)
func (a *Account) SetExternalAccountBinding(ctx context.Context, client *Client, eab EAB) error {
	if err := client.provision(ctx); err != nil {
		return err
	}

	macKey, err := base64.RawURLEncoding.DecodeString(eab.MACKey)
	if err != nil {
		return fmt.Errorf("base64-decoding MAC key: %w", err)
	}

	eabJWS, err := jwsEncodeEAB(a.PrivateKey.Public(), macKey, keyID(eab.KeyID), client.dir.NewAccount)
	if err != nil {
		return fmt.Errorf("signing EAB content: %w", err)
	}

	a.ExternalAccountBinding = eabJWS

	return nil
}

// NewAccount creates a new account on the ACME server.
//
// "A client creates a new account with the server by sending a POST
// request to the server's newAccount URL." §7.3
func (c *Client) NewAccount(ctx context.Context, account Account) (Account, error) {
	if err := c.provision(ctx); err != nil {
		return account, err
	}
	return c.postAccount(ctx, c.dir.NewAccount, accountObject{Account: account})
}

// GetAccount looks up an account on the ACME server.
//
// "If a client wishes to find the URL for an existing account and does
// not want an account to be created if one does not already exist, then
// it SHOULD do so by sending a POST request to the newAccount URL with
// a JWS whose payload has an 'onlyReturnExisting' field set to 'true'."
// §7.3.1
func (c *Client) GetAccount(ctx context.Context, account Account) (Account, error) {
	if err := c.provision(ctx); err != nil {
		return account, err
	}
	return c.postAccount(ctx, c.dir.NewAccount, accountObject{
		Account:            account,
		OnlyReturnExisting: true,
	})
}

// UpdateAccount updates account information on the ACME server.
//
// "If the client wishes to update this information in the future, it
// sends a POST request with updated information to the account URL.
// The server MUST ignore any updates to the 'orders' field,
// 'termsOfServiceAgreed' field (see Section 7.3.3), the 'status' field
// (except as allowed by Section 7.3.6), or any other fields it does not
// recognize." §7.3.2
//
// This method uses the account.Location value as the account URL.
func (c *Client) UpdateAccount(ctx context.Context, account Account) (Account, error) {
	return c.postAccount(ctx, account.Location, accountObject{Account: account})
}

type keyChangeRequest struct {
	Account string          `json:"account"`
	OldKey  json.RawMessage `json:"oldKey"`
}

// AccountKeyRollover changes an account's associated key.
//
// "To change the key associated with an account, the client sends a
// request to the server containing signatures by both the old and new
// keys." §7.3.5
func (c *Client) AccountKeyRollover(ctx context.Context, account Account, newPrivateKey crypto.Signer) (Account, error) {
	if err := c.provision(ctx); err != nil {
		return account, err
	}

	oldPublicKeyJWK, err := jwkEncode(account.PrivateKey.Public())
	if err != nil {
		return account, fmt.Errorf("encoding old private key: %v", err)
	}

	keyChangeReq := keyChangeRequest{
		Account: account.Location,
		OldKey:  []byte(oldPublicKeyJWK),
	}

	innerJWS, err := jwsEncodeJSON(keyChangeReq, newPrivateKey, "", "", c.dir.KeyChange)
	if err != nil {
		return account, fmt.Errorf("encoding inner JWS: %v", err)
	}

	_, err = c.httpPostJWS(ctx, account.PrivateKey, account.Location, c.dir.KeyChange, json.RawMessage(innerJWS), nil)
	if err != nil {
		return account, fmt.Errorf("rolling key on server: %w", err)
	}

	account.PrivateKey = newPrivateKey

	return account, nil

}

func (c *Client) postAccount(ctx context.Context, endpoint string, account accountObject) (Account, error) {
	// Normally, the account URL is the key ID ("kid")... except when the user
	// is trying to get the correct account URL. In that case, we must ignore
	// any existing URL we may have and not set the kid field on the request.
	// Arguably, this is a user error (spec says "If client wishes to find the
	// URL for an existing account", so why would the URL already be filled
	// out?) but it's easy enough to infer their intent and make it work.
	kid := account.Location
	if account.OnlyReturnExisting {
		kid = ""
	}

	resp, err := c.httpPostJWS(ctx, account.PrivateKey, kid, endpoint, account, &account.Account)
	if err != nil {
		return account.Account, err
	}

	account.Location = resp.Header.Get("Location")

	return account.Account, nil
}

type accountObject struct {
	Account

	// If true, newAccount will be read-only, and Account.Location
	// (which holds the account URL) must be empty.
	OnlyReturnExisting bool `json:"onlyReturnExisting,omitempty"`
}

// EAB (External Account Binding) contains information
// necessary to bind or map an ACME account to some
// other account known by the CA.
//
// External account bindings are "used to associate an
// ACME account with an existing account in a non-ACME
// system, such as a CA customer database."
//
// "To enable ACME account binding, the CA operating the
// ACME server needs to provide the ACME client with a
// MAC key and a key identifier, using some mechanism
// outside of ACME." §7.3.4
type EAB struct {
	// "The key identifier MUST be an ASCII string." §7.3.4
	KeyID string `json:"key_id"`

	// "The MAC key SHOULD be provided in base64url-encoded
	// form, to maximize compatibility between non-ACME
	// provisioning systems and ACME clients." §7.3.4
	MACKey string `json:"mac_key"`
}

// Possible status values. From several spec sections:
// - Account §7.1.2 (valid, deactivated, revoked)
// - Order §7.1.3 (pending, ready, processing, valid, invalid)
// - Authorization §7.1.4 (pending, valid, invalid, deactivated, expired, revoked)
// - Challenge §7.1.5 (pending, processing, valid, invalid)
// - Status changes §7.1.6
const (
	StatusPending     = "pending"
	StatusProcessing  = "processing"
	StatusValid       = "valid"
	StatusInvalid     = "invalid"
	StatusDeactivated = "deactivated"
	StatusExpired     = "expired"
	StatusRevoked     = "revoked"
	StatusReady       = "ready"
)
