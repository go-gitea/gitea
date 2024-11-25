// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/require"
)

type WebAuthnCredentialOld struct {
	ID int64 `xorm:"pk autoincr"`
}

func (cred WebAuthnCredentialOld) TableName() string {
	return "webauthn_credential"
}

func TestAddFlagsOnWebAuthnCredential(t *testing.T) {
	x, deferable := base.PrepareTestEnv(t, 0, new(WebAuthnCredentialOld))
	defer deferable()

	_, err := x.Exec("INSERT INTO webauthn_credential (id) VALUES (1)")
	require.NoError(t, err)
	_, err = x.Exec("INSERT INTO webauthn_credential (id) VALUES (2)")
	require.NoError(t, err)

	require.NoError(t, AddFlagsOnWebAuthnCredential(x))

	getFlags := func() (s1, s2 string) {
		x.Select("credential_flags").Table("webauthn_credential").Where("id=1").Get(&s1)
		x.Select("credential_flags").Table("webauthn_credential").Where("id=2").Get(&s2)
		return s1, s2
	}

	s1, s2 := getFlags()
	require.Equal(t, `{"userPresent":false,"userVerified":false,"backupEligible":true,"backupState":false}`, s1)
	require.Equal(t, `{"userPresent":false,"userVerified":false,"backupEligible":true,"backupState":false}`, s2)

	_, err = x.Table("webauthn_credential").Where("id=1").Update(map[string]any{"credential_flags": `{}`})
	require.NoError(t, err)
	s1, s2 = getFlags()
	require.Equal(t, `{}`, s1)
	require.Equal(t, `{"userPresent":false,"userVerified":false,"backupEligible":true,"backupState":false}`, s2)

	require.NoError(t, AddFlagsOnWebAuthnCredential(x))
	s1, s2 = getFlags()
	require.Equal(t, `{}`, s1)
	require.Equal(t, `{"userPresent":false,"userVerified":false,"backupEligible":true,"backupState":false}`, s2)
}
