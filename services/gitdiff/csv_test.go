// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdiff

import (
	"encoding/csv"
	"strings"
	"testing"

	csv_module "code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
)

func TestCSVDiff(t *testing.T) {
	var cases = []struct {
		diff  string
		base  string
		head  string
		cells [][2]TableDiffCellType
	}{
		// case 0
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -0,0 +1,2 @@
+col1,col2
+a,a`,
			base:  "",
			head:  "col1,col2\na,a",
			cells: [][2]TableDiffCellType{{TableDiffCellAdd, TableDiffCellAdd}, {TableDiffCellAdd, TableDiffCellAdd}},
		},
		// case 1
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +1,3 @@
 col1,col2
-a,a
+a,a
+b,b`,
			base:  "col1,col2\na,a",
			head:  "col1,col2\na,a\nb,b",
			cells: [][2]TableDiffCellType{{TableDiffCellEqual, TableDiffCellEqual}, {TableDiffCellEqual, TableDiffCellEqual}, {TableDiffCellAdd, TableDiffCellAdd}},
		},
		// case 2
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,3 +1,2 @@
 col1,col2
-a,a
 b,b`,
			base:  "col1,col2\na,a\nb,b",
			head:  "col1,col2\nb,b",
			cells: [][2]TableDiffCellType{{TableDiffCellEqual, TableDiffCellEqual}, {TableDiffCellDel, TableDiffCellDel}, {TableDiffCellEqual, TableDiffCellEqual}},
		},
		// case 3
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +1,2 @@
 col1,col2
-b,b
+b,c`,
			base:  "col1,col2\nb,b",
			head:  "col1,col2\nb,c",
			cells: [][2]TableDiffCellType{{TableDiffCellEqual, TableDiffCellEqual}, {TableDiffCellEqual, TableDiffCellChanged}},
		},
		// case 4
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +0,0 @@
-col1,col2
-b,c`,
			base:  "col1,col2\nb,c",
			head:  "",
			cells: [][2]TableDiffCellType{{TableDiffCellDel, TableDiffCellDel}, {TableDiffCellDel, TableDiffCellDel}},
		},
	}

	for n, c := range cases {
		diff, err := ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.diff))
		if err != nil {
			t.Errorf("ParsePatch failed: %s", err)
		}

		var baseReader *csv.Reader
		if len(c.base) > 0 {
			baseReader = csv_module.CreateReaderAndGuessDelimiter([]byte(c.base))
		}
		var headReader *csv.Reader
		if len(c.head) > 0 {
			headReader = csv_module.CreateReaderAndGuessDelimiter([]byte(c.head))
		}

		result, err := CreateCsvDiff(diff.Files[0], baseReader, headReader)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(result), "case %d: should be one section", n)

		section := result[0]
		assert.Equal(t, len(c.cells), len(section.Rows), "case %d: should be %d rows", n, len(c.cells))

		for i, row := range section.Rows {
			assert.Equal(t, 2, len(row.Cells), "case %d: row %d should have two cells", n, i)
			for j, cell := range row.Cells {
				assert.Equal(t, c.cells[i][j], cell.Type, "case %d: row %d cell %d should be equal", n, i, j)
			}
		}
	}
}
