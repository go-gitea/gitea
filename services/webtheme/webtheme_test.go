// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webtheme

import (
	"testing"

	"code.gitea.io/gitea/modules/container"

	"github.com/stretchr/testify/assert"
)

func TestParseThemeMetaInfo(t *testing.T) {
	m := parseThemeMetaInfoToMap(`gitea-theme-meta-info { --k1: "v1"; --k2: "a\"b"; }`)
	assert.Equal(t, map[string]string{"--k1": "v1", "--k2": `a"b`}, m)

	schemes := parseThemePreferColorSchemes(`@media (prefers-color-scheme: dark) {} @media (prefers-color-scheme: light) {} @media (prefers-color-scheme:dark) {}`)
	assert.Equal(t, container.SetOf("dark", "light"), schemes)
}
