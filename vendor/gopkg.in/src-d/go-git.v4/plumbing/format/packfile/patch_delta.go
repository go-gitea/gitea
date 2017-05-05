package packfile

import (
	"io/ioutil"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

// See https://github.com/git/git/blob/49fa3dc76179e04b0833542fa52d0f287a4955ac/delta.h
// https://github.com/git/git/blob/c2c5f6b1e479f2c38e0e01345350620944e3527f/patch-delta.c,
// and https://github.com/tarruda/node-git-core/blob/master/src/js/delta.js
// for details about the delta format.

const deltaSizeMin = 4

// ApplyDelta writes to taget the result of applying the modification deltas in delta to base.
func ApplyDelta(target, base plumbing.EncodedObject, delta []byte) error {
	r, err := base.Reader()
	if err != nil {
		return err
	}

	w, err := target.Writer()
	if err != nil {
		return err
	}

	src, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	dst := PatchDelta(src, delta)
	target.SetSize(int64(len(dst)))

	if _, err := w.Write(dst); err != nil {
		return err
	}

	return nil
}

// PatchDelta returns the result of applying the modification deltas in delta to src.
func PatchDelta(src, delta []byte) []byte {
	if len(delta) < deltaSizeMin {
		return nil
	}

	srcSz, delta := decodeLEB128(delta)
	if srcSz != uint(len(src)) {
		return nil
	}

	targetSz, delta := decodeLEB128(delta)
	remainingTargetSz := targetSz

	var dest []byte
	var cmd byte
	for {
		cmd = delta[0]
		delta = delta[1:]
		if isCopyFromSrc(cmd) {
			var offset, sz uint
			offset, delta = decodeOffset(cmd, delta)
			sz, delta = decodeSize(cmd, delta)
			if invalidSize(sz, targetSz) ||
				invalidOffsetSize(offset, sz, srcSz) {
				break
			}
			dest = append(dest, src[offset:offset+sz]...)
			remainingTargetSz -= sz
		} else if isCopyFromDelta(cmd) {
			sz := uint(cmd) // cmd is the size itself
			if invalidSize(sz, targetSz) {
				break
			}
			dest = append(dest, delta[0:sz]...)
			remainingTargetSz -= sz
			delta = delta[sz:]
		} else {
			return nil
		}

		if remainingTargetSz <= 0 {
			break
		}
	}

	return dest
}

// Decodes a number encoded as an unsigned LEB128 at the start of some
// binary data and returns the decoded number and the rest of the
// stream.
//
// This must be called twice on the delta data buffer, first to get the
// expected source buffer size, and again to get the target buffer size.
func decodeLEB128(input []byte) (uint, []byte) {
	var num, sz uint
	var b byte
	for {
		b = input[sz]
		num |= (uint(b) & payload) << (sz * 7) // concats 7 bits chunks
		sz++

		if uint(b)&continuation == 0 || sz == uint(len(input)) {
			break
		}
	}

	return num, input[sz:]
}

const (
	payload      = 0x7f // 0111 1111
	continuation = 0x80 // 1000 0000
)

func isCopyFromSrc(cmd byte) bool {
	return (cmd & 0x80) != 0
}

func isCopyFromDelta(cmd byte) bool {
	return (cmd&0x80) == 0 && cmd != 0
}

func decodeOffset(cmd byte, delta []byte) (uint, []byte) {
	var offset uint
	if (cmd & 0x01) != 0 {
		offset = uint(delta[0])
		delta = delta[1:]
	}
	if (cmd & 0x02) != 0 {
		offset |= uint(delta[0]) << 8
		delta = delta[1:]
	}
	if (cmd & 0x04) != 0 {
		offset |= uint(delta[0]) << 16
		delta = delta[1:]
	}
	if (cmd & 0x08) != 0 {
		offset |= uint(delta[0]) << 24
		delta = delta[1:]
	}

	return offset, delta
}

func decodeSize(cmd byte, delta []byte) (uint, []byte) {
	var sz uint
	if (cmd & 0x10) != 0 {
		sz = uint(delta[0])
		delta = delta[1:]
	}
	if (cmd & 0x20) != 0 {
		sz |= uint(delta[0]) << 8
		delta = delta[1:]
	}
	if (cmd & 0x40) != 0 {
		sz |= uint(delta[0]) << 16
		delta = delta[1:]
	}
	if sz == 0 {
		sz = 0x10000
	}

	return sz, delta
}

func invalidSize(sz, targetSz uint) bool {
	return sz > targetSz
}

func invalidOffsetSize(offset, sz, srcSz uint) bool {
	return sumOverflows(offset, sz) ||
		offset+sz > srcSz
}

func sumOverflows(a, b uint) bool {
	return a+b < a
}
