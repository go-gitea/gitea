// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"testing"

	"gitea.dev/models/db"

	"github.com/stretchr/testify/assert"
)

func TestWithIDTiebreaker(t *testing.T) {
	for orderBy, expected := range map[string]string{
		"":                           "`repo`.id ASC",
		"updated_unix DESC":          "updated_unix DESC, `repo`.id DESC",
		"name":                       "name, `repo`.id ASC",
		"commit_time DESC, name ASC": "commit_time DESC, name ASC, `repo`.id ASC",
		"num_stars desc, id asc":     "num_stars desc, id asc",
		"`repo`.`id` DESC":           "`repo`.`id` DESC",
		"`other`.id DESC":            "`other`.id DESC, `repo`.id DESC",
	} {
		assert.Equal(t, expected, db.WithIDTiebreaker(orderBy, "repo"), orderBy)
	}
}
