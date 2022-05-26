// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
)

func TestDatabase(t *testing.T) {
	defer prepareTestEnv(t)()

	// test a SQL string
	assert.Equal(t, "'it''s fine'", db.QuoteString(db.DefaultContext, "it's fine"))

	// test all ASCII chars
	b := make([]byte, 126) // no 0, no 127, then we have 1-126 ASCII chars
	for i := 0; i < 126; i++ {
		b[i] = byte(i + 1)
	}
	raw := string(b)
	quoted := db.QuoteString(db.DefaultContext, raw)
	var res string
	_, err := db.GetEngine(db.DefaultContext).SQL(fmt.Sprintf("SELECT %s", quoted)).Get(&res)
	assert.NoError(t, err)
	assert.EqualValues(t, raw, res)
}
