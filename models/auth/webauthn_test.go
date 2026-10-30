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

func TestWebAuthnCredentials(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("GetWebAuthnCredentialByID", func(t *testing.T) {
		res, err := auth_model.GetWebAuthnCredentialByID(t.Context(), 32, 1)
		assert.NoError(t, err)
		assert.Equal(t, "WebAuthn credential", res.Name)

		_, err = auth_model.GetWebAuthnCredentialByID(t.Context(), 99999, 1)
		assert.Error(t, err)
		assert.True(t, auth_model.IsErrWebAuthnCredentialNotExist(err))

		_, err = auth_model.GetWebAuthnCredentialByID(t.Context(), 32, 99999)
		assert.Error(t, err)
		assert.True(t, auth_model.IsErrWebAuthnCredentialNotExist(err))
	})

	t.Run("GetWebAuthnCredentialsByUID", func(t *testing.T) {
		res, err := auth_model.GetWebAuthnCredentialsByUID(t.Context(), 32)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "WebAuthn credential", res[0].Name)
	})

	t.Run("CreateCredential", func(t *testing.T) {
		res, err := auth_model.CreateCredential(t.Context(), 1, "WebAuthn Created Credential", &webauthn.Credential{ID: []byte("Test")})
		assert.NoError(t, err)
		assert.Equal(t, "WebAuthn Created Credential", res.Name)
		assert.Equal(t, []byte("Test"), res.CredentialID)

		unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{Name: "WebAuthn Created Credential", UserID: 1})
	})
}

func TestWebAuthnCredential_TableName(t *testing.T) {
	assert.Equal(t, "webauthn_credential", auth_model.WebAuthnCredential{}.TableName())
}

func TestWebAuthnCredential_UpdateSignCount(t *testing.T) {
	for _, signCount := range []uint32{1, 0xffffffff} {
		assert.NoError(t, unittest.PrepareTestDatabase())
		cred := unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1})
		cred.SignCount = signCount
		assert.NoError(t, cred.UpdateSignCount(t.Context()))
		unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{ID: 1, SignCount: signCount})
	}
}
