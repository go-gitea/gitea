// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSSHRootPathAvoidsRealHomeWhileTesting(t *testing.T) {
	defer test.MockVariableValue(&SSH, SSH)()
	defer test.MockVariableValue(&IsInTesting, true)()
	defer test.MockVariableValue(&appTempPathInternal, "")()
	defer test.MockVariableValue(&AppDataPath, t.TempDir())()

	cfg, err := NewConfigProviderFromData("")
	require.NoError(t, err)
	loadSSHFrom(cfg)

	assert.Equal(t, filepath.Join(AppDataPath, "tmp", "ssh"), SSH.RootPath)
}
