package excelize

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

// NewSheet provides function to create a new sheet by given worksheet name,
// when creating a new XLSX file, the default sheet will be create, when you
// create a new file.
func (f *File) NewSheet(name string) int {
	// Check if the worksheet already exists
	if f.GetSheetIndex(name) != 0 {
		return f.SheetCount
	}
	f.SheetCount++
	// Update docProps/app.xml
	f.setAppXML()
	// Update [Content_Types].xml
	f.setContentTypes(f.SheetCount)
	// Create new sheet /xl/worksheets/sheet%d.xml
	f.setSheet(f.SheetCount, name)
	// Update xl/_rels/workbook.xml.rels
	rID := f.addXlsxWorkbookRels(f.SheetCount)
	// Update xl/workbook.xml
	f.setWorkbook(name, rID)
	return f.SheetCount
}

// contentTypesReader provides function to get the pointer to the
// [Content_Types].xml structure after deserialization.
func (f *File) contentTypesReader() *xlsxTypes {
	if f.ContentTypes == nil {
		var content xlsxTypes
		_ = xml.Unmarshal([]byte(f.readXML("[Content_Types].xml")), &content)
		f.ContentTypes = &content
	}
	return f.ContentTypes
}

// contentTypesWriter provides function to save [Content_Types].xml after
// serialize structure.
func (f *File) contentTypesWriter() {
	if f.ContentTypes != nil {
		output, _ := xml.Marshal(f.ContentTypes)
		f.saveFileList("[Content_Types].xml", output)
	}
}

// workbookReader provides function to get the pointer to the xl/workbook.xml
// structure after deserialization.
func (f *File) workbookReader() *xlsxWorkbook {
	if f.WorkBook == nil {
		var content xlsxWorkbook
		_ = xml.Unmarshal([]byte(f.readXML("xl/workbook.xml")), &content)
		f.WorkBook = &content
	}
	return f.WorkBook
}

// workbookWriter provides function to save xl/workbook.xml after serialize
// structure.
func (f *File) workbookWriter() {
	if f.WorkBook != nil {
		output, _ := xml.Marshal(f.WorkBook)
		f.saveFileList("xl/workbook.xml", replaceRelationshipsNameSpaceBytes(output))
	}
}

// worksheetWriter provides function to save xl/worksheets/sheet%d.xml after
// serialize structure.
func (f *File) worksheetWriter() {
	for path, sheet := range f.Sheet {
		if sheet != nil {
			for k, v := range sheet.SheetData.Row {
				f.Sheet[path].SheetData.Row[k].C = trimCell(v.C)
			}
			output, _ := xml.Marshal(sheet)
			f.saveFileList(path, replaceWorkSheetsRelationshipsNameSpaceBytes(output))
			ok := f.checked[path]
			if ok {
				f.checked[path] = false
			}
		}
	}
}

// trimCell provides function to trim blank cells which created by completeCol.
func trimCell(column []xlsxC) []xlsxC {
	col := make([]xlsxC, len(column))
	i := 0
	for _, c := range column {
		if c.S != 0 || c.V != "" || c.F != nil || c.T != "" {
			col[i] = c
			i++
		}
	}
	return col[0:i]
}

// Read and update property of contents type of XLSX.
func (f *File) setContentTypes(index int) {
	content := f.contentTypesReader()
	content.Overrides = append(content.Overrides, xlsxOverride{
		PartName:    "/xl/worksheets/sheet" + strconv.Itoa(index) + ".xml",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml",
	})
}

// Update sheet property by given index.
func (f *File) setSheet(index int, name string) {
	var xlsx xlsxWorksheet
	xlsx.Dimension.Ref = "A1"
	xlsx.SheetViews.SheetView = append(xlsx.SheetViews.SheetView, xlsxSheetView{
		WorkbookViewID: 0,
	})
	path := "xl/worksheets/sheet" + strconv.Itoa(index) + ".xml"
	f.sheetMap[trimSheetName(name)] = path
	f.Sheet[path] = &xlsx
}

// setWorkbook update workbook property of XLSX. Maximum 31 characters are
// allowed in sheet title.
func (f *File) setWorkbook(name string, rid int) {
	content := f.workbookReader()
	content.Sheets.Sheet = append(content.Sheets.Sheet, xlsxSheet{
		Name:    trimSheetName(name),
		SheetID: strconv.Itoa(rid),
		ID:      "rId" + strconv.Itoa(rid),
	})
}

// workbookRelsReader provides function to read and unmarshal workbook
// relationships of XLSX file.
func (f *File) workbookRelsReader() *xlsxWorkbookRels {
	if f.WorkBookRels == nil {
		var content xlsxWorkbookRels
		_ = xml.Unmarshal([]byte(f.readXML("xl/_rels/workbook.xml.rels")), &content)
		f.WorkBookRels = &content
	}
	return f.WorkBookRels
}

// workbookRelsWriter provides function to save xl/_rels/workbook.xml.rels after
// serialize structure.
func (f *File) workbookRelsWriter() {
	if f.WorkBookRels != nil {
		output, _ := xml.Marshal(f.WorkBookRels)
		f.saveFileList("xl/_rels/workbook.xml.rels", output)
	}
}

// addXlsxWorkbookRels update workbook relationships property of XLSX.
func (f *File) addXlsxWorkbookRels(sheet int) int {
	content := f.workbookRelsReader()
	rID := 0
	for _, v := range content.Relationships {
		t, _ := strconv.Atoi(strings.TrimPrefix(v.ID, "rId"))
		if t > rID {
			rID = t
		}
	}
	rID++
	ID := bytes.Buffer{}
	ID.WriteString("rId")
	ID.WriteString(strconv.Itoa(rID))
	target := bytes.Buffer{}
	target.WriteString("worksheets/sheet")
	target.WriteString(strconv.Itoa(sheet))
	target.WriteString(".xml")
	content.Relationships = append(content.Relationships, xlsxWorkbookRelation{
		ID:     ID.String(),
		Target: target.String(),
		Type:   SourceRelationshipWorkSheet,
	})
	return rID
}

// setAppXML update docProps/app.xml file of XML.
func (f *File) setAppXML() {
	f.saveFileList("docProps/app.xml", []byte(templateDocpropsApp))
}

// Some tools that read XLSX files have very strict requirements about the
// structure of the input XML. In particular both Numbers on the Mac and SAS
// dislike inline XML namespace declarations, or namespace prefixes that don't
// match the ones that Excel itself uses. This is a problem because the Go XML
// library doesn't multiple namespace declarations in a single element of a
// document. This function is a horrible hack to fix that after the XML
// marshalling is completed.
func replaceRelationshipsNameSpaceBytes(workbookMarshal []byte) []byte {
	oldXmlns := []byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	newXmlns := []byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="x15" xmlns:x15="http://schemas.microsoft.com/office/spreadsheetml/2010/11/main">`)
	return bytes.Replace(workbookMarshal, oldXmlns, newXmlns, -1)
}

// SetActiveSheet provides function to set default active worksheet of XLSX by
// given index. Note that active index is different with the index that got by
// function GetSheetMap, and it should be greater than 0 and less than total
// worksheet numbers.
func (f *File) SetActiveSheet(index int) {
	if index < 1 {
		index = 1
	}
	index--
	content := f.workbookReader()
	if len(content.BookViews.WorkBookView) > 0 {
		content.BookViews.WorkBookView[0].ActiveTab = index
	} else {
		content.BookViews.WorkBookView = append(content.BookViews.WorkBookView, xlsxWorkBookView{
			ActiveTab: index,
		})
	}
	index++
	for idx, name := range f.GetSheetMap() {
		xlsx := f.workSheetReader(name)
		if index == idx {
			if len(xlsx.SheetViews.SheetView) > 0 {
				xlsx.SheetViews.SheetView[0].TabSelected = true
			} else {
				xlsx.SheetViews.SheetView = append(xlsx.SheetViews.SheetView, xlsxSheetView{
					TabSelected: true,
				})
			}
		} else {
			if len(xlsx.SheetViews.SheetView) > 0 {
				xlsx.SheetViews.SheetView[0].TabSelected = false
			}
		}
	}
}

// GetActiveSheetIndex provides function to get active sheet of XLSX. If not
// found the active sheet will be return integer 0.
func (f *File) GetActiveSheetIndex() int {
	buffer := bytes.Buffer{}
	content := f.workbookReader()
	for _, v := range content.Sheets.Sheet {
		xlsx := xlsxWorksheet{}
		buffer.WriteString("xl/worksheets/sheet")
		buffer.WriteString(strings.TrimPrefix(v.ID, "rId"))
		buffer.WriteString(".xml")
		_ = xml.Unmarshal([]byte(f.readXML(buffer.String())), &xlsx)
		for _, sheetView := range xlsx.SheetViews.SheetView {
			if sheetView.TabSelected {
				ID, _ := strconv.Atoi(strings.TrimPrefix(v.ID, "rId"))
				return ID
			}
		}
		buffer.Reset()
	}
	return 0
}

// SetSheetName provides function to set the worksheet name be given old and new
// worksheet name. Maximum 31 characters are allowed in sheet title and this
// function only changes the name of the sheet and will not update the sheet
// name in the formula or reference associated with the cell. So there may be
// problem formula error or reference missing.
func (f *File) SetSheetName(oldName, newName string) {
	oldName = trimSheetName(oldName)
	newName = trimSheetName(newName)
	content := f.workbookReader()
	for k, v := range content.Sheets.Sheet {
		if v.Name == oldName {
			content.Sheets.Sheet[k].Name = newName
			f.sheetMap[newName] = f.sheetMap[oldName]
			delete(f.sheetMap, oldName)
		}
	}
}

// GetSheetName provides function to get worksheet name of XLSX by given
// worksheet index. If given sheet index is invalid, will return an empty
// string.
func (f *File) GetSheetName(index int) string {
	content := f.workbookReader()
	rels := f.workbookRelsReader()
	for _, rel := range rels.Relationships {
		rID, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rel.Target, "worksheets/sheet"), ".xml"))
		if rID == index {
			for _, v := range content.Sheets.Sheet {
				if v.ID == rel.ID {
					return v.Name
				}
			}
		}
	}
	return ""
}

// GetSheetIndex provides function to get worksheet index of XLSX by given sheet
// name. If given worksheet name is invalid, will return an integer type value
// 0.
func (f *File) GetSheetIndex(name string) int {
	content := f.workbookReader()
	rels := f.workbookRelsReader()
	for _, v := range content.Sheets.Sheet {
		if v.Name == name {
			for _, rel := range rels.Relationships {
				if v.ID == rel.ID {
					rID, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rel.Target, "worksheets/sheet"), ".xml"))
					return rID
				}
			}
		}
	}
	return 0
}

// GetSheetMap provides function to get worksheet name and index map of XLSX.
// For example:
//
//    xlsx, err := excelize.OpenFile("./Book1.xlsx")
//    if err != nil {
//        return
//    }
//    for index, name := range xlsx.GetSheetMap() {
//        fmt.Println(index, name)
//    }
//
func (f *File) GetSheetMap() map[int]string {
	content := f.workbookReader()
	rels := f.workbookRelsReader()
	sheetMap := map[int]string{}
	for _, v := range content.Sheets.Sheet {
		for _, rel := range rels.Relationships {
			if rel.ID == v.ID {
				rID, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rel.Target, "worksheets/sheet"), ".xml"))
				sheetMap[rID] = v.Name
			}
		}
	}
	return sheetMap
}

// getSheetMap provides function to get worksheet name and XML file path map of
// XLSX.
func (f *File) getSheetMap() map[string]string {
	maps := make(map[string]string)
	for idx, name := range f.GetSheetMap() {
		maps[name] = "xl/worksheets/sheet" + strconv.Itoa(idx) + ".xml"
	}
	return maps
}

// SetSheetBackground provides function to set background picture by given
// worksheet name.
func (f *File) SetSheetBackground(sheet, picture string) error {
	var err error
	// Check picture exists first.
	if _, err = os.Stat(picture); os.IsNotExist(err) {
		return err
	}
	ext, ok := supportImageTypes[path.Ext(picture)]
	if !ok {
		return errors.New("Unsupported image extension")
	}
	pictureID := f.countMedia() + 1
	rID := f.addSheetRelationships(sheet, SourceRelationshipImage, "../media/image"+strconv.Itoa(pictureID)+ext, "")
	f.addSheetPicture(sheet, rID)
	f.addMedia(picture, ext)
	f.setContentTypePartImageExtensions()
	return err
}

// DeleteSheet provides function to delete worksheet in a workbook by given
// worksheet name. Use this method with caution, which will affect changes in
// references such as formulas, charts, and so on. If there is any referenced
// value of the deleted worksheet, it will cause a file error when you open it.
// This function will be invalid when only the one worksheet is left.
func (f *File) DeleteSheet(name string) {
	content := f.workbookReader()
	for k, v := range content.Sheets.Sheet {
		if v.Name == trimSheetName(name) && len(content.Sheets.Sheet) > 1 {
			content.Sheets.Sheet = append(content.Sheets.Sheet[:k], content.Sheets.Sheet[k+1:]...)
			sheet := "xl/worksheets/sheet" + strings.TrimPrefix(v.ID, "rId") + ".xml"
			rels := "xl/worksheets/_rels/sheet" + strings.TrimPrefix(v.ID, "rId") + ".xml.rels"
			target := f.deleteSheetFromWorkbookRels(v.ID)
			f.deleteSheetFromContentTypes(target)
			delete(f.sheetMap, name)
			delete(f.XLSX, sheet)
			delete(f.XLSX, rels)
			delete(f.Sheet, sheet)
			f.SheetCount--
		}
	}
	f.SetActiveSheet(len(f.GetSheetMap()))
}

// deleteSheetFromWorkbookRels provides function to remove worksheet
// relationships by given relationships ID in the file
// xl/_rels/workbook.xml.rels.
func (f *File) deleteSheetFromWorkbookRels(rID string) string {
	content := f.workbookRelsReader()
	for k, v := range content.Relationships {
		if v.ID == rID {
			content.Relationships = append(content.Relationships[:k], content.Relationships[k+1:]...)
			return v.Target
		}
	}
	return ""
}

// deleteSheetFromContentTypes provides function to remove worksheet
// relationships by given target name in the file [Content_Types].xml.
func (f *File) deleteSheetFromContentTypes(target string) {
	content := f.contentTypesReader()
	for k, v := range content.Overrides {
		if v.PartName == "/xl/"+target {
			content.Overrides = append(content.Overrides[:k], content.Overrides[k+1:]...)
		}
	}
}

// CopySheet provides function to duplicate a worksheet by gave source and
// target worksheet index. Note that currently doesn't support duplicate
// workbooks that contain tables, charts or pictures. For Example:
//
//    // Sheet1 already exists...
//    index := xlsx.NewSheet("Sheet2")
//    err := xlsx.CopySheet(1, index)
//    return err
//
func (f *File) CopySheet(from, to int) error {
	if from < 1 || to < 1 || from == to || f.GetSheetName(from) == "" || f.GetSheetName(to) == "" {
		return errors.New("Invalid worksheet index")
	}
	return f.copySheet(from, to)
}

// copySheet provides function to duplicate a worksheet by gave source and
// target worksheet name.
func (f *File) copySheet(from, to int) error {
	sheet := f.workSheetReader("sheet" + strconv.Itoa(from))
	worksheet := xlsxWorksheet{}
	err := deepCopy(&worksheet, &sheet)
	if err != nil {
		return err
	}
	path := "xl/worksheets/sheet" + strconv.Itoa(to) + ".xml"
	if len(worksheet.SheetViews.SheetView) > 0 {
		worksheet.SheetViews.SheetView[0].TabSelected = false
	}
	worksheet.Drawing = nil
	worksheet.TableParts = nil
	worksheet.PageSetUp = nil
	f.Sheet[path] = &worksheet
	toRels := "xl/worksheets/_rels/sheet" + strconv.Itoa(to) + ".xml.rels"
	fromRels := "xl/worksheets/_rels/sheet" + strconv.Itoa(from) + ".xml.rels"
	_, ok := f.XLSX[fromRels]
	if ok {
		f.XLSX[toRels] = f.XLSX[fromRels]
	}
	return err
}

// SetSheetVisible provides function to set worksheet visible by given worksheet
// name. A workbook must contain at least one visible worksheet. If the given
// worksheet has been activated, this setting will be invalidated. Sheet state
// values as defined by http://msdn.microsoft.com/en-us/library/office/documentformat.openxml.spreadsheet.sheetstatevalues.aspx
//
//    visible
//    hidden
//    veryHidden
//
// For example, hide Sheet1:
//
//    xlsx.SetSheetVisible("Sheet1", false)
//
func (f *File) SetSheetVisible(name string, visible bool) {
	name = trimSheetName(name)
	content := f.workbookReader()
	if visible {
		for k, v := range content.Sheets.Sheet {
			if v.Name == name {
				content.Sheets.Sheet[k].State = ""
			}
		}
		return
	}
	count := 0
	for _, v := range content.Sheets.Sheet {
		if v.State != "hidden" {
			count++
		}
	}
	for k, v := range content.Sheets.Sheet {
		xlsx := f.workSheetReader(f.GetSheetMap()[k])
		tabSelected := false
		if len(xlsx.SheetViews.SheetView) > 0 {
			tabSelected = xlsx.SheetViews.SheetView[0].TabSelected
		}
		if v.Name == name && count > 1 && !tabSelected {
			content.Sheets.Sheet[k].State = "hidden"
		}
	}
}

// parseFormatPanesSet provides function to parse the panes settings.
func parseFormatPanesSet(formatSet string) (*formatPanes, error) {
	format := formatPanes{}
	err := json.Unmarshal([]byte(formatSet), &format)
	return &format, err
}

// SetPanes provides function to create and remove freeze panes and split panes
// by given worksheet name and panes format set.
//
// activePane defines the pane that is active. The possible values for this
// attribute are defined in the following table:
//
//     Enumeration Value              | Description
//    --------------------------------+-------------------------------------------------------------
//     bottomLeft (Bottom Left Pane)  | Bottom left pane, when both vertical and horizontal
//                                    | splits are applied.
//                                    |
//                                    | This value is also used when only a horizontal split has
//                                    | been applied, dividing the pane into upper and lower
//                                    | regions. In that case, this value specifies the bottom
//                                    | pane.
//                                    |
//    bottomRight (Bottom Right Pane) | Bottom right pane, when both vertical and horizontal
//                                    | splits are applied.
//                                    |
//    topLeft (Top Left Pane)         | Top left pane, when both vertical and horizontal splits
//                                    | are applied.
//                                    |
//                                    | This value is also used when only a horizontal split has
//                                    | been applied, dividing the pane into upper and lower
//                                    | regions. In that case, this value specifies the top pane.
//                                    |
//                                    | This value is also used when only a vertical split has
//                                    | been applied, dividing the pane into right and left
//                                    | regions. In that case, this value specifies the left pane
//                                    |
//    topRight (Top Right Pane)       | Top right pane, when both vertical and horizontal
//                                    | splits are applied.
//                                    |
//                                    | This value is also used when only a vertical split has
//                                    | been applied, dividing the pane into right and left
//                                    | regions. In that case, this value specifies the right
//                                    | pane.
//
// Pane state type is restricted to the values supported currently listed in the following table:
//
//     Enumeration Value              | Description
//    --------------------------------+-------------------------------------------------------------
//     frozen (Frozen)                | Panes are frozen, but were not split being frozen. In
//                                    | this state, when the panes are unfrozen again, a single
//                                    | pane results, with no split.
//                                    |
//                                    | In this state, the split bars are not adjustable.
//                                    |
//     split (Split)                  | Panes are split, but not frozen. In this state, the split
//                                    | bars are adjustable by the user.
//
// x_split (Horizontal Split Position): Horizontal position of the split, in
// 1/20th of a point; 0 (zero) if none. If the pane is frozen, this value
// indicates the number of columns visible in the top pane.
//
// y_split (Vertical Split Position): Vertical position of the split, in 1/20th
// of a point; 0 (zero) if none. If the pane is frozen, this value indicates the
// number of rows visible in the left pane. The possible values for this
// attribute are defined by the W3C XML Schema double datatype.
//
// top_left_cell: Location of the top left visible cell in the bottom right pane
// (when in Left-To-Right mode).
//
// sqref (Sequence of References): Range of the selection. Can be non-contiguous
// set of ranges.
//
// An example of how to freeze column A in the Sheet1 and set the active cell on
// Sheet1!K16:
//
//    xlsx.SetPanes("Sheet1", `{"freeze":true,"split":false,"x_split":1,"y_split":0,"top_left_cell":"B1","active_pane":"topRight","panes":[{"sqref":"K16","active_cell":"K16","pane":"topRight"}]}`)
//
// An example of how to freeze rows 1 to 9 in the Sheet1 and set the active cell
// ranges on Sheet1!A11:XFD11:
//
//    xlsx.SetPanes("Sheet1", `{"freeze":true,"split":false,"x_split":0,"y_split":9,"top_left_cell":"A34","active_pane":"bottomLeft","panes":[{"sqref":"A11:XFD11","active_cell":"A11","pane":"bottomLeft"}]}`)
//
// An example of how to create split panes in the Sheet1 and set the active cell
// on Sheet1!J60:
//
//    xlsx.SetPanes("Sheet1", `{"freeze":false,"split":true,"x_split":3270,"y_split":1800,"top_left_cell":"N57","active_pane":"bottomLeft","panes":[{"sqref":"I36","active_cell":"I36"},{"sqref":"G33","active_cell":"G33","pane":"topRight"},{"sqref":"J60","active_cell":"J60","pane":"bottomLeft"},{"sqref":"O60","active_cell":"O60","pane":"bottomRight"}]}`)
//
// An example of how to unfreeze and remove all panes on Sheet1:
//
//    xlsx.SetPanes("Sheet1", `{"freeze":false,"split":false}`)
//
func (f *File) SetPanes(sheet, panes string) {
	fs, _ := parseFormatPanesSet(panes)
	xlsx := f.workSheetReader(sheet)
	p := &xlsxPane{
		ActivePane:  fs.ActivePane,
		TopLeftCell: fs.TopLeftCell,
		XSplit:      float64(fs.XSplit),
		YSplit:      float64(fs.YSplit),
	}
	if fs.Freeze {
		p.State = "frozen"
	}
	xlsx.SheetViews.SheetView[len(xlsx.SheetViews.SheetView)-1].Pane = p
	if !(fs.Freeze) && !(fs.Split) {
		if len(xlsx.SheetViews.SheetView) > 0 {
			xlsx.SheetViews.SheetView[len(xlsx.SheetViews.SheetView)-1].Pane = nil
		}
	}
	s := []*xlsxSelection{}
	for _, p := range fs.Panes {
		s = append(s, &xlsxSelection{
			ActiveCell: p.ActiveCell,
			Pane:       p.Pane,
			SQRef:      p.SQRef,
		})
	}
	xlsx.SheetViews.SheetView[len(xlsx.SheetViews.SheetView)-1].Selection = s
}

// GetSheetVisible provides function to get worksheet visible by given worksheet
// name. For example, get visible state of Sheet1:
//
//    xlsx.GetSheetVisible("Sheet1")
//
func (f *File) GetSheetVisible(name string) bool {
	content := f.workbookReader()
	visible := false
	for k, v := range content.Sheets.Sheet {
		if v.Name == trimSheetName(name) {
			if content.Sheets.Sheet[k].State == "" || content.Sheets.Sheet[k].State == "visible" {
				visible = true
			}
		}
	}
	return visible
}

// trimSheetName provides function to trim invaild characters by given worksheet
// name.
func trimSheetName(name string) string {
	r := []rune{}
	for _, v := range name {
		switch v {
		case 58, 92, 47, 63, 42, 91, 93: // replace :\/?*[]
			continue
		default:
			r = append(r, v)
		}
	}
	name = string(r)
	if utf8.RuneCountInString(name) > 31 {
		name = string([]rune(name)[0:31])
	}
	return name
}
