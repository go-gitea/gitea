// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"strings"
	"testing"

	"gitea.dev/modules/json"

	"github.com/santhosh-tekuri/jsonschema/v6"
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

// TestMigrateOptionsSSHKeyOwnerIDRoundtrip guards against the regression where
// SSHKeyOwnerID was tagged `json:"-"` and silently lost when the task layer
// serialised/deserialised MigrateOptions through the task table, breaking the
// "use a specific owner's managed key" override for org migrations.
func TestMigrateOptionsSSHKeyOwnerIDRoundtrip(t *testing.T) {
	data, err := json.Marshal(MigrateOptions{SSHKeyOwnerID: 42})
	assert.NoError(t, err)

	var got MigrateOptions
	assert.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, int64(42), got.SSHKeyOwnerID)
}

// TestMigrateOptionsSSHKeyOwnerIDOmitemptyZero ensures the default value (use
// repository owner's key) is omitted from the serialised JSON so existing task
// payloads in the DB stay compact and backward-compatible.
func TestMigrateOptionsSSHKeyOwnerIDOmitemptyZero(t *testing.T) {
	data, err := json.Marshal(MigrateOptions{})
	assert.NoError(t, err)
	assert.NotContains(t, string(data), "ssh_key_owner_id")
}
