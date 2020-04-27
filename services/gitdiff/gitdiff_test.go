// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdiff

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
)

func assertEqual(t *testing.T, s1 string, s2 template.HTML) {
	if s1 != string(s2) {
		t.Errorf("%s should be equal %s", s2, s1)
	}
}

func TestDiffToHTML(t *testing.T) {
	assertEqual(t, "foo <span class=\"added-code\">bar</span> biz", diffToHTML([]dmp.Diff{
		{Type: dmp.DiffEqual, Text: "foo "},
		{Type: dmp.DiffInsert, Text: "bar"},
		{Type: dmp.DiffDelete, Text: " baz"},
		{Type: dmp.DiffEqual, Text: " biz"},
	}, DiffLineAdd))

	assertEqual(t, "foo <span class=\"removed-code\">bar</span> biz", diffToHTML([]dmp.Diff{
		{Type: dmp.DiffEqual, Text: "foo "},
		{Type: dmp.DiffDelete, Text: "bar"},
		{Type: dmp.DiffInsert, Text: " baz"},
		{Type: dmp.DiffEqual, Text: " biz"},
	}, DiffLineDel))
}

func TestParsePatch(t *testing.T) {
	var diff = `diff --git "a/README.md" "b/README.md"
--- a/README.md
+++ b/README.md
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off`
	result, err := ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff))
	if err != nil {
		t.Errorf("ParsePatch failed: %s", err)
	}
	println(result)

	var diff2 = `diff --git "a/A \\ B" "b/A \\ B"
--- "a/A \\ B"
+++ "b/A \\ B"
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off`
	result, err = ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff2))
	if err != nil {
		t.Errorf("ParsePatch failed: %s", err)
	}
	println(result)

	var diff3 = `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off`
	result, err = ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff3))
	if err != nil {
		t.Errorf("ParsePatch failed: %s", err)
	}
	println(result)
}

func setupDefaultDiff() *Diff {
	return &Diff{
		Files: []*DiffFile{
			{
				Name: "README.md",
				Sections: []*DiffSection{
					{
						Lines: []*DiffLine{
							{
								LeftIdx:  4,
								RightIdx: 4,
							},
						},
					},
				},
			},
		},
	}
}
func TestDiff_LoadComments(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 2}).(*models.Issue)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)
	diff := setupDefaultDiff()
	assert.NoError(t, diff.LoadComments(issue, user))
	assert.Len(t, diff.Files[0].Sections[0].Lines[0].Comments, 2)
}

func TestDiffLine_CanComment(t *testing.T) {
	assert.False(t, (&DiffLine{Type: DiffLineSection}).CanComment())
	assert.False(t, (&DiffLine{Type: DiffLineAdd, Comments: []*models.Comment{{Content: "bla"}}}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLineAdd}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLineDel}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLinePlain}).CanComment())
}

func TestDiffLine_GetCommentSide(t *testing.T) {
	assert.Equal(t, "previous", (&DiffLine{Comments: []*models.Comment{{Line: -3}}}).GetCommentSide())
	assert.Equal(t, "proposed", (&DiffLine{Comments: []*models.Comment{{Line: 3}}}).GetCommentSide())
}

func TestGetDiffRangeWithWhitespaceBehavior(t *testing.T) {
	git.Debug = true
	for _, behavior := range []string{"-w", "--ignore-space-at-eol", "-b", ""} {
		diffs, err := GetDiffRangeWithWhitespaceBehavior("./testdata/academic-module", "559c156f8e0178b71cb44355428f24001b08fc68", "bd7063cc7c04689c4d082183d32a604ed27a24f9",
			setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles, behavior)
		assert.NoError(t, err, fmt.Sprintf("Error when diff with %s", behavior))
		for _, f := range diffs.Files {
			assert.True(t, len(f.Sections) > 0, fmt.Sprintf("%s should have sections", f.Name))
		}
	}
}
