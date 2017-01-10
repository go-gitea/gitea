// Package utils contain some utilities which are needed to create barcodes
package utils

import (
	"image"
	"image/color"

	"github.com/boombuler/barcode"
)

type base1DCode struct {
	*BitList
	kind     string
	content  string
	checksum int
}

func (c *base1DCode) Content() string {
	return c.content
}

func (c *base1DCode) Metadata() barcode.Metadata {
	return barcode.Metadata{c.kind, 1}
}

func (c *base1DCode) ColorModel() color.Model {
	return color.Gray16Model
}

func (c *base1DCode) Bounds() image.Rectangle {
	return image.Rect(0, 0, c.Len(), 1)
}

func (c *base1DCode) At(x, y int) color.Color {
	if c.GetBit(x) {
		return color.Black
	}
	return color.White
}

func (c *base1DCode) CheckSum() int {
	return c.checksum
}

// New1DCode creates a new 1D barcode where the bars are represented by the bits in the bars BitList
func New1DCode(codeKind, content string, bars *BitList, checksum int) barcode.Barcode {
	return &base1DCode{bars, codeKind, content, checksum}
}
