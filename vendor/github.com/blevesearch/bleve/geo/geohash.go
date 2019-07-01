// The code here was obtained from:
//   https://github.com/mmcloughlin/geohash

// The MIT License (MIT)
// Copyright (c) 2015 Michael McLoughlin
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package geo

import (
	"math"
)

// encoding encapsulates an encoding defined by a given base32 alphabet.
type encoding struct {
	enc string
	dec [256]byte
}

// newEncoding constructs a new encoding defined by the given alphabet,
// which must be a 32-byte string.
func newEncoding(encoder string) *encoding {
	e := new(encoding)
	e.enc = encoder
	for i := 0; i < len(e.dec); i++ {
		e.dec[i] = 0xff
	}
	for i := 0; i < len(encoder); i++ {
		e.dec[encoder[i]] = byte(i)
	}
	return e
}

// Decode string into bits of a 64-bit word. The string s may be at most 12
// characters.
func (e *encoding) decode(s string) uint64 {
	x := uint64(0)
	for i := 0; i < len(s); i++ {
		x = (x << 5) | uint64(e.dec[s[i]])
	}
	return x
}

// Encode bits of 64-bit word into a string.
func (e *encoding) encode(x uint64) string {
	b := [12]byte{}
	for i := 0; i < 12; i++ {
		b[11-i] = e.enc[x&0x1f]
		x >>= 5
	}
	return string(b[:])
}

// Base32Encoding with the Geohash alphabet.
var base32encoding = newEncoding("0123456789bcdefghjkmnpqrstuvwxyz")

// BoundingBox returns the region encoded by the given string geohash.
func geoBoundingBox(hash string) geoBox {
	bits := uint(5 * len(hash))
	inthash := base32encoding.decode(hash)
	return geoBoundingBoxIntWithPrecision(inthash, bits)
}

// Box represents a rectangle in latitude/longitude space.
type geoBox struct {
	minLat float64
	maxLat float64
	minLng float64
	maxLng float64
}

// Round returns a point inside the box, making an effort to round to minimal
// precision.
func (b geoBox) round() (lat, lng float64) {
	x := maxDecimalPower(b.maxLat - b.minLat)
	lat = math.Ceil(b.minLat/x) * x
	x = maxDecimalPower(b.maxLng - b.minLng)
	lng = math.Ceil(b.minLng/x) * x
	return
}

// precalculated for performance
var exp232 = math.Exp2(32)

// errorWithPrecision returns the error range in latitude and longitude for in
// integer geohash with bits of precision.
func errorWithPrecision(bits uint) (latErr, lngErr float64) {
	b := int(bits)
	latBits := b / 2
	lngBits := b - latBits
	latErr = math.Ldexp(180.0, -latBits)
	lngErr = math.Ldexp(360.0, -lngBits)
	return
}

// minDecimalPlaces returns the minimum number of decimal places such that
// there must exist an number with that many places within any range of width
// r. This is intended for returning minimal precision coordinates inside a
// box.
func maxDecimalPower(r float64) float64 {
	m := int(math.Floor(math.Log10(r)))
	return math.Pow10(m)
}

// Encode the position of x within the range -r to +r as a 32-bit integer.
func encodeRange(x, r float64) uint32 {
	p := (x + r) / (2 * r)
	return uint32(p * exp232)
}

// Decode the 32-bit range encoding X back to a value in the range -r to +r.
func decodeRange(X uint32, r float64) float64 {
	p := float64(X) / exp232
	x := 2*r*p - r
	return x
}

// Squash the even bitlevels of X into a 32-bit word. Odd bitlevels of X are
// ignored, and may take any value.
func squash(X uint64) uint32 {
	X &= 0x5555555555555555
	X = (X | (X >> 1)) & 0x3333333333333333
	X = (X | (X >> 2)) & 0x0f0f0f0f0f0f0f0f
	X = (X | (X >> 4)) & 0x00ff00ff00ff00ff
	X = (X | (X >> 8)) & 0x0000ffff0000ffff
	X = (X | (X >> 16)) & 0x00000000ffffffff
	return uint32(X)
}

// Deinterleave the bits of X into 32-bit words containing the even and odd
// bitlevels of X, respectively.
func deinterleave(X uint64) (uint32, uint32) {
	return squash(X), squash(X >> 1)
}

// BoundingBoxIntWithPrecision returns the region encoded by the integer
// geohash with the specified precision.
func geoBoundingBoxIntWithPrecision(hash uint64, bits uint) geoBox {
	fullHash := hash << (64 - bits)
	latInt, lngInt := deinterleave(fullHash)
	lat := decodeRange(latInt, 90)
	lng := decodeRange(lngInt, 180)
	latErr, lngErr := errorWithPrecision(bits)
	return geoBox{
		minLat: lat,
		maxLat: lat + latErr,
		minLng: lng,
		maxLng: lng + lngErr,
	}
}

// ----------------------------------------------------------------------

// Decode the string geohash to a (lat, lng) point.
func GeoHashDecode(hash string) (lat, lng float64) {
	box := geoBoundingBox(hash)
	return box.round()
}
