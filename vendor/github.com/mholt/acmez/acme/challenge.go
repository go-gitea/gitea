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
	"crypto/sha256"
	"encoding/base64"
)

// Challenge holds information about an ACME challenge.
//
// "An ACME challenge object represents a server's offer to validate a
// client's possession of an identifier in a specific way.  Unlike the
// other objects listed above, there is not a single standard structure
// for a challenge object.  The contents of a challenge object depend on
// the validation method being used.  The general structure of challenge
// objects and an initial set of validation methods are described in
// Section 8." §7.1.5
type Challenge struct {
	// "Challenge objects all contain the following basic fields..." §8

	// type (required, string):  The type of challenge encoded in the
	// object.
	Type string `json:"type"`

	// url (required, string):  The URL to which a response can be posted.
	URL string `json:"url"`

	// status (required, string):  The status of this challenge.  Possible
	// values are "pending", "processing", "valid", and "invalid" (see
	// Section 7.1.6).
	Status string `json:"status"`

	// validated (optional, string):  The time at which the server validated
	// this challenge, encoded in the format specified in [RFC3339].
	// This field is REQUIRED if the "status" field is "valid".
	Validated string `json:"validated,omitempty"`

	// error (optional, object):  Error that occurred while the server was
	// validating the challenge, if any, structured as a problem document
	// [RFC7807].  Multiple errors can be indicated by using subproblems
	// Section 6.7.1.  A challenge object with an error MUST have status
	// equal to "invalid".
	Error *Problem `json:"error,omitempty"`

	// "All additional fields are specified by the challenge type." §8
	// (We also add our own for convenience.)

	// "The token for a challenge is a string comprised entirely of
	// characters in the URL-safe base64 alphabet." §8.1
	//
	// Used by the http-01, tls-alpn-01, and dns-01 challenges.
	Token string `json:"token,omitempty"`

	// A key authorization is a string that concatenates the token for the
	// challenge with a key fingerprint, separated by a "." character (§8.1):
	//
	//     keyAuthorization = token || '.' || base64url(Thumbprint(accountKey))
	//
	// This client package automatically assembles and sets this value for you.
	KeyAuthorization string `json:"keyAuthorization,omitempty"`

	// We attach the identifier that this challenge is associated with, which
	// may be useful information for solving a challenge. It is not part of the
	// structure as defined by the spec but is added by us to provide enough
	// information to solve the DNS-01 challenge.
	Identifier Identifier `json:"identifier,omitempty"`
}

// HTTP01ResourcePath returns the URI path for solving the http-01 challenge.
//
// "The path at which the resource is provisioned is comprised of the
// fixed prefix '/.well-known/acme-challenge/', followed by the 'token'
// value in the challenge." §8.3
func (c Challenge) HTTP01ResourcePath() string {
	return "/.well-known/acme-challenge/" + c.Token
}

// DNS01TXTRecordName returns the name of the TXT record to create for
// solving the dns-01 challenge.
//
// "The client constructs the validation domain name by prepending the
// label '_acme-challenge' to the domain name being validated, then
// provisions a TXT record with the digest value under that name." §8.4
func (c Challenge) DNS01TXTRecordName() string {
	return "_acme-challenge." + c.Identifier.Value
}

// DNS01KeyAuthorization encodes a key authorization value to be used
// in a TXT record for the _acme-challenge DNS record.
//
// "A client fulfills this challenge by constructing a key authorization
// from the 'token' value provided in the challenge and the client's
// account key.  The client then computes the SHA-256 digest [FIPS180-4]
// of the key authorization.
//
// The record provisioned to the DNS contains the base64url encoding of
// this digest." §8.4
func (c Challenge) DNS01KeyAuthorization() string {
	h := sha256.Sum256([]byte(c.KeyAuthorization))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// InitiateChallenge "indicates to the server that it is ready for the challenge
// validation by sending an empty JSON body ('{}') carried in a POST request to
// the challenge URL (not the authorization URL)." §7.5.1
func (c *Client) InitiateChallenge(ctx context.Context, account Account, challenge Challenge) (Challenge, error) {
	if err := c.provision(ctx); err != nil {
		return Challenge{}, err
	}
	_, err := c.httpPostJWS(ctx, account.PrivateKey, account.Location, challenge.URL, struct{}{}, &challenge)
	return challenge, err
}

// The standard or well-known ACME challenge types.
const (
	ChallengeTypeHTTP01    = "http-01"     // RFC 8555 §8.3
	ChallengeTypeDNS01     = "dns-01"      // RFC 8555 §8.4
	ChallengeTypeTLSALPN01 = "tls-alpn-01" // RFC 8737 §3
)
