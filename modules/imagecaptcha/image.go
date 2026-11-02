// Copyright 2011-2014 Dmitry Chestnykh. All rights reserved.
// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package imagecaptcha

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

const (
	imageWidth  = 240
	imageHeight = 80
	fontWidth   = 11
	fontHeight  = 18
	maxSkew     = 0.7
	circleCount = 20
)

type captchaImage struct {
	*image.Paletted
	rng       *rand.Rand
	numWidth  int
	numHeight int
	dotSize   int
}

func drawImage(rng *rand.Rand, code string) *image.Paletted {
	img := &captchaImage{rng: rng}
	img.initPalette()
	img.calculateSizes(len(code))
	border := imageHeight / 5
	maxX := imageWidth - (img.numWidth+img.dotSize)*len(code) - img.dotSize
	maxY := imageHeight - img.numHeight - img.dotSize*2
	x := img.randInt(border, maxX-border)
	y := img.randInt(border, maxY-border)
	for i := range code {
		img.drawDigit(code[i], x, y)
		x += img.numWidth + img.dotSize
	}
	img.strikeThrough()
	img.distort(img.randFloat(5, 10), img.randFloat(100, 200))
	img.fillWithCircles(circleCount, img.dotSize)
	return img.Paletted
}

func (img *captchaImage) initPalette() {
	primary := primaryColors[img.rng.IntN(len(primaryColors))]
	palette := color.Palette{color.Transparent, primary}
	for range circleCount - 1 {
		palette = append(palette, img.randomBrightness(primary))
	}
	img.Paletted = image.NewPaletted(image.Rect(0, 0, imageWidth, imageHeight), palette)
}

func (img *captchaImage) randomBrightness(c color.RGBA) color.RGBA {
	minChannel, maxChannel := min(c.R, c.G, c.B), max(c.R, c.G, c.B)
	shift := img.rng.IntN(math.MaxUint8-int(maxChannel)+1) - int(minChannel)
	return color.RGBA{R: uint8(int(c.R) + shift), G: uint8(int(c.G) + shift), B: uint8(int(c.B) + shift), A: c.A}
}

func (img *captchaImage) randInt(from, to int) int {
	return img.rng.IntN(to+1-from) + from
}

func (img *captchaImage) randFloat(from, to float64) float64 {
	return (to-from)*img.rng.Float64() + from
}

func (img *captchaImage) calculateSizes(digitCount int) {
	border := imageHeight / 4
	width := float64(imageWidth - border*2)
	height := float64(imageHeight - border*2)
	glyphWidth := float64(fontWidth + 1)
	glyphHeight := float64(fontHeight)
	digitWidth := width / float64(digitCount)
	digitHeight := digitWidth * glyphHeight / glyphWidth
	if digitHeight > height {
		digitHeight = height
		digitWidth = glyphWidth / glyphHeight * digitHeight
	}
	img.dotSize = max(int(digitHeight/glyphHeight), 1)
	img.numWidth = int(digitWidth) - img.dotSize
	img.numHeight = int(digitHeight)
}

func (img *captchaImage) drawHorizLine(fromX, toX, y int, colorIndex uint8) {
	for x := fromX; x <= toX; x++ {
		img.SetColorIndex(x, y, colorIndex)
	}
}

func (img *captchaImage) drawCircle(x, y, radius int, colorIndex uint8) {
	decision := 1 - radius
	offsetY := radius
	for offsetX := 0; offsetX <= offsetY; offsetX++ {
		img.drawHorizLine(x-offsetX, x+offsetX, y+offsetY, colorIndex)
		img.drawHorizLine(x-offsetX, x+offsetX, y-offsetY, colorIndex)
		img.drawHorizLine(x-offsetY, x+offsetY, y+offsetX, colorIndex)
		img.drawHorizLine(x-offsetY, x+offsetY, y-offsetX, colorIndex)
		if decision >= 0 {
			offsetY--
			decision -= 2 * offsetY
		}
		decision += 2*(offsetX+1) + 1
	}
}

func (img *captchaImage) fillWithCircles(count, maxRadius int) {
	maxX, maxY := img.Bounds().Max.X, img.Bounds().Max.Y
	for range count {
		colorIndex := uint8(img.randInt(1, circleCount-1))
		radius := img.randInt(1, maxRadius)
		img.drawCircle(img.randInt(radius, maxX-radius), img.randInt(radius, maxY-radius), radius, colorIndex)
	}
}

func (img *captchaImage) strikeThrough() {
	maxX, maxY := img.Bounds().Max.X, img.Bounds().Max.Y
	y := img.randInt(maxY/3, maxY-maxY/3)
	amplitude := img.randFloat(5, 20)
	dx := 2.0 * math.Pi / img.randFloat(80, 180)
	offsetX := amplitude * math.Cos(float64(y)*dx)
	for x := range maxX {
		offsetY := amplitude * math.Sin(float64(x)*dx)
		for row := range img.dotSize {
			radius := img.randInt(0, img.dotSize)
			img.drawCircle(x+int(offsetX), y+int(offsetY)+(row*img.dotSize), radius/2, 1)
		}
	}
}

func (img *captchaImage) drawDigit(c byte, x, y int) {
	if c < '0' || c > '9' {
		return
	}
	digit := c - '0'
	skew := img.randFloat(-maxSkew, maxSkew)
	skewedX := float64(x)
	radius := img.dotSize / 2
	y += img.randInt(-radius, radius)
	fontRows := fontData()
	for row := range fontHeight {
		for col := range fontWidth {
			if fontRows[int(digit)*fontHeight+row][col] == '#' {
				img.drawCircle(x+col*img.dotSize, y+row*img.dotSize, radius, 1)
			}
		}
		skewedX += skew
		x = int(skewedX)
	}
}

func (img *captchaImage) distort(amplitude, period float64) {
	width, height := img.Bounds().Max.X, img.Bounds().Max.Y
	distorted := image.NewPaletted(image.Rect(0, 0, width, height), img.Palette)
	dx := 2.0 * math.Pi / period
	offsetsX := make([]int, height)
	for y := range height {
		offsetsX[y] = int(amplitude * math.Sin(float64(y)*dx))
	}
	for x := range width {
		offsetY := int(amplitude * math.Cos(float64(x)*dx))
		for y := range height {
			distorted.SetColorIndex(x, y, img.ColorIndexAt(x+offsetsX[y], y+offsetY))
		}
	}
	img.Paletted = distorted
}
