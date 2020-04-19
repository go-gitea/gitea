package svg

// uses github.com/tdewolff/hasher
//go:generate hasher -type=Hash -file=hash.go

// Hash defines perfect hashes for a predefined list of strings
type Hash uint32

// Identifiers for the hashes associated with the text in the comments.
const (
	A                            Hash = 0x101   // a
	Alignment_Baseline           Hash = 0x2e12  // alignment-baseline
	BaseProfile                  Hash = 0xb     // baseProfile
	Baseline_Shift               Hash = 0x380e  // baseline-shift
	Buffered_Rendering           Hash = 0x5212  // buffered-rendering
	Clip                         Hash = 0x6404  // clip
	Clip_Path                    Hash = 0x6409  // clip-path
	Clip_Rule                    Hash = 0x8009  // clip-rule
	Color                        Hash = 0xd805  // color
	Color_Interpolation          Hash = 0xd813  // color-interpolation
	Color_Interpolation_Filters  Hash = 0xd81b  // color-interpolation-filters
	Color_Profile                Hash = 0x1f70d // color-profile
	Color_Rendering              Hash = 0x2320f // color-rendering
	ContentScriptType            Hash = 0xa011  // contentScriptType
	ContentStyleType             Hash = 0xb110  // contentStyleType
	Cursor                       Hash = 0xc106  // cursor
	D                            Hash = 0x5901  // d
	Defs                         Hash = 0x36904 // defs
	Direction                    Hash = 0x30c09 // direction
	Display                      Hash = 0x9807  // display
	Dominant_Baseline            Hash = 0x19211 // dominant-baseline
	Enable_Background            Hash = 0x8811  // enable-background
	FeImage                      Hash = 0x14507 // feImage
	Fill                         Hash = 0xc904  // fill
	Fill_Opacity                 Hash = 0x33d0c // fill-opacity
	Fill_Rule                    Hash = 0xc909  // fill-rule
	Filter                       Hash = 0xec06  // filter
	Flood_Color                  Hash = 0xd20b  // flood-color
	Flood_Opacity                Hash = 0x1050d // flood-opacity
	Font                         Hash = 0x11404 // font
	Font_Family                  Hash = 0x1140b // font-family
	Font_Size                    Hash = 0x11f09 // font-size
	Font_Size_Adjust             Hash = 0x11f10 // font-size-adjust
	Font_Stretch                 Hash = 0x1370c // font-stretch
	Font_Style                   Hash = 0x14c0a // font-style
	Font_Variant                 Hash = 0x1560c // font-variant
	Font_Weight                  Hash = 0x1620b // font-weight
	ForeignObject                Hash = 0x16d0d // foreignObject
	G                            Hash = 0x1601  // g
	Glyph_Orientation_Horizontal Hash = 0x1d31c // glyph-orientation-horizontal
	Glyph_Orientation_Vertical   Hash = 0x161a  // glyph-orientation-vertical
	Height                       Hash = 0x6c06  // height
	Href                         Hash = 0x14204 // href
	Image                        Hash = 0x17a05 // image
	Image_Rendering              Hash = 0x17a0f // image-rendering
	Kerning                      Hash = 0x1bc07 // kerning
	Letter_Spacing               Hash = 0x90e   // letter-spacing
	Lighting_Color               Hash = 0x1ee0e // lighting-color
	Line                         Hash = 0x3c04  // line
	Marker                       Hash = 0x18906 // marker
	Marker_End                   Hash = 0x1890a // marker-end
	Marker_Mid                   Hash = 0x1a30a // marker-mid
	Marker_Start                 Hash = 0x1ad0c // marker-start
	Mask                         Hash = 0x1b904 // mask
	Metadata                     Hash = 0x1c308 // metadata
	Missing_Glyph                Hash = 0x1cb0d // missing-glyph
	Opacity                      Hash = 0x10b07 // opacity
	Overflow                     Hash = 0x26208 // overflow
	Paint_Order                  Hash = 0x2ae0b // paint-order
	Path                         Hash = 0x6904  // path
	Pattern                      Hash = 0x20407 // pattern
	Pointer_Events               Hash = 0x20b0e // pointer-events
	Points                       Hash = 0x22706 // points
	Polygon                      Hash = 0x24107 // polygon
	Polyline                     Hash = 0x24808 // polyline
	PreserveAspectRatio          Hash = 0x25013 // preserveAspectRatio
	Rect                         Hash = 0x30e04 // rect
	Rx                           Hash = 0x4f02  // rx
	Ry                           Hash = 0xc602  // ry
	Script                       Hash = 0xf206  // script
	Shape_Rendering              Hash = 0x2180f // shape-rendering
	Solid_Color                  Hash = 0x22c0b // solid-color
	Solid_Opacity                Hash = 0x36c0d // solid-opacity
	Stop_Color                   Hash = 0x12d0a // stop-color
	Stop_Opacity                 Hash = 0x2740c // stop-opacity
	Stroke                       Hash = 0x28006 // stroke
	Stroke_Dasharray             Hash = 0x28010 // stroke-dasharray
	Stroke_Dashoffset            Hash = 0x29011 // stroke-dashoffset
	Stroke_Linecap               Hash = 0x2a10e // stroke-linecap
	Stroke_Linejoin              Hash = 0x2b90f // stroke-linejoin
	Stroke_Miterlimit            Hash = 0x2c811 // stroke-miterlimit
	Stroke_Opacity               Hash = 0x2d90e // stroke-opacity
	Stroke_Width                 Hash = 0x2e70c // stroke-width
	Style                        Hash = 0x15105 // style
	Svg                          Hash = 0x2f303 // svg
	Switch                       Hash = 0x2f606 // switch
	Symbol                       Hash = 0x2fc06 // symbol
	Text_Anchor                  Hash = 0x450b  // text-anchor
	Text_Decoration              Hash = 0x710f  // text-decoration
	Text_Rendering               Hash = 0xf70e  // text-rendering
	Type                         Hash = 0x11004 // type
	Unicode_Bidi                 Hash = 0x3020c // unicode-bidi
	Use                          Hash = 0x31503 // use
	Vector_Effect                Hash = 0x3180d // vector-effect
	Version                      Hash = 0x32507 // version
	ViewBox                      Hash = 0x32c07 // viewBox
	Viewport_Fill                Hash = 0x3340d // viewport-fill
	Viewport_Fill_Opacity        Hash = 0x33415 // viewport-fill-opacity
	Visibility                   Hash = 0x3490a // visibility
	White_Space                  Hash = 0x2690b // white-space
	Width                        Hash = 0x2ee05 // width
	Word_Spacing                 Hash = 0x3530c // word-spacing
	Writing_Mode                 Hash = 0x35f0c // writing-mode
	X                            Hash = 0x4701  // x
	X1                           Hash = 0x5002  // x1
	X2                           Hash = 0x33202 // x2
	Xml_Space                    Hash = 0x37909 // xml:space
	Y                            Hash = 0x1801  // y
	Y1                           Hash = 0x9e02  // y1
	Y2                           Hash = 0xc702  // y2
)

var HashMap = map[string]Hash{
	"a":                            A,
	"alignment-baseline":           Alignment_Baseline,
	"baseProfile":                  BaseProfile,
	"baseline-shift":               Baseline_Shift,
	"buffered-rendering":           Buffered_Rendering,
	"clip":                         Clip,
	"clip-path":                    Clip_Path,
	"clip-rule":                    Clip_Rule,
	"color":                        Color,
	"color-interpolation":          Color_Interpolation,
	"color-interpolation-filters":  Color_Interpolation_Filters,
	"color-profile":                Color_Profile,
	"color-rendering":              Color_Rendering,
	"contentScriptType":            ContentScriptType,
	"contentStyleType":             ContentStyleType,
	"cursor":                       Cursor,
	"d":                            D,
	"defs":                         Defs,
	"direction":                    Direction,
	"display":                      Display,
	"dominant-baseline":            Dominant_Baseline,
	"enable-background":            Enable_Background,
	"feImage":                      FeImage,
	"fill":                         Fill,
	"fill-opacity":                 Fill_Opacity,
	"fill-rule":                    Fill_Rule,
	"filter":                       Filter,
	"flood-color":                  Flood_Color,
	"flood-opacity":                Flood_Opacity,
	"font":                         Font,
	"font-family":                  Font_Family,
	"font-size":                    Font_Size,
	"font-size-adjust":             Font_Size_Adjust,
	"font-stretch":                 Font_Stretch,
	"font-style":                   Font_Style,
	"font-variant":                 Font_Variant,
	"font-weight":                  Font_Weight,
	"foreignObject":                ForeignObject,
	"g":                            G,
	"glyph-orientation-horizontal": Glyph_Orientation_Horizontal,
	"glyph-orientation-vertical":   Glyph_Orientation_Vertical,
	"height":                       Height,
	"href":                         Href,
	"image":                        Image,
	"image-rendering":              Image_Rendering,
	"kerning":                      Kerning,
	"letter-spacing":               Letter_Spacing,
	"lighting-color":               Lighting_Color,
	"line":                         Line,
	"marker":                       Marker,
	"marker-end":                   Marker_End,
	"marker-mid":                   Marker_Mid,
	"marker-start":                 Marker_Start,
	"mask":                         Mask,
	"metadata":                     Metadata,
	"missing-glyph":                Missing_Glyph,
	"opacity":                      Opacity,
	"overflow":                     Overflow,
	"paint-order":                  Paint_Order,
	"path":                         Path,
	"pattern":                      Pattern,
	"pointer-events":               Pointer_Events,
	"points":                       Points,
	"polygon":                      Polygon,
	"polyline":                     Polyline,
	"preserveAspectRatio":          PreserveAspectRatio,
	"rect":                         Rect,
	"rx":                           Rx,
	"ry":                           Ry,
	"script":                       Script,
	"shape-rendering":              Shape_Rendering,
	"solid-color":                  Solid_Color,
	"solid-opacity":                Solid_Opacity,
	"stop-color":                   Stop_Color,
	"stop-opacity":                 Stop_Opacity,
	"stroke":                       Stroke,
	"stroke-dasharray":             Stroke_Dasharray,
	"stroke-dashoffset":            Stroke_Dashoffset,
	"stroke-linecap":               Stroke_Linecap,
	"stroke-linejoin":              Stroke_Linejoin,
	"stroke-miterlimit":            Stroke_Miterlimit,
	"stroke-opacity":               Stroke_Opacity,
	"stroke-width":                 Stroke_Width,
	"style":                        Style,
	"svg":                          Svg,
	"switch":                       Switch,
	"symbol":                       Symbol,
	"text-anchor":                  Text_Anchor,
	"text-decoration":              Text_Decoration,
	"text-rendering":               Text_Rendering,
	"type":                         Type,
	"unicode-bidi":                 Unicode_Bidi,
	"use":                          Use,
	"vector-effect":                Vector_Effect,
	"version":                      Version,
	"viewBox":                      ViewBox,
	"viewport-fill":                Viewport_Fill,
	"viewport-fill-opacity":        Viewport_Fill_Opacity,
	"visibility":                   Visibility,
	"white-space":                  White_Space,
	"width":                        Width,
	"word-spacing":                 Word_Spacing,
	"writing-mode":                 Writing_Mode,
	"x":                            X,
	"x1":                           X1,
	"x2":                           X2,
	"xml:space":                    Xml_Space,
	"y":                            Y,
	"y1":                           Y1,
	"y2":                           Y2,
}

// String returns the text associated with the hash.
func (i Hash) String() string {
	return string(i.Bytes())
}

// Bytes returns the text associated with the hash.
func (i Hash) Bytes() []byte {
	start := uint32(i >> 8)
	n := uint32(i & 0xff)
	if start+n > uint32(len(_Hash_text)) {
		return []byte{}
	}
	return _Hash_text[start : start+n]
}

// ToHash returns a hash Hash for a given []byte. Hash is a uint32 that is associated with the text in []byte. It returns zero if no match found.
func ToHash(s []byte) Hash {
	if 3 < len(s) {
		return HashMap[string(s)]
	}
	h := uint32(_Hash_hash0)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	if i := _Hash_table[h&uint32(len(_Hash_table)-1)]; int(i&0xff) == len(s) {
		t := _Hash_text[i>>8 : i>>8+i&0xff]
		for i := 0; i < len(s); i++ {
			if t[i] != s[i] {
				goto NEXT
			}
		}
		return i
	}
NEXT:
	if i := _Hash_table[(h>>16)&uint32(len(_Hash_table)-1)]; int(i&0xff) == len(s) {
		t := _Hash_text[i>>8 : i>>8+i&0xff]
		for i := 0; i < len(s); i++ {
			if t[i] != s[i] {
				return 0
			}
		}
		return i
	}
	return 0
}

const _Hash_hash0 = 0x2d0dfdc1
const _Hash_maxLen = 28

var _Hash_text = []byte("" +
	"baseProfiletter-spacinglyph-orientation-verticalignment-base" +
	"line-shiftext-anchorx1buffered-renderingclip-patheightext-de" +
	"corationclip-rulenable-backgroundisplay1contentScriptTypecon" +
	"tentStyleTypecursory2fill-ruleflood-color-interpolation-filt" +
	"erscriptext-renderingflood-opacitypefont-familyfont-size-adj" +
	"ustop-colorfont-stretchrefeImagefont-stylefont-variantfont-w" +
	"eightforeignObjectimage-renderingmarker-endominant-baselinem" +
	"arker-midmarker-startmaskerningmetadatamissing-glyph-orienta" +
	"tion-horizontalighting-color-profilepatternpointer-eventshap" +
	"e-renderingpointsolid-color-renderingpolygonpolylinepreserve" +
	"AspectRatioverflowhite-spacestop-opacitystroke-dasharraystro" +
	"ke-dashoffsetstroke-linecapaint-orderstroke-linejoinstroke-m" +
	"iterlimitstroke-opacitystroke-widthsvgswitchsymbolunicode-bi" +
	"directionusevector-effectversionviewBox2viewport-fill-opacit" +
	"yvisibilityword-spacingwriting-modefsolid-opacityxml:space")

var _Hash_table = [1 << 7]Hash{
	0x1:  0x2f303, // svg
	0x2:  0x2f606, // switch
	0x3:  0x18906, // marker
	0x5:  0x1620b, // font-weight
	0x6:  0x1801,  // y
	0x7:  0x33d0c, // fill-opacity
	0x8:  0x2e70c, // stroke-width
	0x9:  0x33415, // viewport-fill-opacity
	0xa:  0x9807,  // display
	0xb:  0xb110,  // contentStyleType
	0xc:  0x30c09, // direction
	0xd:  0x20407, // pattern
	0xf:  0x1f70d, // color-profile
	0x10: 0xc602,  // ry
	0x12: 0x1ad0c, // marker-start
	0x13: 0x1050d, // flood-opacity
	0x14: 0x36c0d, // solid-opacity
	0x15: 0x90e,   // letter-spacing
	0x16: 0x17a05, // image
	0x17: 0x1890a, // marker-end
	0x18: 0x2c811, // stroke-miterlimit
	0x19: 0x20b0e, // pointer-events
	0x1a: 0x12d0a, // stop-color
	0x1b: 0x1b904, // mask
	0x1c: 0xc904,  // fill
	0x1d: 0xc909,  // fill-rule
	0x1f: 0x26208, // overflow
	0x20: 0x1a30a, // marker-mid
	0x21: 0x1bc07, // kerning
	0x22: 0xa011,  // contentScriptType
	0x23: 0x4f02,  // rx
	0x24: 0x1560c, // font-variant
	0x25: 0x3020c, // unicode-bidi
	0x26: 0x2a10e, // stroke-linecap
	0x27: 0xc106,  // cursor
	0x28: 0x25013, // preserveAspectRatio
	0x29: 0x14507, // feImage
	0x2a: 0x2e12,  // alignment-baseline
	0x2b: 0x33202, // x2
	0x2c: 0x450b,  // text-anchor
	0x2d: 0x1d31c, // glyph-orientation-horizontal
	0x2e: 0xf206,  // script
	0x2f: 0x3180d, // vector-effect
	0x30: 0x1370c, // font-stretch
	0x31: 0x36904, // defs
	0x32: 0x29011, // stroke-dashoffset
	0x33: 0x6c06,  // height
	0x34: 0xd81b,  // color-interpolation-filters
	0x36: 0x2ae0b, // paint-order
	0x37: 0x19211, // dominant-baseline
	0x38: 0x11404, // font
	0x39: 0x24808, // polyline
	0x3a: 0x32c07, // viewBox
	0x3b: 0x4701,  // x
	0x3c: 0x11004, // type
	0x3d: 0x28010, // stroke-dasharray
	0x3e: 0x5002,  // x1
	0x3f: 0x5901,  // d
	0x42: 0x8811,  // enable-background
	0x43: 0x30e04, // rect
	0x44: 0x2180f, // shape-rendering
	0x46: 0x37909, // xml:space
	0x47: 0x161a,  // glyph-orientation-vertical
	0x48: 0x31503, // use
	0x49: 0x3490a, // visibility
	0x4a: 0x16d0d, // foreignObject
	0x4b: 0x710f,  // text-decoration
	0x4c: 0x15105, // style
	0x4f: 0x17a0f, // image-rendering
	0x50: 0x24107, // polygon
	0x51: 0x5212,  // buffered-rendering
	0x52: 0x1601,  // g
	0x53: 0x1140b, // font-family
	0x54: 0x14204, // href
	0x55: 0xec06,  // filter
	0x56: 0x1c308, // metadata
	0x57: 0x1ee0e, // lighting-color
	0x58: 0xf70e,  // text-rendering
	0x59: 0x35f0c, // writing-mode
	0x5a: 0x10b07, // opacity
	0x5b: 0x9e02,  // y1
	0x5c: 0xd20b,  // flood-color
	0x5d: 0x2690b, // white-space
	0x60: 0x101,   // a
	0x61: 0xb,     // baseProfile
	0x62: 0x14c0a, // font-style
	0x63: 0x3340d, // viewport-fill
	0x65: 0x8009,  // clip-rule
	0x67: 0x6904,  // path
	0x68: 0xd813,  // color-interpolation
	0x69: 0x2d90e, // stroke-opacity
	0x6a: 0x2b90f, // stroke-linejoin
	0x6b: 0x2ee05, // width
	0x6d: 0x2320f, // color-rendering
	0x6e: 0xc702,  // y2
	0x6f: 0x3c04,  // line
	0x70: 0x1cb0d, // missing-glyph
	0x71: 0x2740c, // stop-opacity
	0x72: 0x11f09, // font-size
	0x73: 0x380e,  // baseline-shift
	0x74: 0x3530c, // word-spacing
	0x75: 0x2fc06, // symbol
	0x76: 0x11f10, // font-size-adjust
	0x77: 0xd805,  // color
	0x79: 0x28006, // stroke
	0x7b: 0x22706, // points
	0x7c: 0x32507, // version
	0x7d: 0x6409,  // clip-path
	0x7e: 0x22c0b, // solid-color
	0x7f: 0x6404,  // clip
}
