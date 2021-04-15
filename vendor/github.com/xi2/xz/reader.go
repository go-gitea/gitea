/*
 * Package xz Go Reader API
 *
 * Author: Michael Cross <https://github.com/xi2>
 *
 * This file has been put into the public domain.
 * You can do whatever you want with this file.
 */

package xz

import (
	"errors"
	"io"
)

// Package specific errors.
var (
	ErrUnsupportedCheck = errors.New("xz: integrity check type not supported")
	ErrMemlimit         = errors.New("xz: LZMA2 dictionary size exceeds max")
	ErrFormat           = errors.New("xz: file format not recognized")
	ErrOptions          = errors.New("xz: compression options not supported")
	ErrData             = errors.New("xz: data is corrupt")
	ErrBuf              = errors.New("xz: data is truncated or corrupt")
)

// DefaultDictMax is the default maximum dictionary size in bytes used
// by the decoder. This value is sufficient to decompress files
// created with XZ Utils "xz -9".
const DefaultDictMax = 1 << 26 // 64 MiB

// inBufSize is the input buffer size used by the decoder.
const inBufSize = 1 << 13 // 8 KiB

// A Reader is an io.Reader that can be used to retrieve uncompressed
// data from an XZ file.
//
// In general, an XZ file can be a concatenation of other XZ
// files. Reads from the Reader return the concatenation of the
// uncompressed data of each.
type Reader struct {
	Header
	r           io.Reader       // the wrapped io.Reader
	multistream bool            // true if reader is in multistream mode
	rEOF        bool            // true after io.EOF received on r
	dEOF        bool            // true after decoder has completed
	padding     int             // bytes of stream padding read (or -1)
	in          [inBufSize]byte // backing array for buf.in
	buf         *xzBuf          // decoder input/output buffers
	dec         *xzDec          // decoder state
	err         error           // the result of the last decoder call
}

// NewReader creates a new Reader reading from r. The decompressor
// will use an LZMA2 dictionary size up to dictMax bytes in
// size. Passing a value of zero sets dictMax to DefaultDictMax.  If
// an individual XZ stream requires a dictionary size greater than
// dictMax in order to decompress, Read will return ErrMemlimit.
//
// If NewReader is passed a value of nil for r then a Reader is
// created such that all read attempts will return io.EOF. This is
// useful if you just want to allocate memory for a Reader which will
// later be initialized with Reset.
//
// Due to internal buffering, the Reader may read more data than
// necessary from r.
func NewReader(r io.Reader, dictMax uint32) (*Reader, error) {
	if dictMax == 0 {
		dictMax = DefaultDictMax
	}
	z := &Reader{
		r:           r,
		multistream: true,
		padding:     -1,
		buf:         &xzBuf{},
	}
	if r == nil {
		z.rEOF, z.dEOF = true, true
	}
	z.dec = xzDecInit(dictMax, &z.Header)
	var err error
	if r != nil {
		_, err = z.Read(nil) // read stream header
	}
	return z, err
}

// decode is a wrapper around xzDecRun that additionally handles
// stream padding. It treats the padding as a kind of stream that
// decodes to nothing.
//
// When decoding padding, z.padding >= 0
// When decoding a real stream, z.padding == -1
func (z *Reader) decode() (ret xzRet) {
	if z.padding >= 0 {
		// read all padding in input buffer
		for z.buf.inPos < len(z.buf.in) &&
			z.buf.in[z.buf.inPos] == 0 {
			z.buf.inPos++
			z.padding++
		}
		switch {
		case z.buf.inPos == len(z.buf.in) && z.rEOF:
			// case: out of padding. no more input data available
			if z.padding%4 != 0 {
				ret = xzDataError
			} else {
				ret = xzStreamEnd
			}
		case z.buf.inPos == len(z.buf.in):
			// case: read more padding next loop iteration
			ret = xzOK
		default:
			// case: out of padding. more input data available
			if z.padding%4 != 0 {
				ret = xzDataError
			} else {
				xzDecReset(z.dec)
				ret = xzStreamEnd
			}
		}
	} else {
		ret = xzDecRun(z.dec, z.buf)
	}
	return
}

func (z *Reader) Read(p []byte) (n int, err error) {
	// restore err
	err = z.err
	// set decoder output buffer to p
	z.buf.out = p
	z.buf.outPos = 0
	for {
		// update n
		n = z.buf.outPos
		// if last call to decoder ended with an error, return that error
		if err != nil {
			break
		}
		// if decoder has finished, return with err == io.EOF
		if z.dEOF {
			err = io.EOF
			break
		}
		// if p full, return with err == nil, unless we have not yet
		// read the stream header with Read(nil)
		if n == len(p) && z.CheckType != checkUnset {
			break
		}
		// if needed, read more data from z.r
		if z.buf.inPos == len(z.buf.in) && !z.rEOF {
			rn, e := z.r.Read(z.in[:])
			if e != nil && e != io.EOF {
				// read error
				err = e
				break
			}
			if e == io.EOF {
				z.rEOF = true
			}
			// set new input buffer in z.buf
			z.buf.in = z.in[:rn]
			z.buf.inPos = 0
		}
		// decode more data
		ret := z.decode()
		switch ret {
		case xzOK:
			// no action needed
		case xzStreamEnd:
			if z.padding >= 0 {
				z.padding = -1
				if !z.multistream || z.rEOF {
					z.dEOF = true
				}
			} else {
				z.padding = 0
			}
		case xzUnsupportedCheck:
			err = ErrUnsupportedCheck
		case xzMemlimitError:
			err = ErrMemlimit
		case xzFormatError:
			err = ErrFormat
		case xzOptionsError:
			err = ErrOptions
		case xzDataError:
			err = ErrData
		case xzBufError:
			err = ErrBuf
		}
		// save err
		z.err = err
	}
	return
}

// Multistream controls whether the reader is operating in multistream
// mode.
//
// If enabled (the default), the Reader expects the input to be a
// sequence of XZ streams, possibly interspersed with stream padding,
// which it reads one after another. The effect is that the
// concatenation of a sequence of XZ streams or XZ files is
// treated as equivalent to the compressed result of the concatenation
// of the sequence. This is standard behaviour for XZ readers.
//
// Calling Multistream(false) disables this behaviour; disabling the
// behaviour can be useful when reading file formats that distinguish
// individual XZ streams. In this mode, when the Reader reaches the
// end of the stream, Read returns io.EOF. To start the next stream,
// call z.Reset(nil) followed by z.Multistream(false). If there is no
// next stream, z.Reset(nil) will return io.EOF.
func (z *Reader) Multistream(ok bool) {
	z.multistream = ok
}

// Reset, for non-nil values of io.Reader r, discards the Reader z's
// state and makes it equivalent to the result of its original state
// from NewReader, but reading from r instead. This permits reusing a
// Reader rather than allocating a new one.
//
// If you wish to leave r unchanged use z.Reset(nil). This keeps r
// unchanged and ensures internal buffering is preserved. If the
// Reader was at the end of a stream it is then ready to read any
// follow on streams. If there are no follow on streams z.Reset(nil)
// returns io.EOF. If the Reader was not at the end of a stream then
// z.Reset(nil) does nothing.
func (z *Reader) Reset(r io.Reader) error {
	switch {
	case r == nil:
		z.multistream = true
		if !z.dEOF {
			return nil
		}
		if z.rEOF {
			return io.EOF
		}
		z.dEOF = false
		_, err := z.Read(nil) // read stream header
		return err
	default:
		z.r = r
		z.multistream = true
		z.rEOF = false
		z.dEOF = false
		z.padding = -1
		z.buf.in = nil
		z.buf.inPos = 0
		xzDecReset(z.dec)
		z.err = nil
		_, err := z.Read(nil) // read stream header
		return err
	}
}
