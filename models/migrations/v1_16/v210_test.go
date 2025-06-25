// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm/schemas"
)

func Test_RemigrateU2FCredentials(t *testing.T) {
	// Create webauthnCredential table
	type WebauthnCredential struct {
		ID              int64 `xorm:"pk autoincr"`
		Name            string
		LowerName       string `xorm:"unique(s)"`
		UserID          int64  `xorm:"INDEX unique(s)"`
		CredentialID    string `xorm:"INDEX VARCHAR(410)"` // CredentalID in U2F is at most 255bytes / 5 * 8 = 408 - add a few extra characters for safety
		PublicKey       []byte
		AttestationType string
		SignCount       uint32 `xorm:"BIGINT"`
		CloneWarning    bool
	}

	// Now migrate the old u2f registrations to the new format
	type U2fRegistration struct {
		ID          int64 `xorm:"pk autoincr"`
		Name        string
		UserID      int64 `xorm:"INDEX"`
		Raw         []byte
		Counter     uint32             `xorm:"BIGINT"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	type ExpectedWebauthnCredential struct {
		ID           int64  `xorm:"pk autoincr"`
		CredentialID string `xorm:"INDEX VARCHAR(410)"` // CredentalID in U2F is at most 255bytes / 5 * 8 = 408 - add a few extra characters for safety
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(WebauthnCredential), new(U2fRegistration), new(ExpectedWebauthnCredential))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	if x.Dialect().URI().DBType == schemas.SQLITE {
		return
	}

	// Run the migration
	if err := RemigrateU2FCredentials(x); err != nil {
		assert.NoError(t, err)
		return
	}

	expected := []ExpectedWebauthnCredential{}
	if err := x.Table("expected_webauthn_credential").Asc("id").Find(&expected); !assert.NoError(t, err) {
		return
	}

	got := []ExpectedWebauthnCredential{}
	if err := x.Table("webauthn_credential").Select("id, credential_id").Asc("id").Find(&got); !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, expected, got)
}
