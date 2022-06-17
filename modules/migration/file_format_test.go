// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

import (
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
)

func TestMigrationJSON_IssueOK(t *testing.T) {
	issues := make([]*Issue, 0, 10)
	err := Load("file_format_testdata/issue_a.json", &issues, true)
	assert.NoError(t, err)
	err = Load("file_format_testdata/issue_a.yml", &issues, true)
	assert.NoError(t, err)
}

func TestMigrationJSON_IssueFail(t *testing.T) {
	issues := make([]*Issue, 0, 10)
	err := Load("file_format_testdata/issue_b.json", &issues, true)
	if _, ok := err.(*jsonschema.ValidationError); ok {
		errors := strings.Split(err.(*jsonschema.ValidationError).GoString(), "\n")
		assert.Contains(t, errors[1], "missing properties")
		assert.Contains(t, errors[1], "poster_id")
	} else {
		t.Fatalf("got: type %T with value %s, want: *jsonschema.ValidationError", err, err)
	}
}

func TestMigrationJSON_MilestoneOK(t *testing.T) {
	milestones := make([]*Milestone, 0, 10)
	err := Load("file_format_testdata/milestones.json", &milestones, true)
	assert.NoError(t, err)
}
