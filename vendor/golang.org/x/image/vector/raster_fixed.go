// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vector

// This file contains a fixed point math implementation of the vector
// graphics rasterizer.

const (
	// ϕ is the number of binary digits after the fixed point.
	//
	// For example, if ϕ == 10 (and int1ϕ is based on the int32 type) then we
	// are using 22.10 fixed point math.
	//
	// When changing this number, also change the assembly code (search for ϕ
	// in the .s files).
	ϕ = 9

	fxOne          int1ϕ = 1 << ϕ
	fxOneAndAHalf  int1ϕ = 1<<ϕ + 1<<(ϕ-1)
	fxOneMinusIota int1ϕ = 1<<ϕ - 1 // Used for rounding up.
)

// int1ϕ is a signed fixed-point number with 1*ϕ binary digits after the fixed
// point.
type int1ϕ int32

// int2ϕ is a signed fixed-point number with 2*ϕ binary digits after the fixed
// point.
//
// The Rasterizer's bufU32 field, nominally of type []uint32 (since that slice
// is also used by other code), can be thought of as a []int2ϕ during the
// fixedLineTo method. Lines of code that are actually like:
//	buf[i] += uint32(etc) // buf has type []uint32.
// can be thought of as
//	buf[i] += int2ϕ(etc)  // buf has type []int2ϕ.
type int2ϕ int32

func fixedMax(x, y int1ϕ) int1ϕ {
	if x > y {
		return x
	}
	return y
}

func fixedMin(x, y int1ϕ) int1ϕ {
	if x < y {
		return x
	}
	return y
}

func fixedFloor(x int1ϕ) int32 { return int32(x >> ϕ) }
func fixedCeil(x int1ϕ) int32  { return int32((x + fxOneMinusIota) >> ϕ) }

func (z *Rasterizer) fixedLineTo(bx, by float32) {
	ax, ay := z.penX, z.penY
	z.penX, z.penY = bx, by
	dir := int1ϕ(1)
	if ay > by {
		dir, ax, ay, bx, by = -1, bx, by, ax, ay
	}
	// Horizontal line segments yield no change in coverage. Almost horizontal
	// segments would yield some change, in ideal math, but the computation
	// further below, involving 1 / (by - ay), is unstable in fixed point math,
	// so we treat the segment as if it was perfectly horizontal.
	if by-ay <= 0.000001 {
		return
	}
	dxdy := (bx - ax) / (by - ay)

	ayϕ := int1ϕ(ay * float32(fxOne))
	byϕ := int1ϕ(by * float32(fxOne))

	x := int1ϕ(ax * float32(fxOne))
	y := fixedFloor(ayϕ)
	yMax := fixedCeil(byϕ)
	if yMax > int32(z.size.Y) {
		yMax = int32(z.size.Y)
	}
	width := int32(z.size.X)

	for ; y < yMax; y++ {
		dy := fixedMin(int1ϕ(y+1)<<ϕ, byϕ) - fixedMax(int1ϕ(y)<<ϕ, ayϕ)
		xNext := x + int1ϕ(float32(dy)*dxdy)
		if y < 0 {
			x = xNext
			continue
		}
		buf := z.bufU32[y*width:]
		d := dy * dir // d ranges up to ±1<<(1*ϕ).
		x0, x1 := x, xNext
		if x > xNext {
			x0, x1 = x1, x0
		}
		x0i := fixedFloor(x0)
		x0Floor := int1ϕ(x0i) << ϕ
		x1i := fixedCeil(x1)
		x1Ceil := int1ϕ(x1i) << ϕ

		if x1i <= x0i+1 {
			xmf := (x+xNext)>>1 - x0Floor
			if i := clamp(x0i+0, width); i < uint(len(buf)) {
				buf[i] += uint32(d * (fxOne - xmf))
			}
			if i := clamp(x0i+1, width); i < uint(len(buf)) {
				buf[i] += uint32(d * xmf)
			}
		} else {
			oneOverS := x1 - x0
			twoOverS := 2 * oneOverS
			x0f := x0 - x0Floor
			oneMinusX0f := fxOne - x0f
			oneMinusX0fSquared := oneMinusX0f * oneMinusX0f
			x1f := x1 - x1Ceil + fxOne
			x1fSquared := x1f * x1f

			// These next two variables are unused, as rounding errors are
			// minimized when we delay the division by oneOverS for as long as
			// possible. These lines of code (and the "In ideal math" comments
			// below) are commented out instead of deleted in order to aid the
			// comparison with the floating point version of the rasterizer.
			//
			// a0 := ((oneMinusX0f * oneMinusX0f) >> 1) / oneOverS
			// am := ((x1f * x1f) >> 1) / oneOverS

			if i := clamp(x0i, width); i < uint(len(buf)) {
				// In ideal math: buf[i] += uint32(d * a0)
				D := oneMinusX0fSquared // D ranges up to ±1<<(2*ϕ).
				D *= d                  // D ranges up to ±1<<(3*ϕ).
				D /= twoOverS
				buf[i] += uint32(D)
			}

			if x1i == x0i+2 {
				if i := clamp(x0i+1, width); i < uint(len(buf)) {
					// In ideal math: buf[i] += uint32(d * (fxOne - a0 - am))
					//
					// (x1i == x0i+2) and (twoOverS == 2 * (x1 - x0)) implies
					// that twoOverS ranges up to +1<<(1*ϕ+2).
					D := twoOverS<<ϕ - oneMinusX0fSquared - x1fSquared // D ranges up to ±1<<(2*ϕ+2).
					D *= d                                             // D ranges up to ±1<<(3*ϕ+2).
					D /= twoOverS
					buf[i] += uint32(D)
				}
			} else {
				// This is commented out for the same reason as a0 and am.
				//
				// a1 := ((fxOneAndAHalf - x0f) << ϕ) / oneOverS

				if i := clamp(x0i+1, width); i < uint(len(buf)) {
					// In ideal math:
					//	buf[i] += uint32(d * (a1 - a0))
					// or equivalently (but better in non-ideal, integer math,
					// with respect to rounding errors),
					//	buf[i] += uint32(A * d / twoOverS)
					// where
					//	A = (a1 - a0) * twoOverS
					//	  = a1*twoOverS - a0*twoOverS
					// Noting that twoOverS/oneOverS equals 2, substituting for
					// a0 and then a1, given above, yields:
					//	A = a1*twoOverS - oneMinusX0fSquared
					//	  = (fxOneAndAHalf-x0f)<<(ϕ+1) - oneMinusX0fSquared
					//	  = fxOneAndAHalf<<(ϕ+1) - x0f<<(ϕ+1) - oneMinusX0fSquared
					//
					// This is a positive number minus two non-negative
					// numbers. For an upper bound on A, the positive number is
					//	P = fxOneAndAHalf<<(ϕ+1)
					//	  < (2*fxOne)<<(ϕ+1)
					//	  = fxOne<<(ϕ+2)
					//	  = 1<<(2*ϕ+2)
					//
					// For a lower bound on A, the two non-negative numbers are
					//	N = x0f<<(ϕ+1) + oneMinusX0fSquared
					//	  ≤ x0f<<(ϕ+1) + fxOne*fxOne
					//	  = x0f<<(ϕ+1) + 1<<(2*ϕ)
					//	  < x0f<<(ϕ+1) + 1<<(2*ϕ+1)
					//	  ≤ fxOne<<(ϕ+1) + 1<<(2*ϕ+1)
					//	  = 1<<(2*ϕ+1) + 1<<(2*ϕ+1)
					//	  = 1<<(2*ϕ+2)
					//
					// Thus, A ranges up to ±1<<(2*ϕ+2). It is possible to
					// derive a tighter bound, but this bound is sufficient to
					// reason about overflow.
					D := (fxOneAndAHalf-x0f)<<(ϕ+1) - oneMinusX0fSquared // D ranges up to ±1<<(2*ϕ+2).
					D *= d                                               // D ranges up to ±1<<(3*ϕ+2).
					D /= twoOverS
					buf[i] += uint32(D)
				}
				dTimesS := uint32((d << (2 * ϕ)) / oneOverS)
				for xi := x0i + 2; xi < x1i-1; xi++ {
					if i := clamp(xi, width); i < uint(len(buf)) {
						buf[i] += dTimesS
					}
				}

				// This is commented out for the same reason as a0 and am.
				//
				// a2 := a1 + (int1ϕ(x1i-x0i-3)<<(2*ϕ))/oneOverS

				if i := clamp(x1i-1, width); i < uint(len(buf)) {
					// In ideal math:
					//	buf[i] += uint32(d * (fxOne - a2 - am))
					// or equivalently (but better in non-ideal, integer math,
					// with respect to rounding errors),
					//	buf[i] += uint32(A * d / twoOverS)
					// where
					//	A = (fxOne - a2 - am) * twoOverS
					//	  = twoOverS<<ϕ - a2*twoOverS - am*twoOverS
					// Noting that twoOverS/oneOverS equals 2, substituting for
					// am and then a2, given above, yields:
					//	A = twoOverS<<ϕ - a2*twoOverS - x1f*x1f
					//	  = twoOverS<<ϕ - a1*twoOverS - (int1ϕ(x1i-x0i-3)<<(2*ϕ))*2 - x1f*x1f
					//	  = twoOverS<<ϕ - a1*twoOverS - int1ϕ(x1i-x0i-3)<<(2*ϕ+1) - x1f*x1f
					// Substituting for a1, given above, yields:
					//	A = twoOverS<<ϕ - ((fxOneAndAHalf-x0f)<<ϕ)*2 - int1ϕ(x1i-x0i-3)<<(2*ϕ+1) - x1f*x1f
					//	  = twoOverS<<ϕ - (fxOneAndAHalf-x0f)<<(ϕ+1) - int1ϕ(x1i-x0i-3)<<(2*ϕ+1) - x1f*x1f
					//	  = B<<ϕ - x1f*x1f
					// where
					//	B = twoOverS - (fxOneAndAHalf-x0f)<<1 - int1ϕ(x1i-x0i-3)<<(ϕ+1)
					//	  = (x1-x0)<<1 - (fxOneAndAHalf-x0f)<<1 - int1ϕ(x1i-x0i-3)<<(ϕ+1)
					//
					// Re-arranging the defintions given above:
					//	x0Floor := int1ϕ(x0i) << ϕ
					//	x0f := x0 - x0Floor
					//	x1Ceil := int1ϕ(x1i) << ϕ
					//	x1f := x1 - x1Ceil + fxOne
					// combined with fxOne = 1<<ϕ yields:
					//	x0 = x0f + int1ϕ(x0i)<<ϕ
					//	x1 = x1f + int1ϕ(x1i-1)<<ϕ
					// so that expanding (x1-x0) yields:
					//	B = (x1f-x0f + int1ϕ(x1i-x0i-1)<<ϕ)<<1 - (fxOneAndAHalf-x0f)<<1 - int1ϕ(x1i-x0i-3)<<(ϕ+1)
					//	  = (x1f-x0f)<<1 + int1ϕ(x1i-x0i-1)<<(ϕ+1) - (fxOneAndAHalf-x0f)<<1 - int1ϕ(x1i-x0i-3)<<(ϕ+1)
					// A large part of the second and fourth terms cancel:
					//	B = (x1f-x0f)<<1 - (fxOneAndAHalf-x0f)<<1 - int1ϕ(-2)<<(ϕ+1)
					//	  = (x1f-x0f)<<1 - (fxOneAndAHalf-x0f)<<1 + 1<<(ϕ+2)
					//	  = (x1f - fxOneAndAHalf)<<1 + 1<<(ϕ+2)
					// The first term, (x1f - fxOneAndAHalf)<<1, is a negative
					// number, bounded below by -fxOneAndAHalf<<1, which is
					// greater than -fxOne<<2, or -1<<(ϕ+2). Thus, B ranges up
					// to ±1<<(ϕ+2). One final simplification:
					//	B = x1f<<1 + (1<<(ϕ+2) - fxOneAndAHalf<<1)
					const C = 1<<(ϕ+2) - fxOneAndAHalf<<1
					D := x1f<<1 + C // D ranges up to ±1<<(1*ϕ+2).
					D <<= ϕ         // D ranges up to ±1<<(2*ϕ+2).
					D -= x1fSquared // D ranges up to ±1<<(2*ϕ+3).
					D *= d          // D ranges up to ±1<<(3*ϕ+3).
					D /= twoOverS
					buf[i] += uint32(D)
				}
			}

			if i := clamp(x1i, width); i < uint(len(buf)) {
				// In ideal math: buf[i] += uint32(d * am)
				D := x1fSquared // D ranges up to ±1<<(2*ϕ).
				D *= d          // D ranges up to ±1<<(3*ϕ).
				D /= twoOverS
				buf[i] += uint32(D)
			}
		}

		x = xNext
	}
}

func fixedAccumulateOpOver(dst []uint8, src []uint32) {
	// Sanity check that len(dst) >= len(src).
	if len(dst) < len(src) {
		return
	}

	acc := int2ϕ(0)
	for i, v := range src {
		acc += int2ϕ(v)
		a := acc
		if a < 0 {
			a = -a
		}
		a >>= 2*ϕ - 16
		if a > 0xffff {
			a = 0xffff
		}
		// This algorithm comes from the standard library's image/draw package.
		dstA := uint32(dst[i]) * 0x101
		maskA := uint32(a)
		outA := dstA*(0xffff-maskA)/0xffff + maskA
		dst[i] = uint8(outA >> 8)
	}
}

func fixedAccumulateOpSrc(dst []uint8, src []uint32) {
	// Sanity check that len(dst) >= len(src).
	if len(dst) < len(src) {
		return
	}

	acc := int2ϕ(0)
	for i, v := range src {
		acc += int2ϕ(v)
		a := acc
		if a < 0 {
			a = -a
		}
		a >>= 2*ϕ - 8
		if a > 0xff {
			a = 0xff
		}
		dst[i] = uint8(a)
	}
}

func fixedAccumulateMask(buf []uint32) {
	acc := int2ϕ(0)
	for i, v := range buf {
		acc += int2ϕ(v)
		a := acc
		if a < 0 {
			a = -a
		}
		a >>= 2*ϕ - 16
		if a > 0xffff {
			a = 0xffff
		}
		buf[i] = uint32(a)
	}
}
