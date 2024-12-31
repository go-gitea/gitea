// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"encoding/csv"
	"errors"
	"io"
)

const (
	unmappedColumn           = -1
	maxRowsToInspect int     = 10
	minRatioToMatch  float32 = 0.8
)

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
func CreateCsvDiff(diffFile *DiffFile, baseReader, headReader *csv.Reader) ([]*TableDiffSection, error) {
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

func createCsvDiff(diffFile *DiffFile, baseReader, headReader *csv.Reader) ([]*TableDiffSection, error) {
	// Given the baseReader and headReader, we are going to create CSV Reader for each, baseCSVReader and b respectively
	baseCSVReader, err := createCsvReader(baseReader, maxRowsToInspect)
	if err != nil {
		return nil, err
	}
	headCSVReader, err := createCsvReader(headReader, maxRowsToInspect)
	if err != nil {
		return nil, err
	}

	// Initializing the mappings of base to head (a2bColMap) and head to base (b2aColMap) columns
	a2bColMap, b2aColMap := getColumnMapping(baseCSVReader, headCSVReader)

	// Determines how many cols there will be in the diff table, which includes deleted columns from base and added columns to base
	numDiffTableCols := len(a2bColMap) + countUnmappedColumns(b2aColMap)
	if len(a2bColMap) < len(b2aColMap) {
		numDiffTableCols = len(b2aColMap) + countUnmappedColumns(a2bColMap)
	}

	// createDiffTableRow takes the row # of the `a` line and `b` line of a diff (starting from 1), 0 if the line doesn't exist (undefined)
	// in the base or head respectively.
	// Returns a TableDiffRow which has the row index
	createDiffTableRow := func(aLineNum, bLineNum int) (*TableDiffRow, error) {
		// diffTableCells is a row of the diff table. It will have a cells for added, deleted, changed, and unchanged content, thus either
		// the same size as the head table or bigger
		diffTableCells := make([]*TableDiffCell, numDiffTableCols)
		var bRow *[]string
		if bLineNum > 0 {
			row, err := headCSVReader.GetRow(bLineNum - 1)
			if err != nil {
				return nil, err
			}
			bRow = &row
		}
		var aRow *[]string
		if aLineNum > 0 {
			row, err := baseCSVReader.GetRow(aLineNum - 1)
			if err != nil {
				return nil, err
			}
			aRow = &row
		}
		if aRow == nil && bRow == nil {
			// No content
			return nil, nil
		}

		aIndex := 0      // tracks where we are in the a2bColMap
		bIndex := 0      // tracks where we are in the b2aColMap
		colsAdded := 0   // incremented whenever we found a column was added
		colsDeleted := 0 // incrememted whenever a column was deleted

		// We loop until both the aIndex and bIndex are greater than their col map, which then we are done
		for aIndex < len(a2bColMap) || bIndex < len(b2aColMap) {
			// Starting from where aIndex is currently pointing, we see if the map is -1 (dleeted) and if is, create column to note that, increment, and look at the next aIndex
			for aIndex < len(a2bColMap) && a2bColMap[aIndex] == -1 && (bIndex >= len(b2aColMap) || aIndex <= bIndex) {
				var aCell string
				if aRow != nil {
					if cell, err := getCell(*aRow, aIndex); err != nil {
						if err != ErrorUndefinedCell {
							return nil, err
						}
					} else {
						aCell = cell
					}
				}
				diffTableCells[bIndex+colsDeleted] = &TableDiffCell{LeftCell: aCell, Type: TableDiffCellDel}
				aIndex++
				colsDeleted++
			}

			// aIndex is now pointing to a column that also exists in b, or is at the end of a2bColMap. If the former,
			// we can just increment aIndex until it points to a -1 column or one greater than the current bIndex
			for aIndex < len(a2bColMap) && a2bColMap[aIndex] != -1 {
				aIndex++
			}

			// Starting from where bIndex is currently pointing, we see if the map is -1 (added) and if is, create column to note that, increment, and look at the next aIndex
			for bIndex < len(b2aColMap) && b2aColMap[bIndex] == -1 && (aIndex >= len(a2bColMap) || bIndex < aIndex) {
				var bCell string
				cellType := TableDiffCellAdd
				if bRow != nil {
					if cell, err := getCell(*bRow, bIndex); err != nil {
						if err != ErrorUndefinedCell {
							return nil, err
						}
					} else {
						bCell = cell
					}
				} else {
					cellType = TableDiffCellDel
				}
				diffTableCells[bIndex+colsDeleted] = &TableDiffCell{RightCell: bCell, Type: cellType}
				bIndex++
				colsAdded++
			}

			// aIndex is now pointing to a column that also exists in a, or is at the end of b2aColMap. If the former,
			// we get the a col and b col values (if they exist), figure out if they are the same or not, and if the column moved, and add it to the diff table
			for bIndex < len(b2aColMap) && b2aColMap[bIndex] != -1 && (aIndex >= len(a2bColMap) || bIndex < aIndex) {
				var diffTableCell TableDiffCell

				var aCell *string
				// get the aCell value if the aRow exists
				if aRow != nil {
					if cell, err := getCell(*aRow, b2aColMap[bIndex]); err != nil {
						if err != ErrorUndefinedCell {
							return nil, err
						}
					} else {
						aCell = &cell
						diffTableCell.LeftCell = cell
					}
				} else {
					diffTableCell.Type = TableDiffCellAdd
				}

				var bCell *string
				// get the bCell value if the bRow exists
				if bRow != nil {
					if cell, err := getCell(*bRow, bIndex); err != nil {
						if err != ErrorUndefinedCell {
							return nil, err
						}
					} else {
						bCell = &cell
						diffTableCell.RightCell = cell
					}
				} else {
					diffTableCell.Type = TableDiffCellDel
				}

				// if both a and b have a row that exists, compare the value and determine if the row has moved
				if aCell != nil && bCell != nil {
					moved := ((bIndex + colsDeleted) != (b2aColMap[bIndex] + colsAdded))
					if *aCell != *bCell {
						if moved {
							diffTableCell.Type = TableDiffCellMovedChanged
						} else {
							diffTableCell.Type = TableDiffCellChanged
						}
					} else {
						if moved {
							diffTableCell.Type = TableDiffCellMovedUnchanged
						} else {
							diffTableCell.Type = TableDiffCellUnchanged
						}
						diffTableCell.LeftCell = ""
					}
				}

				// Add the diff column to the diff row
				diffTableCells[bIndex+colsDeleted] = &diffTableCell
				bIndex++
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
		// Loop through the merged lines to get each row of the CSV diff table for this section
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
func getColumnMapping(baseCSVReader, headCSVReader *csvReader) ([]int, []int) {
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
				rows := min(maxRowsToInspect, max(0, min(len(baseCSVReader.buffer), len(headCSVReader.buffer))-1))
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
