package excelize

import "encoding/xml"

// vmlDrawing directly maps the root element in the file
// xl/drawings/vmlDrawing%d.vml.
type vmlDrawing struct {
	XMLName     xml.Name         `xml:"xml"`
	XMLNSv      string           `xml:"xmlns:v,attr"`
	XMLNSo      string           `xml:"xmlns:o,attr"`
	XMLNSx      string           `xml:"xmlns:x,attr"`
	XMLNSmv     string           `xml:"xmlns:mv,attr"`
	Shapelayout *xlsxShapelayout `xml:"o:shapelayout"`
	Shapetype   *xlsxShapetype   `xml:"v:shapetype"`
	Shape       []xlsxShape      `xml:"v:shape"`
}

// xlsxShapelayout directly maps the shapelayout element. This element contains
// child elements that store information used in the editing and layout of
// shapes.
type xlsxShapelayout struct {
	Ext   string     `xml:"v:ext,attr"`
	IDmap *xlsxIDmap `xml:"o:idmap"`
}

// xlsxIDmap directly maps the idmap element.
type xlsxIDmap struct {
	Ext  string `xml:"v:ext,attr"`
	Data int    `xml:"data,attr"`
}

// xlsxShape directly maps the shape element.
type xlsxShape struct {
	XMLName     xml.Name `xml:"v:shape"`
	ID          string   `xml:"id,attr"`
	Type        string   `xml:"type,attr"`
	Style       string   `xml:"style,attr"`
	Fillcolor   string   `xml:"fillcolor,attr"`
	Insetmode   string   `xml:"urn:schemas-microsoft-com:office:office insetmode,attr,omitempty"`
	Strokecolor string   `xml:"strokecolor,attr,omitempty"`
	Val         string   `xml:",innerxml"`
}

// xlsxShapetype directly maps the shapetype element.
type xlsxShapetype struct {
	ID        string      `xml:"id,attr"`
	Coordsize string      `xml:"coordsize,attr"`
	Spt       int         `xml:"o:spt,attr"`
	Path      string      `xml:"path,attr"`
	Stroke    *xlsxStroke `xml:"v:stroke"`
	VPath     *vPath      `xml:"v:path"`
}

// xlsxStroke directly maps the stroke element.
type xlsxStroke struct {
	Joinstyle string `xml:"joinstyle,attr"`
}

// vPath directly maps the v:path element.
type vPath struct {
	Gradientshapeok string `xml:"gradientshapeok,attr,omitempty"`
	Connecttype     string `xml:"o:connecttype,attr"`
}

// vFill directly maps the v:fill element. This element must be defined within a
// Shape element.
type vFill struct {
	Angle  int    `xml:"angle,attr,omitempty"`
	Color2 string `xml:"color2,attr"`
	Type   string `xml:"type,attr,omitempty"`
	Fill   *oFill `xml:"o:fill"`
}

// oFill directly maps the o:fill element.
type oFill struct {
	Ext  string `xml:"v:ext,attr"`
	Type string `xml:"type,attr,omitempty"`
}

// vShadow directly maps the v:shadow element. This element must be defined
// within a Shape element. In addition, the On attribute must be set to True.
type vShadow struct {
	On       string `xml:"on,attr"`
	Color    string `xml:"color,attr,omitempty"`
	Obscured string `xml:"obscured,attr"`
}

// vTextbox directly maps the v:textbox element. This element must be defined
// within a Shape element.
type vTextbox struct {
	Style string   `xml:"style,attr"`
	Div   *xlsxDiv `xml:"div"`
}

// xlsxDiv directly maps the div element.
type xlsxDiv struct {
	Style string `xml:"style,attr"`
}

// xClientData (Attached Object Data) directly maps the x:ClientData element.
// This element specifies data associated with objects attached to a
// spreadsheet. While this element might contain any of the child elements
// below, only certain combinations are meaningful. The ObjectType attribute
// determines the kind of object the element represents and which subset of
// child elements is appropriate. Relevant groups are identified for each child
// element.
type xClientData struct {
	ObjectType    string `xml:"ObjectType,attr"`
	MoveWithCells string `xml:"x:MoveWithCells,omitempty"`
	SizeWithCells string `xml:"x:SizeWithCells,omitempty"`
	Anchor        string `xml:"x:Anchor"`
	AutoFill      string `xml:"x:AutoFill"`
	Row           int    `xml:"x:Row"`
	Column        int    `xml:"x:Column"`
}

// decodeVmlDrawing defines the structure used to parse the file
// xl/drawings/vmlDrawing%d.vml.
type decodeVmlDrawing struct {
	Shape []decodeShape `xml:"urn:schemas-microsoft-com:vml shape"`
}

// decodeShape defines the structure used to parse the particular shape element.
type decodeShape struct {
	Val string `xml:",innerxml"`
}

// encodeShape defines the structure used to re-serialization shape element.
type encodeShape struct {
	Fill       *vFill       `xml:"v:fill"`
	Shadow     *vShadow     `xml:"v:shadow"`
	Path       *vPath       `xml:"v:path"`
	Textbox    *vTextbox    `xml:"v:textbox"`
	ClientData *xClientData `xml:"x:ClientData"`
}
