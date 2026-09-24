// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
)

func TestGetWebAuthnCredentialByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	res, err := auth_model.GetWebAuthnCredentialByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "WebAuthn credential", res.Name)

	_, err = auth_model.GetWebAuthnCredentialByID(t.Context(), 342432)
	assert.Error(t, err)
	assert.True(t, auth_model.IsErrWebAuthnCredentialNotExist(err))
}

func TestGetWebAuthnCredentialsByUID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	res, err := auth_model.GetWebAuthnCredentialsByUID(t.Context(), 32)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "WebAuthn credential", res[0].Name)
}

func TestWebAuthnCredential_TableName(t *testing.T) {
	assert.Equal(t, "webauthn_credential", auth_model.WebAuthnCredential{}.TableName())
}

func TestWebAuthnCredential_UpdateSignCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	cred := unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1})
	cred.SignCount = 1
	assert.NoError(t, cred.UpdateSignCount(t.Context()))
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1, SignCount: 1})
}

func TestWebAuthnCredential_UpdateLargeCounter(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	cred := unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1})
	cred.SignCount = 0xffffffff
	assert.NoError(t, cred.UpdateSignCount(t.Context()))
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1, SignCount: 0xffffffff})
}

func TestCreateCredential(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	res, err := auth_model.CreateCredential(t.Context(), 1, "WebAuthn Created Credential", &webauthn.Credential{ID: []byte("Test")})
	assert.NoError(t, err)
	assert.Equal(t, "WebAuthn Created Credential", res.Name)
	assert.Equal(t, []byte("Test"), res.CredentialID)

	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{Name: "WebAuthn Created Credential", UserID: 1})
}

func TestRenameCredential(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ok, err := auth_model.RenameCredential(t.Context(), 1, 1, "Not the owner")
	assert.NoError(t, err)
	assert.False(t, ok)
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1, Name: "WebAuthn credential"})

	ok, err = auth_model.RenameCredential(t.Context(), 1, 32, "Renamed Credential")
	assert.NoError(t, err)
	assert.True(t, ok)
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1, Name: "Renamed Credential", LowerName: "renamed credential"})

	// only the letter case changes
	ok, err = auth_model.RenameCredential(t.Context(), 1, 32, "RENAMED credential")
	assert.NoError(t, err)
	assert.True(t, ok)

	other, err := auth_model.CreateCredential(t.Context(), 32, "Other Credential", &webauthn.Credential{ID: []byte("other")})
	assert.NoError(t, err)
	ok, err = auth_model.RenameCredential(t.Context(), other.ID, 32, "renamed CREDENTIAL")
	assert.True(t, auth_model.IsErrWebAuthnCredentialNameAlreadyUsed(err))
	assert.False(t, ok)
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: other.ID, Name: "Other Credential"})
}
