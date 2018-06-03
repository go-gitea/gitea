package excelize

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
)

// NewFile provides function to create new file by default template. For
// example:
//
//    xlsx := NewFile()
//
func NewFile() *File {
	file := make(map[string][]byte)
	file["_rels/.rels"] = []byte(XMLHeader + templateRels)
	file["docProps/app.xml"] = []byte(XMLHeader + templateDocpropsApp)
	file["docProps/core.xml"] = []byte(XMLHeader + templateDocpropsCore)
	file["xl/_rels/workbook.xml.rels"] = []byte(XMLHeader + templateWorkbookRels)
	file["xl/theme/theme1.xml"] = []byte(XMLHeader + templateTheme)
	file["xl/worksheets/sheet1.xml"] = []byte(XMLHeader + templateSheet)
	file["xl/styles.xml"] = []byte(XMLHeader + templateStyles)
	file["xl/workbook.xml"] = []byte(XMLHeader + templateWorkbook)
	file["[Content_Types].xml"] = []byte(XMLHeader + templateContentTypes)
	f := &File{
		sheetMap:   make(map[string]string),
		Sheet:      make(map[string]*xlsxWorksheet),
		SheetCount: 1,
		XLSX:       file,
	}
	f.ContentTypes = f.contentTypesReader()
	f.Styles = f.stylesReader()
	f.WorkBook = f.workbookReader()
	f.WorkBookRels = f.workbookRelsReader()
	f.Sheet["xl/worksheets/sheet1.xml"] = f.workSheetReader("Sheet1")
	f.sheetMap["Sheet1"] = "xl/worksheets/sheet1.xml"
	return f
}

// Save provides function to override the xlsx file with origin path.
func (f *File) Save() error {
	if f.Path == "" {
		return fmt.Errorf("No path defined for file, consider File.WriteTo or File.Write")
	}
	return f.SaveAs(f.Path)
}

// SaveAs provides function to create or update to an xlsx file at the provided
// path.
func (f *File) SaveAs(name string) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	return f.Write(file)
}

// Write provides function to write to an io.Writer.
func (f *File) Write(w io.Writer) error {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	f.contentTypesWriter()
	f.workbookWriter()
	f.workbookRelsWriter()
	f.worksheetWriter()
	f.styleSheetWriter()
	for path, content := range f.XLSX {
		fi, err := zw.Create(path)
		if err != nil {
			return err
		}
		_, err = fi.Write(content)
		if err != nil {
			return err
		}
	}
	err := zw.Close()
	if err != nil {
		return err
	}

	if _, err := buf.WriteTo(w); err != nil {
		return err
	}

	return nil
}
