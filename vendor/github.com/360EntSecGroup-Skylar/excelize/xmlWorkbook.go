package excelize

import "encoding/xml"

// xmlxWorkbookRels contains xmlxWorkbookRelations which maps sheet id and sheet XML.
type xlsxWorkbookRels struct {
	XMLName       xml.Name               `xml:"http://schemas.openxmlformats.org/package/2006/relationships Relationships"`
	Relationships []xlsxWorkbookRelation `xml:"Relationship"`
}

// xmlxWorkbookRelation maps sheet id and xl/worksheets/_rels/sheet%d.xml.rels
type xlsxWorkbookRelation struct {
	ID         string `xml:"Id,attr"`
	Target     string `xml:",attr"`
	Type       string `xml:",attr"`
	TargetMode string `xml:",attr,omitempty"`
}

// xlsxWorkbook directly maps the workbook element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxWorkbook struct {
	XMLName             xml.Name                 `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main workbook"`
	FileVersion         *xlsxFileVersion         `xml:"fileVersion"`
	WorkbookPr          *xlsxWorkbookPr          `xml:"workbookPr"`
	WorkbookProtection  *xlsxWorkbookProtection  `xml:"workbookProtection"`
	BookViews           xlsxBookViews            `xml:"bookViews"`
	Sheets              xlsxSheets               `xml:"sheets"`
	ExternalReferences  *xlsxExternalReferences  `xml:"externalReferences"`
	DefinedNames        *xlsxDefinedNames        `xml:"definedNames"`
	CalcPr              *xlsxCalcPr              `xml:"calcPr"`
	CustomWorkbookViews *xlsxCustomWorkbookViews `xml:"customWorkbookViews"`
	PivotCaches         *xlsxPivotCaches         `xml:"pivotCaches"`
	ExtLst              *xlsxExtLst              `xml:"extLst"`
	FileRecoveryPr      *xlsxFileRecoveryPr      `xml:"fileRecoveryPr"`
}

// xlsxFileRecoveryPr maps sheet recovery information. This element defines
// properties that track the state of the workbook file, such as whether the
// file was saved during a crash, or whether it should be opened in auto-recover
// mode.
type xlsxFileRecoveryPr struct {
	AutoRecover     bool `xml:"autoRecover,attr,omitempty"`
	CrashSave       bool `xml:"crashSave,attr,omitempty"`
	DataExtractLoad bool `xml:"dataExtractLoad,attr,omitempty"`
	RepairLoad      bool `xml:"repairLoad,attr,omitempty"`
}

// xlsxWorkbookProtection directly maps the workbookProtection element. This
// element specifies options for protecting data in the workbook. Applications
// might use workbook protection to prevent anyone from accidentally changing,
// moving, or deleting important data. This protection can be ignored by
// applications which choose not to support this optional protection mechanism.
// When a password is to be hashed and stored in this element, it shall be
// hashed as defined below, starting from a UTF-16LE encoded string value. If
// there is a leading BOM character (U+FEFF) in the encoded password it is
// removed before hash calculation.
type xlsxWorkbookProtection struct {
	LockRevision           bool   `xml:"lockRevision,attr,omitempty"`
	LockStructure          bool   `xml:"lockStructure,attr,omitempty"`
	LockWindows            bool   `xml:"lockWindows,attr,omitempty"`
	RevisionsAlgorithmName string `xml:"revisionsAlgorithmName,attr,omitempty"`
	RevisionsHashValue     string `xml:"revisionsHashValue,attr,omitempty"`
	RevisionsSaltValue     string `xml:"revisionsSaltValue,attr,omitempty"`
	RevisionsSpinCount     int    `xml:"revisionsSpinCount,attr,omitempty"`
	WorkbookAlgorithmName  string `xml:"workbookAlgorithmName,attr,omitempty"`
	WorkbookHashValue      string `xml:"workbookHashValue,attr,omitempty"`
	WorkbookSaltValue      string `xml:"workbookSaltValue,attr,omitempty"`
	WorkbookSpinCount      int    `xml:"workbookSpinCount,attr,omitempty"`
}

// xlsxFileVersion directly maps the fileVersion element. This element defines
// properties that track which version of the application accessed the data and
// source code contained in the file.
type xlsxFileVersion struct {
	AppName      string `xml:"appName,attr,omitempty"`
	CodeName     string `xml:"codeName,attr,omitempty"`
	LastEdited   string `xml:"lastEdited,attr,omitempty"`
	LowestEdited string `xml:"lowestEdited,attr,omitempty"`
	RupBuild     string `xml:"rupBuild,attr,omitempty"`
}

// xlsxWorkbookPr directly maps the workbookPr element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main This element
// defines a collection of workbook properties.
type xlsxWorkbookPr struct {
	AllowRefreshQuery          bool   `xml:"allowRefreshQuery,attr,omitempty"`
	AutoCompressPictures       bool   `xml:"autoCompressPictures,attr,omitempty"`
	BackupFile                 bool   `xml:"backupFile,attr,omitempty"`
	CheckCompatibility         bool   `xml:"checkCompatibility,attr,omitempty"`
	CodeName                   string `xml:"codeName,attr,omitempty"`
	Date1904                   bool   `xml:"date1904,attr,omitempty"`
	DefaultThemeVersion        string `xml:"defaultThemeVersion,attr,omitempty"`
	FilterPrivacy              bool   `xml:"filterPrivacy,attr,omitempty"`
	HidePivotFieldList         bool   `xml:"hidePivotFieldList,attr,omitempty"`
	PromptedSolutions          bool   `xml:"promptedSolutions,attr,omitempty"`
	PublishItems               bool   `xml:"publishItems,attr,omitempty"`
	RefreshAllConnections      bool   `xml:"refreshAllConnections,attr,omitempty"`
	SaveExternalLinkValues     bool   `xml:"saveExternalLinkValues,attr,omitempty"`
	ShowBorderUnselectedTables bool   `xml:"showBorderUnselectedTables,attr,omitempty"`
	ShowInkAnnotation          bool   `xml:"showInkAnnotation,attr,omitempty"`
	ShowObjects                string `xml:"showObjects,attr,omitempty"`
	ShowPivotChartFilter       bool   `xml:"showPivotChartFilter,attr,omitempty"`
	UpdateLinks                string `xml:"updateLinks,attr,omitempty"`
}

// xlsxBookViews directly maps the bookViews element. This element specifies the
// collection of workbook views of the enclosing workbook. Each view can specify
// a window position, filter options, and other configurations. There is no
// limit on the number of workbook views that can be defined for a workbook.
type xlsxBookViews struct {
	WorkBookView []xlsxWorkBookView `xml:"workbookView"`
}

// xlsxWorkBookView directly maps the workbookView element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main This element
// specifies a single Workbook view.
type xlsxWorkBookView struct {
	ActiveTab              int    `xml:"activeTab,attr,omitempty"`
	AutoFilterDateGrouping bool   `xml:"autoFilterDateGrouping,attr,omitempty"`
	FirstSheet             int    `xml:"firstSheet,attr,omitempty"`
	Minimized              bool   `xml:"minimized,attr,omitempty"`
	ShowHorizontalScroll   bool   `xml:"showHorizontalScroll,attr,omitempty"`
	ShowSheetTabs          bool   `xml:"showSheetTabs,attr,omitempty"`
	ShowVerticalScroll     bool   `xml:"showVerticalScroll,attr,omitempty"`
	TabRatio               int    `xml:"tabRatio,attr,omitempty"`
	Visibility             string `xml:"visibility,attr,omitempty"`
	WindowHeight           int    `xml:"windowHeight,attr,omitempty"`
	WindowWidth            int    `xml:"windowWidth,attr,omitempty"`
	XWindow                string `xml:"xWindow,attr,omitempty"`
	YWindow                string `xml:"yWindow,attr,omitempty"`
}

// xlsxSheets directly maps the sheets element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main.
type xlsxSheets struct {
	Sheet []xlsxSheet `xml:"sheet"`
}

// xlsxSheet directly maps the sheet element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxSheet struct {
	Name    string `xml:"name,attr,omitempty"`
	SheetID string `xml:"sheetId,attr,omitempty"`
	ID      string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	State   string `xml:"state,attr,omitempty"`
}

// xlsxExternalReferences directly maps the externalReferences element of the
// external workbook references part.
type xlsxExternalReferences struct {
	ExternalReference []xlsxExternalReference `xml:"externalReference"`
}

// xlsxExternalReference directly maps the externalReference element of the
// external workbook references part.
type xlsxExternalReference struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// xlsxPivotCaches element enumerates pivot cache definition parts used by pivot
// tables and formulas in this workbook.
type xlsxPivotCaches struct {
	PivotCache []xlsxPivotCache `xml:"pivotCache"`
}

// xlsxPivotCache directly maps the pivotCache element.
type xlsxPivotCache struct {
	CacheID int    `xml:"cacheId,attr,omitempty"`
	RID     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// extLst element provides a convention for extending spreadsheetML in
// predefined locations. The locations shall be denoted with the extLst element,
// and are called extension lists. Extension list locations within the markup
// document are specified in the markup specification and can be used to store
// extensions to the markup specification, whether those are future version
// extensions of the markup specification or are private extensions implemented
// independently from the markup specification. Markup within an extension might
// not be understood by a consumer.
type xlsxExtLst struct {
	Ext string `xml:",innerxml"`
}

// xlsxDefinedNames directly maps the definedNames element. This element defines
// the collection of defined names for this workbook. Defined names are
// descriptive names to represent cells, ranges of cells, formulas, or constant
// values. Defined names can be used to represent a range on any worksheet.
type xlsxDefinedNames struct {
	DefinedName []xlsxDefinedName `xml:"definedName"`
}

// xlsxDefinedName directly maps the definedName element from the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main This element
// defines a defined name within this workbook. A defined name is descriptive
// text that is used to represents a cell, range of cells, formula, or constant
// value. For a descriptions of the attributes see https://msdn.microsoft.com/en-us/library/office/documentformat.openxml.spreadsheet.definedname.aspx
type xlsxDefinedName struct {
	Comment           string `xml:"comment,attr,omitempty"`
	CustomMenu        string `xml:"customMenu,attr,omitempty"`
	Description       string `xml:"description,attr,omitempty"`
	Function          bool   `xml:"function,attr,omitempty"`
	FunctionGroupID   int    `xml:"functionGroupId,attr,omitempty"`
	Help              string `xml:"help,attr,omitempty"`
	Hidden            bool   `xml:"hidden,attr,omitempty"`
	LocalSheetID      *int   `xml:"localSheetId,attr"`
	Name              string `xml:"name,attr,omitempty"`
	PublishToServer   bool   `xml:"publishToServer,attr,omitempty"`
	ShortcutKey       string `xml:"shortcutKey,attr,omitempty"`
	StatusBar         string `xml:"statusBar,attr,omitempty"`
	VbProcedure       bool   `xml:"vbProcedure,attr,omitempty"`
	WorkbookParameter bool   `xml:"workbookParameter,attr,omitempty"`
	Xlm               bool   `xml:"xml,attr,omitempty"`
	Data              string `xml:",chardata"`
}

// xlsxCalcPr directly maps the calcPr element. This element defines the
// collection of properties the application uses to record calculation status
// and details. Calculation is the process of computing formulas and then
// displaying the results as values in the cells that contain the formulas.
type xlsxCalcPr struct {
	CalcCompleted         bool    `xml:"calcCompleted,attr,omitempty"`
	CalcID                string  `xml:"calcId,attr,omitempty"`
	CalcMode              string  `xml:"calcMode,attr,omitempty"`
	CalcOnSave            bool    `xml:"calcOnSave,attr,omitempty"`
	ConcurrentCalc        *bool   `xml:"concurrentCalc,attr"`
	ConcurrentManualCount int     `xml:"concurrentManualCount,attr,omitempty"`
	ForceFullCalc         bool    `xml:"forceFullCalc,attr,omitempty"`
	FullCalcOnLoad        bool    `xml:"fullCalcOnLoad,attr,omitempty"`
	FullPrecision         bool    `xml:"fullPrecision,attr,omitempty"`
	Iterate               bool    `xml:"iterate,attr,omitempty"`
	IterateCount          int     `xml:"iterateCount,attr,omitempty"`
	IterateDelta          float64 `xml:"iterateDelta,attr,omitempty"`
	RefMode               string  `xml:"refMode,attr,omitempty"`
}

// xlsxCustomWorkbookViews defines the collection of custom workbook views that
// are defined for this workbook. A customWorkbookView is similar in concept to
// a workbookView in that its attributes contain settings related to the way
// that the workbook should be displayed on a screen by a spreadsheet
// application.
type xlsxCustomWorkbookViews struct {
	CustomWorkbookView []xlsxCustomWorkbookView `xml:"customWorkbookView"`
}

// xlsxCustomWorkbookView directly maps the customWorkbookView element. This
// element specifies a single custom workbook view. A custom workbook view
// consists of a set of display and print settings that you can name and apply
// to a workbook. You can create more than one custom workbook view of the same
// workbook. Custom Workbook Views are not required in order to construct a
// valid SpreadsheetML document, and are not necessary if the document is never
// displayed by a spreadsheet application, or if the spreadsheet application has
// a fixed display for workbooks. However, if a spreadsheet application chooses
// to implement configurable display modes, the customWorkbookView element
// should be used to persist the settings for those display modes.
type xlsxCustomWorkbookView struct {
	ActiveSheetID        *int    `xml:"activeSheetId,attr"`
	AutoUpdate           *bool   `xml:"autoUpdate,attr"`
	ChangesSavedWin      *bool   `xml:"changesSavedWin,attr"`
	GUID                 *string `xml:"guid,attr"`
	IncludeHiddenRowCol  *bool   `xml:"includeHiddenRowCol,attr"`
	IncludePrintSettings *bool   `xml:"includePrintSettings,attr"`
	Maximized            *bool   `xml:"maximized,attr"`
	MergeInterval        int     `xml:"mergeInterval,attr"`
	Minimized            *bool   `xml:"minimized,attr"`
	Name                 *string `xml:"name,attr"`
	OnlySync             *bool   `xml:"onlySync,attr"`
	PersonalView         *bool   `xml:"personalView,attr"`
	ShowComments         *string `xml:"showComments,attr"`
	ShowFormulaBar       *bool   `xml:"showFormulaBar,attr"`
	ShowHorizontalScroll *bool   `xml:"showHorizontalScroll,attr"`
	ShowObjects          *string `xml:"showObjects,attr"`
	ShowSheetTabs        *bool   `xml:"showSheetTabs,attr"`
	ShowStatusbar        *bool   `xml:"showStatusbar,attr"`
	ShowVerticalScroll   *bool   `xml:"showVerticalScroll,attr"`
	TabRatio             *int    `xml:"tabRatio,attr"`
	WindowHeight         *int    `xml:"windowHeight,attr"`
	WindowWidth          *int    `xml:"windowWidth,attr"`
	XWindow              *int    `xml:"xWindow,attr"`
	YWindow              *int    `xml:"yWindow,attr"`
}
