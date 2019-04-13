// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.


package models

import (
	"html/template"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/log"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
)

func assertEqual(t *testing.T, s1 string, s2 template.HTML) {
	if s1 != string(s2) {
		t.Errorf("%s should be equal %s", s2, s1)
	}
}

func assertLineEqual(t *testing.T, d1 *DiffLine, d2 *DiffLine) {
	if d1 != d2 {
		t.Errorf("%v should be equal %v", d1, d2)
	}
}

func TestDiffToHTML(t *testing.T) {
	assertEqual(t, "+foo <span class=\"added-code\">bar</span> biz", diffToHTML([]dmp.Diff{
		{Type: dmp.DiffEqual, Text: "foo "},
		{Type: dmp.DiffInsert, Text: "bar"},
		{Type: dmp.DiffDelete, Text: " baz"},
		{Type: dmp.DiffEqual, Text: " biz"},
	}, DiffLineAdd))

	assertEqual(t, "-foo <span class=\"removed-code\">bar</span> biz", diffToHTML([]dmp.Diff{
		{Type: dmp.DiffEqual, Text: "foo "},
		{Type: dmp.DiffDelete, Text: "bar"},
		{Type: dmp.DiffInsert, Text: " baz"},
		{Type: dmp.DiffEqual, Text: " biz"},
	}, DiffLineDel))
}

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

func TestCutDiffAroundLine(t *testing.T) {
	result := CutDiffAroundLine(strings.NewReader(exampleDiff), 4, false, 3)
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
	newResult := CutDiffAroundLine(strings.NewReader(exampleDiff), 3, true, 3)
	assert.Equal(t, result, newResult, "Must be same result as before since old line 3 == new line 5")

	newResult = CutDiffAroundLine(strings.NewReader(exampleDiff), 6, false, 300)
	assert.Equal(t, exampleDiff, newResult)

	emptyResult := CutDiffAroundLine(strings.NewReader(exampleDiff), 6, false, 0)
	assert.Empty(t, emptyResult)

	// Line is out of scope
	emptyResult = CutDiffAroundLine(strings.NewReader(exampleDiff), 434, false, 0)
	assert.Empty(t, emptyResult)
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
	result := CutDiffAroundLine(strings.NewReader(diff), 4, false, 3)
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
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 2}).(*Issue)
	user := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	diff := setupDefaultDiff()
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, diff.LoadComments(issue, user))
	assert.Len(t, diff.Files[0].Sections[0].Lines[0].Comments, 2)
}

func TestDiffLine_CanComment(t *testing.T) {
	assert.False(t, (&DiffLine{Type: DiffLineSection}).CanComment())
	assert.False(t, (&DiffLine{Type: DiffLineAdd, Comments: []*Comment{{Content: "bla"}}}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLineAdd}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLineDel}).CanComment())
	assert.True(t, (&DiffLine{Type: DiffLinePlain}).CanComment())
}

func TestDiffLine_GetCommentSide(t *testing.T) {
	assert.Equal(t, "previous", (&DiffLine{Comments: []*Comment{{Line: -3}}}).GetCommentSide())
	assert.Equal(t, "proposed", (&DiffLine{Comments: []*Comment{{Line: 3}}}).GetCommentSide())
}

func benchParsePatch(b *testing.B, diffStr string) {
	log.DelLogger("console")
	log.DelLogger("file")
	b.ResetTimer() //Disable logger for becnh
	for i := 0; i < b.N; i++ {
		ParsePatch(1000, 5000, 100, strings.NewReader(diffStr))
	}
}

func BenchmarkParsePatchSimple(b *testing.B) {
	benchParsePatch(b, `diff --git a/integrations/api_issue_test.go b/integrations/api_issue_test.go
index 74436ffe9..ff316cec3 100644
--- a/integrations/api_issue_test.go
+++ b/integrations/api_issue_test.go
@@ -5,13 +5,13 @@
package integrations
import (
-       "fmt"
		"net/http"
		"testing"
		"code.gitea.io/gitea/models"
		api "code.gitea.io/sdk/gitea"
+       "fmt"
		"github.com/stretchr/testify/assert"
)
`)
}

func TestParsePatch(t *testing.T) {
	testCases := []struct {
		result   error
		files    int
		addition int
		deletion int
		diff     string
	}{
		{nil, 1, 1, 1,
			`diff --git a/integrations/api_issue_test.go b/integrations/api_issue_test.go
index 74436ffe9..ff316cec3 100644
--- a/integrations/api_issue_test.go
+++ b/integrations/api_issue_test.go
@@ -5,13 +5,13 @@
 package integrations
 import (
-       "fmt"
        "net/http"
        "testing"
        "code.gitea.io/gitea/models"
        api "code.gitea.io/sdk/gitea"
+       "fmt"
        "github.com/stretchr/testify/assert"
 )
`},
	}
	for _, tc := range testCases {
		diff, err := ParsePatch(1000, 5000, 100, strings.NewReader(tc.diff))
		assert.Equal(t, tc.result, err)
		assert.Equal(t, tc.files, diff.NumFiles())
		assert.Equal(t, tc.addition, diff.TotalAddition)
		assert.Equal(t, tc.deletion, diff.TotalDeletion)
	}
}
