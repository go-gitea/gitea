// SPDX-License-Identifier: MIT

package identicon

import (
	"crypto/md5"
	"fmt"
	"image"
	"image/color"
	"math/rand"
)

const (
	minSize       = 16 // 图片的最小尺寸
	maxForeColors = 32 // 在New()函数中可以指定的最大颜色数量
)

// Identicon 用于产生统一尺寸的头像
//
// 可以根据用户提供的数据，经过一定的算法，自动产生相应的图案和颜色。
type Identicon struct {
	foreColors []color.Color
	backColor  color.Color
	size       int
	rect       image.Rectangle
}

// New 声明一个 Identicon 实例
//
// size 表示整个头像的大小；
// back 表示前景色；
// fore 表示所有可能的前景色，会为每个图像随机挑选一个作为其前景色。
func New(size int, back color.Color, fore ...color.Color) (*Identicon, error) {
	if len(fore) == 0 || len(fore) > maxForeColors {
		return nil, fmt.Errorf("前景色数量必须介于[1]~[%d]之间，当前为[%d]", maxForeColors, len(fore))
	}

	if size < minSize {
		return nil, fmt.Errorf("参数 size 的值(%d)不能小于 %d", size, minSize)
	}

	return &Identicon{
		foreColors: fore,
		backColor:  back,
		size:       size,

		// 画布坐标从0开始，其长度应该是 size-1
		rect: image.Rect(0, 0, size, size),
	}, nil
}

// Make 根据 data 数据产生一张唯一性的头像图片
func (i *Identicon) Make(data []byte) image.Image {
	h := md5.New()
	h.Write(data)
	sum := h.Sum(nil)

	b1 := int(sum[0]+sum[1]+sum[2]) % len(blocks)
	b2 := int(sum[3]+sum[4]+sum[5]) % len(blocks)
	c := int(sum[6]+sum[7]+sum[8]) % len(centerBlocks)
	b1Angle := int(sum[9]+sum[10]) % 4
	b2Angle := int(sum[11]+sum[12]) % 4
	color := int(sum[11]+sum[12]+sum[15]) % len(i.foreColors)

	return i.render(c, b1, b2, b1Angle, b2Angle, color)
}

// Rand 随机生成图案
func (i *Identicon) Rand(r *rand.Rand) image.Image {
	b1 := r.Intn(len(blocks))
	b2 := r.Intn(len(blocks))
	c := r.Intn(len(centerBlocks))
	b1Angle := r.Intn(4)
	b2Angle := r.Intn(4)
	color := r.Intn(len(i.foreColors))

	return i.render(c, b1, b2, b1Angle, b2Angle, color)
}

func (i *Identicon) render(c, b1, b2, b1Angle, b2Angle, foreColor int) image.Image {
	p := image.NewPaletted(i.rect, []color.Color{i.backColor, i.foreColors[foreColor]})
	drawBlocks(p, i.size, centerBlocks[c], blocks[b1], blocks[b2], b1Angle, b2Angle)
	return p
}

// Make 根据 data 数据产生一张唯一性的头像图片
//
// size 头像的大小。
// back, fore头像的背景和前景色。
func Make(size int, back, fore color.Color, data []byte) (image.Image, error) {
	i, err := New(size, back, fore)
	if err != nil {
		return nil, err
	}
	return i.Make(data), nil
}

// 将九个方格都填上内容。
// p 为画板；
// c 为中间方格的填充函数；
// b1、b2 为边上 8 格的填充函数；
// b1Angle 和 b2Angle 为 b1、b2 的起始旋转角度。
func drawBlocks(p *image.Paletted, size int, c, b1, b2 blockFunc, b1Angle, b2Angle int) {
	incr := func(a int) int {
		if a >= 3 {
			a = 0
		} else {
			a++
		}
		return a
	}

	padding := (size % 6) / 2 // 不能除尽的，边上留白。

	blockSize := size / 3
	twoBlockSize := 2 * blockSize

	c(p, blockSize+padding, blockSize+padding, blockSize, 0)

	b1(p, 0+padding, 0+padding, blockSize, b1Angle)
	b2(p, blockSize+padding, 0+padding, blockSize, b2Angle)

	b1Angle = incr(b1Angle)
	b2Angle = incr(b2Angle)
	b1(p, twoBlockSize+padding, 0+padding, blockSize, b1Angle)
	b2(p, twoBlockSize+padding, blockSize+padding, blockSize, b2Angle)

	b1Angle = incr(b1Angle)
	b2Angle = incr(b2Angle)
	b1(p, twoBlockSize+padding, twoBlockSize+padding, blockSize, b1Angle)
	b2(p, blockSize+padding, twoBlockSize+padding, blockSize, b2Angle)

	b1Angle = incr(b1Angle)
	b2Angle = incr(b2Angle)
	b1(p, 0+padding, twoBlockSize+padding, blockSize, b1Angle)
	b2(p, 0+padding, blockSize+padding, blockSize, b2Angle)
}
