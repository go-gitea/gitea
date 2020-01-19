// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build uruyaml

package models

// This file is excluded from build and tests, and is intended for assisting
// in keeping user_repo_unit.yml in sync with the other .yml files.

// To use it, do:
// cd models
// go test -tags "uruyaml sqlite sqlite_unlock_notify" -run TestUserRepoUnitYaml

import (
	"bufio"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserRepoUnitYaml(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, RebuildAllUserRepoUnits(x))

	f, err := os.Create("fixtures/user_repo_unit.yml.new")
	assert.NoError(t, err)
	w := bufio.NewWriter(f)

	units := make([]*UserRepoUnit, 0, 200)
	assert.NoError(t, x.OrderBy("user_id, repo_id").Find(&units))
	for _, u := range units {
		fmt.Fprintf(w, "-\n")
		fmt.Fprintf(w, "  user_id: %d\n", u.UserID)
		fmt.Fprintf(w, "  repo_id: %d\n", u.RepoID)
		fmt.Fprintf(w, "  type: %d\n", u.Type)
		fmt.Fprintf(w, "  mode: %d\n", u.Mode)
		fmt.Fprintf(w, "\n")
	}

	w.Flush()
}
