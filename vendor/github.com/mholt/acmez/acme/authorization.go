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
	"fmt"
	"time"
)

// Authorization "represents a server's authorization for
// an account to represent an identifier.  In addition to the
// identifier, an authorization includes several metadata fields, such
// as the status of the authorization (e.g., 'pending', 'valid', or
// 'revoked') and which challenges were used to validate possession of
// the identifier." §7.1.4
type Authorization struct {
	// identifier (required, object):  The identifier that the account is
	// authorized to represent.
	Identifier Identifier `json:"identifier"`

	// status (required, string):  The status of this authorization.
	// Possible values are "pending", "valid", "invalid", "deactivated",
	// "expired", and "revoked".  See Section 7.1.6.
	Status string `json:"status"`

	// expires (optional, string):  The timestamp after which the server
	// will consider this authorization invalid, encoded in the format
	// specified in [RFC3339].  This field is REQUIRED for objects with
	// "valid" in the "status" field.
	Expires time.Time `json:"expires,omitempty"`

	// challenges (required, array of objects):  For pending authorizations,
	// the challenges that the client can fulfill in order to prove
	// possession of the identifier.  For valid authorizations, the
	// challenge that was validated.  For invalid authorizations, the
	// challenge that was attempted and failed.  Each array entry is an
	// object with parameters required to validate the challenge.  A
	// client should attempt to fulfill one of these challenges, and a
	// server should consider any one of the challenges sufficient to
	// make the authorization valid.
	Challenges []Challenge `json:"challenges"`

	// wildcard (optional, boolean):  This field MUST be present and true
	// for authorizations created as a result of a newOrder request
	// containing a DNS identifier with a value that was a wildcard
	// domain name.  For other authorizations, it MUST be absent.
	// Wildcard domain names are described in Section 7.1.3.
	Wildcard bool `json:"wildcard,omitempty"`

	// "The server allocates a new URL for this authorization and returns a
	// 201 (Created) response with the authorization URL in the Location
	// header field" §7.4.1
	//
	// We transfer the value from the header to this field for storage and
	// recall purposes.
	Location string `json:"-"`
}

// IdentifierValue returns the Identifier.Value field, adjusted
// according to the Wildcard field.
func (authz Authorization) IdentifierValue() string {
	if authz.Wildcard {
		return "*." + authz.Identifier.Value
	}
	return authz.Identifier.Value
}

// fillChallengeFields populates extra fields in the challenge structs so that
// challenges can be solved without needing a bunch of unnecessary extra state.
func (authz *Authorization) fillChallengeFields(account Account) error {
	accountThumbprint, err := jwkThumbprint(account.PrivateKey.Public())
	if err != nil {
		return fmt.Errorf("computing account JWK thumbprint: %v", err)
	}
	for i := 0; i < len(authz.Challenges); i++ {
		authz.Challenges[i].Identifier = authz.Identifier
		if authz.Challenges[i].KeyAuthorization == "" {
			authz.Challenges[i].KeyAuthorization = authz.Challenges[i].Token + "." + accountThumbprint
		}
	}
	return nil
}

// NewAuthorization creates a new authorization for an identifier using
// the newAuthz endpoint of the directory, if available. This function
// creates authzs out of the regular order flow.
//
// "Note that because the identifier in a pre-authorization request is
// the exact identifier to be included in the authorization object, pre-
// authorization cannot be used to authorize issuance of certificates
// containing wildcard domain names." §7.4.1
func (c *Client) NewAuthorization(ctx context.Context, account Account, id Identifier) (Authorization, error) {
	if err := c.provision(ctx); err != nil {
		return Authorization{}, err
	}
	if c.dir.NewAuthz == "" {
		return Authorization{}, fmt.Errorf("server does not support newAuthz endpoint")
	}

	var authz Authorization
	resp, err := c.httpPostJWS(ctx, account.PrivateKey, account.Location, c.dir.NewAuthz, id, &authz)
	if err != nil {
		return authz, err
	}

	authz.Location = resp.Header.Get("Location")

	err = authz.fillChallengeFields(account)
	if err != nil {
		return authz, err
	}

	return authz, nil
}

// GetAuthorization fetches an authorization object from the server.
//
// "Authorization resources are created by the server in response to
// newOrder or newAuthz requests submitted by an account key holder;
// their URLs are provided to the client in the responses to these
// requests."
//
// "When a client receives an order from the server in reply to a
// newOrder request, it downloads the authorization resources by sending
// POST-as-GET requests to the indicated URLs.  If the client initiates
// authorization using a request to the newAuthz resource, it will have
// already received the pending authorization object in the response to
// that request." §7.5
func (c *Client) GetAuthorization(ctx context.Context, account Account, authzURL string) (Authorization, error) {
	if err := c.provision(ctx); err != nil {
		return Authorization{}, err
	}

	var authz Authorization
	_, err := c.httpPostJWS(ctx, account.PrivateKey, account.Location, authzURL, nil, &authz)
	if err != nil {
		return authz, err
	}

	authz.Location = authzURL

	err = authz.fillChallengeFields(account)
	if err != nil {
		return authz, err
	}

	return authz, nil
}

// PollAuthorization polls the authorization resource endpoint until the authorization is
// considered "finalized" which means that it either succeeded, failed, or was abandoned.
// It blocks until that happens or until the configured timeout.
//
// "Usually, the validation process will take some time, so the client
// will need to poll the authorization resource to see when it is
// finalized."
//
// "For challenges where the client can tell when the server
// has validated the challenge (e.g., by seeing an HTTP or DNS request
// from the server), the client SHOULD NOT begin polling until it has
// seen the validation request from the server." §7.5.1
func (c *Client) PollAuthorization(ctx context.Context, account Account, authz Authorization) (Authorization, error) {
	start, interval, maxDuration := time.Now(), c.pollInterval(), c.pollTimeout()

	if authz.Status != "" {
		if finalized, err := authzIsFinalized(authz); finalized {
			return authz, err
		}
	}

	for time.Since(start) < maxDuration {
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return authz, ctx.Err()
		}

		// get the latest authz object
		resp, err := c.httpPostJWS(ctx, account.PrivateKey, account.Location, authz.Location, nil, &authz)
		if err != nil {
			return authz, fmt.Errorf("checking authorization status: %w", err)
		}
		if finalized, err := authzIsFinalized(authz); finalized {
			return authz, err
		}

		// "The server MUST provide information about its retry state to the
		// client via the 'error' field in the challenge and the Retry-After
		// HTTP header field in response to requests to the challenge resource."
		// §8.2
		interval, err = retryAfter(resp, interval)
		if err != nil {
			return authz, err
		}
	}

	return authz, fmt.Errorf("authorization took too long")
}

// DeactivateAuthorization deactivates an authorization on the server, which is
// a good idea if the authorization is not going to be utilized by the client.
//
// "If a client wishes to relinquish its authorization to issue
// certificates for an identifier, then it may request that the server
// deactivate each authorization associated with it by sending POST
// requests with the static object {"status": "deactivated"} to each
// authorization URL." §7.5.2
func (c *Client) DeactivateAuthorization(ctx context.Context, account Account, authzURL string) (Authorization, error) {
	if err := c.provision(ctx); err != nil {
		return Authorization{}, err
	}

	if authzURL == "" {
		return Authorization{}, fmt.Errorf("empty authz url")
	}

	deactivate := struct {
		Status string `json:"status"`
	}{Status: "deactivated"}

	var authz Authorization
	_, err := c.httpPostJWS(ctx, account.PrivateKey, account.Location, authzURL, deactivate, &authz)
	authz.Location = authzURL

	return authz, err
}

// authzIsFinalized returns true if the authorization is finished,
// whether successfully or not. If not, an error will be returned.
// Post-valid statuses that make an authz unusable are treated as
// errors.
func authzIsFinalized(authz Authorization) (bool, error) {
	switch authz.Status {
	case StatusPending:
		// "Authorization objects are created in the 'pending' state." §7.1.6
		return false, nil

	case StatusValid:
		// "If one of the challenges listed in the authorization transitions
		// to the 'valid' state, then the authorization also changes to the
		// 'valid' state." §7.1.6
		return true, nil

	case StatusInvalid:
		// "If the client attempts to fulfill a challenge and fails, or if
		// there is an error while the authorization is still pending, then
		// the authorization transitions to the 'invalid' state." §7.1.6
		var firstProblem Problem
		for _, chal := range authz.Challenges {
			if chal.Error != nil {
				firstProblem = *chal.Error
				break
			}
		}
		firstProblem.Resource = authz
		return true, fmt.Errorf("authorization failed: %w", firstProblem)

	case StatusExpired, StatusDeactivated, StatusRevoked:
		// Once the authorization is in the 'valid' state, it can expire
		// ('expired'), be deactivated by the client ('deactivated', see
		// Section 7.5.2), or revoked by the server ('revoked')." §7.1.6
		return true, fmt.Errorf("authorization %s", authz.Status)

	case "":
		return false, fmt.Errorf("status unknown")

	default:
		return true, fmt.Errorf("server set unrecognized authorization status: %s", authz.Status)
	}
}
