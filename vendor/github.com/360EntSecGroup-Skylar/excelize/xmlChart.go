package excelize

import "encoding/xml"

// xlsxChartSpace directly maps the c:chartSpace element. The chart namespace in
// DrawingML is for representing visualizations of numeric data with column
// charts, pie charts, scatter charts, or other types of charts.
type xlsxChartSpace struct {
	XMLName        xml.Name        `xml:"c:chartSpace"`
	XMLNSc         string          `xml:"xmlns:c,attr"`
	XMLNSa         string          `xml:"xmlns:a,attr"`
	XMLNSr         string          `xml:"xmlns:r,attr"`
	XMLNSc16r2     string          `xml:"xmlns:c16r2,attr"`
	Date1904       *attrValBool    `xml:"c:date1904"`
	Lang           *attrValString  `xml:"c:lang"`
	RoundedCorners *attrValBool    `xml:"c:roundedCorners"`
	Chart          cChart          `xml:"c:chart"`
	SpPr           *cSpPr          `xml:"c:spPr"`
	TxPr           *cTxPr          `xml:"c:txPr"`
	PrintSettings  *cPrintSettings `xml:"c:printSettings"`
}

// cThicknessSpPr directly maps the element that specifies the thickness of the
// walls or floor as a percentage of the largest dimension of the plot volume
// and SpPr element.
type cThicknessSpPr struct {
	Thickness *attrValInt `xml:"c:thickness"`
	SpPr      *cSpPr      `xml:"c:spPr"`
}

// cChart (Chart) directly maps the c:chart element. This element specifies a
// title.
type cChart struct {
	Title            *cTitle            `xml:"c:title"`
	AutoTitleDeleted *cAutoTitleDeleted `xml:"c:autoTitleDeleted"`
	View3D           *cView3D           `xml:"c:view3D"`
	Floor            *cThicknessSpPr    `xml:"c:floor"`
	SideWall         *cThicknessSpPr    `xml:"c:sideWall"`
	BackWall         *cThicknessSpPr    `xml:"c:backWall"`
	PlotArea         *cPlotArea         `xml:"c:plotArea"`
	Legend           *cLegend           `xml:"c:legend"`
	PlotVisOnly      *attrValBool       `xml:"c:plotVisOnly"`
	DispBlanksAs     *attrValString     `xml:"c:dispBlanksAs"`
	ShowDLblsOverMax *attrValBool       `xml:"c:showDLblsOverMax"`
}

// cTitle (Title) directly maps the c:title element. This element specifies a
// title.
type cTitle struct {
	Tx      cTx         `xml:"c:tx,omitempty"`
	Layout  string      `xml:"c:layout,omitempty"`
	Overlay attrValBool `xml:"c:overlay,omitempty"`
	SpPr    cSpPr       `xml:"c:spPr,omitempty"`
	TxPr    cTxPr       `xml:"c:txPr,omitempty"`
}

// cTx (Chart Text) directly maps the c:tx element. This element specifies text
// to use on a chart, including rich text formatting.
type cTx struct {
	StrRef *cStrRef `xml:"c:strRef"`
	Rich   *cRich   `xml:"c:rich,omitempty"`
}

// cRich (Rich Text) directly maps the c:rich element. This element contains a
// string with rich text formatting.
type cRich struct {
	BodyPr   aBodyPr `xml:"a:bodyPr,omitempty"`
	LstStyle string  `xml:"a:lstStyle,omitempty"`
	P        aP      `xml:"a:p"`
}

// aBodyPr (Body Properties) directly maps the a:bodyPr element. This element
// defines the body properties for the text body within a shape.
type aBodyPr struct {
	Anchor           string  `xml:"anchor,attr,omitempty"`
	AnchorCtr        bool    `xml:"anchorCtr,attr"`
	Rot              int     `xml:"rot,attr"`
	BIns             float64 `xml:"bIns,attr,omitempty"`
	CompatLnSpc      bool    `xml:"compatLnSpc,attr,omitempty"`
	ForceAA          bool    `xml:"forceAA,attr,omitempty"`
	FromWordArt      bool    `xml:"fromWordArt,attr,omitempty"`
	HorzOverflow     string  `xml:"horzOverflow,attr,omitempty"`
	LIns             float64 `xml:"lIns,attr,omitempty"`
	NumCol           int     `xml:"numCol,attr,omitempty"`
	RIns             float64 `xml:"rIns,attr,omitempty"`
	RtlCol           bool    `xml:"rtlCol,attr,omitempty"`
	SpcCol           int     `xml:"spcCol,attr,omitempty"`
	SpcFirstLastPara bool    `xml:"spcFirstLastPara,attr"`
	TIns             float64 `xml:"tIns,attr,omitempty"`
	Upright          bool    `xml:"upright,attr,omitempty"`
	Vert             string  `xml:"vert,attr,omitempty"`
	VertOverflow     string  `xml:"vertOverflow,attr,omitempty"`
	Wrap             string  `xml:"wrap,attr,omitempty"`
}

// aP (Paragraph) directly maps the a:p element. This element specifies a
// paragraph of content in the document.
type aP struct {
	PPr        *aPPr        `xml:"a:pPr"`
	R          *aR          `xml:"a:r"`
	EndParaRPr *aEndParaRPr `xml:"a:endParaRPr"`
}

// aPPr (Paragraph Properties) directly maps the a:pPr element. This element
// specifies a set of paragraph properties which shall be applied to the
// contents of the parent paragraph after all style/numbering/table properties
// have been applied to the text. These properties are defined as direct
// formatting, since they are directly applied to the paragraph and supersede
// any formatting from styles.
type aPPr struct {
	DefRPr aRPr `xml:"a:defRPr"`
}

// aSolidFill (Solid Fill) directly maps the solidFill element. This element
// specifies a solid color fill. The shape is filled entirely with the specified
// color.
type aSolidFill struct {
	SchemeClr *aSchemeClr    `xml:"a:schemeClr"`
	SrgbClr   *attrValString `xml:"a:srgbClr"`
}

// aSchemeClr (Scheme Color) directly maps the a:schemeClr element. This
// element specifies a color bound to a user's theme. As with all elements which
// define a color, it is possible to apply a list of color transforms to the
// base color defined.
type aSchemeClr struct {
	Val    string      `xml:"val,attr,omitempty"`
	LumMod *attrValInt `xml:"a:lumMod"`
	LumOff *attrValInt `xml:"a:lumOff"`
}

// attrValInt directly maps the val element with integer data type as an
// attribute。
type attrValInt struct {
	Val int `xml:"val,attr"`
}

// attrValFloat directly maps the val element with float64 data type as an
// attribute。
type attrValFloat struct {
	Val float64 `xml:"val,attr"`
}

// attrValBool directly maps the val element with boolean data type as an
// attribute。
type attrValBool struct {
	Val bool `xml:"val,attr"`
}

// attrValString directly maps the val element with string data type as an
// attribute。
type attrValString struct {
	Val string `xml:"val,attr"`
}

// aCs directly maps the a:cs element.
type aCs struct {
	Typeface string `xml:"typeface,attr"`
}

// aEa directly maps the a:ea element.
type aEa struct {
	Typeface string `xml:"typeface,attr"`
}

// aLatin (Latin Font) directly maps the a:latin element. This element
// specifies that a Latin font be used for a specific run of text. This font is
// specified with a typeface attribute much like the others but is specifically
// classified as a Latin font.
type aLatin struct {
	Typeface string `xml:"typeface,attr"`
}

// aR directly maps the a:r element.
type aR struct {
	RPr aRPr   `xml:"a:rPr,omitempty"`
	T   string `xml:"a:t,omitempty"`
}

// aRPr (Run Properties) directly maps the c:rPr element. This element
// specifies a set of run properties which shall be applied to the contents of
// the parent run after all style formatting has been applied to the text. These
// properties are defined as direct formatting, since they are directly applied
// to the run and supersede any formatting from styles.
type aRPr struct {
	AltLang    string      `xml:"altLang,attr,omitempty"`
	B          bool        `xml:"b,attr"`
	Baseline   int         `xml:"baseline,attr"`
	Bmk        string      `xml:"bmk,attr,omitempty"`
	Cap        string      `xml:"cap,attr,omitempty"`
	Dirty      bool        `xml:"dirty,attr,omitempty"`
	Err        bool        `xml:"err,attr,omitempty"`
	I          bool        `xml:"i,attr"`
	Kern       int         `xml:"kern,attr"`
	Kumimoji   bool        `xml:"kumimoji,attr,omitempty"`
	Lang       string      `xml:"lang,attr,omitempty"`
	NoProof    bool        `xml:"noProof,attr,omitempty"`
	NormalizeH bool        `xml:"normalizeH,attr,omitempty"`
	SmtClean   bool        `xml:"smtClean,attr,omitempty"`
	SmtID      uint64      `xml:"smtId,attr,omitempty"`
	Spc        int         `xml:"spc,attr"`
	Strike     string      `xml:"strike,attr,omitempty"`
	Sz         int         `xml:"sz,attr,omitempty"`
	U          string      `xml:"u,attr,omitempty"`
	SolidFill  *aSolidFill `xml:"a:solidFill"`
	Latin      *aLatin     `xml:"a:latin"`
	Ea         *aEa        `xml:"a:ea"`
	Cs         *aCs        `xml:"a:cs"`
}

// cSpPr (Shape Properties) directly maps the c:spPr element. This element
// specifies the visual shape properties that can be applied to a shape. These
// properties include the shape fill, outline, geometry, effects, and 3D
// orientation.
type cSpPr struct {
	NoFill    *string     `xml:"a:noFill"`
	SolidFill *aSolidFill `xml:"a:solidFill"`
	Ln        *aLn        `xml:"a:ln"`
	Sp3D      *aSp3D      `xml:"a:sp3d"`
	EffectLst *string     `xml:"a:effectLst"`
}

// aSp3D (3-D Shape Properties) directly maps the a:sp3d element. This element
// defines the 3D properties associated with a particular shape in DrawingML.
// The 3D properties which can be applied to a shape are top and bottom bevels,
// a contour and an extrusion.
type aSp3D struct {
	ContourW   int          `xml:"contourW,attr"`
	ContourClr *aContourClr `xml:"a:contourClr"`
}

// aContourClr (Contour Color) directly maps the a:contourClr element. This
// element defines the color for the contour on a shape. The contour of a shape
// is a solid filled line which surrounds the outer edges of the shape.
type aContourClr struct {
	SchemeClr *aSchemeClr `xml:"a:schemeClr"`
}

// aLn (Outline) directly maps the a:ln element. This element specifies an
// outline style that can be applied to a number of different objects such as
// shapes and text. The line allows for the specifying of many different types
// of outlines including even line dashes and bevels.
type aLn struct {
	Algn      string      `xml:"algn,attr,omitempty"`
	Cap       string      `xml:"cap,attr,omitempty"`
	Cmpd      string      `xml:"cmpd,attr,omitempty"`
	W         int         `xml:"w,attr,omitempty" `
	NoFill    string      `xml:"a:noFill,omitempty"`
	Round     string      `xml:"a:round,omitempty"`
	SolidFill *aSolidFill `xml:"a:solidFill"`
}

// cTxPr (Text Properties) directly maps the c:txPr element. This element
// specifies text formatting. The lstStyle element is not supported.
type cTxPr struct {
	BodyPr   aBodyPr `xml:"a:bodyPr,omitempty"`
	LstStyle string  `xml:"a:lstStyle,omitempty"`
	P        aP      `xml:"a:p,omitempty"`
}

// aEndParaRPr (End Paragraph Run Properties) directly maps the a:endParaRPr
// element. This element specifies the text run properties that are to be used
// if another run is inserted after the last run specified. This effectively
// saves the run property state so that it can be applied when the user enters
// additional text. If this element is omitted, then the application can
// determine which default properties to apply. It is recommended that this
// element be specified at the end of the list of text runs within the paragraph
// so that an orderly list is maintained.
type aEndParaRPr struct {
	Lang    string `xml:"lang,attr"`
	AltLang string `xml:"altLang,attr,omitempty"`
	Sz      int    `xml:"sz,attr,omitempty"`
}

// cAutoTitleDeleted (Auto Title Is Deleted) directly maps the
// c:autoTitleDeleted element. This element specifies the title shall not be
// shown for this chart.
type cAutoTitleDeleted struct {
	Val bool `xml:"val,attr"`
}

// cView3D (View In 3D) directly maps the c:view3D element. This element
// specifies the 3-D view of the chart.
type cView3D struct {
	RotX         *attrValInt `xml:"c:rotX"`
	RotY         *attrValInt `xml:"c:rotY"`
	DepthPercent *attrValInt `xml:"c:depthPercent"`
	RAngAx       *attrValInt `xml:"c:rAngAx"`
}

// cPlotArea directly maps the c:plotArea element. This element specifies the
// plot area of the chart.
type cPlotArea struct {
	Layout        *string  `xml:"c:layout"`
	BarChart      *cCharts `xml:"c:barChart"`
	Bar3DChart    *cCharts `xml:"c:bar3DChart"`
	DoughnutChart *cCharts `xml:"c:doughnutChart"`
	LineChart     *cCharts `xml:"c:lineChart"`
	PieChart      *cCharts `xml:"c:pieChart"`
	Pie3DChart    *cCharts `xml:"c:pie3DChart"`
	RadarChart    *cCharts `xml:"c:radarChart"`
	ScatterChart  *cCharts `xml:"c:scatterChart"`
	CatAx         []*cAxs  `xml:"c:catAx"`
	ValAx         []*cAxs  `xml:"c:valAx"`
	SpPr          *cSpPr   `xml:"c:spPr"`
}

// cCharts specifies the common element of the chart.
type cCharts struct {
	BarDir       *attrValString `xml:"c:barDir"`
	Grouping     *attrValString `xml:"c:grouping"`
	RadarStyle   *attrValString `xml:"c:radarStyle"`
	ScatterStyle *attrValString `xml:"c:scatterStyle"`
	VaryColors   *attrValBool   `xml:"c:varyColors"`
	Ser          *[]cSer        `xml:"c:ser"`
	DLbls        *cDLbls        `xml:"c:dLbls"`
	HoleSize     *attrValInt    `xml:"c:holeSize"`
	Smooth       *attrValBool   `xml:"c:smooth"`
	Overlap      *attrValInt    `xml:"c:overlap"`
	AxID         []*attrValInt  `xml:"c:axId"`
}

// cAxs directly maps the c:catAx and c:valAx element.
type cAxs struct {
	AxID          *attrValInt    `xml:"c:axId"`
	Scaling       *cScaling      `xml:"c:scaling"`
	Delete        *attrValBool   `xml:"c:delete"`
	AxPos         *attrValString `xml:"c:axPos"`
	NumFmt        *cNumFmt       `xml:"c:numFmt"`
	MajorTickMark *attrValString `xml:"c:majorTickMark"`
	MinorTickMark *attrValString `xml:"c:minorTickMark"`
	TickLblPos    *attrValString `xml:"c:tickLblPos"`
	SpPr          *cSpPr         `xml:"c:spPr"`
	TxPr          *cTxPr         `xml:"c:txPr"`
	CrossAx       *attrValInt    `xml:"c:crossAx"`
	Crosses       *attrValString `xml:"c:crosses"`
	CrossBetween  *attrValString `xml:"c:crossBetween"`
	Auto          *attrValBool   `xml:"c:auto"`
	LblAlgn       *attrValString `xml:"c:lblAlgn"`
	LblOffset     *attrValInt    `xml:"c:lblOffset"`
	NoMultiLvlLbl *attrValBool   `xml:"c:noMultiLvlLbl"`
}

// cScaling directly maps the c:scaling element. This element contains
// additional axis settings.
type cScaling struct {
	Orientation *attrValString `xml:"c:orientation"`
	Max         *attrValFloat  `xml:"c:max"`
	Min         *attrValFloat  `xml:"c:min"`
}

// cNumFmt (Numbering Format) directly maps the c:numFmt element. This element
// specifies number formatting for the parent element.
type cNumFmt struct {
	FormatCode   string `xml:"formatCode,attr"`
	SourceLinked bool   `xml:"sourceLinked,attr"`
}

// cSer directly maps the c:ser element. This element specifies a series on a
// chart.
type cSer struct {
	IDx              *attrValInt  `xml:"c:idx"`
	Order            *attrValInt  `xml:"c:order"`
	Tx               *cTx         `xml:"c:tx"`
	SpPr             *cSpPr       `xml:"c:spPr"`
	DPt              []*cDPt      `xml:"c:dPt"`
	DLbls            *cDLbls      `xml:"c:dLbls"`
	Marker           *cMarker     `xml:"c:marker"`
	InvertIfNegative *attrValBool `xml:"c:invertIfNegative"`
	Cat              *cCat        `xml:"c:cat"`
	Val              *cVal        `xml:"c:val"`
	XVal             *cCat        `xml:"c:xVal"`
	YVal             *cVal        `xml:"c:yVal"`
	Smooth           *attrValBool `xml:"c:smooth"`
}

// cMarker (Marker) directly maps the c:marker element. This element specifies a
// data marker.
type cMarker struct {
	Symbol *attrValString `xml:"c:symbol"`
	Size   *attrValInt    `xml:"c:size"`
	SpPr   *cSpPr         `xml:"c:spPr"`
}

// cDPt (Data Point) directly maps the c:dPt element. This element specifies a
// single data point.
type cDPt struct {
	IDx      *attrValInt  `xml:"c:idx"`
	Bubble3D *attrValBool `xml:"c:bubble3D"`
	SpPr     *cSpPr       `xml:"c:spPr"`
}

// cCat (Category Axis Data) directly maps the c:cat element. This element
// specifies the data used for the category axis.
type cCat struct {
	StrRef *cStrRef `xml:"c:strRef"`
}

// cStrRef (String Reference) directly maps the c:strRef element. This element
// specifies a reference to data for a single data label or title with a cache
// of the last values used.
type cStrRef struct {
	F        string     `xml:"c:f"`
	StrCache *cStrCache `xml:"c:strCache"`
}

// cStrCache (String Cache) directly maps the c:strCache element. This element
// specifies the last string data used for a chart.
type cStrCache struct {
	Pt      []*cPt      `xml:"c:pt"`
	PtCount *attrValInt `xml:"c:ptCount"`
}

// cPt directly maps the c:pt element. This element specifies data for a
// particular data point.
type cPt struct {
	IDx int     `xml:"idx,attr"`
	V   *string `xml:"c:v"`
}

// cVal directly maps the c:val element. This element specifies the data values
// which shall be used to define the location of data markers on a chart.
type cVal struct {
	NumRef *cNumRef `xml:"c:numRef"`
}

// cNumRef directly maps the c:numRef element. This element specifies a
// reference to numeric data with a cache of the last values used.
type cNumRef struct {
	F        string     `xml:"c:f"`
	NumCache *cNumCache `xml:"c:numCache"`
}

// cNumCache directly maps the c:numCache element. This element specifies the
// last data shown on the chart for a series.
type cNumCache struct {
	FormatCode string      `xml:"c:formatCode"`
	Pt         []*cPt      `xml:"c:pt"`
	PtCount    *attrValInt `xml:"c:ptCount"`
}

// cDLbls (Data Lables) directly maps the c:dLbls element. This element serves
// as a root element that specifies the settings for the data labels for an
// entire series or the entire chart. It contains child elements that specify
// the specific formatting and positioning settings.
type cDLbls struct {
	ShowLegendKey   *attrValBool `xml:"c:showLegendKey"`
	ShowVal         *attrValBool `xml:"c:showVal"`
	ShowCatName     *attrValBool `xml:"c:showCatName"`
	ShowSerName     *attrValBool `xml:"c:showSerName"`
	ShowPercent     *attrValBool `xml:"c:showPercent"`
	ShowBubbleSize  *attrValBool `xml:"c:showBubbleSize"`
	ShowLeaderLines *attrValBool `xml:"c:showLeaderLines"`
}

// cLegend (Legend) directly maps the c:legend element. This element specifies
// the legend.
type cLegend struct {
	Layout    *string        `xml:"c:layout"`
	LegendPos *attrValString `xml:"c:legendPos"`
	Overlay   *attrValBool   `xml:"c:overlay"`
	SpPr      *cSpPr         `xml:"c:spPr"`
	TxPr      *cTxPr         `xml:"c:txPr"`
}

// cPrintSettings directly maps the c:printSettings element. This element
// specifies the print settings for the chart.
type cPrintSettings struct {
	HeaderFooter *string       `xml:"c:headerFooter"`
	PageMargins  *cPageMargins `xml:"c:pageMargins"`
	PageSetup    *string       `xml:"c:pageSetup"`
}

// cPageMargins directly maps the c:pageMargins element. This element specifies
// the page margins for a chart.
type cPageMargins struct {
	B      float64 `xml:"b,attr"`
	Footer float64 `xml:"footer,attr"`
	Header float64 `xml:"header,attr"`
	L      float64 `xml:"l,attr"`
	R      float64 `xml:"r,attr"`
	T      float64 `xml:"t,attr"`
}

// formatChartAxis directly maps the format settings of the chart axis.
type formatChartAxis struct {
	Crossing            string  `json:"crossing"`
	MajorTickMark       string  `json:"major_tick_mark"`
	MinorTickMark       string  `json:"minor_tick_mark"`
	MinorUnitType       string  `json:"minor_unit_type"`
	MajorUnit           int     `json:"major_unit"`
	MajorUnitType       string  `json:"major_unit_type"`
	DisplayUnits        string  `json:"display_units"`
	DisplayUnitsVisible bool    `json:"display_units_visible"`
	DateAxis            bool    `json:"date_axis"`
	ReverseOrder        bool    `json:"reverse_order"`
	Maximum             float64 `json:"maximum"`
	Minimum             float64 `json:"minimum"`
	NumFormat           string  `json:"num_format"`
	NumFont             struct {
		Color     string `json:"color"`
		Bold      bool   `json:"bold"`
		Italic    bool   `json:"italic"`
		Underline bool   `json:"underline"`
	} `json:"num_font"`
	NameLayout formatLayout `json:"name_layout"`
}

type formatChartDimension struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// formatChart directly maps the format settings of the chart.
type formatChart struct {
	Type      string               `json:"type"`
	Series    []formatChartSeries  `json:"series"`
	Format    formatPicture        `json:"format"`
	Dimension formatChartDimension `json:"dimension"`
	Legend    formatChartLegend    `json:"legend"`
	Title     formatChartTitle     `json:"title"`
	XAxis     formatChartAxis      `json:"x_axis"`
	YAxis     formatChartAxis      `json:"y_axis"`
	Chartarea struct {
		Border struct {
			None bool `json:"none"`
		} `json:"border"`
		Fill struct {
			Color string `json:"color"`
		} `json:"fill"`
		Pattern struct {
			Pattern string `json:"pattern"`
			FgColor string `json:"fg_color"`
			BgColor string `json:"bg_color"`
		} `json:"pattern"`
	} `json:"chartarea"`
	Plotarea struct {
		ShowBubbleSize  bool `json:"show_bubble_size"`
		ShowCatName     bool `json:"show_cat_name"`
		ShowLeaderLines bool `json:"show_leader_lines"`
		ShowPercent     bool `json:"show_percent"`
		ShowSerName     bool `json:"show_series_name"`
		ShowVal         bool `json:"show_val"`
		Gradient        struct {
			Colors []string `json:"colors"`
		} `json:"gradient"`
		Border struct {
			Color    string `json:"color"`
			Width    int    `json:"width"`
			DashType string `json:"dash_type"`
		} `json:"border"`
		Fill struct {
			Color string `json:"color"`
		} `json:"fill"`
		Layout formatLayout `json:"layout"`
	} `json:"plotarea"`
	ShowBlanksAs   string `json:"show_blanks_as"`
	ShowHiddenData bool   `json:"show_hidden_data"`
	SetRotation    int    `json:"set_rotation"`
	SetHoleSize    int    `json:"set_hole_size"`
}

// formatChartLegend directly maps the format settings of the chart legend.
type formatChartLegend struct {
	None            bool         `json:"none"`
	DeleteSeries    []int        `json:"delete_series"`
	Font            formatFont   `json:"font"`
	Layout          formatLayout `json:"layout"`
	Position        string       `json:"position"`
	ShowLegendEntry bool         `json:"show_legend_entry"`
	ShowLegendKey   bool         `json:"show_legend_key"`
}

// formatChartSeries directly maps the format settings of the chart series.
type formatChartSeries struct {
	Name       string `json:"name"`
	Categories string `json:"categories"`
	Values     string `json:"values"`
	Line       struct {
		None  bool   `json:"none"`
		Color string `json:"color"`
	} `json:"line"`
	Marker struct {
		Type   string  `json:"type"`
		Size   int     `json:"size,"`
		Width  float64 `json:"width"`
		Border struct {
			Color string `json:"color"`
			None  bool   `json:"none"`
		} `json:"border"`
		Fill struct {
			Color string `json:"color"`
			None  bool   `json:"none"`
		} `json:"fill"`
	} `json:"marker"`
}

// formatChartTitle directly maps the format settings of the chart title.
type formatChartTitle struct {
	None    bool         `json:"none"`
	Name    string       `json:"name"`
	Overlay bool         `json:"overlay"`
	Layout  formatLayout `json:"layout"`
}

// formatLayout directly maps the format settings of the element layout.
type formatLayout struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
