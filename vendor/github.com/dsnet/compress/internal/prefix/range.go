// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

type RangeCode struct {
	Base uint32 // Starting base offset of the range
	Len  uint32 // Bit-length of a subsequent integer to add to base offset
}
type RangeCodes []RangeCode

type RangeEncoder struct {
	rcs     RangeCodes
	lut     [1024]uint32
	minBase uint
}

// End reports the non-inclusive ending range.
func (rc RangeCode) End() uint32 { return rc.Base + (1 << rc.Len) }

// MakeRangeCodes creates a RangeCodes, where each region is assumed to be
// contiguously stacked, without any gaps, with bit-lengths taken from bits.
func MakeRangeCodes(minBase uint, bits []uint) (rc RangeCodes) {
	for _, nb := range bits {
		rc = append(rc, RangeCode{Base: uint32(minBase), Len: uint32(nb)})
		minBase += 1 << nb
	}
	return rc
}

// Base reports the inclusive starting range for all ranges.
func (rcs RangeCodes) Base() uint32 { return rcs[0].Base }

// End reports the non-inclusive ending range for all ranges.
func (rcs RangeCodes) End() uint32 { return rcs[len(rcs)-1].End() }

// checkValid reports whether the RangeCodes is valid. In order to be valid,
// the following must hold true:
//	rcs[i-1].Base <= rcs[i].Base
//	rcs[i-1].End  <= rcs[i].End
//	rcs[i-1].End  >= rcs[i].Base
//
// Practically speaking, each range must be increasing and must not have any
// gaps in between. It is okay for ranges to overlap.
func (rcs RangeCodes) checkValid() bool {
	if len(rcs) == 0 {
		return false
	}
	pre := rcs[0]
	for _, cur := range rcs[1:] {
		preBase, preEnd := pre.Base, pre.End()
		curBase, curEnd := cur.Base, cur.End()
		if preBase > curBase || preEnd > curEnd || preEnd < curBase {
			return false
		}
		pre = cur
	}
	return true
}

func (re *RangeEncoder) Init(rcs RangeCodes) {
	if !rcs.checkValid() {
		panic("invalid range codes")
	}
	*re = RangeEncoder{rcs: rcs, minBase: uint(rcs.Base())}
	for sym, rc := range rcs {
		base := int(rc.Base) - int(re.minBase)
		end := int(rc.End()) - int(re.minBase)
		if base >= len(re.lut) {
			break
		}
		if end > len(re.lut) {
			end = len(re.lut)
		}
		for i := base; i < end; i++ {
			re.lut[i] = uint32(sym)
		}
	}
}

func (re *RangeEncoder) Encode(offset uint) (sym uint) {
	if idx := int(offset - re.minBase); idx < len(re.lut) {
		return uint(re.lut[idx])
	}
	sym = uint(re.lut[len(re.lut)-1])
retry:
	if int(sym) >= len(re.rcs) || re.rcs[sym].Base > uint32(offset) {
		return sym - 1
	}
	sym++
	goto retry // Avoid for-loop so that this function can be inlined
}
