// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/tests"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/stretchr/testify/assert"
)

// one credential serves both logins, so their user verification is coupled
func TestWebAuthnUserVerification(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "/user/settings/security/webauthn/request_register", map[string]string{"name": "test-key"})
	creation := DecodeJSON(t, session.MakeRequest(t, req, http.StatusOK), &protocol.CredentialCreation{})
	assert.Equal(t, protocol.VerificationRequired, creation.Response.AuthenticatorSelection.UserVerification)

	session = loginUserWithPassword(t, "user32", "notpassword") // user32 has a webauthn credential
	req = NewRequest(t, "GET", "/user/webauthn/assertion")
	secondFactor := DecodeJSON(t, session.MakeRequest(t, req, http.StatusOK), &protocol.CredentialAssertion{})
	assert.Equal(t, protocol.VerificationPreferred, secondFactor.Response.UserVerification)

	session = emptyTestSession(t)
	req = NewRequest(t, "GET", "/user/webauthn/passkey/assertion") // also seeds the session for the request below
	passkey := DecodeJSON(t, session.MakeRequest(t, req, http.StatusOK), &protocol.CredentialAssertion{})
	assert.Equal(t, protocol.VerificationRequired, passkey.Response.UserVerification)

	// a malformed response used to dereference a nil user
	req = NewRequestWithJSON(t, "POST", "/user/webauthn/passkey/login", map[string]string{"bogus": "1"})
	session.MakeRequest(t, req, http.StatusForbidden)
}
