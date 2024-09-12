// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"encoding/csv"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	csv_module "code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestCSVDiff(t *testing.T) {
	cases := []struct {
		diff  string
		base  string
		head  string
		cells [][]TableDiffCellType
	}{
		// case 0 - initial commit of a csv
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -0,0 +1,2 @@
+col1,col2
+a,a`,
			base: "",
			head: `col1,col2
a,a`,
			cells: [][]TableDiffCellType{
				{TableDiffCellAdd, TableDiffCellAdd},
				{TableDiffCellAdd, TableDiffCellAdd},
			},
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
			base: `col1,col2
a,a`,
			head: `col1,col2
a,a
b,b`,
			cells: [][]TableDiffCellType{
				{TableDiffCellUnchanged, TableDiffCellUnchanged},
				{TableDiffCellUnchanged, TableDiffCellUnchanged},
				{TableDiffCellAdd, TableDiffCellAdd},
			},
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
			base: `col1,col2
a,a
b,b`,
			head: `col1,col2
b,b`,
			cells: [][]TableDiffCellType{
				{TableDiffCellUnchanged, TableDiffCellUnchanged},
				{TableDiffCellDel, TableDiffCellDel},
				{TableDiffCellUnchanged, TableDiffCellUnchanged},
			},
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
			base: `col1,col2
b,b`,
			head: `col1,col2
b,c`,
			cells: [][]TableDiffCellType{
				{TableDiffCellUnchanged, TableDiffCellUnchanged},
				{TableDiffCellUnchanged, TableDiffCellChanged},
			},
		},
		// case 4 - all deleted
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +0,0 @@
-col1,col2
-b,c`,
			base: `col1,col2
b,c`,
			head: "",
			cells: [][]TableDiffCellType{
				{TableDiffCellDel, TableDiffCellDel},
				{TableDiffCellDel, TableDiffCellDel},
			},
		},
		// case 5 - renames first column
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,3 +1,3 @@
-col1,col2,col3
+cola,col2,col3
 a,b,c`,
			base: `col1,col2,col3
a,b,c`,
			head: `cola,col2,col3
a,b,c`,
			cells: [][]TableDiffCellType{
				{TableDiffCellDel, TableDiffCellAdd, TableDiffCellUnchanged, TableDiffCellUnchanged},
				{TableDiffCellDel, TableDiffCellAdd, TableDiffCellUnchanged, TableDiffCellUnchanged},
			},
		},
		// case 6 - inserts a column after first, deletes last column
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +1,2 @@
-col1,col2,col3
-a,b,c
+col1,col1a,col2
+a,d,b`,
			base: `col1,col2,col3
a,b,c`,
			head: `col1,col1a,col2
a,d,b`,
			cells: [][]TableDiffCellType{
				{TableDiffCellUnchanged, TableDiffCellAdd, TableDiffCellDel, TableDiffCellMovedUnchanged},
				{TableDiffCellUnchanged, TableDiffCellAdd, TableDiffCellDel, TableDiffCellMovedUnchanged},
			},
		},
		// case 7 - deletes first column, inserts column after last
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +1,2 @@
-col1,col2,col3
-a,b,c
+col2,col3,col4
+b,c,d`,
			base: `col1,col2,col3
a,b,c`,
			head: `col2,col3,col4
b,c,d`,
			cells: [][]TableDiffCellType{
				{TableDiffCellDel, TableDiffCellUnchanged, TableDiffCellUnchanged, TableDiffCellAdd},
				{TableDiffCellDel, TableDiffCellUnchanged, TableDiffCellUnchanged, TableDiffCellAdd},
			},
		},
		// case 8 - two columns deleted, 2 added
		{
			diff: `diff --git a/unittest.csv b/unittest.csv
--- a/unittest.csv
+++ b/unittest.csv
@@ -1,2 +1,2 @@
-col1,col2,col
-a,b,c
+col3,col4,col5
+c,d,e`,
			base: `col1,col2,col3
a,b,c`,
			head: `col3,col4,col5
c,d,e`,
			cells: [][]TableDiffCellType{
				{TableDiffCellDel, TableDiffCellMovedUnchanged, TableDiffCellDel, TableDiffCellAdd, TableDiffCellAdd},
				{TableDiffCellDel, TableDiffCellMovedUnchanged, TableDiffCellDel, TableDiffCellAdd, TableDiffCellAdd},
			},
		},
	}

	for n, c := range cases {
		diff, err := ParsePatch(db.DefaultContext, setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.diff), "")
		if err != nil {
			t.Errorf("ParsePatch failed: %s", err)
		}

		var baseReader *csv.Reader
		if len(c.base) > 0 {
			baseReader, err = csv_module.CreateReaderAndDetermineDelimiter(nil, strings.NewReader(c.base))
			if err != nil {
				t.Errorf("CreateReaderAndDetermineDelimiter failed: %s", err)
			}
		}
		var headReader *csv.Reader
		if len(c.head) > 0 {
			headReader, err = csv_module.CreateReaderAndDetermineDelimiter(nil, strings.NewReader(c.head))
			if err != nil {
				t.Errorf("CreateReaderAndDetermineDelimiter failed: %s", err)
			}
		}

		result, err := CreateCsvDiff(diff.Files[0], baseReader, headReader)
		assert.NoError(t, err)
		assert.Len(t, result, 1, "case %d: should be one section", n)

		section := result[0]
		assert.Len(t, section.Rows, len(c.cells), "case %d: should be %d rows", n, len(c.cells))

		for i, row := range section.Rows {
			assert.Len(t, row.Cells, len(c.cells[i]), "case %d: row %d should have %d cells", n, i, len(c.cells[i]))
			for j, cell := range row.Cells {
				assert.Equal(t, c.cells[i][j], cell.Type, "case %d: row %d cell %d should be equal", n, i, j)
			}
		}
	}
}
