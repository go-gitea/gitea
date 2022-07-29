// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/duo-labs/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
)

func TestGetWebAuthnCredentialByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	res, err := GetWebAuthnCredentialByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "WebAuthn credential", res.Name)

	_, err = GetWebAuthnCredentialByID(342432)
	assert.Error(t, err)
	assert.True(t, IsErrWebAuthnCredentialNotExist(err))
}

func TestGetWebAuthnCredentialsByUID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	res, err := GetWebAuthnCredentialsByUID(32)
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "WebAuthn credential", res[0].Name)
}

func TestWebAuthnCredential_TableName(t *testing.T) {
	assert.Equal(t, "webauthn_credential", WebAuthnCredential{}.TableName())
}

func TestWebAuthnCredential_UpdateSignCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	cred := unittest.AssertExistsAndLoadBean(t, &WebAuthnCredential{ID: 1}).(*WebAuthnCredential)
	cred.SignCount = 1
	assert.NoError(t, cred.UpdateSignCount())
	unittest.AssertExistsIf(t, true, &WebAuthnCredential{ID: 1, SignCount: 1})
}

func TestWebAuthnCredential_UpdateLargeCounter(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	cred := unittest.AssertExistsAndLoadBean(t, &WebAuthnCredential{ID: 1}).(*WebAuthnCredential)
	cred.SignCount = 0xffffffff
	assert.NoError(t, cred.UpdateSignCount())
	unittest.AssertExistsIf(t, true, &WebAuthnCredential{ID: 1, SignCount: 0xffffffff})
}

func TestCreateCredential(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	res, err := CreateCredential(1, "WebAuthn Created Credential", &webauthn.Credential{ID: []byte("Test")})
	assert.NoError(t, err)
	assert.Equal(t, "WebAuthn Created Credential", res.Name)
	assert.Equal(t, []byte("Test"), res.CredentialID)

	unittest.AssertExistsIf(t, true, &WebAuthnCredential{Name: "WebAuthn Created Credential", UserID: 1})
}
