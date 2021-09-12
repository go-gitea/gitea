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
		// case 0 - initial commit of a csv
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
		// case 1 - adding 1 row at end
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
			cells: [][2]TableDiffCellType{{TableDiffCellEqual, TableDiffCellEqual}, {TableDiffCellEqual, TableDiffCellEqual}, {TableDiffCellChanged, TableDiffCellChanged}},
		},
		// case 2 - row deleted
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
		// case 3 - row changed
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
		// case 4 - all deleted
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
			baseReader, err = csv_module.CreateReaderAndGuessDelimiter(strings.NewReader(c.base))
			if err != nil {
				t.Errorf("CreateReaderAndGuessDelimiter failed: %s", err)
			}
		}
		var headReader *csv.Reader
		if len(c.head) > 0 {
			headReader, err = csv_module.CreateReaderAndGuessDelimiter(strings.NewReader(c.head))
			if err != nil {
				t.Errorf("CreateReaderAndGuessDelimiter failed: %s", err)
			}
		}

		result, err := CreateCsvDiff(diff.Files[0], baseReader, headReader)
		assert.NoError(t, err)
		assert.Len(t, result, 1, "case %d: should be one section", n)

		section := result[0]
		assert.Len(t, section.Rows, len(c.cells), "case %d: should be %d rows", n, len(c.cells))

		for i, row := range section.Rows {
			assert.Len(t, row.Cells, 2, "case %d: row %d should have two cells", n, i)
			for j, cell := range row.Cells {
				assert.Equal(t, c.cells[i][j], cell.Type, "case %d: row %d cell %d should be equal", n, i, j)
			}
		}
	}
}

func TestCSVDiffHeadingChange(t *testing.T) {
	var cases = []struct {
		diff  string
		base  string
		head  string
		cells [][4]TableDiffCellType
	}{
		// case 0 - renames first column
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,3 +1,3 @@
-col1,col2,col3
+cola,col2,col3
 a,b,c`,
			base:  "col1,col2,col3\na,b,c",
			head:  "cola,col2,col3\na,b,c",
			cells: [][4]TableDiffCellType{{TableDiffCellAdd, TableDiffCellDel, TableDiffCellEqual, TableDiffCellEqual}, {TableDiffCellAdd, TableDiffCellDel, TableDiffCellEqual, TableDiffCellEqual}},
		},
		// case 1 - inserts a column after first, deletes last column
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +1,2 @@
-col1,col2,col3
-a,b,c
+col1,col1a,col2
+a,d,b`,
			base:  "col1,col2,col3\na,b,c",
			head:  "col1,col1a,col2\na,d,b",
			cells: [][4]TableDiffCellType{{TableDiffCellEqual, TableDiffCellAdd, TableDiffCellEqual, TableDiffCellDel}, {TableDiffCellEqual, TableDiffCellAdd, TableDiffCellEqual, TableDiffCellDel}},
		},
		// case 2 - deletes first column, inserts column after last
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +1,2 @@
-col1,col2,col3
-a,b,c
+col2,col3,col4
+b,c,d`,
			base:  "col1,col2,col3\na,b,c",
			head:  "col2,col3,col4\nb,c,d",
			cells: [][4]TableDiffCellType{{TableDiffCellDel, TableDiffCellEqual, TableDiffCellEqual, TableDiffCellAdd}, {TableDiffCellDel, TableDiffCellEqual, TableDiffCellEqual, TableDiffCellAdd}},
		},
	}

	for n, c := range cases {
		diff, err := ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.diff))
		if err != nil {
			t.Errorf("ParsePatch failed: %s", err)
		}

		var baseReader *csv.Reader
		if len(c.base) > 0 {
			baseReader, err = csv_module.CreateReaderAndGuessDelimiter(strings.NewReader(c.base))
			if err != nil {
				t.Errorf("CreateReaderAndGuessDelimiter failed: %s", err)
			}
		}
		var headReader *csv.Reader
		if len(c.head) > 0 {
			headReader, err = csv_module.CreateReaderAndGuessDelimiter(strings.NewReader(c.head))
			if err != nil {
				t.Errorf("CreateReaderAndGuessDelimiter failed: %s", err)
			}
		}

		result, err := CreateCsvDiff(diff.Files[0], baseReader, headReader)
		assert.NoError(t, err)
		assert.Len(t, result, 1, "case %d: should be one section", n)

		section := result[0]
		assert.Len(t, section.Rows, len(c.cells), "case %d: should be %d rows", n, len(c.cells))

		for i, row := range section.Rows {
			assert.Len(t, row.Cells, 4, "case %d: row %d should have four cells", n, i)
			for j, cell := range row.Cells {
				assert.Equal(t, c.cells[i][j], cell.Type, "case %d: row %d cell %d should be equal", n, i, j)
			}
		}
	}
}

func TestCSVDiffMultipleHeadingChanges(t *testing.T) {
	var cases = []struct {
		diff  string
		base  string
		head  string
		cells [][5]TableDiffCellType
	}{
		// case 0 - two columns deleted, 2 added
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +1,2 @@
-col1,col2,col
-a,b,c
+col3,col4,col5
+c,d,e`,
			base:  "col1,col2,col3\na,b,c",
			head:  "col3,col4,col5\nc,d,e",
			cells: [][5]TableDiffCellType{{TableDiffCellDel, TableDiffCellDel, TableDiffCellEqual, TableDiffCellAdd, TableDiffCellAdd}, {TableDiffCellDel, TableDiffCellDel, TableDiffCellEqual, TableDiffCellAdd, TableDiffCellAdd}},
		},
	}

	for n, c := range cases {
		diff, err := ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.diff))
		if err != nil {
			t.Errorf("ParsePatch failed: %s", err)
		}

		var baseReader *csv.Reader
		if len(c.base) > 0 {
			baseReader, err = csv_module.CreateReaderAndGuessDelimiter(strings.NewReader(c.base))
			if err != nil {
				t.Errorf("CreateReaderAndGuessDelimiter failed: %s", err)
			}
		}
		var headReader *csv.Reader
		if len(c.head) > 0 {
			headReader, err = csv_module.CreateReaderAndGuessDelimiter(strings.NewReader(c.head))
			if err != nil {
				t.Errorf("CreateReaderAndGuessDelimiter failed: %s", err)
			}
		}

		result, err := CreateCsvDiff(diff.Files[0], baseReader, headReader)
		assert.NoError(t, err)
		assert.Len(t, result, 1, "case %d: should be one section", n)

		section := result[0]
		assert.Len(t, section.Rows, len(c.cells), "case %d: should be %d rows", n, len(c.cells))

		for i, row := range section.Rows {
			assert.Len(t, row.Cells, 5, "case %d: row %d should have five cells", n, i)
			for j, cell := range row.Cells {
				assert.Equal(t, c.cells[i][j], cell.Type, "case %d: row %d cell %d should be equal", n, i, j)
			}
		}
	}
}
