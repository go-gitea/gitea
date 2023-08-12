// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestFixtureGeneration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(gen func() (string, error), name string) {
		expected, err := gen()
		if !assert.NoError(t, err) {
			return
		}
		bytes, err := os.ReadFile(filepath.Join(unittest.FixturesDir(), name+".yml"))
		if !assert.NoError(t, err) {
			return
		}
		data := string(util.NormalizeEOL(bytes))
		assert.True(t, data == expected, "Differences detected for %s.yml", name)
	}

	test(GetYamlFixturesAccess, "access")
}
