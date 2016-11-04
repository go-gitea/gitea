/*
Copyright (c) 2014, Charlie Vieth <charlie.vieth@gmail.com>

Permission to use, copy, modify, and/or distribute this software for any purpose
with or without fee is hereby granted, provided that the above copyright notice
and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND
FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS
OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER
TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF
THIS SOFTWARE.
*/

package resize

import (
	"image"
	"image/color"
)

// ycc is an in memory YCbCr image.  The Y, Cb and Cr samples are held in a
// single slice to increase resizing performance.
type ycc struct {
	// Pix holds the image's pixels, in Y, Cb, Cr order. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*3].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect image.Rectangle
	// SubsampleRatio is the subsample ratio of the original YCbCr image.
	SubsampleRatio image.YCbCrSubsampleRatio
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *ycc) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*3
}

func (p *ycc) Bounds() image.Rectangle {
	return p.Rect
}

func (p *ycc) ColorModel() color.Model {
	return color.YCbCrModel
}

func (p *ycc) At(x, y int) color.Color {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.YCbCr{}
	}
	i := p.PixOffset(x, y)
	return color.YCbCr{
		p.Pix[i+0],
		p.Pix[i+1],
		p.Pix[i+2],
	}
}

func (p *ycc) Opaque() bool {
	return true
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *ycc) SubImage(r image.Rectangle) image.Image {
	r = r.Intersect(p.Rect)
	if r.Empty() {
		return &ycc{SubsampleRatio: p.SubsampleRatio}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &ycc{
		Pix:            p.Pix[i:],
		Stride:         p.Stride,
		Rect:           r,
		SubsampleRatio: p.SubsampleRatio,
	}
}

// newYCC returns a new ycc with the given bounds and subsample ratio.
func newYCC(r image.Rectangle, s image.YCbCrSubsampleRatio) *ycc {
	w, h := r.Dx(), r.Dy()
	buf := make([]uint8, 3*w*h)
	return &ycc{Pix: buf, Stride: 3 * w, Rect: r, SubsampleRatio: s}
}

// YCbCr converts ycc to a YCbCr image with the same subsample ratio
// as the YCbCr image that ycc was generated from.
func (p *ycc) YCbCr() *image.YCbCr {
	ycbcr := image.NewYCbCr(p.Rect, p.SubsampleRatio)
	var off int

	switch ycbcr.SubsampleRatio {
	case image.YCbCrSubsampleRatio422:
		for y := ycbcr.Rect.Min.Y; y < ycbcr.Rect.Max.Y; y++ {
			yy := (y - ycbcr.Rect.Min.Y) * ycbcr.YStride
			cy := (y - ycbcr.Rect.Min.Y) * ycbcr.CStride
			for x := ycbcr.Rect.Min.X; x < ycbcr.Rect.Max.X; x++ {
				xx := (x - ycbcr.Rect.Min.X)
				yi := yy + xx
				ci := cy + xx/2
				ycbcr.Y[yi] = p.Pix[off+0]
				ycbcr.Cb[ci] = p.Pix[off+1]
				ycbcr.Cr[ci] = p.Pix[off+2]
				off += 3
			}
		}
	case image.YCbCrSubsampleRatio420:
		for y := ycbcr.Rect.Min.Y; y < ycbcr.Rect.Max.Y; y++ {
			yy := (y - ycbcr.Rect.Min.Y) * ycbcr.YStride
			cy := (y/2 - ycbcr.Rect.Min.Y/2) * ycbcr.CStride
			for x := ycbcr.Rect.Min.X; x < ycbcr.Rect.Max.X; x++ {
				xx := (x - ycbcr.Rect.Min.X)
				yi := yy + xx
				ci := cy + xx/2
				ycbcr.Y[yi] = p.Pix[off+0]
				ycbcr.Cb[ci] = p.Pix[off+1]
				ycbcr.Cr[ci] = p.Pix[off+2]
				off += 3
			}
		}
	case image.YCbCrSubsampleRatio440:
		for y := ycbcr.Rect.Min.Y; y < ycbcr.Rect.Max.Y; y++ {
			yy := (y - ycbcr.Rect.Min.Y) * ycbcr.YStride
			cy := (y/2 - ycbcr.Rect.Min.Y/2) * ycbcr.CStride
			for x := ycbcr.Rect.Min.X; x < ycbcr.Rect.Max.X; x++ {
				xx := (x - ycbcr.Rect.Min.X)
				yi := yy + xx
				ci := cy + xx
				ycbcr.Y[yi] = p.Pix[off+0]
				ycbcr.Cb[ci] = p.Pix[off+1]
				ycbcr.Cr[ci] = p.Pix[off+2]
				off += 3
			}
		}
	default:
		// Default to 4:4:4 subsampling.
		for y := ycbcr.Rect.Min.Y; y < ycbcr.Rect.Max.Y; y++ {
			yy := (y - ycbcr.Rect.Min.Y) * ycbcr.YStride
			cy := (y - ycbcr.Rect.Min.Y) * ycbcr.CStride
			for x := ycbcr.Rect.Min.X; x < ycbcr.Rect.Max.X; x++ {
				xx := (x - ycbcr.Rect.Min.X)
				yi := yy + xx
				ci := cy + xx
				ycbcr.Y[yi] = p.Pix[off+0]
				ycbcr.Cb[ci] = p.Pix[off+1]
				ycbcr.Cr[ci] = p.Pix[off+2]
				off += 3
			}
		}
	}
	return ycbcr
}

// imageYCbCrToYCC converts a YCbCr image to a ycc image for resizing.
func imageYCbCrToYCC(in *image.YCbCr) *ycc {
	w, h := in.Rect.Dx(), in.Rect.Dy()
	r := image.Rect(0, 0, w, h)
	buf := make([]uint8, 3*w*h)
	p := ycc{Pix: buf, Stride: 3 * w, Rect: r, SubsampleRatio: in.SubsampleRatio}
	var off int

	switch in.SubsampleRatio {
	case image.YCbCrSubsampleRatio422:
		for y := in.Rect.Min.Y; y < in.Rect.Max.Y; y++ {
			yy := (y - in.Rect.Min.Y) * in.YStride
			cy := (y - in.Rect.Min.Y) * in.CStride
			for x := in.Rect.Min.X; x < in.Rect.Max.X; x++ {
				xx := (x - in.Rect.Min.X)
				yi := yy + xx
				ci := cy + xx/2
				p.Pix[off+0] = in.Y[yi]
				p.Pix[off+1] = in.Cb[ci]
				p.Pix[off+2] = in.Cr[ci]
				off += 3
			}
		}
	case image.YCbCrSubsampleRatio420:
		for y := in.Rect.Min.Y; y < in.Rect.Max.Y; y++ {
			yy := (y - in.Rect.Min.Y) * in.YStride
			cy := (y/2 - in.Rect.Min.Y/2) * in.CStride
			for x := in.Rect.Min.X; x < in.Rect.Max.X; x++ {
				xx := (x - in.Rect.Min.X)
				yi := yy + xx
				ci := cy + xx/2
				p.Pix[off+0] = in.Y[yi]
				p.Pix[off+1] = in.Cb[ci]
				p.Pix[off+2] = in.Cr[ci]
				off += 3
			}
		}
	case image.YCbCrSubsampleRatio440:
		for y := in.Rect.Min.Y; y < in.Rect.Max.Y; y++ {
			yy := (y - in.Rect.Min.Y) * in.YStride
			cy := (y/2 - in.Rect.Min.Y/2) * in.CStride
			for x := in.Rect.Min.X; x < in.Rect.Max.X; x++ {
				xx := (x - in.Rect.Min.X)
				yi := yy + xx
				ci := cy + xx
				p.Pix[off+0] = in.Y[yi]
				p.Pix[off+1] = in.Cb[ci]
				p.Pix[off+2] = in.Cr[ci]
				off += 3
			}
		}
	default:
		// Default to 4:4:4 subsampling.
		for y := in.Rect.Min.Y; y < in.Rect.Max.Y; y++ {
			yy := (y - in.Rect.Min.Y) * in.YStride
			cy := (y - in.Rect.Min.Y) * in.CStride
			for x := in.Rect.Min.X; x < in.Rect.Max.X; x++ {
				xx := (x - in.Rect.Min.X)
				yi := yy + xx
				ci := cy + xx
				p.Pix[off+0] = in.Y[yi]
				p.Pix[off+1] = in.Cb[ci]
				p.Pix[off+2] = in.Cr[ci]
				off += 3
			}
		}
	}
	return &p
}
