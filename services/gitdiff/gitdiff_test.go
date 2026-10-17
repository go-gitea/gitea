// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"strconv"
	"strings"
	"testing"

	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDiffSkipTo(t *testing.T) {
	repo, err := git.ForceFastImportWithInit(t.Context(), t.TempDir(), []git.FastImportCommit{{Ref: "refs/heads/main", Files: []git.FastImportFile{
		{Path: "a.txt", Content: "a\n"}, {Path: "b.txt", Content: "b\n"}, {Path: "c.txt", Content: "c\n"},
	}}})
	require.NoError(t, err)
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	diff, err := GetDiffForAPI(t.Context(), gitRepo, &DiffOptions{
		AfterCommitID:     "main",
		SkipTo:            "b.txt",
		MaxLines:          setting.Git.MaxGitDiffLines,
		MaxLineCharacters: setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:          setting.Git.MaxGitDiffFiles,
	})
	require.NoError(t, err)
	require.Len(t, diff.Files, 2)
	assert.Equal(t, "b.txt", diff.Files[0].Name)
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
			name: "same name",
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
			name: `A \ B`,
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
			filename:    `A \ B`,
			oldFilename: `A \ B`,
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
			name: "ambiguous rename 1",
			gitdiff: `diff --git a/b b/b b/b b/b b/b b/b
similarity index 100%
rename from b b/b b/b b/b b/b
rename to b
`,
			oldFilename: "b b/b b/b b/b b/b",
			filename:    "b",
		},
		{
			name: "ambiguous rename 2",
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
			got, err := ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff))
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
	result, err := ParsePatch(t.Context(), 20, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff))
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if !result.Files[0].IsIncomplete {
		t.Errorf("Files should be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(t.Context(), 40, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff))
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if result.Files[0].IsIncomplete {
		t.Errorf("Files should not be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(t.Context(), 40, 5, setting.Git.MaxGitDiffFiles, strings.NewReader(diff))
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if result.Files[0].IsIncomplete {
		t.Errorf("Files should not be incomplete! %v", result.Files[0])
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

	result, err = ParsePatch(t.Context(), 20, 4096, setting.Git.MaxGitDiffFiles, strings.NewReader(diff))
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if !result.Files[0].IsIncomplete {
		t.Errorf("Files should be incomplete! %v", result.Files[0])
	}
	result, err = ParsePatch(t.Context(), 40, 4096, setting.Git.MaxGitDiffFiles, strings.NewReader(diff))
	if err != nil {
		t.Errorf("There should not be an error: %v", err)
	}
	if result.Files[0].IsIncomplete {
		t.Errorf("Files should not be incomplete! %v", result.Files[0])
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
	_, err = ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff))
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
	_, err = ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff2))
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
	_, err = ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff2a))
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
	_, err = ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(diff3))
	if err != nil {
		t.Errorf("ParsePatch failed: %s", err)
	}
}

func TestParsePatchLongLines(t *testing.T) {
	overSizedContent := strings.Repeat("a", defaultDiffLineBufferSize*2)
	for _, test := range []struct {
		name, full string
		limit      int
		result     string
	}{
		{name: "below", limit: 8, full: "short", result: "short"},
		{name: "exact", limit: 8, full: "1234567", result: "1234567"},
		{name: "above", limit: 8, full: "12345678", result: "1234567"},
		{name: "multiple fragments", limit: 101, full: overSizedContent, result: overSizedContent[:100]},
		{name: "cutoff-truncate", limit: 8, full: "a🙂b🙂", result: "a🙂b"},
		{name: "cutoff-no-truncate", limit: 12, full: "a🙂b🙂", result: "a🙂b🙂"},
	} {
		for _, eol := range []string{"\n", "\r\n"} {
			t.Run(test.name+"/"+strconv.Quote(eol), func(t *testing.T) {
				patch := strings.Join([]string{
					"diff --git a/first b/first",
					"--- a/first",
					"+++ b/first",
					"@@ -1,3 +1,3 @@",
					"-" + test.full,
					"+" + test.full,
					" " + test.full,
					" short",
					"@@ -10 +10 @@",
					" next",
					"diff --git a/second b/second",
					"--- a/second", "+++ b/second",
					"@@ -1 +1 @@",
					" final",
					"",
				}, eol)
				maxLines, maxFiles := 20, 10
				diff, err := ParsePatch(t.Context(), maxLines, test.limit, maxFiles, strings.NewReader(patch))
				require.NoError(t, err)
				require.Len(t, diff.Files, 2)
				file := diff.Files[0]
				assert.False(t, file.IsIncomplete)
				require.Len(t, file.Sections, 2)
				lines := file.Sections[0].Lines
				require.Len(t, lines, 5)
				for i, marker := range []string{"-", "+", " "} {
					assert.Equal(t, marker+test.result, lines[i+1].Content)
					assert.Equal(t, test.result != test.full, lines[i+1].IsTruncated)
				}
				assert.Equal(t, 2, lines[1].Match)
				assert.Equal(t, 1, lines[2].Match)
				assert.Equal(t, &DiffLine{Type: DiffLinePlain, LeftIdx: 3, RightIdx: 3, Content: " short"}, lines[4])
				assert.Equal(t, &DiffLine{Type: DiffLinePlain, LeftIdx: 10, RightIdx: 10, Content: " next"}, file.Sections[1].Lines[1])
				assert.False(t, diff.Files[1].IsIncomplete)
				assert.Equal(t, &DiffLine{Type: DiffLinePlain, LeftIdx: 1, RightIdx: 1, Content: " final"}, diff.Files[1].Sections[0].Lines[1])
			})
		}
	}
}

func TestParsePatchExactLineLimit(t *testing.T) {
	for _, test := range []struct {
		name, hunk   string
		limit, lines int
		incomplete   bool
	}{
		{name: "zero", limit: 0, hunk: "@@ -1,3 +1,3 @@\n one\n two\n three\n", incomplete: true},
		{name: "one", limit: 1, lines: 1, hunk: "@@ -1,3 +1,3 @@\n one\n two\n three\n", incomplete: true},
		{name: "N plus one", limit: 2, lines: 2, hunk: "@@ -1,3 +1,3 @@\n one\n two\n three\n", incomplete: true},
		{name: "N", limit: 3, lines: 3, hunk: "@@ -1,3 +1,3 @@\n one\n two\n three\n"},
		{name: "addition", limit: 1, lines: 1, hunk: "@@ -0,0 +1 @@\n+one\n"},
		{name: "deletion", limit: 1, lines: 1, hunk: "@@ -1 +0,0 @@\n-one\n"},
		{name: "marker has no cost", limit: 1, lines: 1, hunk: "@@ -1 +1 @@\n line\n\\ No newline at end of file\n"},
		{name: "hunk at capacity", limit: 1, lines: 1, hunk: "@@ -1 +1 @@\n one\n@@ -3 +3 @@\n three\n", incomplete: true},
		{name: "long line after capacity", limit: 1, lines: 1, hunk: "@@ -0,0 +1,3 @@\n+one\n+" + strings.Repeat("x", 10000) + "\n+three\n", incomplete: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			patch := "diff --git a/file b/file\n--- a/file\n+++ b/file\n" + test.hunk
			diff, err := ParsePatch(t.Context(), test.limit, 5000, 10, strings.NewReader(patch))
			require.NoError(t, err)
			require.Len(t, diff.Files, 1)
			diffFile := diff.Files[0]
			assert.Equal(t, test.incomplete, diffFile.IsIncomplete)
			if test.limit == 0 {
				require.Len(t, diffFile.Sections, 0)
			} else {
				require.Len(t, diffFile.Sections, 1)
				assert.Len(t, diffFile.Sections[0].Lines, test.lines+1) // lines with a section
			}
		})
	}
}

func TestGetDiffRangeWithWhitespaceBehavior(t *testing.T) {
	gitRepo, err := git.OpenRepositoryLocal(t.Context(), "../../modules/git/tests/repos/repo5_pulls")
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
		ParsePatch(t.Context(), setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(testcase.gitdiff))
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

func diffFileNames(diff *Diff) (names []string) {
	for _, file := range diff.Files {
		names = append(names, file.GetDiffFileName())
	}
	return names
}

func TestSyncUserSpecificDiff_UpdatedFiles(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 7})
	assert.NoError(t, pull.LoadBaseRepo(t.Context()))

	stdin := `blob
mark :1
data 7
change

commit refs/heads/branch1
mark :2
committer test <test@example.com> 1772749114 +0000
data 7
change
from 1978192d98bb1b65e11c2cf37da854fbf94bffd6
M 100644 :1 test2.txt
M 100644 :1 test3.txt

commit refs/heads/branch1
committer test <test@example.com> 1772749114 +0000
data 7
revert
from :2
D test2.txt
D test10.txt`
	require.NoError(t, gitcmd.NewCommand("fast-import").WithRepo(pull.BaseRepo).WithStdinBytes([]byte(stdin)).Run(t.Context()))

	gitRepo, err := git.OpenRepository(t.Context(), pull.BaseRepo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	firstReviewCommit := "1978192d98bb1b65e11c2cf37da854fbf94bffd6"
	firstReviewUpdatedFiles := map[string]pull_model.ViewedState{
		"test1.txt":  pull_model.Viewed,
		"test2.txt":  pull_model.Viewed,
		"test10.txt": pull_model.Viewed,
	}
	_, err = pull_model.UpdateReviewState(t.Context(), user.ID, pull.ID, firstReviewCommit, firstReviewUpdatedFiles)
	assert.NoError(t, err)
	firstReview, err := pull_model.GetNewestReviewState(t.Context(), user.ID, pull.ID)
	assert.NoError(t, err)
	assert.NotNil(t, firstReview)
	assert.Equal(t, firstReviewUpdatedFiles, firstReview.UpdatedFiles)
	assert.Equal(t, 3, firstReview.GetViewedFileCount())

	secondReviewCommit := "f80737c7dc9de0a9c1e051e83cb6897f950c6bb8"
	secondReviewUpdatedFiles := map[string]pull_model.ViewedState{
		"test1.txt":  pull_model.Viewed,
		"test2.txt":  pull_model.HasChanged,
		"test3.txt":  pull_model.HasChanged,
		"test10.txt": pull_model.Viewed,
	}
	secondReviewDiffOpts := &DiffOptions{
		AfterCommitID:     secondReviewCommit,
		BeforeCommitID:    pull.MergeBase,
		MaxLines:          setting.Git.MaxGitDiffLines,
		MaxLineCharacters: setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:          setting.Git.MaxGitDiffFiles,
	}
	secondReviewDiff, err := GetDiffForAPI(t.Context(), gitRepo, secondReviewDiffOpts)
	assert.NoError(t, err)
	secondReview, err := SyncUserSpecificDiff(t.Context(), user.ID, pull, gitRepo, diffFileNames(secondReviewDiff), secondReviewCommit)
	assert.NoError(t, err)
	assert.NotNil(t, secondReview)
	assert.Equal(t, secondReviewUpdatedFiles, secondReview.UpdatedFiles)
	assert.Equal(t, 2, secondReview.GetViewedFileCount())

	thirdReviewCommit := "73424f3a99e140f6399c73a1712654e122d2a74b"
	thirdReviewUpdatedFiles := map[string]pull_model.ViewedState{
		"test1.txt":  pull_model.Viewed,
		"test2.txt":  pull_model.Unviewed,
		"test3.txt":  pull_model.HasChanged,
		"test10.txt": pull_model.Unviewed,
	}
	thirdReviewDiffOpts := &DiffOptions{
		AfterCommitID:     thirdReviewCommit,
		BeforeCommitID:    pull.MergeBase,
		MaxLines:          setting.Git.MaxGitDiffLines,
		MaxLineCharacters: setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:          setting.Git.MaxGitDiffFiles,
	}
	thirdReviewDiff, err := GetDiffForAPI(t.Context(), gitRepo, thirdReviewDiffOpts)
	assert.NoError(t, err)
	thirdReview, err := SyncUserSpecificDiff(t.Context(), user.ID, pull, gitRepo, diffFileNames(thirdReviewDiff), thirdReviewCommit)
	assert.NoError(t, err)
	assert.NotNil(t, thirdReview)
	assert.Equal(t, thirdReviewUpdatedFiles, thirdReview.UpdatedFiles)
	assert.Equal(t, 1, thirdReview.GetViewedFileCount())
}

func TestGetDiffShortStatWithOptions(t *testing.T) {
	repo, err := git.ForceFastImportWithInit(t.Context(), t.TempDir(), []git.FastImportCommit{
		{Ref: "refs/heads/base", Files: []git.FastImportFile{
			{Path: "real.txt", Content: "a1\na2\n"},
			{Path: "whitespace.txt", Content: "b\n"},
		}},
		{Ref: "refs/heads/head", Files: []git.FastImportFile{
			{Path: "real.txt", Content: "A1\nA2\nA3\n"},
			{Path: "whitespace.txt", Content: "b   \n"},
		}},
	})
	require.NoError(t, err)
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	t.Run("NoParent", func(t *testing.T) {
		stat, err := GetDiffShortStat(t.Context(), gitRepo, &DiffCommonOptions{AfterCommitID: "refs/heads/head"})
		require.NoError(t, err)
		assert.Equal(t, &DiffShortStat{NumFiles: 2, TotalAddition: 4, TotalDeletion: 0}, stat)
	})
	diffOptions := DiffCommonOptions{BeforeCommitID: "refs/heads/base", AfterCommitID: "refs/heads/head"}
	t.Run("NormalDiff", func(t *testing.T) {
		stat, err := GetDiffShortStat(t.Context(), gitRepo, &diffOptions)
		require.NoError(t, err)
		assert.Equal(t, &DiffShortStat{NumFiles: 2, TotalAddition: 4, TotalDeletion: 3}, stat)
	})
	t.Run("IgnoreSpace", func(t *testing.T) {
		diffOptions.WhitespaceBehavior = GetWhitespaceFlag("ignore-all")
		stat, err := GetDiffShortStat(t.Context(), gitRepo, &diffOptions)
		require.NoError(t, err)
		assert.Equal(t, &DiffShortStat{NumFiles: 1, TotalAddition: 3, TotalDeletion: 2}, stat)
	})
}
