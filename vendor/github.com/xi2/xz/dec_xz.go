/*
 * XZ decompressor
 *
 * Authors: Lasse Collin <lasse.collin@tukaani.org>
 *          Igor Pavlov <http://7-zip.org/>
 *
 * Translation to Go: Michael Cross <https://github.com/xi2>
 *
 * This file has been put into the public domain.
 * You can do whatever you want with this file.
 */

package xz

/* from linux/include/linux/xz.h **************************************/

/**
 * xzRet - Return codes
 * @xzOK:                   Everything is OK so far. More input or more
 *                          output space is required to continue.
 * @xzStreamEnd:            Operation finished successfully.
 * @xzUnSupportedCheck:     Integrity check type is not supported. Decoding
 *                          is still possible by simply calling xzDecRun
 *                          again.
 * @xzMemlimitError:        A bigger LZMA2 dictionary would be needed than
 *                          allowed by the dictMax argument given to
 *                          xzDecInit.
 * @xzFormatError:          File format was not recognized (wrong magic
 *                          bytes).
 * @xzOptionsError:         This implementation doesn't support the requested
 *                          compression options. In the decoder this means
 *                          that the header CRC32 matches, but the header
 *                          itself specifies something that we don't support.
 * @xzDataError:            Compressed data is corrupt.
 * @xzBufError:             Cannot make any progress.
 *
 * xzBufError is returned when two consecutive calls to XZ code cannot
 * consume any input and cannot produce any new output.  This happens
 * when there is no new input available, or the output buffer is full
 * while at least one output byte is still pending. Assuming your code
 * is not buggy, you can get this error only when decoding a
 * compressed stream that is truncated or otherwise corrupt.
 */
type xzRet int

const (
	xzOK xzRet = iota
	xzStreamEnd
	xzUnsupportedCheck
	xzMemlimitError
	xzFormatError
	xzOptionsError
	xzDataError
	xzBufError
)

/**
 * xzBuf - Passing input and output buffers to XZ code
 * @in:         Input buffer.
 * @inPos:      Current position in the input buffer. This must not exceed
 *              input buffer size.
 * @out:        Output buffer.
 * @outPos:     Current position in the output buffer. This must not exceed
 *              output buffer size.
 *
 * Only the contents of the output buffer from out[outPos] onward, and
 * the variables inPos and outPos are modified by the XZ code.
 */
type xzBuf struct {
	in     []byte
	inPos  int
	out    []byte
	outPos int
}

/* All XZ filter IDs */
type xzFilterID int64

const (
	idDelta       xzFilterID = 0x03
	idBCJX86      xzFilterID = 0x04
	idBCJPowerPC  xzFilterID = 0x05
	idBCJIA64     xzFilterID = 0x06
	idBCJARM      xzFilterID = 0x07
	idBCJARMThumb xzFilterID = 0x08
	idBCJSPARC    xzFilterID = 0x09
	idLZMA2       xzFilterID = 0x21
)

// CheckID is the type of the data integrity check in an XZ stream
// calculated from the uncompressed data.
type CheckID int

func (id CheckID) String() string {
	switch id {
	case CheckNone:
		return "None"
	case CheckCRC32:
		return "CRC32"
	case CheckCRC64:
		return "CRC64"
	case CheckSHA256:
		return "SHA256"
	default:
		return "Unknown"
	}
}

const (
	CheckNone   CheckID = 0x00
	CheckCRC32  CheckID = 0x01
	CheckCRC64  CheckID = 0x04
	CheckSHA256 CheckID = 0x0A
	checkMax    CheckID = 0x0F
	checkUnset  CheckID = -1
)

// An XZ stream contains a stream header which holds information about
// the stream. That information is exposed as fields of the
// Reader. Currently it contains only the stream's data integrity
// check type.
type Header struct {
	CheckType CheckID // type of the stream's data integrity check
}
