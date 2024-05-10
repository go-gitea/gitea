// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"testing"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestGeneralWebSecret(t *testing.T) {
	osExiter := test.MockedOsExiter{}
	defer test.MockVariableValue(&log.OsExiter, osExiter.Exit)()
	assert.Nil(t, GeneralWebSecretBytes)
	_ = GetGeneralTokenSigningSecret()
	assert.Equal(t, 1, osExiter.FetchCode())

	tmpFile := t.TempDir() + "/app.ini"
	_ = os.WriteFile(tmpFile, []byte("[security]\nINSTALL_LOCK=true"), 0o644)
	cfg, _ := NewConfigProviderFromFile(tmpFile)
	loadSecurityFrom(cfg)
	generated := GeneralWebSecretBytes
	assert.Len(t, generated, 32)

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
