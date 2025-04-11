// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	pull_model "code.gitea.io/gitea/models/pull"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestTransformDiffTreeForWeb(t *testing.T) {
	ret := transformDiffTreeForWeb(&gitdiff.DiffTree{Files: []*gitdiff.DiffTreeRecord{
		{
			Status:   "changed",
			HeadPath: "dir-a/dir-a-x/file-deep",
			HeadMode: git.EntryModeBlob,
		},
		{
			Status:   "added",
			HeadPath: "file1",
			HeadMode: git.EntryModeBlob,
		},
	}}, map[string]pull_model.ViewedState{
		"dir-a/dir-a-x/file-deep": pull_model.Viewed,
	})

	assert.Equal(t, WebDiffFileTree{
		TreeRoot: WebDiffFileItem{
			Children: []*WebDiffFileItem{
				{
					EntryMode:   "tree",
					DisplayName: "dir-a/dir-a-x",
					FullName:    "dir-a/dir-a-x",
					Children: []*WebDiffFileItem{
						{
							EntryMode:   "",
							DisplayName: "file-deep",
							FullName:    "dir-a/dir-a-x/file-deep",
							NameHash:    "4acf7eef1c943a09e9f754e93ff190db8583236b",
							DiffStatus:  "changed",
							IsViewed:    true,
						},
					},
				},
				{
					EntryMode:   "",
					DisplayName: "file1",
					FullName:    "file1",
					NameHash:    "60b27f004e454aca81b0480209cce5081ec52390",
					DiffStatus:  "added",
				},
			},
		},
	}, ret)
}
