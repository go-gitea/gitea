// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17

import (
	"encoding/base32"
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_StoreWebauthnCredentialIDAsBytes(t *testing.T) {
	// Create webauthnCredential table
	type WebauthnCredential struct {
		ID              int64 `xorm:"pk autoincr"`
		Name            string
		LowerName       string `xorm:"unique(s)"`
		UserID          int64  `xorm:"INDEX unique(s)"`
		CredentialID    string `xorm:"INDEX VARCHAR(410)"`
		PublicKey       []byte
		AttestationType string
		AAGUID          []byte
		SignCount       uint32 `xorm:"BIGINT"`
		CloneWarning    bool
	}

	type ExpectedWebauthnCredential struct {
		ID           int64  `xorm:"pk autoincr"`
		CredentialID string // CredentialID is at most 1023 bytes as per spec released 20 July 2022
	}

	type ConvertedWebauthnCredential struct {
		ID                int64  `xorm:"pk autoincr"`
		CredentialIDBytes []byte `xorm:"VARBINARY(1024)"` // CredentialID is at most 1023 bytes as per spec released 20 July 2022
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(WebauthnCredential), new(ExpectedWebauthnCredential))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := StoreWebauthnCredentialIDAsBytes(x); err != nil {
		assert.NoError(t, err)
		return
	}

	expected := []ExpectedWebauthnCredential{}
	if err := x.Table("expected_webauthn_credential").Asc("id").Find(&expected); !assert.NoError(t, err) {
		return
	}

	got := []ConvertedWebauthnCredential{}
	if err := x.Table("webauthn_credential").Select("id, credential_id_bytes").Asc("id").Find(&got); !assert.NoError(t, err) {
		return
	}

	for i, e := range expected {
		credIDBytes, _ := base32.HexEncoding.DecodeString(e.CredentialID)
		assert.Equal(t, credIDBytes, got[i].CredentialIDBytes)
	}
}
