// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
func rotate(points []int, x, y, angle int) {
	// the angle is only used internally, and it has been guaranteed to be 0/1/2/3, so we do not check it again
	for i := 0; i < len(points); i += 2 {
		px, py := points[i]-x, points[i+1]-y
		points[i] = px*cos[angle] - py*sin[angle] + x
		points[i+1] = px*sin[angle] + py*cos[angle] + y
	}
}

// check whether the point is inside the polygon (defined by the points)
// the first and the last point must be the same
func pointInPolygon(x, y int, polygonPoints []int) bool {
	if len(polygonPoints) < 8 { // a valid polygon must have more than 2 points
		return false
	}

	// reference: nonzero winding rule, https://en.wikipedia.org/wiki/Nonzero-rule
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
	prevX, prevY := polygonPoints[0], polygonPoints[1]
	prev := (prevY > y) || ((prevX > x) && (prevY == y))
	for i := 2; i < len(polygonPoints); i += 2 {
		currX, currY := polygonPoints[i], polygonPoints[i+1]
		curr := (currY > y) || ((currX > x) && (currY == y))

		if curr == prev {
			prevX, prevY = currX, currY
			continue
		}

		if mul := (prevX-x)*(currY-y) - (currX-x)*(prevY-y); mul >= 0 {
			r++
		} else { // mul < 0
			r--
		}
		prevX, prevY = currX, currY
		prev = curr
	}

	return r == 2 || r == -2
}
