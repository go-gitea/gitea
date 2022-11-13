// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Copied and modified from https://github.com/issue9/identicon/ (MIT License)

package identicon

import "image"

var (
	// the blocks can appear in center, these blocks can be more beautiful
	centerBlocks = []blockFunc{b0, b1, b2, b3, b19, b26, b27}

	// all blocks
	blocks = []blockFunc{b0, b1, b2, b3, b4, b5, b6, b7, b8, b9, b10, b11, b12, b13, b14, b15, b16, b17, b18, b19, b20, b21, b22, b23, b24, b25, b26, b27}
)

type blockFunc func(img *image.Paletted, x, y, size, angle int)

// draw a polygon by points, and the polygon is rotated by angle.
func drawBlock(img *image.Paletted, x, y, size, angle int, points []int) {
	if angle != 0 {
		m := size / 2
		rotate(points, m, m, angle)
	}

	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			if pointInPolygon(i, j, points) {
				img.SetColorIndex(x+i, y+j, 1)
			}
		}
	}
}

// blank
//
//	--------
//	|      |
//	|      |
//	|      |
//	--------
func b0(img *image.Paletted, x, y, size, angle int) {}

// full-filled
//
//	--------
//	|######|
//	|######|
//	|######|
//	--------
func b1(img *image.Paletted, x, y, size, angle int) {
	for i := x; i < x+size; i++ {
		for j := y; j < y+size; j++ {
			img.SetColorIndex(i, j, 1)
		}
	}
}

// a small block
//
//	----------
//	|        |
//	|  ####  |
//	|  ####  |
//	|        |
//	----------
func b2(img *image.Paletted, x, y, size, angle int) {
	l := size / 4
	x += l
	y += l

	for i := x; i < x+2*l; i++ {
		for j := y; j < y+2*l; j++ {
			img.SetColorIndex(i, j, 1)
		}
	}
}

// diamond
//
//	---------
//	|   #   |
//	|  ###  |
//	| ##### |
//	|#######|
//	| ##### |
//	|  ###  |
//	|   #   |
//	---------
func b3(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, 0, []int{
		m, 0,
		size, m,
		m, size,
		0, m,
		m, 0,
	})
}

// b4
//
//	-------
//	|#####|
//	|#### |
//	|###  |
//	|##   |
//	|#    |
//	|------
func b4(img *image.Paletted, x, y, size, angle int) {
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		size, 0,
		0, size,
		0, 0,
	})
}

// b5
//
//	---------
//	|   #   |
//	|  ###  |
//	| ##### |
//	|#######|
func b5(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		size, size,
		0, size,
		m, 0,
	})
}

// b6
//
//	--------
//	|###   |
//	|###   |
//	|###   |
//	--------
func b6(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		m, 0,
		m, size,
		0, size,
		0, 0,
	})
}

// b7 italic cone
//
//	---------
//	| #     |
//	|  ##   |
//	|  #####|
//	|   ####|
//	|--------
func b7(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		size, m,
		size, size,
		m, size,
		0, 0,
	})
}

// b8 three small triangles
//
//	-----------
//	|    #    |
//	|   ###   |
//	|  #####  |
//	|  #   #  |
//	| ### ### |
//	|#########|
//	-----------
func b8(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	mm := m / 2

	// top
	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		3 * mm, m,
		mm, m,
		m, 0,
	})

	// bottom left
	drawBlock(img, x, y, size, angle, []int{
		mm, m,
		m, size,
		0, size,
		mm, m,
	})

	// bottom right
	drawBlock(img, x, y, size, angle, []int{
		3 * mm, m,
		size, size,
		m, size,
		3 * mm, m,
	})
}

// b9 italic triangle
//
//	---------
//	|#      |
//	| ####  |
//	|  #####|
//	|  #### |
//	|   #   |
//	---------
func b9(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		size, m,
		m, size,
		0, 0,
	})
}

// b10
//
//	----------
//	|    ####|
//	|    ### |
//	|    ##  |
//	|    #   |
//	|####    |
//	|###     |
//	|##      |
//	|#       |
//	----------
func b10(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		size, 0,
		m, m,
		m, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		0, m,
		m, m,
		0, size,
		0, m,
	})
}

// b11
//
//	----------
//	|####    |
//	|####    |
//	|####    |
//	|        |
//	|        |
//	----------
func b11(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		m, 0,
		m, m,
		0, m,
		0, 0,
	})
}

// b12
//
//	-----------
//	|         |
//	|         |
//	|#########|
//	|  #####  |
//	|    #    |
//	-----------
func b12(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		0, m,
		size, m,
		m, size,
		0, m,
	})
}

// b13
//
//	-----------
//	|         |
//	|         |
//	|    #    |
//	|  #####  |
//	|#########|
//	-----------
func b13(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		m, m,
		size, size,
		0, size,
		m, m,
	})
}

// b14
//
//	---------
//	|   #   |
//	| ###   |
//	|####   |
//	|       |
//	|       |
//	---------
func b14(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		m, m,
		0, m,
		m, 0,
	})
}

// b15
//
//	----------
//	|#####   |
//	|###     |
//	|#       |
//	|        |
//	|        |
//	----------
func b15(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		m, 0,
		0, m,
		0, 0,
	})
}

// b16
//
//	---------
//	|   #   |
//	| ##### |
//	|#######|
//	|   #   |
//	| ##### |
//	|#######|
//	---------
func b16(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		size, m,
		0, m,
		m, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		m, m,
		size, size,
		0, size,
		m, m,
	})
}

// b17
//
//	----------
//	|#####   |
//	|###     |
//	|#       |
//	|      ##|
//	|      ##|
//	----------
func b17(img *image.Paletted, x, y, size, angle int) {
	m := size / 2

	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		m, 0,
		0, m,
		0, 0,
	})

	quarter := size / 4
	drawBlock(img, x, y, size, angle, []int{
		size - quarter, size - quarter,
		size, size - quarter,
		size, size,
		size - quarter, size,
		size - quarter, size - quarter,
	})
}

// b18
//
//	----------
//	|#####   |
//	|####    |
//	|###     |
//	|##      |
//	|#       |
//	----------
func b18(img *image.Paletted, x, y, size, angle int) {
	m := size / 2

	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		m, 0,
		0, size,
		0, 0,
	})
}

// b19
//
//	----------
//	|########|
//	|###  ###|
//	|#      #|
//	|###  ###|
//	|########|
//	----------
func b19(img *image.Paletted, x, y, size, angle int) {
	m := size / 2

	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		m, 0,
		0, m,
		0, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		size, 0,
		size, m,
		m, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		size, m,
		size, size,
		m, size,
		size, m,
	})

	drawBlock(img, x, y, size, angle, []int{
		0, m,
		m, size,
		0, size,
		0, m,
	})
}

// b20
//
//	----------
//	|  ##     |
//	|###      |
//	|##       |
//	|##       |
//	|#        |
//	----------
func b20(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	q := size / 4

	drawBlock(img, x, y, size, angle, []int{
		q, 0,
		0, size,
		0, m,
		q, 0,
	})
}

// b21
//
//	----------
//	| ####   |
//	|## #####|
//	|##    ##|
//	|##      |
//	|#       |
//	----------
func b21(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	q := size / 4

	drawBlock(img, x, y, size, angle, []int{
		q, 0,
		0, size,
		0, m,
		q, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		q, 0,
		size, q,
		size, m,
		q, 0,
	})
}

// b22
//
//	----------
//	| ####   |
//	|##  ### |
//	|##    ##|
//	|##    ##|
//	|#      #|
//	----------
func b22(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	q := size / 4

	drawBlock(img, x, y, size, angle, []int{
		q, 0,
		0, size,
		0, m,
		q, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		q, 0,
		size, q,
		size, size,
		q, 0,
	})
}

// b23
//
//	----------
//	| #######|
//	|###    #|
//	|##      |
//	|##      |
//	|#       |
//	----------
func b23(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	q := size / 4

	drawBlock(img, x, y, size, angle, []int{
		q, 0,
		0, size,
		0, m,
		q, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		q, 0,
		size, 0,
		size, q,
		q, 0,
	})
}

// b24
//
//	----------
//	| ##  ###|
//	|###  ###|
//	|##  ##  |
//	|##  ##  |
//	|#   #   |
//	----------
func b24(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	q := size / 4

	drawBlock(img, x, y, size, angle, []int{
		q, 0,
		0, size,
		0, m,
		q, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		size, 0,
		m, size,
		m, 0,
	})
}

// b25
//
//	----------
//	|#      #|
//	|##   ###|
//	|##  ##  |
//	|######  |
//	|####    |
//	----------
func b25(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	q := size / 4

	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		0, size,
		q, size,
		0, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		0, m,
		size, 0,
		q, size,
		0, m,
	})
}

// b26
//
//	----------
//	|#      #|
//	|###  ###|
//	|  ####  |
//	|###  ###|
//	|#      #|
//	----------
func b26(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	q := size / 4

	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		m, q,
		q, m,
		0, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		size, 0,
		m + q, m,
		m, q,
		size, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		size, size,
		m, m + q,
		q + m, m,
		size, size,
	})

	drawBlock(img, x, y, size, angle, []int{
		0, size,
		q, m,
		m, q + m,
		0, size,
	})
}

// b27
//
//	----------
//	|########|
//	|##   ###|
//	|#      #|
//	|###   ##|
//	|########|
//	----------
func b27(img *image.Paletted, x, y, size, angle int) {
	m := size / 2
	q := size / 4

	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		size, 0,
		0, q,
		0, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		q + m, 0,
		size, 0,
		size, size,
		q + m, 0,
	})

	drawBlock(img, x, y, size, angle, []int{
		size, q + m,
		size, size,
		0, size,
		size, q + m,
	})

	drawBlock(img, x, y, size, angle, []int{
		0, size,
		0, 0,
		q, size,
		0, size,
	})
}
