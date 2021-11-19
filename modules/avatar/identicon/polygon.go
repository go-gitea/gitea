// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Copied and modified from https://github.com/issue9/identicon/ (MIT License)

package identicon

var (
	// cos(0),cos(90),cos(180),cos(270)
	cos = []int{1, 0, -1, 0}

	// sin(0),sin(90),sin(180),sin(270)
	sin = []int{0, 1, 0, -1}
)

// rotate the points by center point (x,y)
// angle: [0,1,2,3] means [0，90，180，270] degree
func rotate(points []int, x, y int, angle int) {
	if angle < 0 || angle > 3 {
		panic("rotate: angle must be 0/1/2/3")
	}

	for i := 0; i < len(points); i += 2 {
		px, py := points[i]-x, points[i+1]-y
		points[i] = px*cos[angle] - py*sin[angle] + x
		points[i+1] = px*sin[angle] + py*cos[angle] + y
	}
}

// check whether the point is inside the point is in the polygon (points)
// the first and the last point must be the same
func pointInPolygon(x, y int, polygonPoints []int) bool {
	if len(polygonPoints) < 8 { // a valid polygon must have more than 2 points
		return false
	}

	// split the plane into two by the check point horizontally:
	//   y>0，includes (x>0 && y==0)
	//   y<0，includes (x<0 && y==0)
	//
	// then scan every point in the polygon.
	//
	// if current point and previous point are in different planes (eg: curY>0 && prevY<0),
	// check the clock-direction from previous point to current point (use check point as origin).
	// if the direction is clockwise, then r++, otherwise then r--
	// finally, if 2==abs(r), then the check point is inside the polygon

	r := 0
	x1, y1 := polygonPoints[0], polygonPoints[1]
	prev := (y1 > y) || ((x1 > x) && (y1 == y))
	for i := 2; i < len(polygonPoints); i += 2 {
		x2, y2 := polygonPoints[i], polygonPoints[i+1]
		curr := (y2 > y) || ((x2 > x) && (y2 == y))

		if curr == prev {
			x1, y1 = x2, y2
			continue
		}

		if mul := (x1-x)*(y2-y) - (x2-x)*(y1-y); mul >= 0 {
			r++
		} else {
			r--
		}
		x1, y1 = x2, y2
		prev = curr
	}

	return r == 2 || r == -2
}
