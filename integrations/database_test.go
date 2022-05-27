// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
)

func TestDatabase(t *testing.T) {
	defer prepareTestEnv(t)()

	// test a SQL string
	assert.Equal(t, "'a normal string'", db.QuoteSanitized("a normal string"))
	assert.Equal(t, "'a slash  char'", db.QuoteSanitized("a slash \\ char"))
	assert.Equal(t, "'it''s fine'", db.QuoteSanitized("it's fine"))

	// test all ASCII chars with real database
	b := make([]byte, 128)
	for i := byte(0); i < 128; i++ {
		b[i] = i
	}
	raw := string(b)
	quoted := db.QuoteSanitized(raw)
	var res string
	_, err := db.GetEngine(db.DefaultContext).SQL(fmt.Sprintf("SELECT %s", quoted)).Get(&res)
	assert.NoError(t, err)

	expected := raw
	expected = strings.ReplaceAll(expected, "\x00", "")
	expected = strings.ReplaceAll(expected, "\\", "")
	assert.EqualValues(t, expected, res)
}
