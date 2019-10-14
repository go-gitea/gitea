package barcode

import "image"

const (
	TypeAztec           = "Aztec"
	TypeCodabar         = "Codabar"
	TypeCode128         = "Code 128"
	TypeCode39          = "Code 39"
	TypeCode93          = "Code 93"
	TypeDataMatrix      = "DataMatrix"
	TypeEAN8            = "EAN 8"
	TypeEAN13           = "EAN 13"
	TypePDF             = "PDF417"
	TypeQR              = "QR Code"
	Type2of5            = "2 of 5"
	Type2of5Interleaved = "2 of 5 (interleaved)"
)

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
}

// Additional interface that some barcodes might implement to provide
// the value of its checksum.
type BarcodeIntCS interface {
	Barcode
	CheckSum() int
}
