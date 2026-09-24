// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestWebAuthnRename(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2") // sign in before adding credentials, which would require a second factor
	cred, err := auth_model.CreateCredential(t.Context(), 2, "My credential", &webauthn.Credential{ID: []byte("mine")})
	require.NoError(t, err)
	_, err = auth_model.CreateCredential(t.Context(), 2, "Other credential", &webauthn.Credential{ID: []byte("other")})
	require.NoError(t, err)

	htmlDoc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user/settings/security"), http.StatusOK).Body)
	AssertHTMLElement(t, htmlDoc, `#rename-registration form[action$="/webauthn/rename"]`, true)
	assert.Equal(t, "My credential", htmlDoc.Find(fmt.Sprintf(`[data-modal="#rename-registration"][data-modal-id="%d"]`, cred.ID)).AttrOr("data-modal-name", ""))

	rename := func(id int64, name string, expectedStatus int) *httptest.ResponseRecorder {
		req := NewRequestWithValues(t, "POST", "/user/settings/security/webauthn/rename", map[string]string{"id": strconv.FormatInt(id, 10), "name": name})
		return session.MakeRequest(t, req, expectedStatus)
	}

	rename(cred.ID, "Renamed credential", http.StatusOK)
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: cred.ID, Name: "Renamed credential"})

	// changing only the letter case of its own name is allowed
	rename(cred.ID, "RENAMED credential", http.StatusOK)
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: cred.ID, Name: "RENAMED credential"})

	resp := rename(cred.ID, "other CREDENTIAL", http.StatusBadRequest)
	assert.Equal(t, "A security key with the same nickname already exists.", test.ParseJSONError(resp.Body.Bytes()).ErrorMessage)
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: cred.ID, Name: "RENAMED credential"})

	rename(cred.ID, "", http.StatusBadRequest)
	rename(12345, "Missing credential", http.StatusNotFound)

	// the credential of user32 can't be renamed by user2
	rename(1, "Stolen credential", http.StatusNotFound)
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1, UserID: 32, Name: "WebAuthn credential"})
}
