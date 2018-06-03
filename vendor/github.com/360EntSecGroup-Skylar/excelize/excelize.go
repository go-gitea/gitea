package excelize

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

// File define a populated XLSX file struct.
type File struct {
	checked       map[string]bool
	sheetMap      map[string]string
	ContentTypes  *xlsxTypes
	Path          string
	SharedStrings *xlsxSST
	Sheet         map[string]*xlsxWorksheet
	SheetCount    int
	Styles        *xlsxStyleSheet
	WorkBook      *xlsxWorkbook
	WorkBookRels  *xlsxWorkbookRels
	XLSX          map[string][]byte
}

// OpenFile take the name of an XLSX file and returns a populated XLSX file
// struct for it.
func OpenFile(filename string) (*File, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	f, err := OpenReader(file)
	if err != nil {
		return nil, err
	}
	f.Path = filename
	return f, nil
}

// OpenReader take an io.Reader and return a populated XLSX file.
func OpenReader(r io.Reader) (*File, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}

	file, sheetCount, err := ReadZipReader(zr)
	if err != nil {
		return nil, err
	}
	f := &File{
		checked:    make(map[string]bool),
		Sheet:      make(map[string]*xlsxWorksheet),
		SheetCount: sheetCount,
		XLSX:       file,
	}
	f.sheetMap = f.getSheetMap()
	f.Styles = f.stylesReader()
	return f, nil
}

// setDefaultTimeStyle provides function to set default numbers format for
// time.Time type cell value by given worksheet name, cell coordinates and
// number format code.
func (f *File) setDefaultTimeStyle(sheet, axis string, format int) {
	if f.GetCellStyle(sheet, axis) == 0 {
		style, _ := f.NewStyle(`{"number_format": ` + strconv.Itoa(format) + `}`)
		f.SetCellStyle(sheet, axis, axis, style)
	}
}

// workSheetReader provides function to get the pointer to the structure after
// deserialization by given worksheet name.
func (f *File) workSheetReader(sheet string) *xlsxWorksheet {
	name, ok := f.sheetMap[trimSheetName(sheet)]
	if !ok {
		name = "xl/worksheets/" + strings.ToLower(sheet) + ".xml"
	}
	if f.Sheet[name] == nil {
		var xlsx xlsxWorksheet
		_ = xml.Unmarshal(f.readXML(name), &xlsx)
		if f.checked == nil {
			f.checked = make(map[string]bool)
		}
		ok := f.checked[name]
		if !ok {
			checkSheet(&xlsx)
			checkRow(&xlsx)
			f.checked[name] = true
		}
		f.Sheet[name] = &xlsx
	}
	return f.Sheet[name]
}

// checkSheet provides function to fill each row element and make that is
// continuous in a worksheet of XML.
func checkSheet(xlsx *xlsxWorksheet) {
	row := len(xlsx.SheetData.Row)
	if row >= 1 {
		lastRow := xlsx.SheetData.Row[row-1].R
		if lastRow >= row {
			row = lastRow
		}
	}
	sheetData := xlsxSheetData{}
	existsRows := map[int]int{}
	for k := range xlsx.SheetData.Row {
		existsRows[xlsx.SheetData.Row[k].R] = k
	}
	for i := 0; i < row; i++ {
		_, ok := existsRows[i+1]
		if ok {
			sheetData.Row = append(sheetData.Row, xlsx.SheetData.Row[existsRows[i+1]])
		} else {
			sheetData.Row = append(sheetData.Row, xlsxRow{
				R: i + 1,
			})
		}
	}
	xlsx.SheetData = sheetData
}

// replaceWorkSheetsRelationshipsNameSpaceBytes provides function to replace
// xl/worksheets/sheet%d.xml XML tags to self-closing for compatible Microsoft
// Office Excel 2007.
func replaceWorkSheetsRelationshipsNameSpaceBytes(workbookMarshal []byte) []byte {
	var oldXmlns = []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	var newXmlns = []byte(`<worksheet xr:uid="{00000000-0001-0000-0000-000000000000}" xmlns:xr3="http://schemas.microsoft.com/office/spreadsheetml/2016/revision3" xmlns:xr2="http://schemas.microsoft.com/office/spreadsheetml/2015/revision2" xmlns:xr="http://schemas.microsoft.com/office/spreadsheetml/2014/revision" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" xmlns:x14ac="http://schemas.microsoft.com/office/spreadsheetml/2009/9/ac" mc:Ignorable="x14ac xr xr2 xr3" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:mx="http://schemas.microsoft.com/office/mac/excel/2008/main" xmlns:mv="urn:schemas-microsoft-com:mac:vml" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	workbookMarshal = bytes.Replace(workbookMarshal, oldXmlns, newXmlns, -1)
	return workbookMarshal
}

// UpdateLinkedValue fix linked values within a spreadsheet are not updating in
// Office Excel 2007 and 2010. This function will be remove value tag when met a
// cell have a linked value. Reference
// https://social.technet.microsoft.com/Forums/office/en-US/e16bae1f-6a2c-4325-8013-e989a3479066/excel-2010-linked-cells-not-updating?forum=excel
//
// Notice: after open XLSX file Excel will be update linked value and generate
// new value and will prompt save file or not.
//
// For example:
//
//    <row r="19" spans="2:2">
//        <c r="B19">
//            <f>SUM(Sheet2!D2,Sheet2!D11)</f>
//            <v>100</v>
//         </c>
//    </row>
//
// to
//
//    <row r="19" spans="2:2">
//        <c r="B19">
//            <f>SUM(Sheet2!D2,Sheet2!D11)</f>
//        </c>
//    </row>
//
func (f *File) UpdateLinkedValue() {
	for _, name := range f.GetSheetMap() {
		xlsx := f.workSheetReader(name)
		for indexR := range xlsx.SheetData.Row {
			for indexC, col := range xlsx.SheetData.Row[indexR].C {
				if col.F != nil && col.V != "" {
					xlsx.SheetData.Row[indexR].C[indexC].V = ""
					xlsx.SheetData.Row[indexR].C[indexC].T = ""
				}
			}
		}
	}
}

// adjustHelper provides function to adjust rows and columns dimensions,
// hyperlinks, merged cells and auto filter when inserting or deleting rows or
// columns.
//
// sheet: Worksheet name that we're editing
// column: Index number of the column we're inserting/deleting before
// row: Index number of the row we're inserting/deleting before
// offset: Number of rows/column to insert/delete negative values indicate deletion
//
// TODO: adjustPageBreaks, adjustComments, adjustDataValidations, adjustProtectedCells
//
func (f *File) adjustHelper(sheet string, column, row, offset int) {
	xlsx := f.workSheetReader(sheet)
	f.adjustRowDimensions(xlsx, row, offset)
	f.adjustColDimensions(xlsx, column, offset)
	f.adjustHyperlinks(sheet, column, row, offset)
	f.adjustMergeCells(xlsx, column, row, offset)
	f.adjustAutoFilter(xlsx, column, row, offset)
	checkSheet(xlsx)
	checkRow(xlsx)
}

// adjustColDimensions provides function to update column dimensions when
// inserting or deleting rows or columns.
func (f *File) adjustColDimensions(xlsx *xlsxWorksheet, column, offset int) {
	for i, r := range xlsx.SheetData.Row {
		for k, v := range r.C {
			axis := v.R
			col := string(strings.Map(letterOnlyMapF, axis))
			row, _ := strconv.Atoi(strings.Map(intOnlyMapF, axis))
			yAxis := TitleToNumber(col)
			if yAxis >= column && column != -1 {
				xlsx.SheetData.Row[i].C[k].R = ToAlphaString(yAxis+offset) + strconv.Itoa(row)
			}
		}
	}
}

// adjustRowDimensions provides function to update row dimensions when inserting
// or deleting rows or columns.
func (f *File) adjustRowDimensions(xlsx *xlsxWorksheet, rowIndex, offset int) {
	if rowIndex == -1 {
		return
	}
	for i, r := range xlsx.SheetData.Row {
		if r.R >= rowIndex {
			xlsx.SheetData.Row[i].R += offset
			for k, v := range xlsx.SheetData.Row[i].C {
				axis := v.R
				col := string(strings.Map(letterOnlyMapF, axis))
				row, _ := strconv.Atoi(strings.Map(intOnlyMapF, axis))
				xAxis := row + offset
				xlsx.SheetData.Row[i].C[k].R = col + strconv.Itoa(xAxis)
			}
		}
	}
}

// adjustHyperlinks provides function to update hyperlinks when inserting or
// deleting rows or columns.
func (f *File) adjustHyperlinks(sheet string, column, rowIndex, offset int) {
	xlsx := f.workSheetReader(sheet)

	// order is important
	if xlsx.Hyperlinks != nil && offset < 0 {
		for i, v := range xlsx.Hyperlinks.Hyperlink {
			axis := v.Ref
			col := string(strings.Map(letterOnlyMapF, axis))
			row, _ := strconv.Atoi(strings.Map(intOnlyMapF, axis))
			yAxis := TitleToNumber(col)
			if row == rowIndex || yAxis == column {
				f.deleteSheetRelationships(sheet, v.RID)
				if len(xlsx.Hyperlinks.Hyperlink) > 1 {
					xlsx.Hyperlinks.Hyperlink = append(xlsx.Hyperlinks.Hyperlink[:i], xlsx.Hyperlinks.Hyperlink[i+1:]...)
				} else {
					xlsx.Hyperlinks = nil
				}
			}
		}
	}

	if xlsx.Hyperlinks != nil {
		for i, v := range xlsx.Hyperlinks.Hyperlink {
			axis := v.Ref
			col := string(strings.Map(letterOnlyMapF, axis))
			row, _ := strconv.Atoi(strings.Map(intOnlyMapF, axis))
			xAxis := row + offset
			yAxis := TitleToNumber(col)
			if rowIndex != -1 && row >= rowIndex {
				xlsx.Hyperlinks.Hyperlink[i].Ref = col + strconv.Itoa(xAxis)
			}
			if column != -1 && yAxis >= column {
				xlsx.Hyperlinks.Hyperlink[i].Ref = ToAlphaString(yAxis+offset) + strconv.Itoa(row)
			}
		}
	}
}

// adjustMergeCellsHelper provides function to update merged cells when inserting or
// deleting rows or columns.
func (f *File) adjustMergeCellsHelper(xlsx *xlsxWorksheet, column, rowIndex, offset int) {
	if xlsx.MergeCells != nil {
		for k, v := range xlsx.MergeCells.Cells {
			beg := strings.Split(v.Ref, ":")[0]
			end := strings.Split(v.Ref, ":")[1]

			begcol := string(strings.Map(letterOnlyMapF, beg))
			begrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, beg))
			begxAxis := begrow + offset
			begyAxis := TitleToNumber(begcol)

			endcol := string(strings.Map(letterOnlyMapF, end))
			endrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, end))
			endxAxis := endrow + offset
			endyAxis := TitleToNumber(endcol)

			if rowIndex != -1 {
				if begrow > 1 && begrow >= rowIndex {
					beg = begcol + strconv.Itoa(begxAxis)
				}
				if endrow > 1 && endrow >= rowIndex {
					end = endcol + strconv.Itoa(endxAxis)
				}
			}

			if column != -1 {
				if begyAxis >= column {
					beg = ToAlphaString(begyAxis+offset) + strconv.Itoa(endrow)
				}
				if endyAxis >= column {
					end = ToAlphaString(endyAxis+offset) + strconv.Itoa(endrow)
				}
			}

			xlsx.MergeCells.Cells[k].Ref = beg + ":" + end
		}
	}
}

// adjustMergeCells provides function to update merged cells when inserting or
// deleting rows or columns.
func (f *File) adjustMergeCells(xlsx *xlsxWorksheet, column, rowIndex, offset int) {
	f.adjustMergeCellsHelper(xlsx, column, rowIndex, offset)

	if xlsx.MergeCells != nil && offset < 0 {
		for k, v := range xlsx.MergeCells.Cells {
			beg := strings.Split(v.Ref, ":")[0]
			end := strings.Split(v.Ref, ":")[1]
			if beg == end {
				xlsx.MergeCells.Count += offset
				if len(xlsx.MergeCells.Cells) > 1 {
					xlsx.MergeCells.Cells = append(xlsx.MergeCells.Cells[:k], xlsx.MergeCells.Cells[k+1:]...)
				} else {
					xlsx.MergeCells = nil
				}
			}
		}
	}
}

// adjustAutoFilter provides function to update the auto filter when inserting
// or deleting rows or columns.
func (f *File) adjustAutoFilter(xlsx *xlsxWorksheet, column, rowIndex, offset int) {
	f.adjustAutoFilterHelper(xlsx, column, rowIndex, offset)

	if xlsx.AutoFilter != nil {
		beg := strings.Split(xlsx.AutoFilter.Ref, ":")[0]
		end := strings.Split(xlsx.AutoFilter.Ref, ":")[1]

		begcol := string(strings.Map(letterOnlyMapF, beg))
		begrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, beg))
		begxAxis := begrow + offset

		endcol := string(strings.Map(letterOnlyMapF, end))
		endrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, end))
		endxAxis := endrow + offset
		endyAxis := TitleToNumber(endcol)

		if rowIndex != -1 {
			if begrow >= rowIndex {
				beg = begcol + strconv.Itoa(begxAxis)
			}
			if endrow >= rowIndex {
				end = endcol + strconv.Itoa(endxAxis)
			}
		}

		if column != -1 && endyAxis >= column {
			end = ToAlphaString(endyAxis+offset) + strconv.Itoa(endrow)
		}
		xlsx.AutoFilter.Ref = beg + ":" + end
	}
}

// adjustAutoFilterHelper provides function to update the auto filter when
// inserting or deleting rows or columns.
func (f *File) adjustAutoFilterHelper(xlsx *xlsxWorksheet, column, rowIndex, offset int) {
	if xlsx.AutoFilter != nil {
		beg := strings.Split(xlsx.AutoFilter.Ref, ":")[0]
		end := strings.Split(xlsx.AutoFilter.Ref, ":")[1]

		begcol := string(strings.Map(letterOnlyMapF, beg))
		begrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, beg))
		begyAxis := TitleToNumber(begcol)

		endcol := string(strings.Map(letterOnlyMapF, end))
		endyAxis := TitleToNumber(endcol)
		endrow, _ := strconv.Atoi(strings.Map(intOnlyMapF, end))

		if (begrow == rowIndex && offset < 0) || (column == begyAxis && column == endyAxis) {
			xlsx.AutoFilter = nil
			for i, r := range xlsx.SheetData.Row {
				if begrow < r.R && r.R <= endrow {
					xlsx.SheetData.Row[i].Hidden = false
				}
			}
		}
	}
}
