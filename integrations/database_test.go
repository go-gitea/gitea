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
	// Fill the slice with ASCII characters including 1 up until 126.
	b := make([]byte, 126)
	for i := byte(0); i < 126; i++ {
		b[i] = i + 1
	}
	raw := string(b)
	quoted := db.QuoteString(db.DefaultContext, raw)
	var res string
	_, err := db.GetEngine(db.DefaultContext).SQL(fmt.Sprintf("SELECT %s", quoted)).Get(&res)
	assert.NoError(t, err)
	assert.EqualValues(t, raw, res)
}
