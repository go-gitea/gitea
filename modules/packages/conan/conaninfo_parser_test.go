// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conan

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	settingsKey   = "arch"
	settingsValue = "x84_64"
	optionsKey    = "shared"
	optionsValue  = "False"
	requires      = "fmt/7.1.3"
	hash          = "74714915a51073acb548ca1ce29afbac"
	envKey        = "CC"
	envValue      = "gcc-10"

	contentConaninfo = `[settings]
    ` + settingsKey + `=` + settingsValue + `

[requires]
    ` + requires + `

[options]
    ` + optionsKey + `=` + optionsValue + `

[full_settings]
    ` + settingsKey + `=` + settingsValue + `

[full_requires]
    ` + requires + `

[full_options]
    ` + optionsKey + `=` + optionsValue + `

[recipe_hash]
    ` + hash + `

[env]
` + envKey + `=` + envValue + `

`
)

func TestParseConaninfo(t *testing.T) {
	info, err := ParseConaninfo(strings.NewReader(contentConaninfo))
	assert.NotNil(t, info)
	assert.NoError(t, err)
	assert.Equal(
		t,
		map[string]string{
			settingsKey: settingsValue,
		},
		info.Settings,
	)
	assert.Equal(t, info.Settings, info.FullSettings)
	assert.Equal(
		t,
		map[string]string{
			optionsKey: optionsValue,
		},
		info.Options,
	)
	assert.Equal(t, info.Options, info.FullOptions)
	assert.Equal(
		t,
		[]string{requires},
		info.Requires,
	)
	assert.Equal(t, info.Requires, info.FullRequires)
	assert.Equal(t, hash, info.RecipeHash)
	assert.Equal(
		t,
		map[string][]string{
			envKey: {envValue},
		},
		info.Environment,
	)
}
