// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

package prefix

import (
	"encoding/binary"
	"io"

	"github.com/dsnet/compress/internal/errors"
)

// Writer implements a prefix encoder. For performance reasons, Writer will not
// write bytes immediately to the underlying stream.
type Writer struct {
	Offset int64 // Number of bytes written to the underlying io.Writer

	wr        io.Writer
	bufBits   uint64 // Buffer to hold some bits
	numBits   uint   // Number of valid bits in bufBits
	bigEndian bool   // Are bits written in big-endian order?

	buf    [512]byte
	cntBuf int
}

// Init initializes the bit Writer to write to w. If bigEndian is true, then
// bits will be written starting from the most-significant bits of a byte
// (as done in bzip2), otherwise it will write starting from the
// least-significant bits of a byte (such as for deflate and brotli).
func (pw *Writer) Init(w io.Writer, bigEndian bool) {
	*pw = Writer{wr: w, bigEndian: bigEndian}
	return
}

// BitsWritten reports the total number of bits issued to any Write method.
func (pw *Writer) BitsWritten() int64 {
	return 8*pw.Offset + 8*int64(pw.cntBuf) + int64(pw.numBits)
}

// WritePads writes 0-7 bits to the bit buffer to achieve byte-alignment.
func (pw *Writer) WritePads(v uint) {
	nb := -pw.numBits & 7
	pw.bufBits |= uint64(v) << pw.numBits
	pw.numBits += nb
}

// Write writes bytes from buf.
// The bit-ordering mode does not affect this method.
func (pw *Writer) Write(buf []byte) (cnt int, err error) {
	if pw.numBits > 0 || pw.cntBuf > 0 {
		if pw.numBits%8 != 0 {
			return 0, errorf(errors.Invalid, "non-aligned bit buffer")
		}
		if _, err := pw.Flush(); err != nil {
			return 0, err
		}
	}
	cnt, err = pw.wr.Write(buf)
	pw.Offset += int64(cnt)
	return cnt, err
}

// WriteOffset writes ofs in a (sym, extra) fashion using the provided prefix
// Encoder and RangeEncoder.
func (pw *Writer) WriteOffset(ofs uint, pe *Encoder, re *RangeEncoder) {
	sym := re.Encode(ofs)
	pw.WriteSymbol(sym, pe)
	rc := re.rcs[sym]
	pw.WriteBits(ofs-uint(rc.Base), uint(rc.Len))
}

// TryWriteBits attempts to write nb bits using the contents of the bit buffer
// alone. It reports whether it succeeded.
//
// This method is designed to be inlined for performance reasons.
func (pw *Writer) TryWriteBits(v, nb uint) bool {
	if 64-pw.numBits < nb {
		return false
	}
	pw.bufBits |= uint64(v) << pw.numBits
	pw.numBits += nb
	return true
}

// WriteBits writes nb bits of v to the underlying writer.
func (pw *Writer) WriteBits(v, nb uint) {
	if _, err := pw.PushBits(); err != nil {
		errors.Panic(err)
	}
	pw.bufBits |= uint64(v) << pw.numBits
	pw.numBits += nb
}

// TryWriteSymbol attempts to encode the next symbol using the contents of the
// bit buffer alone. It reports whether it succeeded.
//
// This method is designed to be inlined for performance reasons.
func (pw *Writer) TryWriteSymbol(sym uint, pe *Encoder) bool {
	chunk := pe.chunks[uint32(sym)&pe.chunkMask]
	nb := uint(chunk & countMask)
	if 64-pw.numBits < nb {
		return false
	}
	pw.bufBits |= uint64(chunk>>countBits) << pw.numBits
	pw.numBits += nb
	return true
}

// WriteSymbol writes the symbol using the provided prefix Encoder.
func (pw *Writer) WriteSymbol(sym uint, pe *Encoder) {
	if _, err := pw.PushBits(); err != nil {
		errors.Panic(err)
	}
	chunk := pe.chunks[uint32(sym)&pe.chunkMask]
	nb := uint(chunk & countMask)
	pw.bufBits |= uint64(chunk>>countBits) << pw.numBits
	pw.numBits += nb
}

// Flush flushes all complete bytes from the bit buffer to the byte buffer, and
// then flushes all bytes in the byte buffer to the underlying writer.
// After this call, the bit Writer is will only withhold 7 bits at most.
func (pw *Writer) Flush() (int64, error) {
	if pw.numBits < 8 && pw.cntBuf == 0 {
		return pw.Offset, nil
	}
	if _, err := pw.PushBits(); err != nil {
		return pw.Offset, err
	}
	cnt, err := pw.wr.Write(pw.buf[:pw.cntBuf])
	pw.cntBuf -= cnt
	pw.Offset += int64(cnt)
	return pw.Offset, err
}

// PushBits pushes as many bytes as possible from the bit buffer to the byte
// buffer, reporting the number of bits pushed.
func (pw *Writer) PushBits() (uint, error) {
	if pw.cntBuf >= len(pw.buf)-8 {
		cnt, err := pw.wr.Write(pw.buf[:pw.cntBuf])
		pw.cntBuf -= cnt
		pw.Offset += int64(cnt)
		if err != nil {
			return 0, err
		}
	}

	u := pw.bufBits
	if pw.bigEndian {
		// Swap all the bits within each byte.
		u = (u&0xaaaaaaaaaaaaaaaa)>>1 | (u&0x5555555555555555)<<1
		u = (u&0xcccccccccccccccc)>>2 | (u&0x3333333333333333)<<2
		u = (u&0xf0f0f0f0f0f0f0f0)>>4 | (u&0x0f0f0f0f0f0f0f0f)<<4
	}
	// Starting with Go 1.7, the compiler should use a wide integer
	// store here if the architecture supports it.
	binary.LittleEndian.PutUint64(pw.buf[pw.cntBuf:], u)

	nb := pw.numBits / 8 // Number of bytes to copy from bit buffer
	pw.cntBuf += int(nb)
	pw.bufBits >>= 8 * nb
	pw.numBits -= 8 * nb
	return 8 * nb, nil
}
