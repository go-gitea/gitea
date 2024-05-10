// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneralWebSecret(t *testing.T) {
	assert.Nil(t, GeneralWebSecretBytes)
	auto1 := GetGeneralTokenSigningSecret()
	auto2 := GetGeneralTokenSigningSecret()
	assert.Len(t, auto1, 32)
	assert.Equal(t, auto1, auto2)

	tmpFile := t.TempDir() + "/app.ini"
	_ = os.WriteFile(tmpFile, []byte("[security]\nINSTALL_LOCK=true"), 0o644)
	cfg, _ := NewConfigProviderFromFile(tmpFile)
	loadSecurityFrom(cfg)
	generated := GeneralWebSecretBytes
	assert.Len(t, generated, 32)
	assert.NotEqual(t, auto1, generated)

	cfg, _ = NewConfigProviderFromFile(tmpFile)
	GeneralWebSecretBytes = nil
	loadSecurityFrom(cfg)
	loaded := GeneralWebSecretBytes
	assert.Equal(t, generated, loaded)

	_ = os.WriteFile(tmpFile, []byte("[security]\nINSTALL_LOCK=true"), 0o644)
	cfg, _ = NewConfigProviderFromFile(tmpFile)
	loadSecurityFrom(cfg)
	generated2 := GeneralWebSecretBytes
	assert.Len(t, generated2, 32)
	assert.NotEqual(t, generated, generated2)
}
