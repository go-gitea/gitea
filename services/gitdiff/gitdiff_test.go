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

	"gopkg.in/ini.v1"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
)

func assertEqual(t *testing.T, s1 string, s2 template.HTML) {
	if s1 != string(s2) {
		t.Errorf("%s should be equal %s", s2, s1)
	}
}

func TestDiffToHTML(t *testing.T) {
	setting.Cfg = ini.Empty()
	assertEqual(t, "foo <span class=\"added-code\">bar</span> biz", diffToHTML("", []dmp.Diff{
		{Type: dmp.DiffEqual, Text: "foo "},
		{Type: dmp.DiffInsert, Text: "bar"},
		{Type: dmp.DiffDelete, Text: " baz"},
		{Type: dmp.DiffEqual, Text: " biz"},
	}, DiffLineAdd))

	assertEqual(t, "foo <span class=\"removed-code\">bar</span> biz", diffToHTML("", []dmp.Diff{
		{Type: dmp.DiffEqual, Text: "foo "},
		{Type: dmp.DiffDelete, Text: "bar"},
		{Type: dmp.DiffInsert, Text: " baz"},
		{Type: dmp.DiffEqual, Text: " biz"},
	}, DiffLineDel))

	assertEqual(t, "<span class=\"k\">if</span> <span class=\"p\">!</span><span class=\"nx\">nohl</span> <span class=\"o\">&amp;&amp;</span> <span class=\"added-code\"><span class=\"p\">(</span></span><span class=\"nx\">lexer</span> <span class=\"o\">!=</span> <span class=\"kc\">nil</span><span class=\"added-code\"> <span class=\"o\">||</span> <span class=\"nx\">r</span><span class=\"p\">.</span><span class=\"nx\">GuessLanguage</span><span class=\"p\">)</span></span> <span class=\"p\">{</span>", diffToHTML("", []dmp.Diff{
		{Type: dmp.DiffEqual, Text: "<span class=\"k\">if</span> <span class=\"p\">!</span><span class=\"nx\">nohl</span> <span class=\"o\">&amp;&amp;</span> <span class=\""},
		{Type: dmp.DiffInsert, Text: "p\">(</span><span class=\""},
		{Type: dmp.DiffEqual, Text: "nx\">lexer</span> <span class=\"o\">!=</span> <span class=\"kc\">nil"},
		{Type: dmp.DiffInsert, Text: "</span> <span class=\"o\">||</span> <span class=\"nx\">r</span><span class=\"p\">.</span><span class=\"nx\">GuessLanguage</span><span class=\"p\">)"},
		{Type: dmp.DiffEqual, Text: "</span> <span class=\"p\">{</span>"},
	}, DiffLineAdd))

	assertEqual(t, "<span class=\"nx\">tagURL</span> <span class=\"o\">:=</span> <span class=\"removed-code\"><span class=\"nx\">fmt</span><span class=\"p\">.</span><span class=\"nf\">Sprintf</span><span class=\"p\">(</span><span class=\"s\">&#34;## [%s](%s/%s/%s/%s?q=&amp;type=all&amp;state=closed&amp;milestone=%d) - %s&#34;</span><span class=\"p\">,</span> <span class=\"nx\">ge</span><span class=\"p\">.</span><span class=\"nx\">Milestone\"</span></span><span class=\"p\">,</span> <span class=\"nx\">ge</span><span class=\"p\">.</span><span class=\"nx\">BaseURL</span><span class=\"p\">,</span> <span class=\"nx\">ge</span><span class=\"p\">.</span><span class=\"nx\">Owner</span><span class=\"p\">,</span> <span class=\"nx\">ge</span><span class=\"p\">.</span><span class=\"nx\">Repo</span><span class=\"p\">,</span> <span class=\"nx\"><span class=\"removed-code\">from</span><span class=\"p\">,</span> <span class=\"nx\">milestoneID</span><span class=\"p\">,</span> <span class=\"nx\">time</span><span class=\"p\">.</span><span class=\"nf\">Now</span><span class=\"p\">(</span><span class=\"p\">)</span><span class=\"p\">.</span><span class=\"nf\">Format</span><span class=\"p\">(</span><span class=\"s\">&#34;2006-01-02&#34;</span><span class=\"p\">)</span></span><span class=\"p\">)</span>", diffToHTML("", []dmp.Diff{
		{Type: dmp.DiffEqual, Text: "<span class=\"nx\">tagURL</span> <span class=\"o\">:=</span> <span class=\"n"},
		{Type: dmp.DiffDelete, Text: "x\">fmt</span><span class=\"p\">.</span><span class=\"nf\">Sprintf</span><span class=\"p\">(</span><span class=\"s\">&#34;## [%s](%s/%s/%s/%s?q=&amp;type=all&amp;state=closed&amp;milestone=%d) - %s&#34;</span><span class=\"p\">,</span> <span class=\"nx\">ge</span><span class=\"p\">.</span><span class=\"nx\">Milestone\""},
		{Type: dmp.DiffInsert, Text: "f\">getGiteaTagURL</span><span class=\"p\">(</span><span class=\"nx\">client"},
		{Type: dmp.DiffEqual, Text: "</span><span class=\"p\">,</span> <span class=\"nx\">ge</span><span class=\"p\">.</span><span class=\"nx\">BaseURL</span><span class=\"p\">,</span> <span class=\"nx\">ge</span><span class=\"p\">.</span><span class=\"nx\">Owner</span><span class=\"p\">,</span> <span class=\"nx\">ge</span><span class=\"p\">.</span><span class=\"nx\">Repo</span><span class=\"p\">,</span> <span class=\"nx\">"},
		{Type: dmp.DiffDelete, Text: "from</span><span class=\"p\">,</span> <span class=\"nx\">milestoneID</span><span class=\"p\">,</span> <span class=\"nx\">time</span><span class=\"p\">.</span><span class=\"nf\">Now</span><span class=\"p\">(</span><span class=\"p\">)</span><span class=\"p\">.</span><span class=\"nf\">Format</span><span class=\"p\">(</span><span class=\"s\">&#34;2006-01-02&#34;</span><span class=\"p\">)"},
		{Type: dmp.DiffInsert, Text: "ge</span><span class=\"p\">.</span><span class=\"nx\">Milestone</span><span class=\"p\">,</span> <span class=\"nx\">from</span><span class=\"p\">,</span> <span class=\"nx\">milestoneID"},
		{Type: dmp.DiffEqual, Text: "</span><span class=\"p\">)</span>"},
	}, DiffLineDel))

	assertEqual(t, "<span class=\"nx\">r</span><span class=\"p\">.</span><span class=\"nf\">WrapperRenderer</span><span class=\"p\">(</span><span class=\"nx\">w</span><span class=\"p\">,</span> <span class=\"nx\"><span class=\"removed-code\">language</span></span><span class=\"removed-code\"><span class=\"p\">,</span> <span class=\"kc\">true</span><span class=\"p\">,</span> <span class=\"nx\">attrs</span></span><span class=\"p\">,</span> <span class=\"kc\">false</span><span class=\"p\">)</span>", diffToHTML("", []dmp.Diff{
		{Type: dmp.DiffEqual, Text: "<span class=\"nx\">r</span><span class=\"p\">.</span><span class=\"nf\">WrapperRenderer</span><span class=\"p\">(</span><span class=\"nx\">w</span><span class=\"p\">,</span> <span class=\"nx\">"},
		{Type: dmp.DiffDelete, Text: "language</span><span "},
		{Type: dmp.DiffEqual, Text: "c"},
		{Type: dmp.DiffDelete, Text: "lass=\"p\">,</span> <span class=\"kc\">true</span><span class=\"p\">,</span> <span class=\"nx\">attrs"},
		{Type: dmp.DiffEqual, Text: "</span><span class=\"p\">,</span> <span class=\"kc\">false</span><span class=\"p\">)</span>"},
	}, DiffLineDel))

	assertEqual(t, "<span class=\"added-code\">language</span></span><span class=\"added-code\"><span class=\"p\">,</span> <span class=\"kc\">true</span><span class=\"p\">,</span> <span class=\"nx\">attrs</span></span><span class=\"p\">,</span> <span class=\"kc\">false</span><span class=\"p\">)</span>", diffToHTML("", []dmp.Diff{
		{Type: dmp.DiffInsert, Text: "language</span><span "},
		{Type: dmp.DiffEqual, Text: "c"},
		{Type: dmp.DiffInsert, Text: "lass=\"p\">,</span> <span class=\"kc\">true</span><span class=\"p\">,</span> <span class=\"nx\">attrs"},
		{Type: dmp.DiffEqual, Text: "</span><span class=\"p\">,</span> <span class=\"kc\">false</span><span class=\"p\">)</span>"},
	}, DiffLineAdd))
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
