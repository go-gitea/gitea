// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package bzip2

import "github.com/dsnet/compress/internal/errors"

// rleDone is a special "error" to indicate that the RLE stage is done.
var rleDone = errorf(errors.Unknown, "RLE1 stage is completed")

// runLengthEncoding implements the first RLE stage of bzip2. Every sequence
// of 4..255 duplicated bytes is replaced by only the first 4 bytes, and a
// single byte representing the repeat length. Similar to the C bzip2
// implementation, the encoder will always terminate repeat sequences with a
// count (even if it is the end of the buffer), and it will also never produce
// run lengths of 256..259. The decoder can handle the latter case.
//
// For example, if the input was:
//	input:  "AAAAAAABBBBCCCD"
//
// Then the output will be:
//	output: "AAAA\x03BBBB\x00CCCD"
type runLengthEncoding struct {
	buf     []byte
	idx     int
	lastVal byte
	lastCnt int
}

func (rle *runLengthEncoding) Init(buf []byte) {
	*rle = runLengthEncoding{buf: buf}
}

func (rle *runLengthEncoding) Write(buf []byte) (int, error) {
	for i, b := range buf {
		if rle.lastVal != b {
			rle.lastCnt = 0
		}
		rle.lastCnt++
		switch {
		case rle.lastCnt < 4:
			if rle.idx >= len(rle.buf) {
				return i, rleDone
			}
			rle.buf[rle.idx] = b
			rle.idx++
		case rle.lastCnt == 4:
			if rle.idx+1 >= len(rle.buf) {
				return i, rleDone
			}
			rle.buf[rle.idx] = b
			rle.idx++
			rle.buf[rle.idx] = 0
			rle.idx++
		case rle.lastCnt < 256:
			rle.buf[rle.idx-1]++
		default:
			if rle.idx >= len(rle.buf) {
				return i, rleDone
			}
			rle.lastCnt = 1
			rle.buf[rle.idx] = b
			rle.idx++
		}
		rle.lastVal = b
	}
	return len(buf), nil
}

func (rle *runLengthEncoding) Read(buf []byte) (int, error) {
	for i := range buf {
		switch {
		case rle.lastCnt == -4:
			if rle.idx >= len(rle.buf) {
				return i, errorf(errors.Corrupted, "missing terminating run-length repeater")
			}
			rle.lastCnt = int(rle.buf[rle.idx])
			rle.idx++
			if rle.lastCnt > 0 {
				break // Break the switch
			}
			fallthrough // Count was zero, continue the work
		case rle.lastCnt <= 0:
			if rle.idx >= len(rle.buf) {
				return i, rleDone
			}
			b := rle.buf[rle.idx]
			rle.idx++
			if b != rle.lastVal {
				rle.lastCnt = 0
				rle.lastVal = b
			}
		}
		buf[i] = rle.lastVal
		rle.lastCnt--
	}
	return len(buf), nil
}

func (rle *runLengthEncoding) Bytes() []byte { return rle.buf[:rle.idx] }
