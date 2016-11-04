// Copyright 2015 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package identicon

var (
	// 4个元素分别表示cos(0),cos(90),cos(180),cos(270)
	cos = []float64{1, 0, -1, 0}

	// 4个元素分别表示sin(0),sin(90),sin(180),sin(270)
	sin = []float64{0, 1, 0, -1}
)

// 将points中的所有点，以x,y为原点旋转angle个角度。
// angle取值只能是[0,1,2,3]，分别表示[0，90，180，270]
func rotate(points []float64, x, y float64, angle int) {
	if angle < 0 || angle > 3 {
		panic("rotate:参数angle必须0,1,2,3三值之一")
	}

	for i := 0; i < len(points); i += 2 {
		px := points[i] - x
		py := points[i+1] - y
		points[i] = px*cos[angle] - py*sin[angle] + x
		points[i+1] = px*sin[angle] + py*cos[angle] + y
	}
}

// 判断某个点是否在多边形之内，不包含构成多边形的线和点
// x,y 需要判断的点坐标
// points 组成多边形的所顶点，每两个元素表示一点顶点，其中最后一个顶点必须与第一个顶点相同。
func pointInPolygon(x float64, y float64, points []float64) bool {
	if len(points) < 8 { // 只有2个以上的点，才能组成闭合多边形
		return false
	}

	// 大致算法如下：
	// 把整个平面以给定的测试点为原点分两部分:
	// - y>0，包含(x>0 && y==0)
	// - y<0，包含(x<0 && y==0)
	// 依次扫描每一个点，当该点与前一个点处于不同部分时（即一个在y>0区，一个在y<0区），
	// 则判断从前一点到当前点是顺时针还是逆时针（以给定的测试点为原点），如果是顺时针r++，否则r--。
	// 结果为：2==abs(r)。

	r := 0
	x1, y1 := points[0], points[1]
	prev := (y1 > y) || ((x1 > x) && (y1 == y))
	for i := 2; i < len(points); i += 2 {
		x2, y2 := points[i], points[i+1]
		curr := (y2 > y) || ((x2 > x) && (y2 == y))

		if curr == prev {
			x1, y1 = x2, y2
			continue
		}

		mul := (x1-x)*(y2-y) - (x2-x)*(y1-y)
		if mul > 0 {
			r++
		} else if mul < 0 {
			r--
		}
		x1, y1 = x2, y2
		prev = curr
	}

	return r == 2 || r == -2
}
