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
	cfg, err := newConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	extended := &Extended{
		BaseStruct: BaseStruct{
			Second: "queen of hearts",
		},
	}

	_, err = getCronSettings(cfg, "test", extended)
	assert.NoError(t, err)
	assert.True(t, extended.Base)
	assert.EqualValues(t, extended.Second, "white rabbit")
	assert.True(t, extended.Extend)
}
