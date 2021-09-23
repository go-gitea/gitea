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
	TableDiffCellUnchanged TableDiffCellType = iota + 1
	TableDiffCellChanged
	TableDiffCellAdd
	TableDiffCellDel
	TableDiffCellMovedUnchanged
	TableDiffCellMovedChanged
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
	// Given the baseReader and headReader, we are going to create CSV Reader for each, baseCSVReader and b respectively
	baseCSVReader, err := createCsvReader(baseReader, maxRowsToInspect)
	if err != nil {
		return nil, err
	}
	headCSVReader, err := createCsvReader(headReader, maxRowsToInspect)
	if err != nil {
		return nil, err
	}

	// Initalizing the mappings of base to head (base2HeadColMap) and head to base (head2BaseColMap) columns
	base2HeadColMap, head2BaseColMap := getColumnMapping(baseCSVReader, headCSVReader)

	// Determines how many cols there will be in the diff table, which includes deleted columsn from base and added columns to base
	numDiffTableCols := len(base2HeadColMap) + countUnmappedColumns(head2BaseColMap)
	if len(base2HeadColMap) < len(head2BaseColMap) {
		numDiffTableCols = len(head2BaseColMap) + countUnmappedColumns(base2HeadColMap)
	}

	// createDiffTableRow takes the row # of the `a` line and `b` line of a diff (starting from 1), 0 if the line doesn't exist (undefined)
	// in the base or head respectively.
	// Returns a TableDiffRow which has the row index
	createDiffTableRow := func(aLineNum int, bLineNum int) (*TableDiffRow, error) {
		// diffTableCells is a row of the diff table. It will have a cells for added, deleted, changed, and unchanged content, thus either
		// the same size as the head table or bigger
		diffTableCells := make([]*TableDiffCell, numDiffTableCols)
		var aRow *[]string
		if aLineNum > 0 {
			row, err := baseCSVReader.GetRow(aLineNum - 1)
			if err != nil {
				return nil, err
			}
			aRow = &row
		}
		var bRow *[]string
		if bLineNum > 0 {
			row, err := headCSVReader.GetRow(bLineNum - 1)
			if err != nil {
				return nil, err
			}
			bRow = &row
		}
		if bRow == nil {
		} else {
			if len(*bRow) == 0 {
			}
		}
		if aRow == nil && bRow == nil {
			// No content
			return nil, nil
		}
		// First we loop through the head columns and place them in the diff table as they appear, both existing columsn and new columns
		for i := 0; i < len(head2BaseColMap); i++ {
			var bCell *string // Pointer to text of the b line (head), if nil the cell doesn't exist in the head (deleted row in the base)
			if bRow != nil {
				// is an added column and the `b` row exists so the cell should exist, but still will check for undefined cell error
				if cell, err := getCell(*bRow, i); err != nil {
					if err != ErrorUndefinedCell {
						return nil, err
					}
				} else {
					bCell = &cell
				}
			}

			var diffCell TableDiffCell
			addedCols := 0
			if head2BaseColMap[i] == unmappedColumn {
				// This column doesn't exist in the base, so we know it is a new column.
				var cell string
				// Col exists in the head, but we might be displaying a deleted row, so need to check if bCell exists, and if it does, make cell its value
				if bCell != nil {
					cell = *bCell
				}
				diffCell = TableDiffCell{RightCell: cell, Type: TableDiffCellAdd}
				addedCols++
			} else {
				// The column still exists in the head, but we need to figure out if the row exists as well (changed text) or if it is a new row in head
				var aCell *string // Pointer to the texst of the a line (base), if nil the cell doesn't exist in the base (added row in head)
				if aRow != nil {
					// Get the cell contents of the 'a' row. Should exist, but just in case we handle the error and make the contents and empty string
					if cell, err := getCell(*aRow, head2BaseColMap[i]); err != nil {
						if err != ErrorUndefinedCell {
							return nil, err
						}
					} else {
						aCell = &cell
					}
				}
				if bCell == nil {
					// both a & b have the column, but not the row (deleted)
					diffCell = TableDiffCell{LeftCell: *aCell, Type: TableDiffCellDel}
				} else if aCell == nil {
					// both a & b have the column, but not the row (added)
					diffCell = TableDiffCell{RightCell: *bCell, Type: TableDiffCellAdd}
				} else {
					var cellType TableDiffCellType
					if head2BaseColMap[i] > i-addedCols {
						if *aCell != *bCell {
							cellType = TableDiffCellMovedChanged
						} else {
							cellType = TableDiffCellMovedUnchanged
						}
					} else {
						if *aCell != *bCell {
							cellType = TableDiffCellChanged
						} else {
							cellType = TableDiffCellUnchanged
						}
					}
					diffCell = TableDiffCell{LeftCell: *aCell, RightCell: *bCell, Type: cellType}
				}
			}
			diffTableCells[i] = &diffCell
		}

		// Now loop through the base columns to find the unmmapped (deleted) columns in the base
		baseOffset := 0
		for i := 0; i < len(base2HeadColMap); i++ {
			if base2HeadColMap[i] == unmappedColumn {
				// Have an unmapped base column, now need to figure out if the row existed in the base or if it was added
				var aCell *string
				if aRow != nil {
					// is a deleted column and the `a` row exists so the cell should exist, but still will check for undefined cell error
					if cell, err := getCell(*aRow, i); err != nil {
						if err != ErrorUndefinedCell {
							return nil, err
						}
					} else {
						aCell = &cell
					}
				}
				diffTableIndex := i + baseOffset
				if diffTableCells[diffTableIndex] != nil {
					// the diffCells array already has a cell at this i index, so shift this cell and those after it one to the right using copy
					copy(diffTableCells[diffTableIndex+1:], diffTableCells[diffTableIndex:])
				}
				diffCell := TableDiffCell{Type: TableDiffCellDel}
				if aCell != nil {
					diffCell.LeftCell = *aCell
				}
				diffTableCells[diffTableIndex] = &diffCell
				baseOffset++
			}
		}

		return &TableDiffRow{RowIdx: bLineNum, Cells: diffTableCells}, nil
	}

	// diffTableSections are TableDiffSections which represent the diffTableSections we get when doing a diff, each will be its own table in the view
	var diffTableSections []*TableDiffSection

	for i, section := range diffFile.Sections {
		// Each section has multiple diffTableRows
		var diffTableRows []*TableDiffRow
		lines := tryMergeLines(section.Lines)
		// Loop throught the merged lines to get each row of the CSV diff table for this section
		for j, line := range lines {
			if i == 0 && j == 0 && (line[0] != 1 || line[1] != 1) {
				diffTableRow, err := createDiffTableRow(1, 1)
				if err != nil {
					return nil, err
				}
				if diffTableRow != nil {
					diffTableRows = append(diffTableRows, diffTableRow)
				}
			}
			diffTableRow, err := createDiffTableRow(line[0], line[1])
			if err != nil {
				return nil, err
			}
			if diffTableRow != nil {
				diffTableRows = append(diffTableRows, diffTableRow)
			}
		}

		if len(diffTableRows) > 0 {
			diffTableSections = append(diffTableSections, &TableDiffSection{Rows: diffTableRows})
		}
	}

	return diffTableSections, nil
}

// getColumnMapping creates a mapping of columns between a and b
func getColumnMapping(baseCSVReader *csvReader, headCSVReader *csvReader) ([]int, []int) {
	baseRow, _ := baseCSVReader.GetRow(0)
	headRow, _ := headCSVReader.GetRow(0)

	base2HeadColMap := []int{}
	head2BaseColMap := []int{}

	if baseRow != nil {
		base2HeadColMap = make([]int, len(baseRow))
	}
	if headRow != nil {
		head2BaseColMap = make([]int, len(headRow))
	}

	// Initializes all head2base mappings to be unmappedColumn (-1)
	for i := 0; i < len(head2BaseColMap); i++ {
		head2BaseColMap[i] = unmappedColumn
	}

	// Loops through the baseRow and see if there is a match in the head row
	for i := 0; i < len(baseRow); i++ {
		base2HeadColMap[i] = unmappedColumn
		baseCell, err := getCell(baseRow, i)
		if err == nil {
			for j := 0; j < len(headRow); j++ {
				if head2BaseColMap[j] == -1 {
					headCell, err := getCell(headRow, j)
					if err == nil && baseCell == headCell {
						base2HeadColMap[i] = j
						head2BaseColMap[j] = i
						break
					}
				}
			}
		}
	}

	tryMapColumnsByContent(baseCSVReader, base2HeadColMap, headCSVReader, head2BaseColMap)
	tryMapColumnsByContent(headCSVReader, head2BaseColMap, baseCSVReader, base2HeadColMap)

	return base2HeadColMap, head2BaseColMap
}

// tryMapColumnsByContent tries to map missing columns by the content of the first lines.
func tryMapColumnsByContent(baseCSVReader *csvReader, base2HeadColMap []int, headCSVReader *csvReader, head2BaseColMap []int) {
	for i := 0; i < len(base2HeadColMap); i++ {
		headStart := 0
		for base2HeadColMap[i] == unmappedColumn && headStart < len(head2BaseColMap) {
			if head2BaseColMap[headStart] == unmappedColumn {
				rows := util.Min(maxRowsToInspect, util.Max(0, util.Min(len(baseCSVReader.buffer), len(headCSVReader.buffer))-1))
				same := 0
				for j := 1; j <= rows; j++ {
					baseCell, baseErr := getCell(baseCSVReader.buffer[j], i)
					headCell, headErr := getCell(headCSVReader.buffer[j], headStart)
					if baseErr == nil && headErr == nil && baseCell == headCell {
						same++
					}
				}
				if (float32(same) / float32(rows)) > minRatioToMatch {
					base2HeadColMap[i] = headStart
					head2BaseColMap[headStart] = i
				}
			}
			headStart++
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
