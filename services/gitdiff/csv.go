// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdiff

import (
	"encoding/csv"
	"errors"
	"io"

	"code.gitea.io/gitea/modules/util"
)

const unmappedColumn = -1
const maxRowsToInspect int = 10
const minRatioToMatch float32 = 0.8

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

// csvReader wraps a csv.Reader which buffers the first rows.
type csvReader struct {
	reader *csv.Reader
	buffer [][]string
	line   int
	eof    bool
}

// ErrorUndefinedCell is for when a row, column coordinates do not exist in the CSV
var ErrorUndefinedCell = errors.New("undefined cell")

// createCsvReader creates a csvReader and fills the buffer
func createCsvReader(reader *csv.Reader, bufferRowCount int) (*csvReader, error) {
	csv := &csvReader{reader: reader}
	csv.buffer = make([][]string, bufferRowCount)
	for i := 0; i < bufferRowCount && !csv.eof; i++ {
		row, err := csv.readNextRow()
		if err != nil {
			return nil, err
		}
		csv.buffer[i] = row
	}
	csv.line = bufferRowCount
	return csv, nil
}

// GetRow gets a row from the buffer if present or advances the reader to the requested row. On the end of the file only nil gets returned.
func (csv *csvReader) GetRow(row int) ([]string, error) {
	if row < len(csv.buffer) && row >= 0 {
		return csv.buffer[row], nil
	}
	if csv.eof {
		return nil, nil
	}
	for {
		fields, err := csv.readNextRow()
		if err != nil {
			return nil, err
		}
		if csv.eof {
			return nil, nil
		}
		csv.line++
		if csv.line-1 == row {
			return fields, nil
		}
	}
}

func (csv *csvReader) readNextRow() ([]string, error) {
	if csv.eof {
		return nil, nil
	}
	row, err := csv.reader.Read()
	if err != nil {
		if err != io.EOF {
			return nil, err
		}
		csv.eof = true
	}
	return row, nil
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
	var rows []*TableDiffRow
	i := 1
	for {
		row, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		cells := make([]*TableDiffCell, len(row))
		for j := 0; j < len(row); j++ {
			if celltype == TableDiffCellDel {
				cells[j] = &TableDiffCell{LeftCell: row[j], Type: celltype}
			} else {
				cells[j] = &TableDiffCell{RightCell: row[j], Type: celltype}
			}
		}
		rows = append(rows, &TableDiffRow{RowIdx: i, Cells: cells})
		i++
	}

	return []*TableDiffSection{{Rows: rows}}, nil
}

func createCsvDiff(diffFile *DiffFile, baseReader *csv.Reader, headReader *csv.Reader) ([]*TableDiffSection, error) {
	a, err := createCsvReader(baseReader, maxRowsToInspect)
	if err != nil {
		return nil, err
	}

	b, err := createCsvReader(headReader, maxRowsToInspect)
	if err != nil {
		return nil, err
	}

	a2b, b2a := getColumnMapping(a, b)

	columns := len(a2b) + countUnmappedColumns(b2a)
	if len(a2b) < len(b2a) {
		columns = len(b2a) + countUnmappedColumns(a2b)
	}

	createDiffRow := func(aline int, bline int) (*TableDiffRow, error) {
		cells := make([]*TableDiffCell, columns)

		arow, err := a.GetRow(aline - 1)
		if err != nil {
			return nil, err
		}
		brow, err := b.GetRow(bline - 1)
		if err != nil {
			return nil, err
		}
		if len(arow) == 0 && len(brow) == 0 {
			return nil, nil
		}

		for i := 0; i < len(a2b); i++ {
			var aCell string
			aIsUndefined := false
			if aline > 0 {
				if cell, err := getCell(arow, i); err != nil {
					if err != ErrorUndefinedCell {
						return nil, err
					}
					aIsUndefined = true
				} else {
					aCell = cell
				}
			} else {
				aIsUndefined = true
			}

			if a2b[i] == unmappedColumn {
				cells[i] = &TableDiffCell{LeftCell: aCell, Type: TableDiffCellDel}
			} else {
				var bCell string
				bIsUndefined := false
				if bline > 0 {
					if cell, err := getCell(brow, a2b[i]); err != nil {
						if err != ErrorUndefinedCell {
							return nil, err
						}
						bIsUndefined = true
					} else {
						bCell = cell
					}
				} else {
					bIsUndefined = true
				}
				var cellType TableDiffCellType
				if aIsUndefined && !bIsUndefined {
					cellType = TableDiffCellAdd
				} else if bIsUndefined {
					cellType = TableDiffCellDel
				} else if aCell == bCell {
					cellType = TableDiffCellEqual
				} else {
					cellType = TableDiffCellChanged
				}
				cells[i] = &TableDiffCell{LeftCell: aCell, RightCell: bCell, Type: cellType}
			}
		}
		cellsIndex := 0
		for i := 0; i < len(b2a); i++ {
			if b2a[i] == unmappedColumn {
				var bCell string
				bIsUndefined := false
				if bline > 0 {
					if cell, err := getCell(brow, i); err != nil {
						if err != ErrorUndefinedCell {
							return nil, err
						}
						bIsUndefined = true
					} else {
						bCell = cell
					}
				} else {
					bIsUndefined = true
				}
				if cells[cellsIndex] != nil && len(cells) >= cellsIndex+1 {
					copy(cells[cellsIndex+1:], cells[cellsIndex:])
				}
				var cellType TableDiffCellType
				if bIsUndefined {
					cellType = TableDiffCellDel
				} else {
					cellType = TableDiffCellAdd
				}
				cells[cellsIndex] = &TableDiffCell{RightCell: bCell, Type: cellType}
			} else if cellsIndex < b2a[i] {
				cellsIndex = b2a[i]
			}
			cellsIndex++
		}

		return &TableDiffRow{RowIdx: bline, Cells: cells}, nil
	}

	var sections []*TableDiffSection

	for i, section := range diffFile.Sections {
		var rows []*TableDiffRow
		lines := tryMergeLines(section.Lines)
		for j, line := range lines {
			if i == 0 && j == 0 && (line[0] != 1 || line[1] != 1) {
				diffRow, err := createDiffRow(1, 1)
				if err != nil {
					return nil, err
				}
				if diffRow != nil {
					rows = append(rows, diffRow)
				}
			}
			diffRow, err := createDiffRow(line[0], line[1])
			if err != nil {
				return nil, err
			}
			if diffRow != nil {
				rows = append(rows, diffRow)
			}
		}

		if len(rows) > 0 {
			sections = append(sections, &TableDiffSection{Rows: rows})
		}
	}

	return sections, nil
}

// getColumnMapping creates a mapping of columns between a and b
func getColumnMapping(a *csvReader, b *csvReader) ([]int, []int) {
	arow, _ := a.GetRow(0)
	brow, _ := b.GetRow(0)

	a2b := []int{}
	b2a := []int{}

	if arow != nil {
		a2b = make([]int, len(arow))
	}
	if brow != nil {
		b2a = make([]int, len(brow))
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
func tryMapColumnsByContent(a *csvReader, a2b []int, b *csvReader, b2a []int) {
	for i := 0; i < len(a2b); i++ {
		bStart := 0
		for a2b[i] == unmappedColumn && bStart < len(b2a) {
			if b2a[bStart] == unmappedColumn {
				rows := util.Min(maxRowsToInspect, util.Max(0, util.Min(len(a.buffer), len(b.buffer))-1))
				same := 0
				for j := 1; j <= rows; j++ {
					aCell, aErr := getCell(a.buffer[j], i)
					bCell, bErr := getCell(b.buffer[j], bStart)
					if aErr == nil && bErr == nil && aCell == bCell {
						same++
					}
				}
				if (float32(same) / float32(rows)) > minRatioToMatch {
					a2b[i] = bStart
					b2a[bStart] = i
				}
			}
			bStart++
		}
	}
}

// getCell returns the specific cell or nil if not present.
func getCell(row []string, column int) (string, error) {
	if column < len(row) {
		return row[column], nil
	}
	return "", ErrorUndefinedCell
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

// tryMergeLines maps the separated line numbers of a git diff. The result is assumed to be ordered.
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
			if j > 0 && result[j-1][1] == 0 {
				temp := j
				for temp > 0 && result[temp-1][1] == 0 {
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
