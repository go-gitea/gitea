package barcode

import "image"

// Contains some meta information about a barcode
type Metadata struct {
	// the name of the barcode kind
	CodeKind string
	// contains 1 for 1D barcodes or 2 for 2D barcodes
	Dimensions byte
}

// a rendered and encoded barcode
type Barcode interface {
	image.Image
	// returns some meta information about the barcode
	Metadata() Metadata
	// the data that was encoded in this barcode
	Content() string
	CheckSum() int
}
