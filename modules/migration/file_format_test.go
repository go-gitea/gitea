// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
)

func TestMigrationJSON_IssueOK(t *testing.T) {
	issues := make([]*Issue, 0, 10)
	err := Load("file_format_testdata/issue_a.json", &issues, true)
	assert.NoError(t, err)
	if assert.Len(t, issues, 1) && assert.Len(t, issues[0].Reactions, 1) {
		assert.Equal(t, time.Date(1986, 4, 12, 23, 20, 50, 520000000, time.UTC), issues[0].Reactions[0].Created)
	}
	err = Load("file_format_testdata/issue_a.yml", &issues, true)
	assert.NoError(t, err)
	if assert.Len(t, issues, 1) && assert.Len(t, issues[0].Reactions, 1) {
		assert.True(t, issues[0].Reactions[0].Created.IsZero())
		assert.Equal(t, []string{"legacy-assignee"}, issues[0].Assignees)
	}
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
