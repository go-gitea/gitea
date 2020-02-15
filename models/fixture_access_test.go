// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build access_fixtures

package models

// This file is excluded from build and tests, and is intended for assisting
// in keeping access.yml in sync with the other .yml files.

// To use it, do:
// cd models
// go test -tags "access_fixtures sqlite sqlite_unlock_notify" -run TestBuildAccessFixturesYaml

import (
	"bufio"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildAccessFixturesYaml(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repos := make([]*Repository, 0, 50)
	assert.NoError(t, x.Find(&repos))
	for _, repo := range repos {
		repo.MustOwner()
		assert.NoError(t, repo.RecalculateAccesses())
	}

	f, err := os.Create("fixtures/access.yml")
	assert.NoError(t, err)
	w := bufio.NewWriter(f)

	accesses := make([]*Access, 0, 200)
	assert.NoError(t, x.OrderBy("user_id, repo_id").Find(&accesses))
	for i, a := range accesses {
		fmt.Fprintf(w, "-\n")
		fmt.Fprintf(w, "  id: %d\n", i+1)
		fmt.Fprintf(w, "  user_id: %d\n", a.UserID)
		fmt.Fprintf(w, "  repo_id: %d\n", a.RepoID)
		fmt.Fprintf(w, "  mode: %d\n", a.Mode)
		fmt.Fprintf(w, "\n")
	}

	w.Flush()
	f.Close()
}
