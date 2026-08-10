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

	_, err = x.Exec("INSERT INTO email_hash (hash, email, hash_type) VALUES (?, ?, ?)", strings.Repeat("b", 64), "gitea@example.com", "sha256")
	require.NoError(t, err, "the new schema must hold a SHA256 hash")
}
