package excelize

import "encoding/xml"

// xlsxTypes directly maps the types element of content types for relationship
// parts, it takes a Multipurpose Internet Mail Extension (MIME) media type as a
// value.
type xlsxTypes struct {
	XMLName   xml.Name       `xml:"http://schemas.openxmlformats.org/package/2006/content-types Types"`
	Overrides []xlsxOverride `xml:"Override"`
	Defaults  []xlsxDefault  `xml:"Default"`
}

// xlsxOverride directly maps the override element in the namespace
// http://schemas.openxmlformats.org/package/2006/content-types
type xlsxOverride struct {
	PartName    string `xml:",attr"`
	ContentType string `xml:",attr"`
}

// xlsxDefault directly maps the default element in the namespace
// http://schemas.openxmlformats.org/package/2006/content-types
type xlsxDefault struct {
	Extension   string `xml:",attr"`
	ContentType string `xml:",attr"`
}
