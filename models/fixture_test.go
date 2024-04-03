// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestFixtureGeneration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(ctx context.Context, gen func(ctx context.Context) (string, error), name string) {
		expected, err := gen(ctx)
		if !assert.NoError(t, err) {
			return
		}
		p := filepath.Join(unittest.FixturesDir(), name+".yml")
		bytes, err := os.ReadFile(p)
		if !assert.NoError(t, err) {
			return
		}
		data := string(util.NormalizeEOL(bytes))
		assert.EqualValues(t, expected, data, "Differences detected for %s", p)
	}

	test(db.DefaultContext, GetYamlFixturesAccess, "access")
}
