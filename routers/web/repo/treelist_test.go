// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"
	"testing"

	pull_model "gitea.dev/models/pull"
	"gitea.dev/modules/fileicon"
	"gitea.dev/modules/git"
	"gitea.dev/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestTransformDiffTreeForWeb(t *testing.T) {
	renderedIconPool := fileicon.NewRenderedIconPool()
	ret := transformDiffTreeForWeb(renderedIconPool, &gitdiff.DiffTree{Files: []*gitdiff.DiffTreeRecord{
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
		{
			Status:   "renamed",
			BasePath: "file2-old",
			HeadPath: "file2",
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
					Children: []*WebDiffFileItem{
						{
							EntryMode:   "",
							DisplayName: "file-deep",
							NameHash:    "4acf7eef1c943a09e9f754e93ff190db8583236b",
							DiffStatus:  "changed",
							IsViewed:    true,
						},
					},
				},
				{
					EntryMode:   "",
					DisplayName: "file1",
					NameHash:    "60b27f004e454aca81b0480209cce5081ec52390",
					DiffStatus:  "added",
				},
				{
					EntryMode:   "",
					DisplayName: "file2",
					OldFullName: "file2-old",
					NameHash:    "cb99b709a1978bd205ab9dfd4c5aaa1fc91c7523",
					DiffStatus:  "renamed",
				},
			},
		},
		Icons:          []template.HTML{`<svg class="svg git-entry-icon octicon-file" width="16" height="16" aria-hidden="true"><use href="#svg-mfi-file"></use></svg>`},
		FolderIcon:     `<span>octicon-file-directory-fill(16/)</span>`,
		FolderOpenIcon: `<span>octicon-file-directory-open-fill(16/)</span>`,
	}, ret)
}
