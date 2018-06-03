package excelize

import "encoding/xml"

// Source relationship and namespace.
const (
	SourceRelationship              = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	SourceRelationshipChart         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart"
	SourceRelationshipComments      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	SourceRelationshipImage         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	SourceRelationshipTable         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/table"
	SourceRelationshipDrawingML     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing"
	SourceRelationshipDrawingVML    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/vmlDrawing"
	SourceRelationshipHyperLink     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	SourceRelationshipWorkSheet     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"
	SourceRelationshipChart201506   = "http://schemas.microsoft.com/office/drawing/2015/06/chart"
	SourceRelationshipChart20070802 = "http://schemas.microsoft.com/office/drawing/2007/8/2/chart"
	SourceRelationshipChart2014     = "http://schemas.microsoft.com/office/drawing/2014/chart"
	SourceRelationshipCompatibility = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	NameSpaceDrawingML              = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NameSpaceDrawingMLChart         = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	NameSpaceDrawingMLSpreadSheet   = "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"
	NameSpaceSpreadSheet            = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	NameSpaceXML                    = "http://www.w3.org/XML/1998/namespace"
)

var supportImageTypes = map[string]string{".gif": ".gif", ".jpg": ".jpeg", ".jpeg": ".jpeg", ".png": ".png"}

// xlsxCNvPr directly maps the cNvPr (Non-Visual Drawing Properties). This
// element specifies non-visual canvas properties. This allows for additional
// information that does not affect the appearance of the picture to be stored.
type xlsxCNvPr struct {
	ID         int             `xml:"id,attr"`
	Name       string          `xml:"name,attr"`
	Descr      string          `xml:"descr,attr"`
	Title      string          `xml:"title,attr,omitempty"`
	HlinkClick *xlsxHlinkClick `xml:"a:hlinkClick"`
}

// xlsxHlinkClick (Click Hyperlink) Specifies the on-click hyperlink
// information to be applied to a run of text. When the hyperlink text is
// clicked the link is fetched.
type xlsxHlinkClick struct {
	R              string `xml:"xmlns:r,attr,omitempty"`
	RID            string `xml:"r:id,attr,omitempty"`
	InvalidURL     string `xml:"invalidUrl,attr,omitempty"`
	Action         string `xml:"action,attr,omitempty"`
	TgtFrame       string `xml:"tgtFrame,attr,omitempty"`
	Tooltip        string `xml:"tooltip,attr,omitempty"`
	History        bool   `xml:"history,attr,omitempty"`
	HighlightClick bool   `xml:"highlightClick,attr,omitempty"`
	EndSnd         bool   `xml:"endSnd,attr,omitempty"`
}

// xlsxPicLocks directly maps the picLocks (Picture Locks). This element
// specifies all locking properties for a graphic frame. These properties inform
// the generating application about specific properties that have been
// previously locked and thus should not be changed.
type xlsxPicLocks struct {
	NoAdjustHandles    bool `xml:"noAdjustHandles,attr,omitempty"`
	NoChangeArrowheads bool `xml:"noChangeArrowheads,attr,omitempty"`
	NoChangeAspect     bool `xml:"noChangeAspect,attr"`
	NoChangeShapeType  bool `xml:"noChangeShapeType,attr,omitempty"`
	NoCrop             bool `xml:"noCrop,attr,omitempty"`
	NoEditPoints       bool `xml:"noEditPoints,attr,omitempty"`
	NoGrp              bool `xml:"noGrp,attr,omitempty"`
	NoMove             bool `xml:"noMove,attr,omitempty"`
	NoResize           bool `xml:"noResize,attr,omitempty"`
	NoRot              bool `xml:"noRot,attr,omitempty"`
	NoSelect           bool `xml:"noSelect,attr,omitempty"`
}

// xlsxBlip directly maps the blip element in the namespace
// http://purl.oclc.org/ooxml/officeDoc ument/relationships - This element
// specifies the existence of an image (binary large image or picture) and
// contains a reference to the image data.
type xlsxBlip struct {
	Embed  string `xml:"r:embed,attr"`
	Cstate string `xml:"cstate,attr,omitempty"`
	R      string `xml:"xmlns:r,attr"`
}

// xlsxStretch directly maps the stretch element. This element specifies that a
// BLIP should be stretched to fill the target rectangle. The other option is a
// tile where a BLIP is tiled to fill the available area.
type xlsxStretch struct {
	FillRect string `xml:"a:fillRect"`
}

// xlsxOff directly maps the colOff and rowOff element. This element is used to
// specify the column offset within a cell.
type xlsxOff struct {
	X int `xml:"x,attr"`
	Y int `xml:"y,attr"`
}

// xlsxExt directly maps the ext element.
type xlsxExt struct {
	Cx int `xml:"cx,attr"`
	Cy int `xml:"cy,attr"`
}

// xlsxPrstGeom directly maps the prstGeom (Preset geometry). This element
// specifies when a preset geometric shape should be used instead of a custom
// geometric shape. The generating application should be able to render all
// preset geometries enumerated in the ST_ShapeType list.
type xlsxPrstGeom struct {
	Prst string `xml:"prst,attr"`
}

// xlsxXfrm directly maps the xfrm (2D Transform for Graphic Frame). This
// element specifies the transform to be applied to the corresponding graphic
// frame. This transformation is applied to the graphic frame just as it would
// be for a shape or group shape.
type xlsxXfrm struct {
	Off xlsxOff `xml:"a:off"`
	Ext xlsxExt `xml:"a:ext"`
}

// xlsxCNvPicPr directly maps the cNvPicPr (Non-Visual Picture Drawing
// Properties). This element specifies the non-visual properties for the picture
// canvas. These properties are to be used by the generating application to
// determine how certain properties are to be changed for the picture object in
// question.
type xlsxCNvPicPr struct {
	PicLocks xlsxPicLocks `xml:"a:picLocks"`
}

// directly maps the nvPicPr (Non-Visual Properties for a Picture). This element
// specifies all non-visual properties for a picture. This element is a
// container for the non-visual identification properties, shape properties and
// application properties that are to be associated with a picture. This allows
// for additional information that does not affect the appearance of the picture
// to be stored.
type xlsxNvPicPr struct {
	CNvPr    xlsxCNvPr    `xml:"xdr:cNvPr"`
	CNvPicPr xlsxCNvPicPr `xml:"xdr:cNvPicPr"`
}

// xlsxBlipFill directly maps the blipFill (Picture Fill). This element
// specifies the kind of picture fill that the picture object has. Because a
// picture has a picture fill already by default, it is possible to have two
// fills specified for a picture object.
type xlsxBlipFill struct {
	Blip    xlsxBlip    `xml:"a:blip"`
	Stretch xlsxStretch `xml:"a:stretch"`
}

// xlsxSpPr directly maps the spPr (Shape Properties). This element specifies
// the visual shape properties that can be applied to a picture. These are the
// same properties that are allowed to describe the visual properties of a shape
// but are used here to describe the visual appearance of a picture within a
// document.
type xlsxSpPr struct {
	Xfrm     xlsxXfrm     `xml:"a:xfrm"`
	PrstGeom xlsxPrstGeom `xml:"a:prstGeom"`
}

// xlsxPic elements encompass the definition of pictures within the DrawingML
// framework. While pictures are in many ways very similar to shapes they have
// specific properties that are unique in order to optimize for picture-
// specific scenarios.
type xlsxPic struct {
	NvPicPr  xlsxNvPicPr  `xml:"xdr:nvPicPr"`
	BlipFill xlsxBlipFill `xml:"xdr:blipFill"`
	SpPr     xlsxSpPr     `xml:"xdr:spPr"`
}

// xlsxFrom specifies the starting anchor.
type xlsxFrom struct {
	Col    int `xml:"xdr:col"`
	ColOff int `xml:"xdr:colOff"`
	Row    int `xml:"xdr:row"`
	RowOff int `xml:"xdr:rowOff"`
}

// xlsxTo directly specifies the ending anchor.
type xlsxTo struct {
	Col    int `xml:"xdr:col"`
	ColOff int `xml:"xdr:colOff"`
	Row    int `xml:"xdr:row"`
	RowOff int `xml:"xdr:rowOff"`
}

// xdrClientData directly maps the clientData element. An empty element which
// specifies (via attributes) certain properties related to printing and
// selection of the drawing object. The fLocksWithSheet attribute (either true
// or false) determines whether to disable selection when the sheet is
// protected, and fPrintsWithSheet attribute (either true or false) determines
// whether the object is printed when the sheet is printed.
type xdrClientData struct {
	FLocksWithSheet  bool `xml:"fLocksWithSheet,attr"`
	FPrintsWithSheet bool `xml:"fPrintsWithSheet,attr"`
}

// xdrCellAnchor directly maps the oneCellAnchor (One Cell Anchor Shape Size)
// and twoCellAnchor (Two Cell Anchor Shape Size). This element specifies a two
// cell anchor placeholder for a group, a shape, or a drawing element. It moves
// with cells and its extents are in EMU units.
type xdrCellAnchor struct {
	EditAs       string         `xml:"editAs,attr,omitempty"`
	From         *xlsxFrom      `xml:"xdr:from"`
	To           *xlsxTo        `xml:"xdr:to"`
	Ext          *xlsxExt       `xml:"xdr:ext"`
	Sp           *xdrSp         `xml:"xdr:sp"`
	Pic          *xlsxPic       `xml:"xdr:pic,omitempty"`
	GraphicFrame string         `xml:",innerxml"`
	ClientData   *xdrClientData `xml:"xdr:clientData"`
}

// xlsxWsDr directly maps the root element for a part of this content type shall
// wsDr.
type xlsxWsDr struct {
	XMLName       xml.Name         `xml:"xdr:wsDr"`
	OneCellAnchor []*xdrCellAnchor `xml:"xdr:oneCellAnchor"`
	TwoCellAnchor []*xdrCellAnchor `xml:"xdr:twoCellAnchor"`
	A             string           `xml:"xmlns:a,attr,omitempty"`
	Xdr           string           `xml:"xmlns:xdr,attr,omitempty"`
	R             string           `xml:"xmlns:r,attr,omitempty"`
}

// xlsxGraphicFrame (Graphic Frame) directly maps the xdr:graphicFrame element.
// This element specifies the existence of a graphics frame. This frame contains
// a graphic that was generated by an external source and needs a container in
// which to be displayed on the slide surface.
type xlsxGraphicFrame struct {
	XMLName          xml.Name             `xml:"xdr:graphicFrame"`
	Macro            string               `xml:"macro,attr"`
	NvGraphicFramePr xlsxNvGraphicFramePr `xml:"xdr:nvGraphicFramePr"`
	Xfrm             xlsxXfrm             `xml:"xdr:xfrm"`
	Graphic          *xlsxGraphic         `xml:"a:graphic"`
}

// xlsxNvGraphicFramePr (Non-Visual Properties for a Graphic Frame) directly
// maps the xdr:nvGraphicFramePr element. This element specifies all non-visual
// properties for a graphic frame. This element is a container for the non-
// visual identification properties, shape properties and application properties
// that are to be associated with a graphic frame. This allows for additional
// information that does not affect the appearance of the graphic frame to be
// stored.
type xlsxNvGraphicFramePr struct {
	CNvPr                *xlsxCNvPr `xml:"xdr:cNvPr"`
	ChicNvGraphicFramePr string     `xml:"xdr:cNvGraphicFramePr"`
}

// xlsxGraphic (Graphic Object) directly maps the a:graphic element. This
// element specifies the existence of a single graphic object. Document authors
// should refer to this element when they wish to persist a graphical object of
// some kind. The specification for this graphical object is provided entirely
// by the document author and referenced within the graphicData child element.
type xlsxGraphic struct {
	GraphicData *xlsxGraphicData `xml:"a:graphicData"`
}

// xlsxGraphicData (Graphic Object Data) directly maps the a:graphicData
// element. This element specifies the reference to a graphic object within the
// document. This graphic object is provided entirely by the document authors
// who choose to persist this data within the document.
type xlsxGraphicData struct {
	URI   string     `xml:"uri,attr"`
	Chart *xlsxChart `xml:"c:chart,omitempty"`
}

// xlsxChart (Chart) directly maps the c:chart element.
type xlsxChart struct {
	C   string `xml:"xmlns:c,attr"`
	RID string `xml:"r:id,attr"`
	R   string `xml:"xmlns:r,attr"`
}

// xdrSp (Shape) directly maps the xdr:sp element. This element specifies the
// existence of a single shape. A shape can either be a preset or a custom
// geometry, defined using the SpreadsheetDrawingML framework. In addition to a
// geometry each shape can have both visual and non-visual properties attached.
// Text and corresponding styling information can also be attached to a shape.
// This shape is specified along with all other shapes within either the shape
// tree or group shape elements.
type xdrSp struct {
	Macro    string     `xml:"macro,attr"`
	Textlink string     `xml:"textlink,attr"`
	NvSpPr   *xdrNvSpPr `xml:"xdr:nvSpPr"`
	SpPr     *xlsxSpPr  `xml:"xdr:spPr"`
	Style    *xdrStyle  `xml:"xdr:style"`
	TxBody   *xdrTxBody `xml:"xdr:txBody"`
}

// xdrNvSpPr (Non-Visual Properties for a Shape) directly maps the xdr:nvSpPr
// element. This element specifies all non-visual properties for a shape. This
// element is a container for the non-visual identification properties, shape
// properties and application properties that are to be associated with a shape.
// This allows for additional information that does not affect the appearance of
// the shape to be stored.
type xdrNvSpPr struct {
	CNvPr   *xlsxCNvPr  `xml:"xdr:cNvPr"`
	CNvSpPr *xdrCNvSpPr `xml:"xdr:cNvSpPr"`
}

// xdrCNvSpPr (Connection Non-Visual Shape Properties) directly maps the
// xdr:cNvSpPr element. This element specifies the set of non-visual properties
// for a connection shape. These properties specify all data about the
// connection shape which do not affect its display within a spreadsheet.
type xdrCNvSpPr struct {
	TxBox bool `xml:"txBox,attr"`
}

// xdrStyle (Shape Style) directly maps the xdr:style element. The element
// specifies the style that is applied to a shape and the corresponding
// references for each of the style components such as lines and fills.
type xdrStyle struct {
	LnRef     *aRef     `xml:"a:lnRef"`
	FillRef   *aRef     `xml:"a:fillRef"`
	EffectRef *aRef     `xml:"a:effectRef"`
	FontRef   *aFontRef `xml:"a:fontRef"`
}

// aRef directly maps the a:lnRef, a:fillRef and a:effectRef element.
type aRef struct {
	Idx       int            `xml:"idx,attr"`
	ScrgbClr  *aScrgbClr     `xml:"a:scrgbClr"`
	SchemeClr *attrValString `xml:"a:schemeClr"`
	SrgbClr   *attrValString `xml:"a:srgbClr"`
}

// aScrgbClr (RGB Color Model - Percentage Variant) directly maps the a:scrgbClr
// element. This element specifies a color using the red, green, blue RGB color
// model. Each component, red, green, and blue is expressed as a percentage from
// 0% to 100%. A linear gamma of 1.0 is assumed.
type aScrgbClr struct {
	R float64 `xml:"r,attr"`
	G float64 `xml:"g,attr"`
	B float64 `xml:"b,attr"`
}

// aFontRef (Font Reference) directly maps the a:fontRef element. This element
// represents a reference to a themed font. When used it specifies which themed
// font to use along with a choice of color.
type aFontRef struct {
	Idx       string         `xml:"idx,attr"`
	SchemeClr *attrValString `xml:"a:schemeClr"`
}

// xdrTxBody (Shape Text Body) directly maps the xdr:txBody element. This
// element specifies the existence of text to be contained within the
// corresponding shape. All visible text and visible text related properties are
// contained within this element. There can be multiple paragraphs and within
// paragraphs multiple runs of text.
type xdrTxBody struct {
	BodyPr *aBodyPr `xml:"a:bodyPr"`
	P      []*aP    `xml:"a:p"`
}

// formatPicture directly maps the format settings of the picture.
type formatPicture struct {
	FPrintsWithSheet bool    `json:"print_obj"`
	FLocksWithSheet  bool    `json:"locked"`
	NoChangeAspect   bool    `json:"lock_aspect_ratio"`
	OffsetX          int     `json:"x_offset"`
	OffsetY          int     `json:"y_offset"`
	XScale           float64 `json:"x_scale"`
	YScale           float64 `json:"y_scale"`
	Hyperlink        string  `json:"hyperlink"`
	HyperlinkType    string  `json:"hyperlink_type"`
	Positioning      string  `json:"positioning"`
}

// formatShape directly maps the format settings of the shape.
type formatShape struct {
	Type      string                 `json:"type"`
	Width     int                    `json:"width"`
	Height    int                    `json:"height"`
	Format    formatPicture          `json:"format"`
	Color     formatShapeColor       `json:"color"`
	Paragraph []formatShapeParagraph `json:"paragraph"`
}

// formatShapeParagraph directly maps the format settings of the paragraph in
// the shape.
type formatShapeParagraph struct {
	Font formatFont `json:"font"`
	Text string     `json:"text"`
}

// formatShapeColor directly maps the color settings of the shape.
type formatShapeColor struct {
	Line   string `json:"line"`
	Fill   string `json:"fill"`
	Effect string `json:"effect"`
}
