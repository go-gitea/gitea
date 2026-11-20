// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"math/big"
	"strings"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepairPackageSigningKeys(t *testing.T) {
	type UserSetting struct {
		ID           int64 `xorm:"pk autoincr"`
		UserID       int64
		SettingKey   string
		SettingValue string `xorm:"TEXT"`
	}
	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(UserSetting))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, RepairPackageSigningKeys(t.Context(), x))

	settings := map[int64]*UserSetting{}
	require.NoError(t, x.Find(&settings))
	assert.Equal(t, "valid", settings[4].SettingValue)
	assert.Equal(t, "untouched", settings[6].SettingValue)

	readIdentity := func(armored string) *openpgp.Identity {
		entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armored))
		require.NoError(t, err)
		return entities[0].PrimaryIdentity()
	}
	identity := readIdentity(settings[2].SettingValue)
	assert.Equal(t, readIdentity(settings[1].SettingValue).SelfSignature.CreationTime, identity.SelfSignature.CreationTime)
	signature := identity.SelfSignature.RSASignature
	assert.Equal(t, new(big.Int).SetBytes(signature.Bytes()).BitLen(), int(signature.BitLength()))
}
