// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"encoding/hex"
	"slices"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(WebauthnCredential), new(U2fRegistration), new(ExpectedWebauthnCredential))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	if x.Dialect().URI().DBType == schemas.SQLITE {
		return
	}

	// Run the migration
	if err := RemigrateU2FCredentials(t.Context(), x); err != nil {
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

const u2fRawMessageFormatsExample81Hex = "" +
	"0504b174bc49c7ca254b70d2e5c207cee9cf174820ebd77ea3c65508c26da51b657c1cc6b952f8621697936482da0a6d3d38" +
	"26a59095daf6cd7c03e2e60385d2f6d9402a552dfdb7477ed65fd84133f86196010b2215b57da75d315b7b9e8fe2e3925a60" +
	"19551bab61d16591659cbaf00b4950f7abfe6660e2e006f76868b772d70c253082013c3081e4a003020102020a4790128000" +
	"1155957352300a06082a8648ce3d0403023017311530130603550403130c476e756262792050696c6f74301e170d31323038" +
	"31343138323933325a170d3133303831343138323933325a3031312f302d0603550403132650696c6f74476e756262792d30" +
	"2e342e312d34373930313238303030313135353935373335323059301306072a8648ce3d020106082a8648ce3d0301070342" +
	"00048d617e65c9508e64bcc5673ac82a6799da3c1446682c258c463fffdf58dfd2fa3e6c378b53d795c4a4dffb4199edd786" +
	"2f23abaf0203b4b8911ba0569994e101300a06082a8648ce3d0403020347003044022060cdb6061e9c22262d1aac1d96d8c7" +
	"0829b2366531dda268832cb836bcd30dfa0220631b1459f09e6330055722c8d89b7f48883b9089b88d60d1d9795902b30410" +
	"df304502201471899bcc3987e62e8202c9b39c33c19033f7340352dba80fcab017db9230e402210082677d673d891933ade6" +
	"f617e5dbde2e247e70423fd5ad7804a6d3d3961ef871"

func Test_parseU2FRegistration(t *testing.T) {
	raw, err := hex.DecodeString(u2fRawMessageFormatsExample81Hex)
	require.NoError(t, err)

	keyHandle, publicKey, err := parseU2FRegistration(raw)
	require.NoError(t, err)
	assert.Equal(t, "2a552dfdb7477ed65fd84133f86196010b2215b57da75d315b7b9e8fe2e3925a6019551bab61d16591659cbaf00b4950f7abfe6660e2e006f76868b772d70c25", hex.EncodeToString(keyHandle))
	assert.Equal(t, "04b174bc49c7ca254b70d2e5c207cee9cf174820ebd77ea3c65508c26da51b657c1cc6b952f8621697936482da0a6d3d3826a59095daf6cd7c03e2e60385d2f6d9", hex.EncodeToString(publicKey))

	invalidPoint := slices.Clone(raw)
	invalidPoint[1] = 0x02

	for name, input := range map[string][]byte{
		"too short":            raw[:68],
		"bad reserved byte":    append([]byte{0x04}, raw[1:]...),
		"invalid point":        invalidPoint,
		"truncated key handle": raw[:67+int(raw[66])-1],
	} {
		_, _, err := parseU2FRegistration(input)
		assert.Error(t, err, name)
	}
}
