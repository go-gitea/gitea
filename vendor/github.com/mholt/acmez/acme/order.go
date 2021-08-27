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
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// Order is an object that "represents a client's request for a certificate
// and is used to track the progress of that order through to issuance.
// Thus, the object contains information about the requested
// certificate, the authorizations that the server requires the client
// to complete, and any certificates that have resulted from this order."
// §7.1.3
type Order struct {
	// status (required, string):  The status of this order.  Possible
	// values are "pending", "ready", "processing", "valid", and
	// "invalid".  See Section 7.1.6.
	Status string `json:"status"`

	// expires (optional, string):  The timestamp after which the server
	// will consider this order invalid, encoded in the format specified
	// in [RFC3339].  This field is REQUIRED for objects with "pending"
	// or "valid" in the status field.
	Expires time.Time `json:"expires,omitempty"`

	// identifiers (required, array of object):  An array of identifier
	// objects that the order pertains to.
	Identifiers []Identifier `json:"identifiers"`

	// notBefore (optional, string):  The requested value of the notBefore
	// field in the certificate, in the date format defined in [RFC3339].
	NotBefore *time.Time `json:"notBefore,omitempty"`

	// notAfter (optional, string):  The requested value of the notAfter
	// field in the certificate, in the date format defined in [RFC3339].
	NotAfter *time.Time `json:"notAfter,omitempty"`

	// error (optional, object):  The error that occurred while processing
	// the order, if any.  This field is structured as a problem document
	// [RFC7807].
	Error *Problem `json:"error,omitempty"`

	// authorizations (required, array of string):  For pending orders, the
	// authorizations that the client needs to complete before the
	// requested certificate can be issued (see Section 7.5), including
	// unexpired authorizations that the client has completed in the past
	// for identifiers specified in the order.  The authorizations
	// required are dictated by server policy; there may not be a 1:1
	// relationship between the order identifiers and the authorizations
	// required.  For final orders (in the "valid" or "invalid" state),
	// the authorizations that were completed.  Each entry is a URL from
	// which an authorization can be fetched with a POST-as-GET request.
	Authorizations []string `json:"authorizations"`

	// finalize (required, string):  A URL that a CSR must be POSTed to once
	// all of the order's authorizations are satisfied to finalize the
	// order.  The result of a successful finalization will be the
	// population of the certificate URL for the order.
	Finalize string `json:"finalize"`

	// certificate (optional, string):  A URL for the certificate that has
	// been issued in response to this order.
	Certificate string `json:"certificate"`

	// Similar to new-account, the server returns a 201 response with
	// the URL to the order object in the Location header.
	//
	// We transfer the value from the header to this field for
	// storage and recall purposes.
	Location string `json:"-"`
}

// Identifier is used in order and authorization (authz) objects.
type Identifier struct {
	// type (required, string):  The type of identifier.  This document
	// defines the "dns" identifier type.  See the registry defined in
	// Section 9.7.7 for any others.
	Type string `json:"type"`

	// value (required, string):  The identifier itself.
	Value string `json:"value"`
}

// NewOrder creates a new order with the server.
//
// "The client begins the certificate issuance process by sending a POST
// request to the server's newOrder resource." §7.4
func (c *Client) NewOrder(ctx context.Context, account Account, order Order) (Order, error) {
	if err := c.provision(ctx); err != nil {
		return order, err
	}
	resp, err := c.httpPostJWS(ctx, account.PrivateKey, account.Location, c.dir.NewOrder, order, &order)
	if err != nil {
		return order, err
	}
	order.Location = resp.Header.Get("Location")
	return order, nil
}

// FinalizeOrder finalizes the order with the server and polls under the server has
// updated the order status. The CSR must be in ASN.1 DER-encoded format. If this
// succeeds, the certificate is ready to download once this returns.
//
// "Once the client believes it has fulfilled the server's requirements,
// it should send a POST request to the order resource's finalize URL." §7.4
func (c *Client) FinalizeOrder(ctx context.Context, account Account, order Order, csrASN1DER []byte) (Order, error) {
	if err := c.provision(ctx); err != nil {
		return order, err
	}

	body := struct {
		// csr (required, string):  A CSR encoding the parameters for the
		// certificate being requested [RFC2986].  The CSR is sent in the
		// base64url-encoded version of the DER format.  (Note: Because this
		// field uses base64url, and does not include headers, it is
		// different from PEM.) §7.4
		CSR string `json:"csr"`
	}{
		CSR: base64.RawURLEncoding.EncodeToString(csrASN1DER),
	}

	resp, err := c.httpPostJWS(ctx, account.PrivateKey, account.Location, order.Finalize, body, &order)
	if err != nil {
		// "A request to finalize an order will result in error if the order is
		// not in the 'ready' state.  In such cases, the server MUST return a
		// 403 (Forbidden) error with a problem document of type
		// 'orderNotReady'.  The client should then send a POST-as-GET request
		// to the order resource to obtain its current state.  The status of the
		// order will indicate what action the client should take (see below)."
		// §7.4
		var problem Problem
		if errors.As(err, &problem) {
			if problem.Type != ProblemTypeOrderNotReady {
				return order, err
			}
		} else {
			return order, err
		}
	}

	// unlike with accounts and authorizations, the spec isn't clear on whether
	// the server MUST set this on finalizing the order, but their example shows a
	// Location header, so I guess if it's set in the response, we should keep it
	if newLocation := resp.Header.Get("Location"); newLocation != "" {
		order.Location = newLocation
	}

	if finished, err := orderIsFinished(order); finished {
		return order, err
	}

	// TODO: "The elements of the "authorizations" and "identifiers" arrays are
	// immutable once set. If a client observes a change
	// in the contents of either array, then it SHOULD consider the order
	// invalid."

	maxDuration := c.pollTimeout()
	start := time.Now()
	for time.Since(start) < maxDuration {
		// querying an order is expensive on the server-side, so we
		// shouldn't do it too frequently; honor server preference
		interval, err := retryAfter(resp, c.pollInterval())
		if err != nil {
			return order, err
		}
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return order, ctx.Err()
		}

		resp, err = c.httpPostJWS(ctx, account.PrivateKey, account.Location, order.Location, nil, &order)
		if err != nil {
			return order, fmt.Errorf("polling order status: %w", err)
		}

		// (same reasoning as above)
		if newLocation := resp.Header.Get("Location"); newLocation != "" {
			order.Location = newLocation
		}

		if finished, err := orderIsFinished(order); finished {
			return order, err
		}
	}

	return order, fmt.Errorf("order took too long")
}

// orderIsFinished returns true if the order processing is complete,
// regardless of success or failure. If this function returns true,
// polling an order status should stop. If there is an error with the
// order, an error will be returned. This function should be called
// only after a request to finalize an order. See §7.4.
func orderIsFinished(order Order) (bool, error) {
	switch order.Status {
	case StatusInvalid:
		// "invalid": The certificate will not be issued.  Consider this
		//      order process abandoned.
		return true, fmt.Errorf("final order is invalid: %w", order.Error)

	case StatusPending:
		// "pending": The server does not believe that the client has
		//      fulfilled the requirements.  Check the "authorizations" array for
		//      entries that are still pending.
		return true, fmt.Errorf("order pending, authorizations remaining: %v", order.Authorizations)

	case StatusReady:
		// "ready": The server agrees that the requirements have been
		//      fulfilled, and is awaiting finalization.  Submit a finalization
		//      request.
		// (we did just submit a finalization request, so this is an error)
		return true, fmt.Errorf("unexpected state: %s - order already finalized", order.Status)

	case StatusProcessing:
		// "processing": The certificate is being issued.  Send a GET request
		//      after the time given in the "Retry-After" header field of the
		//      response, if any.
		return false, nil

	case StatusValid:
		// "valid": The server has issued the certificate and provisioned its
		//      URL to the "certificate" field of the order.  Download the
		//      certificate.
		return true, nil

	default:
		return true, fmt.Errorf("unrecognized order status: %s", order.Status)
	}
}
