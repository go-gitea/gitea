package excelize

import "encoding/xml"

// xlsxStyleSheet directly maps the stylesheet element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxStyleSheet struct {
	XMLName      xml.Name          `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main styleSheet"`
	NumFmts      *xlsxNumFmts      `xml:"numFmts,omitempty"`
	Fonts        *xlsxFonts        `xml:"fonts,omitempty"`
	Fills        *xlsxFills        `xml:"fills,omitempty"`
	Borders      *xlsxBorders      `xml:"borders,omitempty"`
	CellStyleXfs *xlsxCellStyleXfs `xml:"cellStyleXfs,omitempty"`
	CellXfs      *xlsxCellXfs      `xml:"cellXfs,omitempty"`
	CellStyles   *xlsxCellStyles   `xml:"cellStyles,omitempty"`
	Dxfs         *xlsxDxfs         `xml:"dxfs,omitempty"`
	TableStyles  *xlsxTableStyles  `xml:"tableStyles,omitempty"`
	Colors       *xlsxStyleColors  `xml:"colors,omitempty"`
	ExtLst       *xlsxExtLst       `xml:"extLst"`
}

// xlsxAlignment formatting information pertaining to text alignment in cells.
// There are a variety of choices for how text is aligned both horizontally and
// vertically, as well as indentation settings, and so on.
type xlsxAlignment struct {
	Horizontal      string `xml:"horizontal,attr,omitempty"`
	Indent          int    `xml:"indent,attr,omitempty"`
	JustifyLastLine bool   `xml:"justifyLastLine,attr,omitempty"`
	ReadingOrder    uint64 `xml:"readingOrder,attr,omitempty"`
	RelativeIndent  int    `xml:"relativeIndent,attr,omitempty"`
	ShrinkToFit     bool   `xml:"shrinkToFit,attr,omitempty"`
	TextRotation    int    `xml:"textRotation,attr,omitempty"`
	Vertical        string `xml:"vertical,attr,omitempty"`
	WrapText        bool   `xml:"wrapText,attr,omitempty"`
}

// xlsxProtection (Protection Properties) contains protection properties
// associated with the cell. Each cell has protection properties that can be
// set. The cell protection properties do not take effect unless the sheet has
// been protected.
type xlsxProtection struct {
	Hidden bool `xml:"hidden,attr"`
	Locked bool `xml:"locked,attr"`
}

// xlsxLine directly maps the line style element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need.
type xlsxLine struct {
	Style string     `xml:"style,attr,omitempty"`
	Color *xlsxColor `xml:"color,omitempty"`
}

// xlsxColor is a common mapping used for both the fgColor and bgColor elements.
// Foreground color of the cell fill pattern. Cell fill patterns operate with
// two colors: a background color and a foreground color. These combine together
// to make a patterned cell fill. Background color of the cell fill pattern.
// Cell fill patterns operate with two colors: a background color and a
// foreground color. These combine together to make a patterned cell fill.
type xlsxColor struct {
	Auto    bool    `xml:"auto,attr,omitempty"`
	RGB     string  `xml:"rgb,attr,omitempty"`
	Indexed int     `xml:"indexed,attr,omitempty"`
	Theme   *int    `xml:"theme,attr"`
	Tint    float64 `xml:"tint,attr,omitempty"`
}

// xlsxFonts directly maps the font element. This element contains all font
// definitions for this workbook.
type xlsxFonts struct {
	Count int         `xml:"count,attr"`
	Font  []*xlsxFont `xml:"font"`
}

// font directly maps the font element.
type font struct {
	Name     *attrValString `xml:"name"`
	Charset  *attrValInt    `xml:"charset"`
	Family   *attrValInt    `xml:"family"`
	B        bool           `xml:"b,omitempty"`
	I        bool           `xml:"i,omitempty"`
	Strike   bool           `xml:"strike,omitempty"`
	Outline  bool           `xml:"outline,omitempty"`
	Shadow   bool           `xml:"shadow,omitempty"`
	Condense bool           `xml:"condense,omitempty"`
	Extend   bool           `xml:"extend,omitempty"`
	Color    *xlsxColor     `xml:"color"`
	Sz       *attrValInt    `xml:"sz"`
	U        *attrValString `xml:"u"`
	Scheme   *attrValString `xml:"scheme"`
}

// xlsxFont directly maps the font element. This element defines the properties
// for one of the fonts used in this workbook.
type xlsxFont struct {
	Font string `xml:",innerxml"`
}

// xlsxFills directly maps the fills element. This element defines the cell
// fills portion of the Styles part, consisting of a sequence of fill records. A
// cell fill consists of a background color, foreground color, and pattern to be
// applied across the cell.
type xlsxFills struct {
	Count int         `xml:"count,attr"`
	Fill  []*xlsxFill `xml:"fill,omitempty"`
}

// xlsxFill directly maps the fill element. This element specifies fill
// formatting.
type xlsxFill struct {
	PatternFill  *xlsxPatternFill  `xml:"patternFill,omitempty"`
	GradientFill *xlsxGradientFill `xml:"gradientFill,omitempty"`
}

// xlsxPatternFill directly maps the patternFill element in the namespace
// http://schemas.openxmlformats.org/spreadsheetml/2006/main - currently I have
// not checked it for completeness - it does as much as I need. This element is
// used to specify cell fill information for pattern and solid color cell fills.
// For solid cell fills (no pattern), fgColor is used. For cell fills with
// patterns specified, then the cell fill color is specified by the bgColor
// element.
type xlsxPatternFill struct {
	PatternType string    `xml:"patternType,attr,omitempty"`
	FgColor     xlsxColor `xml:"fgColor,omitempty"`
	BgColor     xlsxColor `xml:"bgColor,omitempty"`
}

// xlsxGradientFill defines a gradient-style cell fill. Gradient cell fills can
// use one or two colors as the end points of color interpolation.
type xlsxGradientFill struct {
	Bottom float64                 `xml:"bottom,attr,omitempty"`
	Degree float64                 `xml:"degree,attr,omitempty"`
	Left   float64                 `xml:"left,attr,omitempty"`
	Right  float64                 `xml:"right,attr,omitempty"`
	Top    float64                 `xml:"top,attr,omitempty"`
	Type   string                  `xml:"type,attr,omitempty"`
	Stop   []*xlsxGradientFillStop `xml:"stop,omitempty"`
}

// xlsxGradientFillStop directly maps the stop element.
type xlsxGradientFillStop struct {
	Position float64   `xml:"position,attr"`
	Color    xlsxColor `xml:"color,omitempty"`
}

// xlsxBorders directly maps the borders element. This element contains borders
// formatting information, specifying all border definitions for all cells in
// the workbook.
type xlsxBorders struct {
	Count  int           `xml:"count,attr"`
	Border []*xlsxBorder `xml:"border,omitempty"`
}

// xlsxBorder directly maps the border element. Expresses a single set of cell
// border formats (left, right, top, bottom, diagonal). Color is optional. When
// missing, 'automatic' is implied.
type xlsxBorder struct {
	DiagonalDown bool     `xml:"diagonalDown,attr,omitempty"`
	DiagonalUp   bool     `xml:"diagonalUp,attr,omitempty"`
	Outline      bool     `xml:"outline,attr,omitempty"`
	Left         xlsxLine `xml:"left,omitempty"`
	Right        xlsxLine `xml:"right,omitempty"`
	Top          xlsxLine `xml:"top,omitempty"`
	Bottom       xlsxLine `xml:"bottom,omitempty"`
	Diagonal     xlsxLine `xml:"diagonal,omitempty"`
}

// xlsxCellStyles directly maps the cellStyles element. This element contains
// the named cell styles, consisting of a sequence of named style records. A
// named cell style is a collection of direct or themed formatting (e.g., cell
// border, cell fill, and font type/size/style) grouped together into a single
// named style, and can be applied to a cell.
type xlsxCellStyles struct {
	XMLName   xml.Name         `xml:"cellStyles"`
	Count     int              `xml:"count,attr"`
	CellStyle []*xlsxCellStyle `xml:"cellStyle,omitempty"`
}

// xlsxCellStyle directly maps the cellStyle element. This element represents
// the name and related formatting records for a named cell style in this
// workbook.
type xlsxCellStyle struct {
	XMLName       xml.Name `xml:"cellStyle"`
	BuiltInID     *int     `xml:"builtinId,attr,omitempty"`
	CustomBuiltIn *bool    `xml:"customBuiltin,attr,omitempty"`
	Hidden        *bool    `xml:"hidden,attr,omitempty"`
	ILevel        *bool    `xml:"iLevel,attr,omitempty"`
	Name          string   `xml:"name,attr"`
	XfID          int      `xml:"xfId,attr"`
}

// xlsxCellStyleXfs directly maps the cellStyleXfs element. This element
// contains the master formatting records (xf's) which define the formatting for
// all named cell styles in this workbook. Master formatting records reference
// individual elements of formatting (e.g., number format, font definitions,
// cell fills, etc) by specifying a zero-based index into those collections.
// Master formatting records also specify whether to apply or ignore particular
// aspects of formatting.
type xlsxCellStyleXfs struct {
	Count int      `xml:"count,attr"`
	Xf    []xlsxXf `xml:"xf,omitempty"`
}

// xlsxXf directly maps the xf element. A single xf element describes all of the
// formatting for a cell.
type xlsxXf struct {
	ApplyAlignment    bool            `xml:"applyAlignment,attr"`
	ApplyBorder       bool            `xml:"applyBorder,attr"`
	ApplyFill         bool            `xml:"applyFill,attr"`
	ApplyFont         bool            `xml:"applyFont,attr"`
	ApplyNumberFormat bool            `xml:"applyNumberFormat,attr"`
	ApplyProtection   bool            `xml:"applyProtection,attr"`
	BorderID          int             `xml:"borderId,attr"`
	FillID            int             `xml:"fillId,attr"`
	FontID            int             `xml:"fontId,attr"`
	NumFmtID          int             `xml:"numFmtId,attr"`
	PivotButton       bool            `xml:"pivotButton,attr,omitempty"`
	QuotePrefix       bool            `xml:"quotePrefix,attr,omitempty"`
	XfID              *int            `xml:"xfId,attr"`
	Alignment         *xlsxAlignment  `xml:"alignment"`
	Protection        *xlsxProtection `xml:"protection"`
}

// xlsxCellXfs directly maps the cellXfs element. This element contains the
// master formatting records (xf) which define the formatting applied to cells
// in this workbook. These records are the starting point for determining the
// formatting for a cell. Cells in the Sheet Part reference the xf records by
// zero-based index.
type xlsxCellXfs struct {
	Count int      `xml:"count,attr"`
	Xf    []xlsxXf `xml:"xf,omitempty"`
}

// xlsxDxfs directly maps the dxfs element. This element contains the master
// differential formatting records (dxf's) which define formatting for all non-
// cell formatting in this workbook. Whereas xf records fully specify a
// particular aspect of formatting (e.g., cell borders) by referencing those
// formatting definitions elsewhere in the Styles part, dxf records specify
// incremental (or differential) aspects of formatting directly inline within
// the dxf element. The dxf formatting is to be applied on top of or in addition
// to any formatting already present on the object using the dxf record.
type xlsxDxfs struct {
	Count int        `xml:"count,attr"`
	Dxfs  []*xlsxDxf `xml:"dxf,omitempty"`
}

// xlsxDxf directly maps the dxf element. A single dxf record, expressing
// incremental formatting to be applied.
type xlsxDxf struct {
	Dxf string `xml:",innerxml"`
}

// dxf directly maps the dxf element.
type dxf struct {
	Font       *font           `xml:"font"`
	NumFmt     *xlsxNumFmt     `xml:"numFmt"`
	Fill       *xlsxFill       `xml:"fill"`
	Alignment  *xlsxAlignment  `xml:"alignment"`
	Border     *xlsxBorder     `xml:"border"`
	Protection *xlsxProtection `xml:"protection"`
	ExtLst     *xlsxExt        `xml:"extLst"`
}

// xlsxTableStyles directly maps the tableStyles element. This element
// represents a collection of Table style definitions for Table styles and
// PivotTable styles used in this workbook. It consists of a sequence of
// tableStyle records, each defining a single Table style.
type xlsxTableStyles struct {
	Count             int               `xml:"count,attr"`
	DefaultPivotStyle string            `xml:"defaultPivotStyle,attr"`
	DefaultTableStyle string            `xml:"defaultTableStyle,attr"`
	TableStyles       []*xlsxTableStyle `xml:"tableStyle,omitempty"`
}

// xlsxTableStyle directly maps the tableStyle element. This element represents
// a single table style definition that indicates how a spreadsheet application
// should format and display a table.
type xlsxTableStyle struct {
	Name              string `xml:"name,attr,omitempty"`
	Pivot             int    `xml:"pivot,attr"`
	Count             int    `xml:"count,attr,omitempty"`
	Table             bool   `xml:"table,attr,omitempty"`
	TableStyleElement string `xml:",innerxml"`
}

// xlsxNumFmts directly maps the numFmts element. This element defines the
// number formats in this workbook, consisting of a sequence of numFmt records,
// where each numFmt record defines a particular number format, indicating how
// to format and render the numeric value of a cell.
type xlsxNumFmts struct {
	Count  int           `xml:"count,attr"`
	NumFmt []*xlsxNumFmt `xml:"numFmt,omitempty"`
}

// xlsxNumFmt directly maps the numFmt element. This element specifies number
// format properties which indicate how to format and render the numeric value
// of a cell.
type xlsxNumFmt struct {
	NumFmtID   int    `xml:"numFmtId,attr,omitempty"`
	FormatCode string `xml:"formatCode,attr,omitempty"`
}

// xlsxStyleColors directly maps the colors element. Color information
// associated with this stylesheet. This collection is written whenever the
// legacy color palette has been modified (backwards compatibility settings) or
// a custom color has been selected while using this workbook.
type xlsxStyleColors struct {
	Color string `xml:",innerxml"`
}

// formatFont directly maps the styles settings of the fonts.
type formatFont struct {
	Bold      bool   `json:"bold"`
	Italic    bool   `json:"italic"`
	Underline string `json:"underline"`
	Family    string `json:"family"`
	Size      int    `json:"size"`
	Color     string `json:"color"`
}

// formatStyle directly maps the styles settings of the cells.
type formatStyle struct {
	Border []struct {
		Type  string `json:"type"`
		Color string `json:"color"`
		Style int    `json:"style"`
	} `json:"border"`
	Fill struct {
		Type    string   `json:"type"`
		Pattern int      `json:"pattern"`
		Color   []string `json:"color"`
		Shading int      `json:"shading"`
	} `json:"fill"`
	Font      *formatFont `json:"font"`
	Alignment *struct {
		Horizontal      string `json:"horizontal"`
		Indent          int    `json:"indent"`
		JustifyLastLine bool   `json:"justify_last_line"`
		ReadingOrder    uint64 `json:"reading_order"`
		RelativeIndent  int    `json:"relative_indent"`
		ShrinkToFit     bool   `json:"shrink_to_fit"`
		TextRotation    int    `json:"text_rotation"`
		Vertical        string `json:"vertical"`
		WrapText        bool   `json:"wrap_text"`
	} `json:"alignment"`
	Protection *struct {
		Hidden bool `json:"hidden"`
		Locked bool `json:"locked"`
	} `json:"protection"`
	NumFmt        int     `json:"number_format"`
	DecimalPlaces int     `json:"decimal_places"`
	CustomNumFmt  *string `json:"custom_number_format"`
	Lang          string  `json:"lang"`
	NegRed        bool    `json:"negred"`
}
