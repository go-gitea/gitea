// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"maps"
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestMakeAbsoluteAssetURL(t *testing.T) {
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234", "https://localhost:2345"))
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234/", "https://localhost:2345"))
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234/", "https://localhost:2345/"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/", "/foo/"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/foo/"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo", "/bar"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/bar"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/bar/"))
}

func TestMakeManifestData(t *testing.T) {
	jsonBytes := MakeManifestData(`Example App '\"`, "https://example.com", "https://example.com/foo/bar")
	assert.True(t, json.Valid(jsonBytes))
}

func TestLoadCommonSettingsClearsStartupProblems(t *testing.T) {
	cfg, _ := NewConfigProviderFromData(`
[oauth2]
ENABLE = true
`)
	CfgProvider = cfg

	LoadCommonSettings()
	assert.NotEmpty(t, StartupProblems, "expected at least one startup problem from deprecated ENABLE setting")

	LoadCommonSettings()
	seen := make(map[string]int, len(StartupProblems))
	for _, msg := range StartupProblems {
		seen[msg]++
	}
	for msg, count := range seen {
		assert.Equal(t, 1, count, "StartupProblems contains duplicate entry: %s", msg)
	}
}

func TestLoadCommonSettingsClearsConfiguredPaths(t *testing.T) {
	cfg, _ := NewConfigProviderFromData("")
	CfgProvider = cfg

	LoadCommonSettings()
	firstPaths := make(map[string]string, len(configuredPaths))
	maps.Copy(firstPaths, configuredPaths)

	LoadCommonSettings()
	assert.Equal(t, firstPaths, configuredPaths, "configuredPaths should be identical after a second load, not accumulated")
}
