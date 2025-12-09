// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"strconv"
	"strings"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			got, err := ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff), testcase.skipTo)
			if (err != nil) != testcase.wantErr {
				t.Errorf("ParsePatch(%q) error = %v, wantErr %v", testcase.name, err, testcase.wantErr)
				return
			}

			gotMarshaled, _ := json.MarshalIndent(got, "", "  ")
			if len(got.Files) != 1 {
				t.Errorf("ParsePath(%q) did not receive 1 file:\n%s", testcase.name, string(gotMarshaled))
				return
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
			got, err := ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff), "")
			if (err != nil) != testcase.wantErr {
				t.Errorf("ParsePatch(%q) error = %v, wantErr %v", testcase.name, err, testcase.wantErr)
				return
			}

			gotMarshaled, _ := json.MarshalIndent(got, "", "  ")
			if len(got.Files) != 1 {
				t.Errorf("ParsePath(%q) did not receive 1 file:\n%s", testcase.name, string(gotMarshaled))
				return
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

	for i := range 35 {
		diffBuilder.WriteString("+line" + strconv.Itoa(i) + "\n")
	}
	diff = diffBuilder.String()
	result, err := ParsePatch(t.Context(), 20, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if !result.Files[0].IsIncomplete {
		t.Errorf("Files should be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(t.Context(), 40, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if result.Files[0].IsIncomplete {
		t.Errorf("Files should not be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(t.Context(), 40, 5, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
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

	for i := range 33 {
		diffBuilder.WriteString("+line" + strconv.Itoa(i) + "\n")
	}
	diffBuilder.WriteString("+line33")
	for range 512 {
		diffBuilder.WriteString("0123456789ABCDEF")
	}
	diffBuilder.WriteByte('\n')
	diffBuilder.WriteString("+line" + strconv.Itoa(34) + "\n")
	diffBuilder.WriteString("+line" + strconv.Itoa(35) + "\n")
	diff = diffBuilder.String()

	result, err = ParsePatch(t.Context(), 20, 4096, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if !result.Files[0].IsIncomplete {
		t.Errorf("Files should be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(t.Context(), 40, 4096, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
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
	_, err = ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff), "")
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
	_, err = ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff2), "")
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
	_, err = ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff2a), "")
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
	_, err = ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff3), "")
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

func TestDiff_LoadCommentsNoOutdated(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	diff := setupDefaultDiff()
	assert.NoError(t, diff.LoadComments(t.Context(), issue, user, false))
	assert.Len(t, diff.Files[0].Sections[0].Lines[0].Comments, 2)
}

func TestDiff_LoadCommentsWithOutdated(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	diff := setupDefaultDiff()
	assert.NoError(t, diff.LoadComments(t.Context(), issue, user, true))
	assert.Len(t, diff.Files[0].Sections[0].Lines[0].Comments, 3)
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
	gitRepo, err := git.OpenRepository(t.Context(), "../../modules/git/tests/repos/repo5_pulls")
	require.NoError(t, err)

	defer gitRepo.Close()
	for _, behavior := range []gitcmd.TrustedCmdArgs{{"-w"}, {"--ignore-space-at-eol"}, {"-b"}, nil} {
		diffs, err := GetDiffForAPI(t.Context(), gitRepo,
			&DiffOptions{
				AfterCommitID:      "d8e0bbb45f200e67d9a784ce55bd90821af45ebd",
				BeforeCommitID:     "72866af952e98d02a73003501836074b286a78f6",
				MaxLines:           setting.Git.MaxGitDiffLines,
				MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
				MaxFiles:           1,
				WhitespaceBehavior: behavior,
			})
		require.NoError(t, err, "Error when diff with WhitespaceBehavior=%s", behavior)
		assert.True(t, diffs.IsIncomplete)
		assert.Len(t, diffs.Files, 1)
		for _, f := range diffs.Files {
			assert.NotEmpty(t, f.Sections, "Diff file %q should have sections", f.Name)
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
		ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff), "")
	}
}

func TestGeneratePatchForUnchangedLineFromReader(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		treePath     string
		line         int64
		contextLines int
		want         string
		wantErr      bool
	}{
		{
			name:         "single line with context",
			content:      "line1\nline2\nline3\nline4\nline5\n",
			treePath:     "test.txt",
			line:         3,
			contextLines: 1,
			want: `diff --git a/test.txt b/test.txt
--- a/test.txt
+++ b/test.txt
@@ -2,2 +2,2 @@
 line2
 line3
`,
		},
		{
			name:         "negative line number (left side)",
			content:      "line1\nline2\nline3\nline4\nline5\n",
			treePath:     "test.txt",
			line:         -3,
			contextLines: 1,
			want: `diff --git a/test.txt b/test.txt
--- a/test.txt
+++ b/test.txt
@@ -2,2 +2,2 @@
 line2
 line3
`,
		},
		{
			name:         "line near start of file",
			content:      "line1\nline2\nline3\n",
			treePath:     "test.txt",
			line:         2,
			contextLines: 5,
			want: `diff --git a/test.txt b/test.txt
--- a/test.txt
+++ b/test.txt
@@ -1,2 +1,2 @@
 line1
 line2
`,
		},
		{
			name:         "first line with context",
			content:      "line1\nline2\nline3\n",
			treePath:     "test.txt",
			line:         1,
			contextLines: 3,
			want: `diff --git a/test.txt b/test.txt
--- a/test.txt
+++ b/test.txt
@@ -1,1 +1,1 @@
 line1
`,
		},
		{
			name:         "zero context lines",
			content:      "line1\nline2\nline3\n",
			treePath:     "test.txt",
			line:         2,
			contextLines: 0,
			want: `diff --git a/test.txt b/test.txt
--- a/test.txt
+++ b/test.txt
@@ -2,1 +2,1 @@
 line2
`,
		},
		{
			name:         "multi-line context",
			content:      "package main\n\nfunc main() {\n  fmt.Println(\"Hello\")\n}\n",
			treePath:     "main.go",
			line:         4,
			contextLines: 2,
			want: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -2,3 +2,3 @@
<SP>
 func main() {
   fmt.Println("Hello")
`,
		},
		{
			name:         "empty file",
			content:      "",
			treePath:     "empty.txt",
			line:         1,
			contextLines: 1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			got, err := generatePatchForUnchangedLineFromReader(reader, tt.treePath, tt.line, tt.contextLines)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, strings.ReplaceAll(tt.want, "<SP>", " "), got)
			}
		})
	}
}

func TestCalculateHiddenCommentIDsForLine(t *testing.T) {
	tests := []struct {
		name         string
		line         *DiffLine
		lineComments map[int64][]*issues_model.Comment
		expected     []int64
	}{
		{
			name: "comments in hidden range",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx: 10,
					RightIdx:     20,
				},
			},
			lineComments: map[int64][]*issues_model.Comment{
				15: {{ID: 100}, {ID: 101}},
				12: {{ID: 102}},
			},
			expected: []int64{100, 101, 102},
		},
		{
			name: "comments outside hidden range",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx: 10,
					RightIdx:     20,
				},
			},
			lineComments: map[int64][]*issues_model.Comment{
				5:  {{ID: 100}},
				25: {{ID: 101}},
			},
			expected: nil,
		},
		{
			name: "negative line numbers (left side)",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx: 10,
					RightIdx:     20,
				},
			},
			lineComments: map[int64][]*issues_model.Comment{
				-15: {{ID: 100}}, // Left-side comment, should NOT be counted
				15:  {{ID: 101}}, // Right-side comment, should be counted
			},
			expected: []int64{101}, // Only right-side comment
		},
		{
			name: "boundary conditions - normal expansion (both boundaries exclusive)",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:  10,
					RightIdx:      20,
					RightHunkSize: 5, // Normal case: next section has content
				},
			},
			lineComments: map[int64][]*issues_model.Comment{
				10: {{ID: 100}}, // at LastRightIdx (visible line), should NOT be included
				20: {{ID: 101}}, // at RightIdx (visible line), should NOT be included
				11: {{ID: 102}}, // just inside range, should be included
				19: {{ID: 103}}, // just inside range, should be included
			},
			expected: []int64{102, 103},
		},
		{
			name: "boundary conditions - end of file expansion (RightIdx inclusive)",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:  54,
					RightIdx:      70,
					RightHunkSize: 0, // End of file: no more content after
				},
			},
			lineComments: map[int64][]*issues_model.Comment{
				54: {{ID: 54}}, // at LastRightIdx (visible line), should NOT be included
				70: {{ID: 70}}, // at RightIdx (last hidden line), SHOULD be included
				60: {{ID: 60}}, // inside range, should be included
			},
			expected: []int64{60, 70}, // Lines 60 and 70 are hidden
		},
		{
			name: "real-world scenario - start of file with hunk",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:  0,  // No previous visible section
					RightIdx:      26, // Line 26 is first visible line of hunk
					RightHunkSize: 9,  // Normal hunk with content
				},
			},
			lineComments: map[int64][]*issues_model.Comment{
				1:  {{ID: 1}},  // Line 1 is hidden
				26: {{ID: 26}}, // Line 26 is visible (hunk start) - should NOT be hidden
				10: {{ID: 10}}, // Line 10 is hidden
				15: {{ID: 15}}, // Line 15 is hidden
			},
			expected: []int64{1, 10, 15}, // Lines 1, 10, 15 are hidden; line 26 is visible
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			FillHiddenCommentIDsForDiffLine(tt.line, tt.lineComments)
			assert.ElementsMatch(t, tt.expected, tt.line.SectionInfo.HiddenCommentIDs)
		})
	}
}

func TestDiffLine_RenderBlobExcerptButtons(t *testing.T) {
	tests := []struct {
		name             string
		line             *DiffLine
		fileNameHash     string
		data             *DiffBlobExcerptData
		expectContains   []string
		expectNotContain []string
	}{
		{
			name: "expand up button with hidden comments",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:     0,
					RightIdx:         26,
					LeftIdx:          26,
					LastLeftIdx:      0,
					LeftHunkSize:     0,
					RightHunkSize:    0,
					HiddenCommentIDs: []int64{100},
				},
			},
			fileNameHash: "abc123",
			data: &DiffBlobExcerptData{
				BaseLink:      "/repo/blob_excerpt",
				AfterCommitID: "commit123",
				DiffStyle:     "unified",
			},
			expectContains: []string{
				"octicon-fold-up",
				"direction=up",
				"code-comment-more",
				"1 hidden comment(s)",
			},
		},
		{
			name: "expand up and down buttons with pull request",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:     10,
					RightIdx:         50,
					LeftIdx:          10,
					LastLeftIdx:      5,
					LeftHunkSize:     5,
					RightHunkSize:    5,
					HiddenCommentIDs: []int64{200, 201},
				},
			},
			fileNameHash: "def456",
			data: &DiffBlobExcerptData{
				BaseLink:       "/repo/blob_excerpt",
				AfterCommitID:  "commit456",
				DiffStyle:      "split",
				PullIssueIndex: 42,
			},
			expectContains: []string{
				"octicon-fold-down",
				"octicon-fold-up",
				"direction=down",
				"direction=up",
				`data-hidden-comment-ids=",200,201,"`, // use leading and trailing commas to ensure exact match by CSS selector `attr*=",id,"`
				"pull_issue_index=42",
				"2 hidden comment(s)",
			},
		},
		{
			name: "no hidden comments",
			line: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:     10,
					RightIdx:         20,
					LeftIdx:          10,
					LastLeftIdx:      5,
					LeftHunkSize:     5,
					RightHunkSize:    5,
					HiddenCommentIDs: nil,
				},
			},
			fileNameHash: "ghi789",
			data: &DiffBlobExcerptData{
				BaseLink:      "/repo/blob_excerpt",
				AfterCommitID: "commit789",
			},
			expectContains: []string{
				"code-expander-button",
			},
			expectNotContain: []string{
				"code-comment-more",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.line.RenderBlobExcerptButtons(tt.fileNameHash, tt.data)
			resultStr := string(result)

			for _, expected := range tt.expectContains {
				assert.Contains(t, resultStr, expected, "Expected to contain: %s", expected)
			}

			for _, notExpected := range tt.expectNotContain {
				assert.NotContains(t, resultStr, notExpected, "Expected NOT to contain: %s", notExpected)
			}
		})
	}
}

func TestDiffLine_GetExpandDirection(t *testing.T) {
	cases := []struct {
		name      string
		diffLine  *DiffLine
		direction string
	}{
		{
			name:      "NotSectionLine",
			diffLine:  &DiffLine{Type: DiffLineAdd, SectionInfo: &DiffLineSectionInfo{}},
			direction: "",
		},
		{
			name:      "NilSectionInfo",
			diffLine:  &DiffLine{Type: DiffLineSection, SectionInfo: nil},
			direction: "",
		},
		{
			name: "NoHiddenLines",
			// last block stops at line 100, next block starts at line 101, so no hidden lines, no expansion.
			diffLine: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx: 100,
					LastLeftIdx:  100,
					RightIdx:     101,
					LeftIdx:      101,
				},
			},
			direction: "",
		},
		{
			name: "FileHead",
			diffLine: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx: 0, // LastXxxIdx = 0 means this is the first section in the file.
					LastLeftIdx:  0,
					RightIdx:     1,
					LeftIdx:      1,
				},
			},
			direction: "",
		},
		{
			name: "FileHeadHiddenLines",
			diffLine: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx: 0,
					LastLeftIdx:  0,
					RightIdx:     101,
					LeftIdx:      101,
				},
			},
			direction: "up",
		},
		{
			name: "HiddenSingleHunk",
			diffLine: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:  100,
					LastLeftIdx:   100,
					RightIdx:      102,
					LeftIdx:       102,
					RightHunkSize: 1234, // non-zero dummy value
					LeftHunkSize:  5678, // non-zero dummy value
				},
			},
			direction: "single",
		},
		{
			name: "HiddenSingleFullHunk",
			// the hidden lines can exactly fit into one hunk
			diffLine: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:  100,
					LastLeftIdx:   100,
					RightIdx:      100 + BlobExcerptChunkSize + 1,
					LeftIdx:       100 + BlobExcerptChunkSize + 1,
					RightHunkSize: 1234, // non-zero dummy value
					LeftHunkSize:  5678, // non-zero dummy value
				},
			},
			direction: "single",
		},
		{
			name: "HiddenUpDownHunks",
			diffLine: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:  100,
					LastLeftIdx:   100,
					RightIdx:      100 + BlobExcerptChunkSize + 2,
					LeftIdx:       100 + BlobExcerptChunkSize + 2,
					RightHunkSize: 1234, // non-zero dummy value
					LeftHunkSize:  5678, // non-zero dummy value
				},
			},
			direction: "updown",
		},
		{
			name: "FileTail",
			diffLine: &DiffLine{
				Type: DiffLineSection,
				SectionInfo: &DiffLineSectionInfo{
					LastRightIdx:  100,
					LastLeftIdx:   100,
					RightIdx:      102,
					LeftIdx:       102,
					RightHunkSize: 0,
					LeftHunkSize:  0,
				},
			},
			direction: "down",
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.direction, c.diffLine.GetExpandDirection(), "case %s expected direction: %s", c.name, c.direction)
	}
}
