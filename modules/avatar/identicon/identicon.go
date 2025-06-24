// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Copied and modified from https://github.com/issue9/identicon/ (MIT License)
// Generate pseudo-random avatars by IP, E-mail, etc.

package identicon

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
)

const minImageSize = 16

// Identicon is used to generate pseudo-random avatars
type Identicon struct {
	foreColors []color.Color
	backColor  color.Color
	size       int
	rect       image.Rectangle
}

// New returns an Identicon struct with the correct settings
// size image size
// back background color
// fore all possible foreground colors. only one foreground color will be picked randomly for one image
func New(size int, back color.Color, fore ...color.Color) (*Identicon, error) {
	if len(fore) == 0 {
		return nil, errors.New("foreground is not set")
	}

	if size < minImageSize {
		return nil, fmt.Errorf("size %d is smaller than min size %d", size, minImageSize)
	}

	return &Identicon{
		foreColors: fore,
		backColor:  back,
		size:       size,
		rect:       image.Rect(0, 0, size, size),
	}, nil
}

// Make generates an avatar by data
func (i *Identicon) Make(data []byte) image.Image {
	h := sha256.New()
	h.Write(data)
	sum := h.Sum(nil)

	b1 := int(sum[0]+sum[1]+sum[2]) % len(blocks)
	b2 := int(sum[3]+sum[4]+sum[5]) % len(blocks)
	c := int(sum[6]+sum[7]+sum[8]) % len(centerBlocks)
	b1Angle := int(sum[9]+sum[10]) % 4
	b2Angle := int(sum[11]+sum[12]) % 4
	foreColor := int(sum[11]+sum[12]+sum[15]) % len(i.foreColors)

	return i.render(c, b1, b2, b1Angle, b2Angle, foreColor)
}

func (i *Identicon) render(c, b1, b2, b1Angle, b2Angle, foreColor int) image.Image {
	p := image.NewPaletted(i.rect, []color.Color{i.backColor, i.foreColors[foreColor]})
	drawBlocks(p, i.size, centerBlocks[c], blocks[b1], blocks[b2], b1Angle, b2Angle)
	return p
}

/*
# Algorithm

Origin: An image is splitted into 9 areas

```
  -------------
  | 1 | 2 | 3 |
  -------------
  | 4 | 5 | 6 |
  -------------
  | 7 | 8 | 9 |
  -------------
```

Area 1/3/9/7 use a 90-degree rotating pattern.
Area 1/3/9/7 use another 90-degree rotating pattern.
Area 5 uses a random pattern.

The Patched Fix: make the image left-right mirrored to get rid of something like "swastika"
*/

// draw blocks to the paletted
// c: the block drawer for the center block
// b1,b2: the block drawers for other blocks (around the center block)
// b1Angle,b2Angle: the angle for the rotation of b1/b2
func drawBlocks(p *image.Paletted, size int, c, b1, b2 blockFunc, b1Angle, b2Angle int) {
	nextAngle := func(a int) int {
		return (a + 1) % 4
	}

	padding := (size % 3) / 2 // in cased the size can not be aligned by 3 blocks.

	blockSize := size / 3
	twoBlockSize := 2 * blockSize

	// center
	c(p, blockSize+padding, blockSize+padding, blockSize, 0)

	// left top (1)
	b1(p, 0+padding, 0+padding, blockSize, b1Angle)
	// center top (2)
	b2(p, blockSize+padding, 0+padding, blockSize, b2Angle)

	b1Angle = nextAngle(b1Angle)
	b2Angle = nextAngle(b2Angle)
	// right top (3)
	// b1(p, twoBlockSize+padding, 0+padding, blockSize, b1Angle)
	// right middle (6)
	// b2(p, twoBlockSize+padding, blockSize+padding, blockSize, b2Angle)

	b1Angle = nextAngle(b1Angle)
	b2Angle = nextAngle(b2Angle)
	// right bottom (9)
	// b1(p, twoBlockSize+padding, twoBlockSize+padding, blockSize, b1Angle)
	// center bottom (8)
	b2(p, blockSize+padding, twoBlockSize+padding, blockSize, b2Angle)

	b1Angle = nextAngle(b1Angle)
	b2Angle = nextAngle(b2Angle)
	// lef bottom (7)
	b1(p, 0+padding, twoBlockSize+padding, blockSize, b1Angle)
	// left middle (4)
	b2(p, 0+padding, blockSize+padding, blockSize, b2Angle)

	// then we make it left-right mirror, so we didn't draw 3/6/9 before
	for x := 0; x < size/2; x++ {
		for y := range size {
			p.SetColorIndex(size-x, y, p.ColorIndexAt(x, y))
		}
	}
}
