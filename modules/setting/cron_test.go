// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getCronSettings(t *testing.T) {
	type BaseStruct struct {
		Base   bool
		Second string
	}

	type Extended struct {
		BaseStruct
		Extend bool
	}

	iniStr := `
[cron.test]
BASE = true
SECOND = white rabbit
EXTEND = true
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	extended := &Extended{
		BaseStruct: BaseStruct{
			Second: "queen of hearts",
		},
	}

	_, err = getCronSettings(cfg, "test", extended)
	assert.NoError(t, err)
	assert.True(t, extended.Base)
	assert.Equal(t, "white rabbit", extended.Second)
	assert.True(t, extended.Extend)
}

// Test_getCronSettings2 tests that getCronSettings can not handle two levels of embedding
func Test_getCronSettings2(t *testing.T) {
	type BaseStruct struct {
		Enabled    bool
		RunAtStart bool
		Schedule   string
	}

	type Extended struct {
		BaseStruct
		Extend bool
	}
	type Extended2 struct {
		Extended
		Third string
	}

	iniStr := `
[cron.test]
ENABLED = TRUE
RUN_AT_START = TRUE
SCHEDULE = @every 1h
EXTEND = true
THIRD = white rabbit
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	extended := &Extended2{
		Extended: Extended{
			BaseStruct: BaseStruct{
				Enabled:    false,
				RunAtStart: false,
				Schedule:   "@every 72h",
			},
			Extend: false,
		},
		Third: "black rabbit",
	}

	_, err = getCronSettings(cfg, "test", extended)
	assert.NoError(t, err)

	// This confirms the first level of embedding works
	assert.Equal(t, "white rabbit", extended.Third)
	assert.True(t, extended.Extend)

	// This confirms 2 levels of embedding doesn't work
	assert.False(t, extended.Enabled)
	assert.False(t, extended.RunAtStart)
	assert.Equal(t, "@every 72h", extended.Schedule)
}
