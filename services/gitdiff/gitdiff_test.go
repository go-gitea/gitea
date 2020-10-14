// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdiff

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
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

func TestParsePatch_singlefile(t *testing.T) {
	type testcase struct {
		name        string
		gitdiff     string
		wantErr     bool
		addition    int
		deletion    int
		oldFilename string
		filename    string
	}

	tests := []testcase{
		{
			name: "readme.md2readme.md",
			gitdiff: `diff --git "\\a/README.md" "\\b/README.md"
--- "\\a/README.md"
+++ "\\b/README.md"
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off
`,
			addition:    4,
			deletion:    1,
			filename:    "README.md",
			oldFilename: "README.md",
		},
		{
			name: "A \\ B",
			gitdiff: `diff --git "a/A \\ B" "b/A \\ B"
--- "a/A \\ B"
+++ "b/A \\ B"
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off`,
			addition:    4,
			deletion:    1,
			filename:    "A \\ B",
			oldFilename: "A \\ B",
		},
		{
			name: "really weird filename",
			gitdiff: `diff --git "\\a/a b/file b/a a/file" "\\b/a b/file b/a a/file"
index d2186f1..f5c8ed2 100644
--- "\\a/a b/file b/a a/file"	
+++ "\\b/a b/file b/a a/file"	
@@ -1,3 +1,2 @@
 Create a weird file.
 
-and what does diff do here?
\ No newline at end of file`,
			addition:    0,
			deletion:    1,
			filename:    "a b/file b/a a/file",
			oldFilename: "a b/file b/a a/file",
		},
		{
			name: "delete file with blanks",
			gitdiff: `diff --git "\\a/file with blanks" "\\b/file with blanks"
deleted file mode 100644
index 898651a..0000000
--- "\\a/file with blanks"	
+++ /dev/null
@@ -1,5 +0,0 @@
-a blank file
-
-has a couple o line
-
-the 5th line is the last
`,
			addition:    0,
			deletion:    5,
			filename:    "file with blanks",
			oldFilename: "file with blanks",
		},
		{
			name: "rename a—as",
			gitdiff: `diff --git "a/\360\243\220\265b\342\200\240vs" "b/a\342\200\224as"
similarity index 100%
rename from "\360\243\220\265b\342\200\240vs"
rename to "a\342\200\224as"
`,
			addition:    0,
			deletion:    0,
			oldFilename: "𣐵b†vs",
			filename:    "a—as",
		},
		{
			name: "rename with spaces",
			gitdiff: `diff --git "\\a/a b/file b/a a/file" "\\b/a b/a a/file b/b file"
similarity index 100%
rename from a b/file b/a a/file
rename to a b/a a/file b/b file
`,
			oldFilename: "a b/file b/a a/file",
			filename:    "a b/a a/file b/b file",
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			got, err := ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff))
			if (err != nil) != testcase.wantErr {
				t.Errorf("ParsePatch() error = %v, wantErr %v", err, testcase.wantErr)
				return
			}
			gotMarshaled, _ := json.MarshalIndent(got, "  ", "  ")
			if got.NumFiles != 1 {
				t.Errorf("ParsePath() did not receive 1 file:\n%s", string(gotMarshaled))
				return
			}
			if got.TotalAddition != testcase.addition {
				t.Errorf("ParsePath() does not have correct totalAddition %d, wanted %d", got.TotalAddition, testcase.addition)
			}
			if got.TotalDeletion != testcase.deletion {
				t.Errorf("ParsePath() did not have correct totalDeletion %d, wanted %d", got.TotalDeletion, testcase.deletion)
			}
			file := got.Files[0]
			if file.Addition != testcase.addition {
				t.Errorf("ParsePath() does not have correct file addition %d, wanted %d", file.Addition, testcase.addition)
			}
			if file.Deletion != testcase.deletion {
				t.Errorf("ParsePath() did not have correct file deletion %d, wanted %d", file.Deletion, testcase.deletion)
			}
			if file.OldName != testcase.oldFilename {
				t.Errorf("ParsePath() did not have correct OldName %s, wanted %s", file.OldName, testcase.oldFilename)
			}
			if file.Name != testcase.filename {
				t.Errorf("ParsePath() did not have correct Name %s, wanted %s", file.Name, testcase.filename)
			}
		})
	}

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

	var diff2a = `diff --git "a/A \\ B" b/A/B
--- "a/A \\ B"
+++ b/A/B
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off`
	result, err = ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff2a))
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
