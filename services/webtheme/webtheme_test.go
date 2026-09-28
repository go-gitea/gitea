// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webtheme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
