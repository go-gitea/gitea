// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package acme

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DeactivateReg permanently disables an existing account associated with c.Key.
// A deactivated account can no longer request certificate issuance or access
// resources related to the account, such as orders or authorizations.
//
// It works only with RFC8555 compliant CAs.
func (c *Client) DeactivateReg(ctx context.Context) error {
	url := string(c.accountKID(ctx))
	if url == "" {
		return ErrNoAccount
	}
	req := json.RawMessage(`{"status": "deactivated"}`)
	res, err := c.post(ctx, nil, url, req, wantStatus(http.StatusOK))
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

// registerRFC is quivalent to c.Register but for RFC-compliant CAs.
// It expects c.Discover to have already been called.
// TODO: Implement externalAccountBinding.
func (c *Client) registerRFC(ctx context.Context, acct *Account, prompt func(tosURL string) bool) (*Account, error) {
	c.cacheMu.Lock() // guard c.kid access
	defer c.cacheMu.Unlock()

	req := struct {
		TermsAgreed bool     `json:"termsOfServiceAgreed,omitempty"`
		Contact     []string `json:"contact,omitempty"`
	}{
		Contact: acct.Contact,
	}
	if c.dir.Terms != "" {
		req.TermsAgreed = prompt(c.dir.Terms)
	}
	res, err := c.post(ctx, c.Key, c.dir.RegURL, req, wantStatus(
		http.StatusOK,      // account with this key already registered
		http.StatusCreated, // new account created
	))
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	a, err := responseAccount(res)
	if err != nil {
		return nil, err
	}
	// Cache Account URL even if we return an error to the caller.
	// It is by all means a valid and usable "kid" value for future requests.
	c.kid = keyID(a.URI)
	if res.StatusCode == http.StatusOK {
		return nil, ErrAccountAlreadyExists
	}
	return a, nil
}

// updateGegRFC is equivalent to c.UpdateReg but for RFC-compliant CAs.
// It expects c.Discover to have already been called.
func (c *Client) updateRegRFC(ctx context.Context, a *Account) (*Account, error) {
	url := string(c.accountKID(ctx))
	if url == "" {
		return nil, ErrNoAccount
	}
	req := struct {
		Contact []string `json:"contact,omitempty"`
	}{
		Contact: a.Contact,
	}
	res, err := c.post(ctx, nil, url, req, wantStatus(http.StatusOK))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return responseAccount(res)
}

// getGegRFC is equivalent to c.GetReg but for RFC-compliant CAs.
// It expects c.Discover to have already been called.
func (c *Client) getRegRFC(ctx context.Context) (*Account, error) {
	req := json.RawMessage(`{"onlyReturnExisting": true}`)
	res, err := c.post(ctx, c.Key, c.dir.RegURL, req, wantStatus(http.StatusOK))
	if e, ok := err.(*Error); ok && e.ProblemType == "urn:ietf:params:acme:error:accountDoesNotExist" {
		return nil, ErrNoAccount
	}
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	return responseAccount(res)
}

func responseAccount(res *http.Response) (*Account, error) {
	var v struct {
		Status  string
		Contact []string
		Orders  string
	}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("acme: invalid response: %v", err)
	}
	return &Account{
		URI:       res.Header.Get("Location"),
		Status:    v.Status,
		Contact:   v.Contact,
		OrdersURL: v.Orders,
	}, nil
}
