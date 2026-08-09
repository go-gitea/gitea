// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"strings"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestRecreateEmailHashTable(t *testing.T) {
	type EmailHash struct {
		Hash  string `xorm:"pk varchar(32)"`
		Email string `xorm:"UNIQUE NOT NULL"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(EmailHash))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&EmailHash{Hash: strings.Repeat("a", 32), Email: "gitea@example.com"})
	require.NoError(t, err)

	require.NoError(t, RecreateEmailHashTable(t.Context(), x))

	count, err := x.Count(new(EmailHash))
	require.NoError(t, err)
	require.EqualValues(t, 0, count, "the unreachable MD5 rows must be gone")

	sha256Hash := strings.Repeat("b", 64)
	_, err = x.Exec("INSERT INTO email_hash (hash, email) VALUES (?, ?)", sha256Hash, "gitea@example.com")
	require.NoError(t, err, "the hash column must hold a SHA256 hash")

	_, err = x.Exec("INSERT INTO email_hash (hash, email) VALUES (?, ?)", strings.Repeat("c", 64), "gitea@example.com")
	require.Error(t, err, "the unique email index must survive the recreate")
}
