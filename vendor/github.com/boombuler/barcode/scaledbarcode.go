package barcode

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
)

type wrapFunc func(x, y int) color.Color

type scaledBarcode struct {
	wrapped     Barcode
	wrapperFunc wrapFunc
	rect        image.Rectangle
}

type intCSscaledBC struct {
	scaledBarcode
}

func (bc *scaledBarcode) Content() string {
	return bc.wrapped.Content()
}

func (bc *scaledBarcode) Metadata() Metadata {
	return bc.wrapped.Metadata()
}

func (bc *scaledBarcode) ColorModel() color.Model {
	return bc.wrapped.ColorModel()
}

func (bc *scaledBarcode) Bounds() image.Rectangle {
	return bc.rect
}

func (bc *scaledBarcode) At(x, y int) color.Color {
	return bc.wrapperFunc(x, y)
}

func (bc *intCSscaledBC) CheckSum() int {
	if cs, ok := bc.wrapped.(BarcodeIntCS); ok {
		return cs.CheckSum()
	}
	return 0
}

// Scale returns a resized barcode with the given width and height.
func Scale(bc Barcode, width, height int) (Barcode, error) {
	switch bc.Metadata().Dimensions {
	case 1:
		return scale1DCode(bc, width, height)
	case 2:
		return scale2DCode(bc, width, height)
	}

	return nil, errors.New("unsupported barcode format")
}

func newScaledBC(wrapped Barcode, wrapperFunc wrapFunc, rect image.Rectangle) Barcode {
	result := &scaledBarcode{
		wrapped:     wrapped,
		wrapperFunc: wrapperFunc,
		rect:        rect,
	}

	if _, ok := wrapped.(BarcodeIntCS); ok {
		return &intCSscaledBC{*result}
	}
	return result
}

func scale2DCode(bc Barcode, width, height int) (Barcode, error) {
	orgBounds := bc.Bounds()
	orgWidth := orgBounds.Max.X - orgBounds.Min.X
	orgHeight := orgBounds.Max.Y - orgBounds.Min.Y

	factor := int(math.Min(float64(width)/float64(orgWidth), float64(height)/float64(orgHeight)))
	if factor <= 0 {
		return nil, fmt.Errorf("can not scale barcode to an image smaller than %dx%d", orgWidth, orgHeight)
	}

	offsetX := (width - (orgWidth * factor)) / 2
	offsetY := (height - (orgHeight * factor)) / 2

	wrap := func(x, y int) color.Color {
		if x < offsetX || y < offsetY {
			return color.White
		}
		x = (x - offsetX) / factor
		y = (y - offsetY) / factor
		if x >= orgWidth || y >= orgHeight {
			return color.White
		}
		return bc.At(x, y)
	}

	return newScaledBC(
		bc,
		wrap,
		image.Rect(0, 0, width, height),
	), nil
}

func scale1DCode(bc Barcode, width, height int) (Barcode, error) {
	orgBounds := bc.Bounds()
	orgWidth := orgBounds.Max.X - orgBounds.Min.X
	factor := int(float64(width) / float64(orgWidth))

	if factor <= 0 {
		return nil, fmt.Errorf("can not scale barcode to an image smaller than %dx1", orgWidth)
	}
	offsetX := (width - (orgWidth * factor)) / 2

	wrap := func(x, y int) color.Color {
		if x < offsetX {
			return color.White
		}
		x = (x - offsetX) / factor

		if x >= orgWidth {
			return color.White
		}
		return bc.At(x, 0)
	}

	return newScaledBC(
		bc,
		wrap,
		image.Rect(0, 0, width, height),
	), nil
}
