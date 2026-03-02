// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCommitStatusEventNameAndCommitID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repoID := int64(1)
	pushCommitSHA := "abc123def456abc123def456abc123def456abc1"

	pushPayload, err := json.Marshal(&api.PushPayload{
		HeadCommit: &api.PayloadCommit{ID: pushCommitSHA},
	})
	require.NoError(t, err)

	nextIndex := func() int64 {
		idx, err := db.GetNextResourceIndex(ctx, "action_run_index", repoID)
		require.NoError(t, err)
		return idx
	}

	pushRun := &actions_model.ActionRun{
		Index:         nextIndex(),
		RepoID:        repoID,
		Event:         webhook_module.HookEventPush,
		EventPayload:  string(pushPayload),
		TriggerUserID: 1,
		CommitSHA:     pushCommitSHA,
		Ref:           "refs/heads/main",
		WorkflowID:    "push.yml",
		Title:         "push run",
	}
	require.NoError(t, db.Insert(ctx, pushRun))

	t.Run("WorkflowRunWithPushParent", func(t *testing.T) {
		wrPayload, err := json.Marshal(&api.WorkflowRunPayload{
			WorkflowRun: &api.ActionWorkflowRun{
				ID: pushRun.ID,
			},
		})
		require.NoError(t, err)

		wrRun := &actions_model.ActionRun{
			Index:         nextIndex(),
			RepoID:        repoID,
			Event:         webhook_module.HookEventWorkflowRun,
			EventPayload:  string(wrPayload),
			TriggerUserID: 1,
			CommitSHA:     "default-branch-head-000000000000000000000",
			Ref:           "refs/heads/main",
			WorkflowID:    "wr.yml",
			Title:         "workflow_run run",
		}
		require.NoError(t, db.Insert(ctx, wrRun))

		event, commitID, err := getCommitStatusEventNameAndCommitID(ctx, wrRun)
		require.NoError(t, err)
		assert.Equal(t, "workflow_run", event)
		assert.Equal(t, pushCommitSHA, commitID)
	})

	t.Run("WorkflowRunChainedTwoLevels", func(t *testing.T) {
		midPayload, err := json.Marshal(&api.WorkflowRunPayload{
			WorkflowRun: &api.ActionWorkflowRun{
				ID: pushRun.ID,
			},
		})
		require.NoError(t, err)

		midRun := &actions_model.ActionRun{
			Index:         nextIndex(),
			RepoID:        repoID,
			Event:         webhook_module.HookEventWorkflowRun,
			EventPayload:  string(midPayload),
			TriggerUserID: 1,
			CommitSHA:     "default-branch-head-000000000000000000000",
			Ref:           "refs/heads/main",
			WorkflowID:    "mid.yml",
			Title:         "mid workflow_run",
		}
		require.NoError(t, db.Insert(ctx, midRun))

		leafPayload, err := json.Marshal(&api.WorkflowRunPayload{
			WorkflowRun: &api.ActionWorkflowRun{
				ID: midRun.ID,
			},
		})
		require.NoError(t, err)

		leafRun := &actions_model.ActionRun{
			Index:         nextIndex(),
			RepoID:        repoID,
			Event:         webhook_module.HookEventWorkflowRun,
			EventPayload:  string(leafPayload),
			TriggerUserID: 1,
			CommitSHA:     "default-branch-head-200000000000000000000",
			Ref:           "refs/heads/main",
			WorkflowID:    "leaf.yml",
			Title:         "leaf workflow_run",
		}
		require.NoError(t, db.Insert(ctx, leafRun))

		event, commitID, err := getCommitStatusEventNameAndCommitID(ctx, leafRun)
		require.NoError(t, err)
		assert.Equal(t, "workflow_run", event)
		assert.Equal(t, pushCommitSHA, commitID)
	})

	t.Run("WorkflowRunNilWorkflowRun", func(t *testing.T) {
		payload, err := json.Marshal(&api.WorkflowRunPayload{
			WorkflowRun: nil,
		})
		require.NoError(t, err)

		run := &actions_model.ActionRun{
			Index:         nextIndex(),
			RepoID:        repoID,
			Event:         webhook_module.HookEventWorkflowRun,
			EventPayload:  string(payload),
			TriggerUserID: 1,
			CommitSHA:     "some-sha-0000000000000000000000000000000",
			Ref:           "refs/heads/main",
			WorkflowID:    "nil.yml",
			Title:         "nil workflow_run",
		}
		require.NoError(t, db.Insert(ctx, run))

		event, commitID, err := getCommitStatusEventNameAndCommitID(ctx, run)
		require.NoError(t, err)
		assert.Empty(t, event)
		assert.Empty(t, commitID)
	})
}
