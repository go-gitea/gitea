// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdiff

import (
	"encoding/csv"
	"errors"

	"code.gitea.io/gitea/modules/util"
)

const unmappedColumn = -1

// TableDiffCellType represents the type of a TableDiffCell.
type TableDiffCellType uint8

// TableDiffCellType possible values.
const (
	TableDiffCellEqual TableDiffCellType = iota + 1
	TableDiffCellChanged
	TableDiffCellAdd
	TableDiffCellDel
)

// TableDiffCell represents a cell of a TableDiffRow
type TableDiffCell struct {
	LeftCell  string
	RightCell string
	Type      TableDiffCellType
}

// TableDiffRow represents a row of a TableDiffSection.
type TableDiffRow struct {
	RowIdx int
	Cells  []*TableDiffCell
}

// TableDiffSection represents a section of a DiffFile.
type TableDiffSection struct {
	Rows []*TableDiffRow
}

// CreateCsvDiff creates a tabular diff based on two CSV readers.
func CreateCsvDiff(diffFile *DiffFile, baseReader *csv.Reader, headReader *csv.Reader) ([]*TableDiffSection, error) {
	if baseReader != nil && headReader != nil {
		return createCsvDiff(diffFile, baseReader, headReader)
	}

	if baseReader != nil {
		return createCsvDiffSingle(baseReader, TableDiffCellDel)
	}
	return createCsvDiffSingle(headReader, TableDiffCellAdd)
}

// createCsvDiffSingle creates a tabular diff based on a single CSV reader. All cells are added or deleted.
func createCsvDiffSingle(reader *csv.Reader, celltype TableDiffCellType) ([]*TableDiffSection, error) {
	a, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	rows := make([]*TableDiffRow, len(a))
	for i, row := range a {
		cells := make([]*TableDiffCell, len(row))
		for j := 0; j < len(row); j++ {
			cells[j] = &TableDiffCell{ LeftCell: row[j], Type: celltype }
		}
		rows[i] = &TableDiffRow{ RowIdx: i + 1, Cells: cells }
	}

	return []*TableDiffSection{&TableDiffSection{ Rows: rows}}, nil
}

func createCsvDiff(diffFile *DiffFile, baseReader *csv.Reader, headReader *csv.Reader) ([]*TableDiffSection, error) {
	arows, err := baseReader.ReadAll()
	if err != nil {
		return nil, err
	}
	a := arows[:]

	brows, err := headReader.ReadAll()
	if err != nil {
		return nil, err
	}
	b := brows[:]

	a2b, b2a := getColumnMapping(a, b)

	columns := len(a2b) + countUnmappedColumns(b2a)
	if len(a2b) < len(b2a) {
		columns = len(b2a) + countUnmappedColumns(a2b)
	}

	createDiffRow := func(aline int, bline int) *TableDiffRow {
		cells := make([]*TableDiffCell, columns)
		
		if aline == 0 || bline == 0 {
			var (
				row []string
				celltype TableDiffCellType
			)
			if bline == 0 {
				row = getRow(a, aline - 1)
				celltype = TableDiffCellDel
			} else {
				row = getRow(b, bline - 1)
				celltype = TableDiffCellAdd
			}
			if row == nil {
				return nil
			}
			for i := 0; i < len(row); i++ {
				cells[i] = &TableDiffCell{ LeftCell: row[i], Type: celltype }
			}
			return &TableDiffRow{ RowIdx: bline, Cells: cells }
		}

		arow := getRow(a, aline - 1)
		brow := getRow(b, bline - 1)
		if len(arow) == 0 && len(brow) == 0 {
			return nil
		}

		for i := 0; i < len(a2b); i++ {
			acell, _ := getCell(arow, i)
			if a2b[i] == unmappedColumn {
				cells[i] = &TableDiffCell{ LeftCell: acell, Type: TableDiffCellDel }
			} else {
				bcell, _ := getCell(brow, a2b[i])
				
				celltype := TableDiffCellChanged
				if acell == bcell {
					celltype = TableDiffCellEqual
				}

				cells[i] = &TableDiffCell{ LeftCell: acell, RightCell: bcell, Type: celltype }
			}
		}
		for i := 0; i < len(b2a); i++ {
			if b2a[i] == unmappedColumn {
				bcell, _ := getCell(brow, i)
				cells[i] = &TableDiffCell{ RightCell: bcell, Type: TableDiffCellAdd }
			}
		}
		
		return &TableDiffRow{ RowIdx: bline, Cells: cells }
	}

	var sections []*TableDiffSection

	for i, section := range diffFile.Sections {
		var rows []*TableDiffRow
		lines := tryMergeLines(section.Lines)
		for j, line := range lines {
			if i == 0 && j == 0 && (line[0] != 1 || line[1] != 1) {
				diffRow := createDiffRow(1, 1)
				if diffRow != nil {
					rows = append(rows, diffRow)
				}
			}
			diffRow := createDiffRow(line[0], line[1])
			if diffRow != nil {
				rows = append(rows, diffRow)
			}
		}

		if len(rows) > 0 {
			sections = append(sections, &TableDiffSection{ Rows: rows})
		}
	}

	return sections, nil
}

// getColumnMapping creates a mapping of columns between a and b
func getColumnMapping(a [][]string, b [][]string) ([]int, []int) {
	arow := getRow(a, 0)
	brow := getRow(b, 0)

	a2b := []int{}
	b2a := []int{}

	if arow != nil {
		a2b = make([]int, len(a[0]))
	}
	if brow != nil {
		b2a = make([]int, len(b[0]))
	}

	for i := 0; i < len(b2a); i++ {
		b2a[i] = unmappedColumn
	}

	bcol := 0
	for i := 0; i < len(a2b); i++ {
		a2b[i] = unmappedColumn

		acell, ea := getCell(arow, i)
		if ea == nil {
			for j := bcol; j < len(b2a); j++ {
				bcell, eb := getCell(brow, j)
				if eb == nil && acell == bcell {
					a2b[i] = j
					b2a[j] = i
					bcol = j + 1
					break
				}
			}
		}
	}

	tryMapColumnsByContent(a, a2b, b, b2a)
	tryMapColumnsByContent(b, b2a, a, a2b)

	return a2b, b2a
}

// tryMapColumnsByContent tries to map missing columns by the content of the first lines.
func tryMapColumnsByContent(a [][]string, a2b []int, b [][]string, b2a []int) {
	const MaxRows int = 10
	const MinRatio float32 = 0.8

	start := 0
	for i := 0; i < len(a2b); i++ {
		if a2b[i] == unmappedColumn {
			if b2a[start] == unmappedColumn {
				rows := util.Min(MaxRows, util.Max(0, util.Min(len(a), len(b)) - 1))
				same := 0
				for j := 1; j <= rows; j++ {
					acell, ea := getCell(getRow(a, j), i)
					bcell, eb := getCell(getRow(b, j), start + 1)
					if ea == nil && eb == nil && acell == bcell {
						same++
					}
				}
				if (float32(same) / float32(rows)) > MinRatio {
					a2b[i] = start + 1
					b2a[start + 1] = i
				}
			}
		}
		start = a2b[i]
	}
}

// getRow returns the specific row or nil if not present.
func getRow(records [][]string, row int) []string {
	if row < len(records) {
		return records[row]
	}
	return nil
}

// getCell returns the specific cell or nil if not present.
func getCell(row []string, column int) (string, error) {
	if column < len(row) {
		return row[column], nil
	}
	return "", errors.New("Undefined column")
}

// countUnmappedColumns returns the count of unmapped columns.
func countUnmappedColumns(mapping []int) int {
	count := 0
	for i := 0; i < len(mapping); i++ {
		if mapping[i] == unmappedColumn {
			count++
		}
	}
	return count
}

// tryMergeLines maps the seperated line numbers of a git diff.
func tryMergeLines(lines []*DiffLine) [][2]int {
	ids := make([][2]int, len(lines))

	i := 0
	for _, line := range lines {
		if line.Type != DiffLineSection {
			ids[i][0] = line.LeftIdx
			ids[i][1] = line.RightIdx
			i++
		}
	}

	ids = ids[:i]

	result := make([][2]int, len(ids))

	j := 0
	for i = 0; i < len(ids); i++ {
		if ids[i][0] == 0 {
			if j > 0 && result[j - 1][1] == 0 {
				temp := j
				for temp > 0 && result[temp - 1][1] == 0 {
					temp--
				}
				result[temp][1] = ids[i][1]
				continue
			}
		}
		result[j] = ids[i]
		j++
	}

	return result[:j]
}