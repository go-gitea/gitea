// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadServerDomainAndURLWithAlternativeRootURLs(t *testing.T) {
	defer test.MockVariableValue(&AppURL)()
	defer test.MockVariableValue(&AppSubURL)()
	defer test.MockVariableValue(&AppDomain)()
	defer test.MockVariableValue(&AlternativeRootURLs)()
	defer test.MockVariableValue(&PublicURLDetection)()
	defer test.MockVariableValue(&UseSubURLPath)()
	defer test.MockVariableValue(&StaticURLPrefix)()

	cfg, err := NewConfigProviderFromData("[server]\nROOT_URL = https://git.example.com:443/sub , http://192.168.1.2:3000/sub/,")
	require.NoError(t, err)
	loadServerDomainAndURL(cfg.Section("server"), "http")
	assert.Equal(t, "https://git.example.com/sub/", AppURL)
	assert.Equal(t, "/sub", AppSubURL)
	assert.Equal(t, []string{"http://192.168.1.2:3000/sub/"}, AlternativeRootURLs)
}
