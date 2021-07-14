// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_unwrapLDAPSourceCfg(t *testing.T) {
	// LoginSource represents an external way for authorizing users.
	type LoginSource struct {
		ID        int64 `xorm:"pk autoincr"`
		Type      int
		IsActived bool   `xorm:"INDEX NOT NULL DEFAULT false"`
		Cfg       string `xorm:"TEXT"`
		Expected  string `xorm:"TEXT"`
	}

	// Prepare and load the testing database
	x, deferable := prepareTestEnv(t, 0, new(LoginSource))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	// Run the migration
	if err := unwrapLDAPSourceCfg(x); err != nil {
		assert.NoError(t, err)
		return
	}

	const batchSize = 100
	for start := 0; ; start += batchSize {
		sources := make([]*LoginSource, 0, batchSize)
		if len(sources) == 0 {
			break
		}

		for _, source := range sources {
			assert.Equal(t, string(source.Cfg), string(source.Expected), "unwrapLDAPSourceCfg failed for %d", source.ID)
		}
	}

}
