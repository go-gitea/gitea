// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const exampleDiff = `diff --git a/README.md b/README.md
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

const breakingDiff = `diff --git a/aaa.sql b/aaa.sql
index d8e4c92..19dc8ad 100644
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some coment 2
+-- some comment 3
 create or replace procedure test(p1 varchar2)
 is
 begin
---new comment
 dbms_output.put_line(p1);
+--some other comment
 end;
 /
`

func TestCutDiffAroundLine(t *testing.T) {
	result, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 4, false, 3)
	assert.NoError(t, err)
	resultByLine := strings.Split(result, "\n")
	assert.Len(t, resultByLine, 7)
	// Check if headers got transferred
	assert.Equal(t, "diff --git a/README.md b/README.md", resultByLine[0])
	assert.Equal(t, "--- a/README.md", resultByLine[1])
	assert.Equal(t, "+++ b/README.md", resultByLine[2])
	// Check if hunk header is calculated correctly
	assert.Equal(t, "@@ -2,2 +3,2 @@", resultByLine[3])
	// Check if line got transferred
	assert.Equal(t, "+ Build Status", resultByLine[4])

	// Must be same result as before since old line 3 == new line 5
	newResult, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 3, true, 3)
	assert.NoError(t, err)
	assert.Equal(t, result, newResult, "Must be same result as before since old line 3 == new line 5")

	newResult, err = CutDiffAroundLine(strings.NewReader(exampleDiff), 6, false, 300)
	assert.NoError(t, err)
	assert.Equal(t, exampleDiff, newResult)

	emptyResult, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 6, false, 0)
	assert.NoError(t, err)
	assert.Empty(t, emptyResult)

	// Line is out of scope
	emptyResult, err = CutDiffAroundLine(strings.NewReader(exampleDiff), 434, false, 0)
	assert.NoError(t, err)
	assert.Empty(t, emptyResult)

	// Handle minus diffs properly
	minusDiff, err := CutDiffAroundLine(strings.NewReader(breakingDiff), 2, false, 4)
	assert.NoError(t, err)

	expected := `diff --git a/aaa.sql b/aaa.sql
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some coment 2`
	assert.Equal(t, expected, minusDiff)

	// Handle minus diffs properly
	minusDiff, err = CutDiffAroundLine(strings.NewReader(breakingDiff), 3, false, 4)
	assert.NoError(t, err)

	expected = `diff --git a/aaa.sql b/aaa.sql
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some coment 2
+-- some comment 3`

	assert.Equal(t, expected, minusDiff)
}

func BenchmarkCutDiffAroundLine(b *testing.B) {
	for n := 0; n < b.N; n++ {
		CutDiffAroundLine(strings.NewReader(exampleDiff), 3, true, 3)
	}
}

func ExampleCutDiffAroundLine() {
	const diff = `diff --git a/README.md b/README.md
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
	result, _ := CutDiffAroundLine(strings.NewReader(diff), 4, false, 3)
	println(result)
}

func TestParseDiffHunkString(t *testing.T) {
	leftLine, leftHunk, rightLine, rightHunk := ParseDiffHunkString("@@ -19,3 +19,5 @@ AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER")
	assert.EqualValues(t, 19, leftLine)
	assert.EqualValues(t, 3, leftHunk)
	assert.EqualValues(t, 19, rightLine)
	assert.EqualValues(t, 5, rightHunk)
}
