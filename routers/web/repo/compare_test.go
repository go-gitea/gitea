// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestAttachHiddenCommentIDs(t *testing.T) {
	section := &gitdiff.DiffSection{
		Lines: []*gitdiff.DiffLine{
			{
				Type: gitdiff.DiffLineSection,
				SectionInfo: &gitdiff.DiffLineSectionInfo{
					LastRightIdx: 10,
					RightIdx:     20,
				},
			},
			{
				Type: gitdiff.DiffLinePlain,
			},
			{
				Type: gitdiff.DiffLineSection,
				SectionInfo: &gitdiff.DiffLineSectionInfo{
					LastRightIdx: 30,
					RightIdx:     40,
				},
			},
		},
	}

	lineComments := map[int64][]*issues_model.Comment{
		15: {{ID: 100}}, // in first section's hidden range
		35: {{ID: 200}}, // in second section's hidden range
		50: {{ID: 300}}, // outside any range
	}

	attachHiddenCommentIDs(section, lineComments)

	assert.Equal(t, []int64{100}, section.Lines[0].SectionInfo.HiddenCommentIDs)
	assert.Nil(t, section.Lines[1].SectionInfo.HiddenCommentIDs)
	assert.Equal(t, []int64{200}, section.Lines[2].SectionInfo.HiddenCommentIDs)
}

func TestAttachCommentsToLines(t *testing.T) {
	section := &gitdiff.DiffSection{
		Lines: []*gitdiff.DiffLine{
			{LeftIdx: 5, RightIdx: 10},
			{LeftIdx: 6, RightIdx: 11},
		},
	}

	lineComments := map[int64][]*issues_model.Comment{
		-5: {{ID: 100, CreatedUnix: 1000}},                               // left side comment
		10: {{ID: 200, CreatedUnix: 2000}},                               // right side comment
		11: {{ID: 300, CreatedUnix: 1500}, {ID: 301, CreatedUnix: 2500}}, // multiple comments
	}

	attachCommentsToLines(section, lineComments)

	// First line should have left and right comments
	assert.Len(t, section.Lines[0].Comments, 2)
	assert.Equal(t, int64(100), section.Lines[0].Comments[0].ID)
	assert.Equal(t, int64(200), section.Lines[0].Comments[1].ID)

	// Second line should have two comments, sorted by creation time
	assert.Len(t, section.Lines[1].Comments, 2)
	assert.Equal(t, int64(300), section.Lines[1].Comments[0].ID)
	assert.Equal(t, int64(301), section.Lines[1].Comments[1].ID)
}
