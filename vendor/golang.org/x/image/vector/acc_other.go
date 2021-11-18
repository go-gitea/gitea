// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !amd64 appengine !gc noasm

package vector

const haveAccumulateSIMD = false

func fixedAccumulateOpOverSIMD(dst []uint8, src []uint32)     {}
func fixedAccumulateOpSrcSIMD(dst []uint8, src []uint32)      {}
func fixedAccumulateMaskSIMD(buf []uint32)                    {}
func floatingAccumulateOpOverSIMD(dst []uint8, src []float32) {}
func floatingAccumulateOpSrcSIMD(dst []uint8, src []float32)  {}
func floatingAccumulateMaskSIMD(dst []uint32, src []float32)  {}
