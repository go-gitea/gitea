package excelize

import "encoding/xml"

// decodeCellAnchor directly maps the oneCellAnchor (One Cell Anchor Shape Size)
// and twoCellAnchor (Two Cell Anchor Shape Size). This element specifies a two
// cell anchor placeholder for a group, a shape, or a drawing element. It moves
// with cells and its extents are in EMU units.
type decodeCellAnchor struct {
	EditAs  string `xml:"editAs,attr,omitempty"`
	Content string `xml:",innerxml"`
}

// decodeWsDr directly maps the root element for a part of this content type
// shall wsDr. In order to solve the problem that the label structure is changed
// after serialization and deserialization, two different structures are
// defined. decodeWsDr just for deserialization.
type decodeWsDr struct {
	A             string              `xml:"xmlns a,attr"`
	Xdr           string              `xml:"xmlns xdr,attr"`
	R             string              `xml:"xmlns r,attr"`
	OneCellAnchor []*decodeCellAnchor `xml:"oneCellAnchor,omitempty"`
	TwoCellAnchor []*decodeCellAnchor `xml:"twoCellAnchor,omitempty"`
	XMLName       xml.Name            `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing wsDr,omitempty"`
}

// decodeTwoCellAnchor directly maps the oneCellAnchor (One Cell Anchor Shape
// Size) and twoCellAnchor (Two Cell Anchor Shape Size). This element specifies
// a two cell anchor placeholder for a group, a shape, or a drawing element. It
// moves with cells and its extents are in EMU units.
type decodeTwoCellAnchor struct {
	From       *decodeFrom       `xml:"from"`
	To         *decodeTo         `xml:"to"`
	Pic        *decodePic        `xml:"pic,omitempty"`
	ClientData *decodeClientData `xml:"clientData"`
}

// decodeCNvPr directly maps the cNvPr (Non-Visual Drawing Properties). This
// element specifies non-visual canvas properties. This allows for additional
// information that does not affect the appearance of the picture to be stored.
type decodeCNvPr struct {
	ID    int    `xml:"id,attr"`
	Name  string `xml:"name,attr"`
	Descr string `xml:"descr,attr"`
	Title string `xml:"title,attr,omitempty"`
}

// decodePicLocks directly maps the picLocks (Picture Locks). This element
// specifies all locking properties for a graphic frame. These properties inform
// the generating application about specific properties that have been
// previously locked and thus should not be changed.
type decodePicLocks struct {
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

// decodeBlip directly maps the blip element in the namespace
// http://purl.oclc.org/ooxml/officeDoc ument/relationships - This element
// specifies the existence of an image (binary large image or picture) and
// contains a reference to the image data.
type decodeBlip struct {
	Embed  string `xml:"embed,attr"`
	Cstate string `xml:"cstate,attr,omitempty"`
	R      string `xml:"r,attr"`
}

// decodeStretch directly maps the stretch element. This element specifies that
// a BLIP should be stretched to fill the target rectangle. The other option is
// a tile where a BLIP is tiled to fill the available area.
type decodeStretch struct {
	FillRect string `xml:"fillRect"`
}

// decodeOff directly maps the colOff and rowOff element. This element is used
// to specify the column offset within a cell.
type decodeOff struct {
	X int `xml:"x,attr"`
	Y int `xml:"y,attr"`
}

// decodeExt directly maps the ext element.
type decodeExt struct {
	Cx int `xml:"cx,attr"`
	Cy int `xml:"cy,attr"`
}

// decodePrstGeom directly maps the prstGeom (Preset geometry). This element
// specifies when a preset geometric shape should be used instead of a custom
// geometric shape. The generating application should be able to render all
// preset geometries enumerated in the ST_ShapeType list.
type decodePrstGeom struct {
	Prst string `xml:"prst,attr"`
}

// decodeXfrm directly maps the xfrm (2D Transform for Graphic Frame). This
// element specifies the transform to be applied to the corresponding graphic
// frame. This transformation is applied to the graphic frame just as it would
// be for a shape or group shape.
type decodeXfrm struct {
	Off decodeOff `xml:"off"`
	Ext decodeExt `xml:"ext"`
}

// decodeCNvPicPr directly maps the cNvPicPr (Non-Visual Picture Drawing
// Properties). This element specifies the non-visual properties for the picture
// canvas. These properties are to be used by the generating application to
// determine how certain properties are to be changed for the picture object in
// question.
type decodeCNvPicPr struct {
	PicLocks decodePicLocks `xml:"picLocks"`
}

// directly maps the nvPicPr (Non-Visual Properties for a Picture). This element
// specifies all non-visual properties for a picture. This element is a
// container for the non-visual identification properties, shape properties and
// application properties that are to be associated with a picture. This allows
// for additional information that does not affect the appearance of the picture
// to be stored.
type decodeNvPicPr struct {
	CNvPr    decodeCNvPr    `xml:"cNvPr"`
	CNvPicPr decodeCNvPicPr `xml:"cNvPicPr"`
}

// decodeBlipFill directly maps the blipFill (Picture Fill). This element
// specifies the kind of picture fill that the picture object has. Because a
// picture has a picture fill already by default, it is possible to have two
// fills specified for a picture object.
type decodeBlipFill struct {
	Blip    decodeBlip    `xml:"blip"`
	Stretch decodeStretch `xml:"stretch"`
}

// decodeSpPr directly maps the spPr (Shape Properties). This element specifies
// the visual shape properties that can be applied to a picture. These are the
// same properties that are allowed to describe the visual properties of a shape
// but are used here to describe the visual appearance of a picture within a
// document.
type decodeSpPr struct {
	Xfrm     decodeXfrm     `xml:"a:xfrm"`
	PrstGeom decodePrstGeom `xml:"a:prstGeom"`
}

// decodePic elements encompass the definition of pictures within the DrawingML
// framework. While pictures are in many ways very similar to shapes they have
// specific properties that are unique in order to optimize for picture-
// specific scenarios.
type decodePic struct {
	NvPicPr  decodeNvPicPr  `xml:"nvPicPr"`
	BlipFill decodeBlipFill `xml:"blipFill"`
	SpPr     decodeSpPr     `xml:"spPr"`
}

// decodeFrom specifies the starting anchor.
type decodeFrom struct {
	Col    int `xml:"col"`
	ColOff int `xml:"colOff"`
	Row    int `xml:"row"`
	RowOff int `xml:"rowOff"`
}

// decodeTo directly specifies the ending anchor.
type decodeTo struct {
	Col    int `xml:"col"`
	ColOff int `xml:"colOff"`
	Row    int `xml:"row"`
	RowOff int `xml:"rowOff"`
}

// decodeClientData directly maps the clientData element. An empty element which
// specifies (via attributes) certain properties related to printing and
// selection of the drawing object. The fLocksWithSheet attribute (either true
// or false) determines whether to disable selection when the sheet is
// protected, and fPrintsWithSheet attribute (either true or false) determines
// whether the object is printed when the sheet is printed.
type decodeClientData struct {
	FLocksWithSheet  bool `xml:"fLocksWithSheet,attr"`
	FPrintsWithSheet bool `xml:"fPrintsWithSheet,attr"`
}
