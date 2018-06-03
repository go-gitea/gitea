package excelize

import "encoding/xml"

// xlsxTable directly maps the table element. A table helps organize and provide
// structure to lists of information in a worksheet. Tables have clearly labeled
// columns, rows, and data regions. Tables make it easier for users to sort,
// analyze, format, manage, add, and delete information. This element is the
// root element for a table that is not a single cell XML table.
type xlsxTable struct {
	XMLName              xml.Name            `xml:"table"`
	XMLNS                string              `xml:"xmlns,attr"`
	DataCellStyle        string              `xml:"dataCellStyle,attr,omitempty"`
	DataDxfID            int                 `xml:"dataDxfId,attr,omitempty"`
	DisplayName          string              `xml:"displayName,attr,omitempty"`
	HeaderRowBorderDxfID int                 `xml:"headerRowBorderDxfId,attr,omitempty"`
	HeaderRowCellStyle   string              `xml:"headerRowCellStyle,attr,omitempty"`
	HeaderRowCount       int                 `xml:"headerRowCount,attr,omitempty"`
	HeaderRowDxfID       int                 `xml:"headerRowDxfId,attr,omitempty"`
	ID                   int                 `xml:"id,attr"`
	InsertRow            bool                `xml:"insertRow,attr,omitempty"`
	InsertRowShift       bool                `xml:"insertRowShift,attr,omitempty"`
	Name                 string              `xml:"name,attr"`
	Published            bool                `xml:"published,attr,omitempty"`
	Ref                  string              `xml:"ref,attr"`
	TotalsRowCount       int                 `xml:"totalsRowCount,attr,omitempty"`
	TotalsRowDxfID       int                 `xml:"totalsRowDxfId,attr,omitempty"`
	TotalsRowShown       bool                `xml:"totalsRowShown,attr"`
	AutoFilter           *xlsxAutoFilter     `xml:"autoFilter"`
	TableColumns         *xlsxTableColumns   `xml:"tableColumns"`
	TableStyleInfo       *xlsxTableStyleInfo `xml:"tableStyleInfo"`
}

// xlsxAutoFilter temporarily hides rows based on a filter criteria, which is
// applied column by column to a table of data in the worksheet. This collection
// expresses AutoFilter settings.
type xlsxAutoFilter struct {
	Ref          string            `xml:"ref,attr"`
	FilterColumn *xlsxFilterColumn `xml:"filterColumn"`
}

// xlsxFilterColumn directly maps the filterColumn element. The filterColumn
// collection identifies a particular column in the AutoFilter range and
// specifies filter information that has been applied to this column. If a
// column in the AutoFilter range has no criteria specified, then there is no
// corresponding filterColumn collection expressed for that column.
type xlsxFilterColumn struct {
	ColID         int                `xml:"colId,attr"`
	HiddenButton  bool               `xml:"hiddenButton,attr,omitempty"`
	ShowButton    bool               `xml:"showButton,attr,omitempty"`
	CustomFilters *xlsxCustomFilters `xml:"customFilters"`
	Filters       *xlsxFilters       `xml:"filters"`
	ColorFilter   *xlsxColorFilter   `xml:"colorFilter"`
	DynamicFilter *xlsxDynamicFilter `xml:"dynamicFilter"`
	IconFilter    *xlsxIconFilter    `xml:"iconFilter"`
	Top10         *xlsxTop10         `xml:"top10"`
}

// xlsxCustomFilters directly maps the customFilters element. When there is more
// than one custom filter criteria to apply (an 'and' or 'or' joining two
// criteria), then this element groups the customFilter elements together.
type xlsxCustomFilters struct {
	And          bool                `xml:"and,attr,omitempty"`
	CustomFilter []*xlsxCustomFilter `xml:"customFilter"`
}

// xlsxCustomFilter directly maps the customFilter element. A custom AutoFilter
// specifies an operator and a value. There can be at most two customFilters
// specified, and in that case the parent element specifies whether the two
// conditions are joined by 'and' or 'or'. For any cells whose values do not
// meet the specified criteria, the corresponding rows shall be hidden from view
// when the filter is applied.
type xlsxCustomFilter struct {
	Operator string `xml:"operator,attr,omitempty"`
	Val      string `xml:"val,attr,omitempty"`
}

// xlsxFilters directly maps the filters (Filter Criteria) element. When
// multiple values are chosen to filter by, or when a group of date values are
// chosen to filter by, this element groups those criteria together.
type xlsxFilters struct {
	Blank         bool                 `xml:"blank,attr,omitempty"`
	CalendarType  string               `xml:"calendarType,attr,omitempty"`
	Filter        []*xlsxFilter        `xml:"filter"`
	DateGroupItem []*xlsxDateGroupItem `xml:"dateGroupItem"`
}

// xlsxFilter directly maps the filter element. This element expresses a filter
// criteria value.
type xlsxFilter struct {
	Val string `xml:"val,attr,omitempty"`
}

// xlsxColorFilter directly maps the colorFilter element. This element specifies
// the color to filter by and whether to use the cell's fill or font color in
// the filter criteria. If the cell's font or fill color does not match the
// color specified in the criteria, the rows corresponding to those cells are
// hidden from view.
type xlsxColorFilter struct {
	CellColor bool `xml:"cellColor,attr"`
	DxfID     int  `xml:"dxfId,attr"`
}

// xlsxDynamicFilter directly maps the dynamicFilter element. This collection
// specifies dynamic filter criteria. These criteria are considered dynamic
// because they can change, either with the data itself (e.g., "above average")
// or with the current system date (e.g., show values for "today"). For any
// cells whose values do not meet the specified criteria, the corresponding rows
// shall be hidden from view when the filter is applied.
type xlsxDynamicFilter struct {
	MaxValISO string  `xml:"maxValIso,attr,omitempty"`
	Type      string  `xml:"type,attr,omitempty"`
	Val       float64 `xml:"val,attr,omitempty"`
	ValISO    string  `xml:"valIso,attr,omitempty"`
}

// xlsxIconFilter directly maps the iconFilter element. This element specifies
// the icon set and particular icon within that set to filter by. For any cells
// whose icon does not match the specified criteria, the corresponding rows
// shall be hidden from view when the filter is applied.
type xlsxIconFilter struct {
	IconID  int    `xml:"iconId,attr"`
	IconSet string `xml:"iconSet,attr,omitempty"`
}

// xlsxTop10 directly maps the top10 element. This element specifies the top N
// (percent or number of items) to filter by.
type xlsxTop10 struct {
	FilterVal float64 `xml:"filterVal,attr,omitempty"`
	Percent   bool    `xml:"percent,attr,omitempty"`
	Top       bool    `xml:"top,attr"`
	Val       float64 `xml:"val,attr,omitempty"`
}

// xlsxDateGroupItem directly maps the dateGroupItem element. This collection is
// used to express a group of dates or times which are used in an AutoFilter
// criteria. [Note: See parent element for an example. end note] Values are
// always written in the calendar type of the first date encountered in the
// filter range, so that all subsequent dates, even when formatted or
// represented by other calendar types, can be correctly compared for the
// purposes of filtering.
type xlsxDateGroupItem struct {
	DateTimeGrouping string `xml:"dateTimeGrouping,attr,omitempty"`
	Day              int    `xml:"day,attr,omitempty"`
	Hour             int    `xml:"hour,attr,omitempty"`
	Minute           int    `xml:"minute,attr,omitempty"`
	Month            int    `xml:"month,attr,omitempty"`
	Second           int    `xml:"second,attr,omitempty"`
	Year             int    `xml:"year,attr,omitempty"`
}

// xlsxTableColumns directly maps the element representing the collection of all
// table columns for this table.
type xlsxTableColumns struct {
	Count       int                `xml:"count,attr"`
	TableColumn []*xlsxTableColumn `xml:"tableColumn"`
}

// xlsxTableColumn directly maps the element representing a single column for
// this table.
type xlsxTableColumn struct {
	DataCellStyle      string `xml:"dataCellStyle,attr,omitempty"`
	DataDxfID          int    `xml:"dataDxfId,attr,omitempty"`
	HeaderRowCellStyle string `xml:"headerRowCellStyle,attr,omitempty"`
	HeaderRowDxfID     int    `xml:"headerRowDxfId,attr,omitempty"`
	ID                 int    `xml:"id,attr"`
	Name               string `xml:"name,attr"`
	QueryTableFieldID  int    `xml:"queryTableFieldId,attr,omitempty"`
	TotalsRowCellStyle string `xml:"totalsRowCellStyle,attr,omitempty"`
	TotalsRowDxfID     int    `xml:"totalsRowDxfId,attr,omitempty"`
	TotalsRowFunction  string `xml:"totalsRowFunction,attr,omitempty"`
	TotalsRowLabel     string `xml:"totalsRowLabel,attr,omitempty"`
	UniqueName         string `xml:"uniqueName,attr,omitempty"`
}

// xlsxTableStyleInfo directly maps the tableStyleInfo element. This element
// describes which style is used to display this table, and specifies which
// portions of the table have the style applied.
type xlsxTableStyleInfo struct {
	Name              string `xml:"name,attr,omitempty"`
	ShowFirstColumn   bool   `xml:"showFirstColumn,attr"`
	ShowLastColumn    bool   `xml:"showLastColumn,attr"`
	ShowRowStripes    bool   `xml:"showRowStripes,attr"`
	ShowColumnStripes bool   `xml:"showColumnStripes,attr"`
}

// formatTable directly maps the format settings of the table.
type formatTable struct {
	TableName         string `json:"table_name"`
	TableStyle        string `json:"table_style"`
	ShowFirstColumn   bool   `json:"show_first_column"`
	ShowLastColumn    bool   `json:"show_last_column"`
	ShowRowStripes    bool   `json:"show_row_stripes"`
	ShowColumnStripes bool   `json:"show_column_stripes"`
}

// formatAutoFilter directly maps the auto filter settings.
type formatAutoFilter struct {
	Column     string `json:"column"`
	Expression string `json:"expression"`
	FilterList []struct {
		Column string `json:"column"`
		Value  []int  `json:"value"`
	} `json:"filter_list"`
}
