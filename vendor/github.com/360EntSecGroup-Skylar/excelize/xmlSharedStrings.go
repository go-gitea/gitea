package excelize

import "encoding/xml"

// xlsxSST directly maps the sst element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main. String values may
// be stored directly inside spreadsheet cell elements; however, storing the
// same value inside multiple cell elements can result in very large worksheet
// Parts, possibly resulting in performance degradation. The Shared String Table
// is an indexed list of string values, shared across the workbook, which allows
// implementations to store values only once.
type xlsxSST struct {
	XMLName     xml.Name `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main sst"`
	Count       int      `xml:"count,attr"`
	UniqueCount int      `xml:"uniqueCount,attr"`
	SI          []xlsxSI `xml:"si"`
}

// xlsxSI directly maps the si element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked this for completeness - it does as much as I need.
type xlsxSI struct {
	T string  `xml:"t"`
	R []xlsxR `xml:"r"`
}

// xlsxR directly maps the r element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked this for completeness - it does as much as I need.
type xlsxR struct {
	RPr *xlsxRPr `xml:"rPr"`
	T   string   `xml:"t"`
}

// xlsxRPr (Run Properties) specifies a set of run properties which shall be
// applied to the contents of the parent run after all style formatting has been
// applied to the text. These properties are defined as direct formatting, since
// they are directly applied to the run and supersede any formatting from
// styles.
type xlsxRPr struct {
	B      string         `xml:"b,omitempty"`
	Sz     *attrValFloat  `xml:"sz"`
	Color  *xlsxColor     `xml:"color"`
	RFont  *attrValString `xml:"rFont"`
	Family *attrValInt    `xml:"family"`
}
