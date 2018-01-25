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
+       "fmt"
		"net/http"
		"testing"

		"code.gitea.io/gitea/models"
		api "code.gitea.io/sdk/gitea"

-       "fmt"
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
+       "fmt"
        "net/http"
        "testing"

        "code.gitea.io/gitea/models"
        api "code.gitea.io/sdk/gitea"

-       "fmt"
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
