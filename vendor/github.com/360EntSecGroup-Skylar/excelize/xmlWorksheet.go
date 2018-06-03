package excelize

import "encoding/xml"

// xlsxWorksheet directly maps the worksheet element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxWorksheet struct {
	XMLName               xml.Name                     `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main worksheet"`
	SheetPr               *xlsxSheetPr                 `xml:"sheetPr"`
	Dimension             xlsxDimension                `xml:"dimension"`
	SheetViews            xlsxSheetViews               `xml:"sheetViews,omitempty"`
	SheetFormatPr         *xlsxSheetFormatPr           `xml:"sheetFormatPr"`
	Cols                  *xlsxCols                    `xml:"cols,omitempty"`
	SheetData             xlsxSheetData                `xml:"sheetData"`
	SheetProtection       *xlsxSheetProtection         `xml:"sheetProtection"`
	AutoFilter            *xlsxAutoFilter              `xml:"autoFilter"`
	MergeCells            *xlsxMergeCells              `xml:"mergeCells"`
	PhoneticPr            *xlsxPhoneticPr              `xml:"phoneticPr"`
	ConditionalFormatting []*xlsxConditionalFormatting `xml:"conditionalFormatting"`
	DataValidations       *xlsxDataValidations         `xml:"dataValidations"`
	Hyperlinks            *xlsxHyperlinks              `xml:"hyperlinks"`
	PrintOptions          *xlsxPrintOptions            `xml:"printOptions"`
	PageMargins           *xlsxPageMargins             `xml:"pageMargins"`
	PageSetUp             *xlsxPageSetUp               `xml:"pageSetup"`
	HeaderFooter          *xlsxHeaderFooter            `xml:"headerFooter"`
	Drawing               *xlsxDrawing                 `xml:"drawing"`
	LegacyDrawing         *xlsxLegacyDrawing           `xml:"legacyDrawing"`
	Picture               *xlsxPicture                 `xml:"picture"`
	TableParts            *xlsxTableParts              `xml:"tableParts"`
	ExtLst                *xlsxExtLst                  `xml:"extLst"`
}

// xlsxDrawing change r:id to rid in the namespace.
type xlsxDrawing struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// xlsxHeaderFooter directly maps the headerFooter element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - When printed or
// viewed in page layout view (§18.18.69), each page of a worksheet can have a
// page header, a page footer, or both. The headers and footers on odd-numbered
// pages can differ from those on even-numbered pages, and the headers and
// footers on the first page can differ from those on odd- and even-numbered
// pages. In the latter case, the first page is not considered an odd page.
type xlsxHeaderFooter struct {
	DifferentFirst   bool             `xml:"differentFirst,attr,omitempty"`
	DifferentOddEven bool             `xml:"differentOddEven,attr,omitempty"`
	OddHeader        []*xlsxOddHeader `xml:"oddHeader"`
	OddFooter        []*xlsxOddFooter `xml:"oddFooter"`
}

// xlsxOddHeader directly maps the oddHeader element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxOddHeader struct {
	Content string `xml:",chardata"`
}

// xlsxOddFooter directly maps the oddFooter element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxOddFooter struct {
	Content string `xml:",chardata"`
}

// xlsxPageSetUp directly maps the pageSetup element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - Page setup
// settings for the worksheet.
type xlsxPageSetUp struct {
	BlackAndWhite      bool    `xml:"blackAndWhite,attr,omitempty"`
	CellComments       string  `xml:"cellComments,attr,omitempty"`
	Copies             int     `xml:"copies,attr,omitempty"`
	Draft              bool    `xml:"draft,attr,omitempty"`
	Errors             string  `xml:"errors,attr,omitempty"`
	FirstPageNumber    int     `xml:"firstPageNumber,attr,omitempty"`
	FitToHeight        *int    `xml:"fitToHeight,attr"`
	FitToWidth         int     `xml:"fitToWidth,attr,omitempty"`
	HorizontalDPI      float32 `xml:"horizontalDpi,attr,omitempty"`
	RID                string  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	Orientation        string  `xml:"orientation,attr,omitempty"`
	PageOrder          string  `xml:"pageOrder,attr,omitempty"`
	PaperHeight        string  `xml:"paperHeight,attr,omitempty"`
	PaperSize          string  `xml:"paperSize,attr,omitempty"`
	PaperWidth         string  `xml:"paperWidth,attr,omitempty"`
	Scale              int     `xml:"scale,attr,omitempty"`
	UseFirstPageNumber bool    `xml:"useFirstPageNumber,attr,omitempty"`
	UsePrinterDefaults bool    `xml:"usePrinterDefaults,attr,omitempty"`
	VerticalDPI        float32 `xml:"verticalDpi,attr,omitempty"`
}

// xlsxPrintOptions directly maps the printOptions element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - Print options for
// the sheet. Printer-specific settings are stored separately in the Printer
// Settings part.
type xlsxPrintOptions struct {
	GridLines          bool `xml:"gridLines,attr,omitempty"`
	GridLinesSet       bool `xml:"gridLinesSet,attr,omitempty"`
	Headings           bool `xml:"headings,attr,omitempty"`
	HorizontalCentered bool `xml:"horizontalCentered,attr,omitempty"`
	VerticalCentered   bool `xml:"verticalCentered,attr,omitempty"`
}

// xlsxPageMargins directly maps the pageMargins element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - Page margins for
// a sheet or a custom sheet view.
type xlsxPageMargins struct {
	Bottom float64 `xml:"bottom,attr"`
	Footer float64 `xml:"footer,attr"`
	Header float64 `xml:"header,attr"`
	Left   float64 `xml:"left,attr"`
	Right  float64 `xml:"right,attr"`
	Top    float64 `xml:"top,attr"`
}

// xlsxSheetFormatPr directly maps the sheetFormatPr element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main. This element
// specifies the sheet formatting properties.
type xlsxSheetFormatPr struct {
	BaseColWidth     uint8   `xml:"baseColWidth,attr,omitempty"`
	DefaultColWidth  float64 `xml:"defaultColWidth,attr,omitempty"`
	DefaultRowHeight float64 `xml:"defaultRowHeight,attr"`
	CustomHeight     bool    `xml:"customHeight,attr,omitempty"`
	ZeroHeight       bool    `xml:"zeroHeight,attr,omitempty"`
	ThickTop         bool    `xml:"thickTop,attr,omitempty"`
	ThickBottom      bool    `xml:"thickBottom,attr,omitempty"`
	OutlineLevelRow  uint8   `xml:"outlineLevelRow,attr,omitempty"`
	OutlineLevelCol  uint8   `xml:"outlineLevelCol,attr,omitempty"`
}

// xlsxSheetViews directly maps the sheetViews element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - Worksheet views
// collection.
type xlsxSheetViews struct {
	SheetView []xlsxSheetView `xml:"sheetView"`
}

// xlsxSheetView directly maps the sheetView element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need. A single sheet
// view definition. When more than one sheet view is defined in the file, it
// means that when opening the workbook, each sheet view corresponds to a
// separate window within the spreadsheet application, where each window is
// showing the particular sheet containing the same workbookViewId value, the
// last sheetView definition is loaded, and the others are discarded. When
// multiple windows are viewing the same sheet, multiple sheetView elements
// (with corresponding workbookView entries) are saved.
// See https://msdn.microsoft.com/en-us/library/office/documentformat.openxml.spreadsheet.sheetview.aspx
type xlsxSheetView struct {
	WindowProtection         bool             `xml:"windowProtection,attr,omitempty"`
	ShowFormulas             bool             `xml:"showFormulas,attr,omitempty"`
	ShowGridLines            *bool            `xml:"showGridLines,attr"`
	ShowRowColHeaders        *bool            `xml:"showRowColHeaders,attr"`
	ShowZeros                bool             `xml:"showZeros,attr,omitempty"`
	RightToLeft              bool             `xml:"rightToLeft,attr,omitempty"`
	TabSelected              bool             `xml:"tabSelected,attr,omitempty"`
	ShowWhiteSpace           *bool            `xml:"showWhiteSpace,attr"`
	ShowOutlineSymbols       bool             `xml:"showOutlineSymbols,attr,omitempty"`
	DefaultGridColor         *bool            `xml:"defaultGridColor,attr"`
	View                     string           `xml:"view,attr,omitempty"`
	TopLeftCell              string           `xml:"topLeftCell,attr,omitempty"`
	ColorID                  int              `xml:"colorId,attr,omitempty"`
	ZoomScale                float64          `xml:"zoomScale,attr,omitempty"`
	ZoomScaleNormal          float64          `xml:"zoomScaleNormal,attr,omitempty"`
	ZoomScalePageLayoutView  float64          `xml:"zoomScalePageLayoutView,attr,omitempty"`
	ZoomScaleSheetLayoutView float64          `xml:"zoomScaleSheetLayoutView,attr,omitempty"`
	WorkbookViewID           int              `xml:"workbookViewId,attr"`
	Pane                     *xlsxPane        `xml:"pane,omitempty"`
	Selection                []*xlsxSelection `xml:"selection"`
}

// xlsxSelection directly maps the selection element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - Worksheet view
// selection.
type xlsxSelection struct {
	ActiveCell   string `xml:"activeCell,attr,omitempty"`
	ActiveCellID *int   `xml:"activeCellId,attr"`
	Pane         string `xml:"pane,attr,omitempty"`
	SQRef        string `xml:"sqref,attr,omitempty"`
}

// xlsxSelection directly maps the selection element. Worksheet view pane.
type xlsxPane struct {
	ActivePane  string  `xml:"activePane,attr,omitempty"`
	State       string  `xml:"state,attr,omitempty"` // Either "split" or "frozen"
	TopLeftCell string  `xml:"topLeftCell,attr,omitempty"`
	XSplit      float64 `xml:"xSplit,attr,omitempty"`
	YSplit      float64 `xml:"ySplit,attr,omitempty"`
}

// xlsxSheetPr directly maps the sheetPr element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - Sheet-level
// properties.
type xlsxSheetPr struct {
	XMLName                           xml.Name         `xml:"sheetPr"`
	CodeName                          string           `xml:"codeName,attr,omitempty"`
	EnableFormatConditionsCalculation *bool            `xml:"enableFormatConditionsCalculation,attr"`
	FilterMode                        bool             `xml:"filterMode,attr,omitempty"`
	Published                         *bool            `xml:"published,attr"`
	SyncHorizontal                    bool             `xml:"syncHorizontal,attr,omitempty"`
	SyncVertical                      bool             `xml:"syncVertical,attr,omitempty"`
	TransitionEntry                   bool             `xml:"transitionEntry,attr,omitempty"`
	TabColor                          *xlsxTabColor    `xml:"tabColor,omitempty"`
	PageSetUpPr                       *xlsxPageSetUpPr `xml:"pageSetUpPr,omitempty"`
}

// xlsxPageSetUpPr directly maps the pageSetupPr element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - Page setup
// properties of the worksheet.
type xlsxPageSetUpPr struct {
	AutoPageBreaks bool `xml:"autoPageBreaks,attr,omitempty"`
	FitToPage      bool `xml:"fitToPage,attr,omitempty"` // Flag indicating whether the Fit to Page print option is enabled.
}

// xlsxTabColor directly maps the tabColor element in the namespace currently I
// have not checked it for completeness - it does as much as I need.
type xlsxTabColor struct {
	Theme int     `xml:"theme,attr,omitempty"`
	Tint  float64 `xml:"tint,attr,omitempty"`
}

// xlsxCols directly maps the cols element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxCols struct {
	Col []xlsxCol `xml:"col"`
}

// xlsxCol directly maps the col (Column Width & Formatting). Defines column
// width and column formatting for one or more columns of the worksheet.
type xlsxCol struct {
	BestFit      bool    `xml:"bestFit,attr,omitempty"`
	Collapsed    bool    `xml:"collapsed,attr"`
	CustomWidth  bool    `xml:"customWidth,attr,omitempty"`
	Hidden       bool    `xml:"hidden,attr"`
	Max          int     `xml:"max,attr"`
	Min          int     `xml:"min,attr"`
	OutlineLevel uint8   `xml:"outlineLevel,attr,omitempty"`
	Phonetic     bool    `xml:"phonetic,attr,omitempty"`
	Style        int     `xml:"style,attr"`
	Width        float64 `xml:"width,attr"`
}

// xlsxDimension directly maps the dimension element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - This element
// specifies the used range of the worksheet. It specifies the row and column
// bounds of used cells in the worksheet. This is optional and is not required.
// Used cells include cells with formulas, text content, and cell formatting.
// When an entire column is formatted, only the first cell in that column is
// considered used.
type xlsxDimension struct {
	Ref string `xml:"ref,attr"`
}

// xlsxSheetData directly maps the sheetData element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxSheetData struct {
	XMLName xml.Name  `xml:"sheetData"`
	Row     []xlsxRow `xml:"row"`
}

// xlsxRow directly maps the row element. The element expresses information
// about an entire row of a worksheet, and contains all cell definitions for a
// particular row in the worksheet.
type xlsxRow struct {
	Collapsed    bool    `xml:"collapsed,attr,omitempty"`
	CustomFormat bool    `xml:"customFormat,attr,omitempty"`
	CustomHeight bool    `xml:"customHeight,attr,omitempty"`
	Hidden       bool    `xml:"hidden,attr,omitempty"`
	Ht           float64 `xml:"ht,attr,omitempty"`
	OutlineLevel uint8   `xml:"outlineLevel,attr,omitempty"`
	Ph           bool    `xml:"ph,attr,omitempty"`
	R            int     `xml:"r,attr,omitempty"`
	S            int     `xml:"s,attr,omitempty"`
	Spans        string  `xml:"spans,attr,omitempty"`
	ThickBot     bool    `xml:"thickBot,attr,omitempty"`
	ThickTop     bool    `xml:"thickTop,attr,omitempty"`
	C            []xlsxC `xml:"c"`
}

// xlsxMergeCell directly maps the mergeCell element. A single merged cell.
type xlsxMergeCell struct {
	Ref string `xml:"ref,attr,omitempty"`
}

// xlsxMergeCells directly maps the mergeCells element. This collection
// expresses all the merged cells in the sheet.
type xlsxMergeCells struct {
	Count int              `xml:"count,attr,omitempty"`
	Cells []*xlsxMergeCell `xml:"mergeCell,omitempty"`
}

// xlsxDataValidations expresses all data validation information for cells in a
// sheet which have data validation features applied.
type xlsxDataValidations struct {
	Count          int    `xml:"count,attr,omitempty"`
	DisablePrompts bool   `xml:"disablePrompts,attr,omitempty"`
	XWindow        int    `xml:"xWindow,attr,omitempty"`
	YWindow        int    `xml:"yWindow,attr,omitempty"`
	DataValidation string `xml:",innerxml"`
}

// xlsxC directly maps the c element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
//
// This simple type is restricted to the values listed in the following table:
//
//      Enumeration Value         | Description
//     ---------------------------+---------------------------------
//      b (Boolean)               | Cell containing a boolean.
//      d (Date)                  | Cell contains a date in the ISO 8601 format.
//      e (Error)                 | Cell containing an error.
//      inlineStr (Inline String) | Cell containing an (inline) rich string, i.e., one not in the shared string table. If this cell type is used, then the cell value is in the is element rather than the v element in the cell (c element).
//      n (Number)                | Cell containing a number.
//      s (Shared String)         | Cell containing a shared string.
//      str (String)              | Cell containing a formula string.
//
type xlsxC struct {
	R string `xml:"r,attr"`           // Cell ID, e.g. A1
	S int    `xml:"s,attr,omitempty"` // Style reference.
	// Str string `xml:"str,attr,omitempty"` // Style reference.
	T        string   `xml:"t,attr,omitempty"` // Type.
	F        *xlsxF   `xml:"f,omitempty"`      // Formula
	V        string   `xml:"v,omitempty"`      // Value
	IS       *xlsxIS  `xml:"is"`
	XMLSpace xml.Attr `xml:"space,attr,omitempty"`
}

// xlsxIS directly maps the t element. Cell containing an (inline) rich
// string, i.e., one not in the shared string table. If this cell type is
// used, then the cell value is in the is element rather than the v element in
// the cell (c element).
type xlsxIS struct {
	T string `xml:"t"`
}

// xlsxF directly maps the f element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxF struct {
	Content string `xml:",chardata"`
	T       string `xml:"t,attr,omitempty"`   // Formula type
	Ref     string `xml:"ref,attr,omitempty"` // Shared formula ref
	Si      string `xml:"si,attr,omitempty"`  // Shared formula index
}

// xlsxSheetProtection collection expresses the sheet protection options to
// enforce when the sheet is protected.
type xlsxSheetProtection struct {
	AlgorithmName      string `xml:"algorithmName,attr,omitempty"`
	AutoFilter         int    `xml:"autoFilter,attr,omitempty"`
	DeleteColumns      int    `xml:"deleteColumns,attr,omitempty"`
	DeleteRows         int    `xml:"deleteRows,attr,omitempty"`
	FormatCells        int    `xml:"formatCells,attr,omitempty"`
	FormatColumns      int    `xml:"formatColumns,attr,omitempty"`
	FormatRows         int    `xml:"formatRows,attr,omitempty"`
	HashValue          string `xml:"hashValue,attr,omitempty"`
	InsertColumns      int    `xml:"insertColumns,attr,omitempty"`
	InsertHyperlinks   int    `xml:"insertHyperlinks,attr,omitempty"`
	InsertRows         int    `xml:"insertRows,attr,omitempty"`
	Objects            int    `xml:"objects,attr,omitempty"`
	PivotTables        int    `xml:"pivotTables,attr,omitempty"`
	SaltValue          string `xml:"saltValue,attr,omitempty"`
	Scenarios          int    `xml:"scenarios,attr,omitempty"`
	SelectLockedCells  int    `xml:"selectLockedCells,attr,omitempty"`
	SelectUnlockedCell int    `xml:"selectUnlockedCell,attr,omitempty"`
	Sheet              int    `xml:"sheet,attr,omitempty"`
	Sort               int    `xml:"sort,attr,omitempty"`
	SpinCount          int    `xml:"spinCount,attr,omitempty"`
}

// xlsxPhoneticPr (Phonetic Properties) represents a collection of phonetic
// properties that affect the display of phonetic text for this String Item
// (si). Phonetic text is used to give hints as to the pronunciation of an East
// Asian language, and the hints are displayed as text within the spreadsheet
// cells across the top portion of the cell. Since the phonetic hints are text,
// every phonetic hint is expressed as a phonetic run (rPh), and these
// properties specify how to display that phonetic run.
type xlsxPhoneticPr struct {
	Alignment string `xml:"alignment,attr,omitempty"`
	FontID    *int   `xml:"fontId,attr"`
	Type      string `xml:"type,attr,omitempty"`
}

// A Conditional Format is a format, such as cell shading or font color, that a
// spreadsheet application can automatically apply to cells if a specified
// condition is true. This collection expresses conditional formatting rules
// applied to a particular cell or range.
type xlsxConditionalFormatting struct {
	SQRef  string        `xml:"sqref,attr,omitempty"`
	CfRule []*xlsxCfRule `xml:"cfRule"`
}

// xlsxCfRule (Conditional Formatting Rule) represents a description of a
// conditional formatting rule.
type xlsxCfRule struct {
	AboveAverage *bool           `xml:"aboveAverage,attr"`
	Bottom       bool            `xml:"bottom,attr,omitempty"`
	DxfID        *int            `xml:"dxfId,attr"`
	EqualAverage bool            `xml:"equalAverage,attr,omitempty"`
	Operator     string          `xml:"operator,attr,omitempty"`
	Percent      bool            `xml:"percent,attr,omitempty"`
	Priority     int             `xml:"priority,attr,omitempty"`
	Rank         int             `xml:"rank,attr,omitempty"`
	StdDev       int             `xml:"stdDev,attr,omitempty"`
	StopIfTrue   bool            `xml:"stopIfTrue,attr,omitempty"`
	Text         string          `xml:"text,attr,omitempty"`
	TimePeriod   string          `xml:"timePeriod,attr,omitempty"`
	Type         string          `xml:"type,attr,omitempty"`
	Formula      []string        `xml:"formula,omitempty"`
	ColorScale   *xlsxColorScale `xml:"colorScale"`
	DataBar      *xlsxDataBar    `xml:"dataBar"`
	IconSet      *xlsxIconSet    `xml:"iconSet"`
	ExtLst       *xlsxExtLst     `xml:"extLst"`
}

// xlsxColorScale (Color Scale) describes a gradated color scale in this
// conditional formatting rule.
type xlsxColorScale struct {
	Cfvo  []*xlsxCfvo  `xml:"cfvo"`
	Color []*xlsxColor `xml:"color"`
}

// dataBar (Data Bar) describes a data bar conditional formatting rule.
type xlsxDataBar struct {
	MaxLength int          `xml:"maxLength,attr,omitempty"`
	MinLength int          `xml:"minLength,attr,omitempty"`
	ShowValue bool         `xml:"showValue,attr,omitempty"`
	Cfvo      []*xlsxCfvo  `xml:"cfvo"`
	Color     []*xlsxColor `xml:"color"`
}

// xlsxIconSet (Icon Set) describes an icon set conditional formatting rule.
type xlsxIconSet struct {
	Cfvo      []*xlsxCfvo `xml:"cfvo"`
	IconSet   string      `xml:"iconSet,attr,omitempty"`
	ShowValue bool        `xml:"showValue,attr,omitempty"`
	Percent   bool        `xml:"percent,attr,omitempty"`
	Reverse   bool        `xml:"reverse,attr,omitempty"`
}

// cfvo (Conditional Format Value Object) describes the values of the
// interpolation points in a gradient scale.
type xlsxCfvo struct {
	Gte    bool        `xml:"gte,attr,omitempty"`
	Type   string      `xml:"type,attr,omitempty"`
	Val    int         `xml:"val,attr"`
	ExtLst *xlsxExtLst `xml:"extLst"`
}

// xlsxHyperlinks directly maps the hyperlinks element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - A hyperlink can
// be stored in a package as a relationship. Hyperlinks shall be identified by
// containing a target which specifies the destination of the given hyperlink.
type xlsxHyperlinks struct {
	Hyperlink []xlsxHyperlink `xml:"hyperlink"`
}

// xlsxHyperlink directly maps the hyperlink element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main
type xlsxHyperlink struct {
	Ref      string `xml:"ref,attr"`
	Location string `xml:"location,attr,omitempty"`
	Display  string `xml:"display,attr,omitempty"`
	RID      string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// xlsxTableParts directly maps the tableParts element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - The table element
// has several attributes applied to identify the table and the data range it
// covers. The table id attribute needs to be unique across all table parts, the
// same goes for the name and displayName. The displayName has the further
// restriction that it must be unique across all defined names in the workbook.
// Later on we will see that you can define names for many elements, such as
// cells or formulas. The name value is used for the object model in Microsoft
// Office Excel. The displayName is used for references in formulas. The ref
// attribute is used to identify the cell range that the table covers. This
// includes not only the table data, but also the table header containing column
// names.
// To add columns to your table you add new tableColumn elements to the
// tableColumns container. Similar to the shared string table the collection
// keeps a count attribute identifying the number of columns. Besides the table
// definition in the table part there is also the need to identify which tables
// are displayed in the worksheet. The worksheet part has a separate element
// tableParts to store this information. Each table part is referenced through
// the relationship ID and again a count of the number of table parts is
// maintained. The following markup sample is taken from the documents
// accompanying this book. The sheet data element has been removed to reduce the
// size of the sample. To reference the table, just add the tableParts element,
// of course after having created and stored the table part. For example:
//
//    <worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
//        ...
//        <tableParts count="1">
// 		      <tablePart r:id="rId1" />
//        </tableParts>
//    </worksheet>
//
type xlsxTableParts struct {
	Count      int              `xml:"count,attr,omitempty"`
	TableParts []*xlsxTablePart `xml:"tablePart"`
}

// xlsxTablePart directly maps the tablePart element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main
type xlsxTablePart struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// xlsxPicture directly maps the picture element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - Background sheet
// image. For example:
//
//    <picture r:id="rId1"/>
//
type xlsxPicture struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// xlsxLegacyDrawing directly maps the legacyDrawing element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - A comment is a
// rich text note that is attached to, and associated with, a cell, separate
// from other cell content. Comment content is stored separate from the cell,
// and is displayed in a drawing object (like a text box) that is separate from,
// but associated with, a cell. Comments are used as reminders, such as noting
// how a complex formula works, or to provide feedback to other users. Comments
// can also be used to explain assumptions made in a formula or to call out
// something special about the cell.
type xlsxLegacyDrawing struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// formatPanes directly maps the settings of the panes.
type formatPanes struct {
	Freeze      bool   `json:"freeze"`
	Split       bool   `json:"split"`
	XSplit      int    `json:"x_split"`
	YSplit      int    `json:"y_split"`
	TopLeftCell string `json:"top_left_cell"`
	ActivePane  string `json:"active_pane"`
	Panes       []struct {
		SQRef      string `json:"sqref"`
		ActiveCell string `json:"active_cell"`
		Pane       string `json:"pane"`
	} `json:"panes"`
}

// formatConditional directly maps the conditional format settings of the cells.
type formatConditional struct {
	Type         string `json:"type"`
	AboveAverage bool   `json:"above_average"`
	Percent      bool   `json:"percent"`
	Format       int    `json:"format"`
	Criteria     string `json:"criteria"`
	Value        string `json:"value,omitempty"`
	Minimum      string `json:"minimum,omitempty"`
	Maximum      string `json:"maximum,omitempty"`
	MinType      string `json:"min_type,omitempty"`
	MidType      string `json:"mid_type,omitempty"`
	MaxType      string `json:"max_type,omitempty"`
	MinValue     string `json:"min_value,omitempty"`
	MidValue     string `json:"mid_value,omitempty"`
	MaxValue     string `json:"max_value,omitempty"`
	MinColor     string `json:"min_color,omitempty"`
	MidColor     string `json:"mid_color,omitempty"`
	MaxColor     string `json:"max_color,omitempty"`
	MinLength    string `json:"min_length,omitempty"`
	MaxLength    string `json:"max_length,omitempty"`
	MultiRange   string `json:"multi_range,omitempty"`
	BarColor     string `json:"bar_color,omitempty"`
}
