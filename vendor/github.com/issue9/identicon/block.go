// SPDX-License-Identifier: MIT

package identicon

import "image"

var (
	// 可以出现在中间的方块，一般为了美观，都是对称图像。
	centerBlocks = []blockFunc{b0, b1, b2, b3, b19, b26, b27}

	// 所有方块
	blocks = []blockFunc{b0, b1, b2, b3, b4, b5, b6, b7, b8, b9, b10, b11, b12, b13, b14, b15, b16, b17, b18, b19, b20, b21, b22, b23, b24, b25, b26, b27}
)

// 所有 block 函数的类型
type blockFunc func(img *image.Paletted, x, y, size int, angle int)

// 将多边形 points 旋转 angle 个角度，然后输出到 img 上，起点为 x,y 坐标
//
// points 中的坐标是基于左上角是原点的坐标系。
func drawBlock(img *image.Paletted, x, y, size int, angle int, points []int) {
	if angle > 0 { // 0 角度不需要转换
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

// 全空白
//
//  --------
//  |      |
//  |      |
//  |      |
//  --------
func b0(img *image.Paletted, x, y, size int, angle int) {}

// 全填充正方形
//
//  --------
//  |######|
//  |######|
//  |######|
//  --------
func b1(img *image.Paletted, x, y, size int, angle int) {
	for i := x; i < x+size; i++ {
		for j := y; j < y+size; j++ {
			img.SetColorIndex(i, j, 1)
		}
	}
}

// 中间小方块
//  ----------
//  |        |
//  |  ####  |
//  |  ####  |
//  |        |
//  ----------
func b2(img *image.Paletted, x, y, size int, angle int) {
	l := size / 4
	x = x + l
	y = y + l

	for i := x; i < x+2*l; i++ {
		for j := y; j < y+2*l; j++ {
			img.SetColorIndex(i, j, 1)
		}
	}
}

// 菱形
//
//  ---------
//  |   #   |
//  |  ###  |
//  | ##### |
//  |#######|
//  | ##### |
//  |  ###  |
//  |   #   |
//  ---------
func b3(img *image.Paletted, x, y, size int, angle int) {
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
//  -------
//  |#####|
//  |#### |
//  |###  |
//  |##   |
//  |#    |
//  |------
func b4(img *image.Paletted, x, y, size int, angle int) {
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		size, 0,
		0, size,
		0, 0,
	})
}

// b5
//
//  ---------
//  |   #   |
//  |  ###  |
//  | ##### |
//  |#######|
func b5(img *image.Paletted, x, y, size int, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		size, size,
		0, size,
		m, 0,
	})
}

// b6 矩形
//
//  --------
//  |###   |
//  |###   |
//  |###   |
//  --------
func b6(img *image.Paletted, x, y, size int, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		m, 0,
		m, size,
		0, size,
		0, 0,
	})
}

// b7 斜放的锥形
//
//  ---------
//  | #     |
//  |  ##   |
//  |  #####|
//  |   ####|
//  |--------
func b7(img *image.Paletted, x, y, size int, angle int) {
	m := size / 2
	drawBlock(img, x, y, size, angle, []int{
		0, 0,
		size, m,
		size, size,
		m, size,
		0, 0,
	})
}

// b8 三个堆叠的三角形
//
//  -----------
//  |    #    |
//  |   ###   |
//  |  #####  |
//  |  #   #  |
//  | ### ### |
//  |#########|
//  -----------
func b8(img *image.Paletted, x, y, size int, angle int) {
	m := size / 2
	mm := m / 2

	// 顶部三角形
	drawBlock(img, x, y, size, angle, []int{
		m, 0,
		3 * mm, m,
		mm, m,
		m, 0,
	})

	// 底下左边
	drawBlock(img, x, y, size, angle, []int{
		mm, m,
		m, size,
		0, size,
		mm, m,
	})

	// 底下右边
	drawBlock(img, x, y, size, angle, []int{
		3 * mm, m,
		size, size,
		m, size,
		3 * mm, m,
	})
}

// b9 斜靠的三角形
//
//  ---------
//  |#      |
//  | ####  |
//  |  #####|
//  |  #### |
//  |   #   |
//  ---------
func b9(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |    ####|
//  |    ### |
//  |    ##  |
//  |    #   |
//  |####    |
//  |###     |
//  |##      |
//  |#       |
//  ----------
func b10(img *image.Paletted, x, y, size int, angle int) {
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

// b11 左上角1/4大小的方块
//
//  ----------
//  |####    |
//  |####    |
//  |####    |
//  |        |
//  |        |
//  ----------
func b11(img *image.Paletted, x, y, size int, angle int) {
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
//  -----------
//  |         |
//  |         |
//  |#########|
//  |  #####  |
//  |    #    |
//  -----------
func b12(img *image.Paletted, x, y, size int, angle int) {
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
//  -----------
//  |         |
//  |         |
//  |    #    |
//  |  #####  |
//  |#########|
//  -----------
func b13(img *image.Paletted, x, y, size int, angle int) {
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
//  ---------
//  |   #   |
//  | ###   |
//  |####   |
//  |       |
//  |       |
//  ---------
func b14(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |#####   |
//  |###     |
//  |#       |
//  |        |
//  |        |
//  ----------
func b15(img *image.Paletted, x, y, size int, angle int) {
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
//  ---------
//  |   #   |
//  | ##### |
//  |#######|
//  |   #   |
//  | ##### |
//  |#######|
//  ---------
func b16(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |#####   |
//  |###     |
//  |#       |
//  |      ##|
//  |      ##|
//  ----------
func b17(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |#####   |
//  |####    |
//  |###     |
//  |##      |
//  |#       |
//  ----------
func b18(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |########|
//  |###  ###|
//  |#      #|
//  |###  ###|
//  |########|
//  ----------
func b19(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |  ##     |
//  |###      |
//  |##       |
//  |##       |
//  |#        |
//  ----------
func b20(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  | ####   |
//  |## #####|
//  |##    ##|
//  |##      |
//  |#       |
//  ----------
func b21(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  | ####   |
//  |##  ### |
//  |##    ##|
//  |##    ##|
//  |#      #|
//  ----------
func b22(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  | #######|
//  |###    #|
//  |##      |
//  |##      |
//  |#       |
//  ----------
func b23(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  | ##  ###|
//  |###  ###|
//  |##  ##  |
//  |##  ##  |
//  |#   #   |
//  ----------
func b24(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |#      #|
//  |##   ###|
//  |##  ##  |
//  |######  |
//  |####    |
//  ----------
func b25(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |#      #|
//  |###  ###|
//  |  ####  |
//  |###  ###|
//  |#      #|
//  ----------
func b26(img *image.Paletted, x, y, size int, angle int) {
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
//  ----------
//  |########|
//  |##   ###|
//  |#      #|
//  |###   ##|
//  |########|
//  ----------
func b27(img *image.Paletted, x, y, size int, angle int) {
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
