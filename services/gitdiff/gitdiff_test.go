// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
)

func TestDiffToHTML(t *testing.T) {
	assert.Equal(t, "foo <span class=\"added-code\">bar</span> biz", diffToHTML(nil, []dmp.Diff{
		{Type: dmp.DiffEqual, Text: "foo "},
		{Type: dmp.DiffInsert, Text: "bar"},
		{Type: dmp.DiffDelete, Text: " baz"},
		{Type: dmp.DiffEqual, Text: " biz"},
	}, DiffLineAdd))

	assert.Equal(t, "foo <span class=\"removed-code\">bar</span> biz", diffToHTML(nil, []dmp.Diff{
		{Type: dmp.DiffEqual, Text: "foo "},
		{Type: dmp.DiffDelete, Text: "bar"},
		{Type: dmp.DiffInsert, Text: " baz"},
		{Type: dmp.DiffEqual, Text: " biz"},
	}, DiffLineDel))
}

func TestParsePatch_skipTo(t *testing.T) {
	type testcase struct {
		name        string
		gitdiff     string
		wantErr     bool
		addition    int
		deletion    int
		oldFilename string
		filename    string
		skipTo      string
	}
	tests := []testcase{
		{
			name: "readme.md2readme.md",
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
+ cut off
diff --git "\\a/README.md" "\\b/README.md"
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
			skipTo:      "README.md",
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
			skipTo:      "A \\ B",
		},
		{
			name: "A \\ B",
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
diff --git "a/A \\ B" "b/A \\ B"
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
			skipTo:      "A \\ B",
		},
		{
			name: "readme.md2readme.md",
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
+ cut off
diff --git "a/A \\ B" "b/A \\ B"
--- "a/A \\ B"
+++ "b/A \\ B"
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off
diff --git "\\a/README.md" "\\b/README.md"
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
			skipTo:      "README.md",
		},
	}
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			got, err := ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff), testcase.skipTo)
			if (err != nil) != testcase.wantErr {
				t.Errorf("ParsePatch(%q) error = %v, wantErr %v", testcase.name, err, testcase.wantErr)
				return
			}

			gotMarshaled, _ := json.MarshalIndent(got, "", "  ")
			if got.NumFiles != 1 {
				t.Errorf("ParsePath(%q) did not receive 1 file:\n%s", testcase.name, string(gotMarshaled))
				return
			}
			if got.TotalAddition != testcase.addition {
				t.Errorf("ParsePath(%q) does not have correct totalAddition %d, wanted %d", testcase.name, got.TotalAddition, testcase.addition)
			}
			if got.TotalDeletion != testcase.deletion {
				t.Errorf("ParsePath(%q) did not have correct totalDeletion %d, wanted %d", testcase.name, got.TotalDeletion, testcase.deletion)
			}
			file := got.Files[0]
			if file.Addition != testcase.addition {
				t.Errorf("ParsePath(%q) does not have correct file addition %d, wanted %d", testcase.name, file.Addition, testcase.addition)
			}
			if file.Deletion != testcase.deletion {
				t.Errorf("ParsePath(%q) did not have correct file deletion %d, wanted %d", testcase.name, file.Deletion, testcase.deletion)
			}
			if file.OldName != testcase.oldFilename {
				t.Errorf("ParsePath(%q) did not have correct OldName %q, wanted %q", testcase.name, file.OldName, testcase.oldFilename)
			}
			if file.Name != testcase.filename {
				t.Errorf("ParsePath(%q) did not have correct Name %q, wanted %q", testcase.name, file.Name, testcase.filename)
			}
		})
	}
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
--- "\\a/a b/file b/a a/file"	` + `
+++ "\\b/a b/file b/a a/file"	` + `
@@ -1,3 +1,2 @@
 Create a weird file.
 ` + `
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
--- "\\a/file with blanks" ` + `
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
		{
			name: "ambiguous deleted",
			gitdiff: `diff --git a/b b/b b/b b/b
deleted file mode 100644
index 92e798b..0000000
--- a/b b/b` + "\t" + `
+++ /dev/null
@@ -1 +0,0 @@
-b b/b
`,
			oldFilename: "b b/b",
			filename:    "b b/b",
			addition:    0,
			deletion:    1,
		},
		{
			name: "ambiguous addition",
			gitdiff: `diff --git a/b b/b b/b b/b
new file mode 100644
index 0000000..92e798b
--- /dev/null
+++ b/b b/b` + "\t" + `
@@ -0,0 +1 @@
+b b/b
`,
			oldFilename: "b b/b",
			filename:    "b b/b",
			addition:    1,
			deletion:    0,
		},
		{
			name: "rename",
			gitdiff: `diff --git a/b b/b b/b b/b b/b b/b
similarity index 100%
rename from b b/b b/b b/b b/b
rename to b
`,
			oldFilename: "b b/b b/b b/b b/b",
			filename:    "b",
		},
		{
			name: "ambiguous 1",
			gitdiff: `diff --git a/b b/b b/b b/b b/b b/b
similarity index 100%
rename from b b/b b/b b/b b/b
rename to b
`,
			oldFilename: "b b/b b/b b/b b/b",
			filename:    "b",
		},
		{
			name: "ambiguous 2",
			gitdiff: `diff --git a/b b/b b/b b/b b/b b/b
similarity index 100%
rename from b b/b b/b b/b
rename to b b/b
`,
			oldFilename: "b b/b b/b b/b",
			filename:    "b b/b",
		},
		{
			name: "minuses-and-pluses",
			gitdiff: `diff --git a/minuses-and-pluses b/minuses-and-pluses
index 6961180..9ba1a00 100644
--- a/minuses-and-pluses
+++ b/minuses-and-pluses
@@ -1,4 +1,4 @@
--- 1st line
-++ 2nd line
--- 3rd line
-++ 4th line
+++ 1st line
+-- 2nd line
+++ 3rd line
+-- 4th line
`,
			oldFilename: "minuses-and-pluses",
			filename:    "minuses-and-pluses",
			addition:    4,
			deletion:    4,
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			got, err := ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff), "")
			if (err != nil) != testcase.wantErr {
				t.Errorf("ParsePatch(%q) error = %v, wantErr %v", testcase.name, err, testcase.wantErr)
				return
			}

			gotMarshaled, _ := json.MarshalIndent(got, "", "  ")
			if got.NumFiles != 1 {
				t.Errorf("ParsePath(%q) did not receive 1 file:\n%s", testcase.name, string(gotMarshaled))
				return
			}
			if got.TotalAddition != testcase.addition {
				t.Errorf("ParsePath(%q) does not have correct totalAddition %d, wanted %d", testcase.name, got.TotalAddition, testcase.addition)
			}
			if got.TotalDeletion != testcase.deletion {
				t.Errorf("ParsePath(%q) did not have correct totalDeletion %d, wanted %d", testcase.name, got.TotalDeletion, testcase.deletion)
			}
			file := got.Files[0]
			if file.Addition != testcase.addition {
				t.Errorf("ParsePath(%q) does not have correct file addition %d, wanted %d", testcase.name, file.Addition, testcase.addition)
			}
			if file.Deletion != testcase.deletion {
				t.Errorf("ParsePath(%q) did not have correct file deletion %d, wanted %d", testcase.name, file.Deletion, testcase.deletion)
			}
			if file.OldName != testcase.oldFilename {
				t.Errorf("ParsePath(%q) did not have correct OldName %q, wanted %q", testcase.name, file.OldName, testcase.oldFilename)
			}
			if file.Name != testcase.filename {
				t.Errorf("ParsePath(%q) did not have correct Name %q, wanted %q", testcase.name, file.Name, testcase.filename)
			}
		})
	}

	// Test max lines
	diffBuilder := &strings.Builder{}

	diff := `diff --git a/newfile2 b/newfile2
new file mode 100644
index 0000000..6bb8f39
--- /dev/null
+++ b/newfile2
@@ -0,0 +1,35 @@
`
	diffBuilder.WriteString(diff)

	for i := 0; i < 35; i++ {
		diffBuilder.WriteString("+line" + strconv.Itoa(i) + "\n")
	}
	diff = diffBuilder.String()
	result, err := ParsePatch(20, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if !result.Files[0].IsIncomplete {
		t.Errorf("Files should be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(40, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if result.Files[0].IsIncomplete {
		t.Errorf("Files should not be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(40, 5, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if !result.Files[0].IsIncomplete {
		t.Errorf("Files should be incomplete! %v", result.Files[0])
	}

	// Test max characters
	diff = `diff --git a/newfile2 b/newfile2
new file mode 100644
index 0000000..6bb8f39
--- /dev/null
+++ b/newfile2
@@ -0,0 +1,35 @@
`
	diffBuilder.Reset()
	diffBuilder.WriteString(diff)

	for i := 0; i < 33; i++ {
		diffBuilder.WriteString("+line" + strconv.Itoa(i) + "\n")
	}
	diffBuilder.WriteString("+line33")
	for i := 0; i < 512; i++ {
		diffBuilder.WriteString("0123456789ABCDEF")
	}
	diffBuilder.WriteByte('\n')
	diffBuilder.WriteString("+line" + strconv.Itoa(34) + "\n")
	diffBuilder.WriteString("+line" + strconv.Itoa(35) + "\n")
	diff = diffBuilder.String()

	result, err = ParsePatch(20, 4096, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if !result.Files[0].IsIncomplete {
		t.Errorf("Files should be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(40, 4096, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if !result.Files[0].IsIncomplete {
		t.Errorf("Files should be incomplete! %v", result.Files[0])
	}

	diff = `diff --git "a/README.md" "b/README.md"
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
	_, err = ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("ParsePatch failed: %s", err)
	}

	diff2 := `diff --git "a/A \\ B" "b/A \\ B"
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
	_, err = ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff2), "")
	if err != nil {
		t.Errorf("ParsePatch failed: %s", err)
	}

	diff2a := `diff --git "a/A \\ B" b/A/B
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
	_, err = ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff2a), "")
	if err != nil {
		t.Errorf("ParsePatch failed: %s", err)
	}

	diff3 := `diff --git a/README.md b/README.md
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
	_, err = ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff3), "")
	if err != nil {
		t.Errorf("ParsePatch failed: %s", err)
	}
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
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	diff := setupDefaultDiff()
	assert.NoError(t, diff.LoadComments(db.DefaultContext, issue, user))
	assert.Len(t, diff.Files[0].Sections[0].Lines[0].Comments, 2)
}

func TestDiffLine_CanComment(t *testing.T) {
	assert.False(t, (&DiffLine{Type: DiffLineSection}).CanComment())
	assert.False(t, (&DiffLine{Type: DiffLineAdd, Comments: []*issues_model.Comment{{Content: "bla"}}}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLineAdd}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLineDel}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLinePlain}).CanComment())
}

func TestDiffLine_GetCommentSide(t *testing.T) {
	assert.Equal(t, "previous", (&DiffLine{Comments: []*issues_model.Comment{{Line: -3}}}).GetCommentSide())
	assert.Equal(t, "proposed", (&DiffLine{Comments: []*issues_model.Comment{{Line: 3}}}).GetCommentSide())
}

func TestGetDiffRangeWithWhitespaceBehavior(t *testing.T) {
	gitRepo, err := git.OpenRepository(git.DefaultContext, "./testdata/academic-module")
	if !assert.NoError(t, err) {
		return
	}
	defer gitRepo.Close()
	for _, behavior := range []git.TrustedCmdArgs{{"-w"}, {"--ignore-space-at-eol"}, {"-b"}, nil} {
		diffs, err := GetDiff(gitRepo,
			&DiffOptions{
				AfterCommitID:      "bd7063cc7c04689c4d082183d32a604ed27a24f9",
				BeforeCommitID:     "559c156f8e0178b71cb44355428f24001b08fc68",
				MaxLines:           setting.Git.MaxGitDiffLines,
				MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
				MaxFiles:           setting.Git.MaxGitDiffFiles,
				WhitespaceBehavior: behavior,
			})
		assert.NoError(t, err, fmt.Sprintf("Error when diff with %s", behavior))
		for _, f := range diffs.Files {
			assert.True(t, len(f.Sections) > 0, fmt.Sprintf("%s should have sections", f.Name))
		}
	}
}

func TestNoCrashes(t *testing.T) {
	type testcase struct {
		gitdiff string
	}

	tests := []testcase{
		{
			gitdiff: "diff --git \n--- a\t\n",
		},
		{
			gitdiff: "diff --git \"0\n",
		},
	}
	for _, testcase := range tests {
		// It shouldn't crash, so don't care about the output.
		ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff), "")
	}
}
