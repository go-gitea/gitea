package excelize

import (
	"bytes"
	"math"
	"strconv"
	"strings"
)

// Define the default cell size and EMU unit of measurement.
const (
	defaultColWidthPixels  float64 = 64
	defaultRowHeightPixels float64 = 20
	EMU                    int     = 9525
)

// GetColVisible provides a function to get visible of a single column by given
// worksheet name and column name. For example, get visible state of column D
// in Sheet1:
//
//    xlsx.GetColVisible("Sheet1", "D")
//
func (f *File) GetColVisible(sheet, column string) bool {
	xlsx := f.workSheetReader(sheet)
	col := TitleToNumber(strings.ToUpper(column)) + 1
	visible := true
	if xlsx.Cols == nil {
		return visible
	}
	for c := range xlsx.Cols.Col {
		if xlsx.Cols.Col[c].Min <= col && col <= xlsx.Cols.Col[c].Max {
			visible = !xlsx.Cols.Col[c].Hidden
		}
	}
	return visible
}

// SetColVisible provides a function to set visible of a single column by given
// worksheet name and column name. For example, hide column D in Sheet1:
//
//    xlsx.SetColVisible("Sheet1", "D", false)
//
func (f *File) SetColVisible(sheet, column string, visible bool) {
	xlsx := f.workSheetReader(sheet)
	c := TitleToNumber(strings.ToUpper(column)) + 1
	col := xlsxCol{
		Min:         c,
		Max:         c,
		Hidden:      !visible,
		CustomWidth: true,
	}
	if xlsx.Cols == nil {
		cols := xlsxCols{}
		cols.Col = append(cols.Col, col)
		xlsx.Cols = &cols
		return
	}
	for v := range xlsx.Cols.Col {
		if xlsx.Cols.Col[v].Min <= c && c <= xlsx.Cols.Col[v].Max {
			col = xlsx.Cols.Col[v]
		}
	}
	col.Min = c
	col.Max = c
	col.Hidden = !visible
	col.CustomWidth = true
	xlsx.Cols.Col = append(xlsx.Cols.Col, col)
}

// GetColOutlineLevel provides a function to get outline level of a single
// column by given worksheet name and column name. For example, get outline
// level of column D in Sheet1:
//
//    xlsx.GetColOutlineLevel("Sheet1", "D")
//
func (f *File) GetColOutlineLevel(sheet, column string) uint8 {
	xlsx := f.workSheetReader(sheet)
	col := TitleToNumber(strings.ToUpper(column)) + 1
	level := uint8(0)
	if xlsx.Cols == nil {
		return level
	}
	for c := range xlsx.Cols.Col {
		if xlsx.Cols.Col[c].Min <= col && col <= xlsx.Cols.Col[c].Max {
			level = xlsx.Cols.Col[c].OutlineLevel
		}
	}
	return level
}

// SetColOutlineLevel provides a function to set outline level of a single
// column by given worksheet name and column name. For example, set outline
// level of column D in Sheet1 to 2:
//
//    xlsx.SetColOutlineLevel("Sheet1", "D", 2)
//
func (f *File) SetColOutlineLevel(sheet, column string, level uint8) {
	xlsx := f.workSheetReader(sheet)
	c := TitleToNumber(strings.ToUpper(column)) + 1
	col := xlsxCol{
		Min:          c,
		Max:          c,
		OutlineLevel: level,
		CustomWidth:  true,
	}
	if xlsx.Cols == nil {
		cols := xlsxCols{}
		cols.Col = append(cols.Col, col)
		xlsx.Cols = &cols
		return
	}
	for v := range xlsx.Cols.Col {
		if xlsx.Cols.Col[v].Min <= c && c <= xlsx.Cols.Col[v].Max {
			col = xlsx.Cols.Col[v]
		}
	}
	col.Min = c
	col.Max = c
	col.OutlineLevel = level
	col.CustomWidth = true
	xlsx.Cols.Col = append(xlsx.Cols.Col, col)
}

// SetColWidth provides function to set the width of a single column or multiple
// columns. For example:
//
//    xlsx := excelize.NewFile()
//    xlsx.SetColWidth("Sheet1", "A", "H", 20)
//    err := xlsx.Save()
//    if err != nil {
//        fmt.Println(err)
//    }
//
func (f *File) SetColWidth(sheet, startcol, endcol string, width float64) {
	min := TitleToNumber(strings.ToUpper(startcol)) + 1
	max := TitleToNumber(strings.ToUpper(endcol)) + 1
	if min > max {
		min, max = max, min
	}
	xlsx := f.workSheetReader(sheet)
	col := xlsxCol{
		Min:         min,
		Max:         max,
		Width:       width,
		CustomWidth: true,
	}
	if xlsx.Cols != nil {
		xlsx.Cols.Col = append(xlsx.Cols.Col, col)
	} else {
		cols := xlsxCols{}
		cols.Col = append(cols.Col, col)
		xlsx.Cols = &cols
	}
}

// positionObjectPixels calculate the vertices that define the position of a
// graphical object within the worksheet in pixels.
//
//          +------------+------------+
//          |     A      |      B     |
//    +-----+------------+------------+
//    |     |(x1,y1)     |            |
//    |  1  |(A1)._______|______      |
//    |     |    |              |     |
//    |     |    |              |     |
//    +-----+----|    OBJECT    |-----+
//    |     |    |              |     |
//    |  2  |    |______________.     |
//    |     |            |        (B2)|
//    |     |            |     (x2,y2)|
//    +-----+------------+------------+
//
// Example of an object that covers some of the area from cell A1 to B2.
//
// Based on the width and height of the object we need to calculate 8 vars:
//
//    colStart, rowStart, colEnd, rowEnd, x1, y1, x2, y2.
//
// We also calculate the absolute x and y position of the top left vertex of
// the object. This is required for images.
//
// The width and height of the cells that the object occupies can be
// variable and have to be taken into account.
//
// The values of col_start and row_start are passed in from the calling
// function. The values of col_end and row_end are calculated by
// subtracting the width and height of the object from the width and
// height of the underlying cells.
//
//    colStart        # Col containing upper left corner of object.
//    x1              # Distance to left side of object.
//
//    rowStart        # Row containing top left corner of object.
//    y1              # Distance to top of object.
//
//    colEnd          # Col containing lower right corner of object.
//    x2              # Distance to right side of object.
//
//    rowEnd          # Row containing bottom right corner of object.
//    y2              # Distance to bottom of object.
//
//    width           # Width of object frame.
//    height          # Height of object frame.
//
//    xAbs            # Absolute distance to left side of object.
//    yAbs            # Absolute distance to top side of object.
//
func (f *File) positionObjectPixels(sheet string, colStart, rowStart, x1, y1, width, height int) (int, int, int, int, int, int, int, int) {
	xAbs := 0
	yAbs := 0

	// Calculate the absolute x offset of the top-left vertex.
	for colID := 1; colID <= colStart; colID++ {
		xAbs += f.getColWidth(sheet, colID)
	}
	xAbs += x1

	// Calculate the absolute y offset of the top-left vertex.
	// Store the column change to allow optimisations.
	for rowID := 1; rowID <= rowStart; rowID++ {
		yAbs += f.getRowHeight(sheet, rowID)
	}
	yAbs += y1

	// Adjust start column for offsets that are greater than the col width.
	for x1 >= f.getColWidth(sheet, colStart) {
		x1 -= f.getColWidth(sheet, colStart)
		colStart++
	}

	// Adjust start row for offsets that are greater than the row height.
	for y1 >= f.getRowHeight(sheet, rowStart) {
		y1 -= f.getRowHeight(sheet, rowStart)
		rowStart++
	}

	// Initialise end cell to the same as the start cell.
	colEnd := colStart
	rowEnd := rowStart

	width += x1
	height += y1

	// Subtract the underlying cell widths to find end cell of the object.
	for width >= f.getColWidth(sheet, colEnd) {
		colEnd++
		width -= f.getColWidth(sheet, colEnd)
	}

	// Subtract the underlying cell heights to find end cell of the object.
	for height >= f.getRowHeight(sheet, rowEnd) {
		rowEnd++
		height -= f.getRowHeight(sheet, rowEnd)
	}

	// The end vertices are whatever is left from the width and height.
	x2 := width
	y2 := height
	return colStart, rowStart, xAbs, yAbs, colEnd, rowEnd, x2, y2
}

// getColWidth provides function to get column width in pixels by given sheet
// name and column index.
func (f *File) getColWidth(sheet string, col int) int {
	xlsx := f.workSheetReader(sheet)
	if xlsx.Cols != nil {
		var width float64
		for _, v := range xlsx.Cols.Col {
			if v.Min <= col && col <= v.Max {
				width = v.Width
			}
		}
		if width != 0 {
			return int(convertColWidthToPixels(width))
		}
	}
	// Optimisation for when the column widths haven't changed.
	return int(defaultColWidthPixels)
}

// GetColWidth provides function to get column width by given worksheet name and
// column index.
func (f *File) GetColWidth(sheet, column string) float64 {
	col := TitleToNumber(strings.ToUpper(column)) + 1
	xlsx := f.workSheetReader(sheet)
	if xlsx.Cols != nil {
		var width float64
		for _, v := range xlsx.Cols.Col {
			if v.Min <= col && col <= v.Max {
				width = v.Width
			}
		}
		if width != 0 {
			return width
		}
	}
	// Optimisation for when the column widths haven't changed.
	return defaultColWidthPixels
}

// InsertCol provides function to insert a new column before given column index.
// For example, create a new column before column C in Sheet1:
//
//    xlsx.InsertCol("Sheet1", "C")
//
func (f *File) InsertCol(sheet, column string) {
	col := TitleToNumber(strings.ToUpper(column))
	f.adjustHelper(sheet, col, -1, 1)
}

// RemoveCol provides function to remove single column by given worksheet name
// and column index. For example, remove column C in Sheet1:
//
//    xlsx.RemoveCol("Sheet1", "C")
//
func (f *File) RemoveCol(sheet, column string) {
	xlsx := f.workSheetReader(sheet)
	for r := range xlsx.SheetData.Row {
		for k, v := range xlsx.SheetData.Row[r].C {
			axis := v.R
			col := string(strings.Map(letterOnlyMapF, axis))
			if col == column {
				xlsx.SheetData.Row[r].C = append(xlsx.SheetData.Row[r].C[:k], xlsx.SheetData.Row[r].C[k+1:]...)
			}
		}
	}
	col := TitleToNumber(strings.ToUpper(column))
	f.adjustHelper(sheet, col, -1, -1)
}

// Completion column element tags of XML in a sheet.
func completeCol(xlsx *xlsxWorksheet, row, cell int) {
	buffer := bytes.Buffer{}
	for r := range xlsx.SheetData.Row {
		if len(xlsx.SheetData.Row[r].C) < cell {
			start := len(xlsx.SheetData.Row[r].C)
			for iii := start; iii < cell; iii++ {
				buffer.WriteString(ToAlphaString(iii))
				buffer.WriteString(strconv.Itoa(r + 1))
				xlsx.SheetData.Row[r].C = append(xlsx.SheetData.Row[r].C, xlsxC{
					R: buffer.String(),
				})
				buffer.Reset()
			}
		}
	}
}

// convertColWidthToPixels provieds function to convert the width of a cell from
// user's units to pixels. Excel rounds the column width to the nearest pixel.
// If the width hasn't been set by the user we use the default value. If the
// column is hidden it has a value of zero.
func convertColWidthToPixels(width float64) float64 {
	var padding float64 = 5
	var pixels float64
	var maxDigitWidth float64 = 7
	if width == 0 {
		return pixels
	}
	if width < 1 {
		pixels = (width * 12) + 0.5
		return math.Ceil(pixels)
	}
	pixels = (width*maxDigitWidth + 0.5) + padding
	return math.Ceil(pixels)
}
