// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webtheme

import (
	"maps"
	"slices"
	"testing"
	"time"

	"gitea.dev/modules/public"

	"github.com/stretchr/testify/assert"
)

func TestResolveInternalNameLegacyPrefix(t *testing.T) {
	// the built-in themes were renamed from "gitea-*" to "teabag-*"
	assert.Equal(t, "teabag-auto", resolveInternalName("gitea-auto"))
	assert.Equal(t, "teabag-dark", resolveInternalName("gitea-dark"))
	assert.Equal(t, "teabag-light-protanopia-deuteranopia", resolveInternalName("gitea-light-protanopia-deuteranopia"))
	// current names and unrelated names are left alone
	assert.Equal(t, "teabag-dark", resolveInternalName("teabag-dark"))
	assert.Equal(t, "", resolveInternalName(""))
	assert.Equal(t, "my-gitea-like-theme", resolveInternalName("my-gitea-like-theme"))
}

// mockThemeCollection seeds the theme cache with the given themes for the duration of the test.
func mockThemeCollection(t *testing.T, themeMap map[string]*ThemeMetaInfo) {
	t.Helper()
	old := themeCollection.Load()
	t.Cleanup(func() { themeCollection.Store(old) })
	themeCollection.Store(&themeCollectionStruct{
		lastCheckTime:    time.Now(),
		usingViteDevMode: public.IsViteDevMode(),
		themeList:        slices.Collect(maps.Values(themeMap)),
		themeMap:         themeMap,
	})
}

func TestResolveInternalNameSet(t *testing.T) {
	// an admin allow-list written before the rename must still match the renamed themes
	allowed := resolveInternalNameSet([]string{"gitea-dark", "teabag-light"})
	assert.True(t, allowed.Contains("gitea-dark"))
	assert.True(t, allowed.Contains("teabag-dark"))
	assert.True(t, allowed.Contains("teabag-light"))
	assert.False(t, allowed.Contains("teabag-auto"))
}

func TestGetThemeMetaInfoLegacyName(t *testing.T) {
	builtIn := &ThemeMetaInfo{FileName: "theme-teabag-dark.AbCdEfGh.css", InternalName: "teabag-dark", DisplayName: "Dark"}
	custom := &ThemeMetaInfo{FileName: "theme-gitea-sunrise.css", InternalName: "gitea-sunrise", DisplayName: "Sunrise"}
	mockThemeCollection(t, map[string]*ThemeMetaInfo{
		builtIn.InternalName: builtIn,
		custom.InternalName:  custom,
	})

	t.Run("CurrentName", func(t *testing.T) {
		assert.Same(t, builtIn, GetThemeMetaInfo("teabag-dark"))
	})
	t.Run("LegacyName", func(t *testing.T) {
		info := GetThemeMetaInfo("gitea-dark")
		assert.Same(t, builtIn, info, "a setting stored before the rename must resolve to the renamed theme")
		assert.Equal(t, "teabag-dark", info.InternalName)
	})
	t.Run("LegacyNameOfCustomTheme", func(t *testing.T) {
		assert.Same(t, custom, GetThemeMetaInfo("gitea-sunrise"), "an exact match must win over the rename")
	})
	t.Run("UnknownName", func(t *testing.T) {
		assert.Nil(t, GetThemeMetaInfo("teabag-sunrise"))
	})
}

func TestParseThemeMetaInfo(t *testing.T) {
	m := parseThemeMetaInfoToMap(`teabag-theme-meta-info {
	--k1: "v1";
	--k2: "v\"2";
	--k3: 'v3';
	--k4: 'v\'4';
	--k5: v5;
}`)
	assert.Equal(t, map[string]string{
		"--k1": "v1",
		"--k2": `v"2`,
		"--k3": "v3",
		"--k4": "v'4",
		"--k5": "v5",
	}, m)

	// if an auto theme imports others, the meta info should be extracted from the last one
	// the meta in imported themes should be ignored to avoid incorrect overriding
	m = parseThemeMetaInfoToMap(`
@media (prefers-color-scheme: dark) { teabag-theme-meta-info { --k1: foo; } }
@media (prefers-color-scheme: light) { teabag-theme-meta-info { --k1: bar; } }
teabag-theme-meta-info {
	--k2: real;
}`)
	assert.Equal(t, map[string]string{"--k2": "real"}, m)

	// compressed CSS, no trailing semicolon
	m = parseThemeMetaInfoToMap(`teabag-theme-meta-info{--k1:"v1"}`)
	assert.Equal(t, map[string]string{"--k1": "v1"}, m)
	m = parseThemeMetaInfoToMap(`teabag-theme-meta-info{--k1:"v1";--k2:"v2"}`)
	assert.Equal(t, map[string]string{"--k1": "v1", "--k2": "v2"}, m)
}
