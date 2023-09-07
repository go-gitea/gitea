// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionTask_FixtureDumper(t *testing.T) {
	task := &ActionTask{
		Started:        12,
		Stopped:        12,
		LogInStorage:   true,
		LogIndexes:     LogIndexes([]int64{1, 2, 3}),
		CommitSHA:      "aaaaaa",
		TokenSalt:      "123",
		TokenHash:      "123",
		TokenLastEight: "123",
		LogFilename:    "123",
	}

	result := strings.Builder{}
	err := task.FixtureDumper(&result)

	assert.NoError(t, err)
	assert.EqualValues(t, `-
  id: 0
  job_id: 0
  attempt: 0
  runner_id: 0
  status: 0
  started: 12
  stopped: 12
  repo_id: 0
  owner_id: 0
  commit_sha: aaaaaa
  is_fork_pull_request: false
  token_hash: 123
  token_salt: 123
  token_last_eight: 123
  log_filename: 123
  log_in_storage: true
  log_length: 0
  log_size: 0
  log_indexes: 0x020406
  log_expired: false
  created: 0
  updated: 0

`, result.String())
}
