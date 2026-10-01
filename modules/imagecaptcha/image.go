// Copyright 2011-2014 Dmitry Chestnykh. All rights reserved.
// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package imagecaptcha

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"strings"
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

func drawImage(rng *rand.Rand, digits []byte) *image.Paletted {
	img := &captchaImage{
		Paletted: image.NewPaletted(image.Rect(0, 0, imageWidth, imageHeight), newPalette(rng)),
		rng:      rng,
	}
	img.calculateSizes(len(digits))
	border := imageHeight / 5
	maxX := imageWidth - (img.numWidth+img.dotSize)*len(digits) - img.dotSize
	maxY := imageHeight - img.numHeight - img.dotSize*2
	x := img.randInt(border, maxX-border)
	y := img.randInt(border, maxY-border)
	for _, digit := range digits {
		img.drawDigit(digit, x, y)
		x += img.numWidth + img.dotSize
	}
	img.strikeThrough()
	img.distort(img.randFloat(5, 10), img.randFloat(100, 200))
	img.fillWithCircles(circleCount, img.dotSize)
	return img.Paletted
}

func newPalette(rng *rand.Rand) color.Palette {
	primary := primaryColors[rng.IntN(len(primaryColors))]
	palette := color.Palette{color.Transparent, primary}
	for range circleCount - 1 {
		palette = append(palette, randomBrightness(rng, primary))
	}
	return palette
}

func randomBrightness(rng *rand.Rand, c color.RGBA) color.RGBA {
	minChannel, maxChannel := min(c.R, c.G, c.B), max(c.R, c.G, c.B)
	shift := rng.IntN(math.MaxUint8-int(maxChannel)+1) - int(minChannel)
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
	deltaX := 1
	deltaY := -2 * radius
	offsetX := 0
	offsetY := radius

	img.SetColorIndex(x, y+radius, colorIndex)
	img.SetColorIndex(x, y-radius, colorIndex)
	img.drawHorizLine(x-radius, x+radius, y, colorIndex)

	for offsetX < offsetY {
		if decision >= 0 {
			offsetY--
			deltaY += 2
			decision += deltaY
		}
		offsetX++
		deltaX += 2
		decision += deltaX
		img.drawHorizLine(x-offsetX, x+offsetX, y+offsetY, colorIndex)
		img.drawHorizLine(x-offsetX, x+offsetX, y-offsetY, colorIndex)
		img.drawHorizLine(x-offsetY, x+offsetY, y+offsetX, colorIndex)
		img.drawHorizLine(x-offsetY, x+offsetY, y-offsetX, colorIndex)
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
	period := img.randFloat(80, 180)
	dx := 2.0 * math.Pi / period
	for x := range maxX {
		offsetX := amplitude * math.Cos(float64(y)*dx)
		offsetY := amplitude * math.Sin(float64(x)*dx)
		for row := range img.dotSize {
			radius := img.randInt(0, img.dotSize)
			img.drawCircle(x+int(offsetX), y+int(offsetY)+(row*img.dotSize), radius/2, 1)
		}
	}
}

func (img *captchaImage) drawDigit(digit byte, x, y int) {
	skew := img.randFloat(-maxSkew, maxSkew)
	skewedX := float64(x)
	radius := img.dotSize / 2
	y += img.randInt(-radius, radius)
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

var fontRows = strings.Fields(`
...#####...
..#######..
.###...###.
.##.....##.
###.....##.
##.......##
##.......##
##.......##
##.......##
##.......##
##.......##
##.......##
##.......##
##......###
.##.....##.
.###...###.
..#######..
...#####...

.....##....
....###....
...####....
..#####....
..##.##....
..#..##....
.....##....
.....##....
.....##....
.....##....
.....##....
.....##....
.....##....
.....##....
.....##....
.....##....
.##########
.##########

...####....
.########..
###....###.
.#......##.
........##.
........##.
........##.
.......##..
.......##..
......##...
.....##....
....###....
...###.....
..###......
.###.......
.##........
###########
###########

..######...
#########..
##.....###.
........##.
........##.
........##.
......###..
..#####....
..#######..
.......###.
........###
.........##
.........##
.........##
........###
#......###.
#########..
.#######...

.......##..
......###..
.....####..
.....#.##..
....##.##..
...##..##..
...##..##..
..##...##..
.##....##..
.##....##..
##.....##..
#......##..
###########
###########
.......##..
.......##..
.......##..
.......##..

.#########.
.#########.
.##........
.##........
.##........
.##........
.#######...
.########..
.......###.
........###
.........##
.........##
.........##
.........##
........###
##.....###.
#########..
..######...

.....#####.
...#######.
..###......
.##........
.##........
.#.........
##..####...
##.#######.
####....##.
###.....###
##.......##
##.......##
##.......##
##.......##
.##.....###
.###...###.
..#######..
...#####...

###########
###########
###......##
##......##.
........##.
.......###.
.......##..
.......##..
......##...
......##...
.....###...
.....##....
....###....
....##.....
....##.....
...###.....
...##......
..###......

...#####...
..########.
.###....###
.##......##
.##......##
.##......##
..##....##.
..#######..
....####...
..###.###..
.###...###.
###.....###
##.......##
##.......##
##.......##
###.....##.
.#########.
...#####...

...#####...
.########..
.##....###.
##......##.
##.......##
##.......##
##.......##
##......###
.##....####
.#######.##
...####..##
.........##
........##.
........##.
.......###.
......###..
.#######...
.#####.....
`)
