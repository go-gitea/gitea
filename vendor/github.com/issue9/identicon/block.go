// Copyright 2015 by caixw, All rights reserved
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package identicon

import (
	"image"
	"sync"
)

var pool = sync.Pool{
	New: func() interface{} { return make([]float64, 0, 10) },
}

var (
	// 可以出现在中间的方块，一般为了美观，都是对称图像。
	centerBlocks = []blockFunc{b0, b1, b2, b3}

	// 所有方块
	blocks = []blockFunc{b0, b1, b2, b3, b4, b5, b6, b7, b8, b9, b10, b11, b12, b13, b14, b15, b16}
)

// 所有block函数的类型
type blockFunc func(img *image.Paletted, x, y, size float64, angle int)

// 将多边形points旋转angle个角度，然后输出到img上，起点为x,y坐标
func drawBlock(img *image.Paletted, x, y, size float64, angle int, points []float64) {
	if angle > 0 { // 0角度不需要转换
		// 中心坐标与x,y的距离，方便下面指定中心坐标(x+m,y+m)，
		// 0.5的偏移值不能少，否则坐靠右，非正中央
		m := size/2 - 0.5
		rotate(points, x+m, y+m, angle)
	}

	for i := x; i < x+size; i++ {
		for j := y; j < y+size; j++ {
			if pointInPolygon(i, j, points) {
				img.SetColorIndex(int(i), int(j), 1)
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
func b0(img *image.Paletted, x, y, size float64, angle int) {
}

// 全填充正方形
//
//  --------
//  |######|
//  |######|
//  |######|
//  --------
func b1(img *image.Paletted, x, y, size float64, angle int) {
	isize := int(size)
	ix := int(x)
	iy := int(y)
	for i := ix + 1; i < ix+isize; i++ {
		for j := iy + 1; j < iy+isize; j++ {
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
func b2(img *image.Paletted, x, y, size float64, angle int) {
	l := size / 4
	x = x + l
	y = y + l

	for i := x; i < x+2*l; i++ {
		for j := y; j < y+2*l; j++ {
			img.SetColorIndex(int(i), int(j), 1)
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
func b3(img *image.Paletted, x, y, size float64, angle int) {
	m := size / 2
	points := pool.Get().([]float64)[:0]

	drawBlock(img, x, y, size, 0, append(points,
		x+m, y,
		x+size, y+m,
		x+m, y+size,
		x, y+m,
		x+m, y,
	))

	pool.Put(points)
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
func b4(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	drawBlock(img, x, y, size, angle, append(points,
		x, y,
		x+size, y,
		x, y+size,
		x, y,
	))

	pool.Put(points)
}

// b5
//
//  ---------
//  |   #   |
//  |  ###  |
//  | ##### |
//  |#######|
func b5(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x+m, y,
		x+size,
		y+size,
		x, y+size,
		x+m, y,
	))

	pool.Put(points)
}

// b6 矩形
//
//  --------
//  |###   |
//  |###   |
//  |###   |
//  --------
func b6(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x, y,
		x+m, y,
		x+m, y+size,
		x, y+size,
		x, y,
	))

	pool.Put(points)
}

// b7 斜放的锥形
//
//  ---------
//  | #     |
//  |  ##   |
//  |  #####|
//  |   ####|
//  |--------
func b7(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x, y,
		x+size, y+m,
		x+size, y+size,
		x+m, y+size,
		x, y,
	))

	pool.Put(points)
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
func b8(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	mm := m / 2

	// 顶部三角形
	drawBlock(img, x, y, size, angle, append(points,
		x+m, y,
		x+3*mm, y+m,
		x+mm, y+m,
		x+m, y,
	))

	// 底下左边
	drawBlock(img, x, y, size, angle, append(points[:0],
		x+mm, y+m,
		x+m, y+size,
		x, y+size,
		x+mm, y+m,
	))

	// 底下右边
	drawBlock(img, x, y, size, angle, append(points[:0],
		x+3*mm, y+m,
		x+size, y+size,
		x+m, y+size,
		x+3*mm, y+m,
	))

	pool.Put(points)
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
func b9(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x, y,
		x+size, y+m,
		x+m, y+size,
		x, y,
	))

	pool.Put(points)
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
func b10(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x+m, y,
		x+size, y,
		x+m, y+m,
		x+m, y,
	))

	drawBlock(img, x, y, size, angle, append(points[:0],
		x, y+m,
		x+m, y+m,
		x, y+size,
		x, y+m,
	))

	pool.Put(points)
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
func b11(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x, y,
		x+m, y,
		x+m, y+m,
		x, y+m,
		x, y,
	))

	pool.Put(points)
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
func b12(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x, y+m,
		x+size, y+m,
		x+m, y+size,
		x, y+m,
	))

	pool.Put(points)
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
func b13(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x+m, y+m,
		x+size, y+size,
		x, y+size,
		x+m, y+m,
	))

	pool.Put(points)
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
func b14(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x+m, y,
		x+m, y+m,
		x, y+m,
		x+m, y,
	))

	pool.Put(points)
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
func b15(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x, y,
		x+m, y,
		x, y+m,
		x, y,
	))

	pool.Put(points)
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
func b16(img *image.Paletted, x, y, size float64, angle int) {
	points := pool.Get().([]float64)[:0]
	m := size / 2
	drawBlock(img, x, y, size, angle, append(points,
		x+m, y,
		x+size, y+m,
		x, y+m,
		x+m, y,
	))

	drawBlock(img, x, y, size, angle, append(points[:0],
		x+m, y+m,
		x+size, y+size,
		x, y+size,
		x+m, y+m,
	))

	pool.Put(points)
}
