package gomemcached

import (
	"encoding/binary"
	"fmt"
	"io"
)

// The maximum reasonable body length to expect.
// Anything larger than this will result in an error.
// The current limit, 20MB, is the size limit supported by ep-engine.
var MaxBodyLen = int(20 * 1024 * 1024)

// MCRequest is memcached Request
type MCRequest struct {
	// The command being issued
	Opcode CommandCode
	// The CAS (if applicable, or 0)
	Cas uint64
	// An opaque value to be returned with this request
	Opaque uint32
	// The vbucket to which this command belongs
	VBucket uint16
	// Command extras, key, and body
	Extras, Key, Body, ExtMeta []byte
	// Datatype identifier
	DataType uint8
	// len() calls are expensive - cache this in case for collection
	Keylen int
	// Flexible Framing Extras
	FramingExtras []FrameInfo
	// Stored length of incoming framing extras
	FramingElen int
}

// Size gives the number of bytes this request requires.
func (req *MCRequest) Size() int {
	return HDR_LEN + len(req.Extras) + len(req.Key) + len(req.Body) + len(req.ExtMeta) + req.FramingElen
}

// A debugging string representation of this request
func (req MCRequest) String() string {
	return fmt.Sprintf("{MCRequest opcode=%s, bodylen=%d, key='%s'}",
		req.Opcode, len(req.Body), req.Key)
}

func (req *MCRequest) fillRegularHeaderBytes(data []byte) int {
	//  Byte/     0       |       1       |       2       |       3       |
	//     /              |               |               |               |
	//    |0 1 2 3 4 5 6 7|0 1 2 3 4 5 6 7|0 1 2 3 4 5 6 7|0 1 2 3 4 5 6 7|
	//    +---------------+---------------+---------------+---------------+
	//   0| Magic         | Opcode        | Key length                    |
	//    +---------------+---------------+---------------+---------------+
	//   4| Extras length | Data type     | vbucket id                    |
	//    +---------------+---------------+---------------+---------------+
	//   8| Total body length                                             |
	//    +---------------+---------------+---------------+---------------+
	//  12| Opaque                                                        |
	//    +---------------+---------------+---------------+---------------+
	//  16| CAS                                                           |
	//    |                                                               |
	//    +---------------+---------------+---------------+---------------+
	//    Total 24 bytes

	pos := 0
	data[pos] = REQ_MAGIC
	pos++
	data[pos] = byte(req.Opcode)
	pos++
	binary.BigEndian.PutUint16(data[pos:pos+2],
		uint16(len(req.Key)))
	pos += 2

	// 4
	data[pos] = byte(len(req.Extras))
	pos++
	// Data type
	if req.DataType != 0 {
		data[pos] = byte(req.DataType)
	}
	pos++
	binary.BigEndian.PutUint16(data[pos:pos+2], req.VBucket)
	pos += 2

	// 8
	binary.BigEndian.PutUint32(data[pos:pos+4],
		uint32(len(req.Body)+len(req.Key)+len(req.Extras)+len(req.ExtMeta)))
	pos += 4

	// 12
	binary.BigEndian.PutUint32(data[pos:pos+4], req.Opaque)
	pos += 4

	// 16
	if req.Cas != 0 {
		binary.BigEndian.PutUint64(data[pos:pos+8], req.Cas)
	}
	pos += 8

	if len(req.Extras) > 0 {
		copy(data[pos:pos+len(req.Extras)], req.Extras)
		pos += len(req.Extras)
	}

	if len(req.Key) > 0 {
		copy(data[pos:pos+len(req.Key)], req.Key)
		pos += len(req.Key)
	}
	return pos
}

// Returns pos and if trailing by half byte
func (req *MCRequest) fillFlexHeaderBytes(data []byte) (int, bool) {

	//  Byte/     0       |       1       |       2       |       3       |
	//     /              |               |               |               |
	//    |0 1 2 3 4 5 6 7|0 1 2 3 4 5 6 7|0 1 2 3 4 5 6 7|0 1 2 3 4 5 6 7|
	//    +---------------+---------------+---------------+---------------+
	//   0| Magic (0x08)  | Opcode        | Framing extras| Key Length    |
	//    +---------------+---------------+---------------+---------------+
	//   4| Extras length | Data type     | vbucket id                    |
	//    +---------------+---------------+---------------+---------------+
	//   8| Total body length                                             |
	//    +---------------+---------------+---------------+---------------+
	//  12| Opaque                                                        |
	//    +---------------+---------------+---------------+---------------+
	//  16| CAS                                                           |
	//    |                                                               |
	//    +---------------+---------------+---------------+---------------+
	//    Total 24 bytes

	data[0] = FLEX_MAGIC
	data[1] = byte(req.Opcode)
	data[2] = byte(req.FramingElen)
	data[3] = byte(req.Keylen)
	elen := len(req.Extras)
	data[4] = byte(elen)
	if req.DataType != 0 {
		data[5] = byte(req.DataType)
	}
	binary.BigEndian.PutUint16(data[6:8], req.VBucket)
	binary.BigEndian.PutUint32(data[8:12],
		uint32(len(req.Body)+req.Keylen+elen+len(req.ExtMeta)+req.FramingElen))
	binary.BigEndian.PutUint32(data[12:16], req.Opaque)
	if req.Cas != 0 {
		binary.BigEndian.PutUint64(data[16:24], req.Cas)
	}
	pos := HDR_LEN

	// Add framing infos
	var framingExtras []byte
	var outputBytes []byte
	var mergeModeSrc []byte
	var frameBytes int
	var halfByteMode bool
	var mergeMode bool
	for _, frameInfo := range req.FramingExtras {
		if !mergeMode {
			outputBytes, halfByteMode = frameInfo.Bytes()
			if !halfByteMode {
				framingExtras = append(framingExtras, outputBytes...)
				frameBytes += len(outputBytes)
			} else {
				mergeMode = true
				mergeModeSrc = outputBytes
			}
		} else {
			outputBytes, halfByteMode = frameInfo.Bytes()
			outputBytes := ShiftByteSliceRight4Bits(outputBytes)
			if halfByteMode {
				// Previous halfbyte merge with this halfbyte will result in a complete byte
				mergeMode = false
				outputBytes = Merge2HalfByteSlices(mergeModeSrc, outputBytes)
				framingExtras = append(framingExtras, outputBytes...)
				frameBytes += len(outputBytes)
			} else {
				// Merge half byte with a non-half byte will result in a combined half-byte that will
				// become the source for the next iteration
				mergeModeSrc = Merge2HalfByteSlices(mergeModeSrc, outputBytes)
			}
		}
	}

	if mergeMode {
		// Commit the temporary merge area into framingExtras
		framingExtras = append(framingExtras, mergeModeSrc...)
		frameBytes += len(mergeModeSrc)
	}

	copy(data[pos:pos+frameBytes], framingExtras)

	pos += frameBytes

	// Add Extras
	if len(req.Extras) > 0 {
		if mergeMode {
			outputBytes = ShiftByteSliceRight4Bits(req.Extras)
			data = Merge2HalfByteSlices(data, outputBytes)
		} else {
			copy(data[pos:pos+elen], req.Extras)
		}
		pos += elen
	}

	// Add keys
	if req.Keylen > 0 {
		if mergeMode {
			outputBytes = ShiftByteSliceRight4Bits(req.Key)
			data = Merge2HalfByteSlices(data, outputBytes)
		} else {
			copy(data[pos:pos+req.Keylen], req.Key)
		}
		pos += req.Keylen
	}

	return pos, mergeMode
}

func (req *MCRequest) FillHeaderBytes(data []byte) (int, bool) {
	if req.FramingElen == 0 {
		return req.fillRegularHeaderBytes(data), false
	} else {
		return req.fillFlexHeaderBytes(data)
	}
}

// HeaderBytes will return the wire representation of the request header
// (with the extras and key).
func (req *MCRequest) HeaderBytes() []byte {
	data := make([]byte, HDR_LEN+len(req.Extras)+len(req.Key)+req.FramingElen)

	req.FillHeaderBytes(data)

	return data
}

// Bytes will return the wire representation of this request.
func (req *MCRequest) Bytes() []byte {
	data := make([]byte, req.Size())

	pos, halfByteMode := req.FillHeaderBytes(data)
	// TODO - the halfByteMode should be revisited for a more efficient
	// way of doing things

	if len(req.Body) > 0 {
		if halfByteMode {
			shifted := ShiftByteSliceRight4Bits(req.Body)
			data = Merge2HalfByteSlices(data, shifted)
		} else {
			copy(data[pos:pos+len(req.Body)], req.Body)
		}
	}

	if len(req.ExtMeta) > 0 {
		if halfByteMode {
			shifted := ShiftByteSliceRight4Bits(req.ExtMeta)
			data = Merge2HalfByteSlices(data, shifted)
		} else {
			copy(data[pos+len(req.Body):pos+len(req.Body)+len(req.ExtMeta)], req.ExtMeta)
		}
	}
	return data
}

// Transmit will send this request message across a writer.
func (req *MCRequest) Transmit(w io.Writer) (n int, err error) {
	if len(req.Body) < 128 {
		n, err = w.Write(req.Bytes())
	} else {
		n, err = w.Write(req.HeaderBytes())
		if err == nil {
			m := 0
			m, err = w.Write(req.Body)
			n += m
		}
	}
	return
}

func (req *MCRequest) receiveHeaderCommon(hdrBytes []byte) (elen, totalBodyLen int) {
	elen = int(hdrBytes[4])
	// Data type at 5
	req.DataType = uint8(hdrBytes[5])

	req.Opcode = CommandCode(hdrBytes[1])
	// Vbucket at 6:7
	req.VBucket = binary.BigEndian.Uint16(hdrBytes[6:])
	totalBodyLen = int(binary.BigEndian.Uint32(hdrBytes[8:]))

	req.Opaque = binary.BigEndian.Uint32(hdrBytes[12:])
	req.Cas = binary.BigEndian.Uint64(hdrBytes[16:])
	return
}

func (req *MCRequest) receiveRegHeader(hdrBytes []byte) (elen, totalBodyLen int) {
	elen, totalBodyLen = req.receiveHeaderCommon(hdrBytes)
	req.Keylen = int(binary.BigEndian.Uint16(hdrBytes[2:]))
	return
}

func (req *MCRequest) receiveFlexibleFramingHeader(hdrBytes []byte) (elen, totalBodyLen, framingElen int) {
	elen, totalBodyLen = req.receiveHeaderCommon(hdrBytes)

	//	 For flexible framing header, key length is a single byte at byte index 3
	req.Keylen = int(binary.BigEndian.Uint16(hdrBytes[2:]) & 0x0ff)
	// Flexible framing lengh is a single byte at index 2
	framingElen = int(binary.BigEndian.Uint16(hdrBytes[2:]) >> 8)
	req.FramingElen = framingElen
	return
}

func (req *MCRequest) populateRegularBody(r io.Reader, totalBodyLen, elen int) (int, error) {
	var m int
	var err error
	if totalBodyLen > 0 {
		buf := make([]byte, totalBodyLen)
		m, err = io.ReadFull(r, buf)
		if err == nil {
			if req.Opcode >= TAP_MUTATION &&
				req.Opcode <= TAP_CHECKPOINT_END &&
				len(buf) > 1 {
				// In these commands there is "engine private"
				// data at the end of the extras.  The first 2
				// bytes of extra data give its length.
				elen += int(binary.BigEndian.Uint16(buf))
			}

			req.Extras = buf[0:elen]
			req.Key = buf[elen : req.Keylen+elen]

			// get the length of extended metadata
			extMetaLen := 0
			if elen > 29 {
				extMetaLen = int(binary.BigEndian.Uint16(req.Extras[28:30]))
			}

			bodyLen := totalBodyLen - req.Keylen - elen - extMetaLen
			if bodyLen > MaxBodyLen {
				return m, fmt.Errorf("%d is too big (max %d)",
					bodyLen, MaxBodyLen)
			}

			req.Body = buf[req.Keylen+elen : req.Keylen+elen+bodyLen]
			req.ExtMeta = buf[req.Keylen+elen+bodyLen:]
		}
	}
	return m, err
}

func (req *MCRequest) populateFlexBody(r io.Reader, totalBodyLen, elen, framingElen int) (int, error) {
	var m int
	var err error
	if totalBodyLen > 0 {
		buf := make([]byte, totalBodyLen)
		m, err = io.ReadFull(r, buf)
		if err != nil {
			return m, err
		}
		err = req.populateFlexBodyInternal(buf, totalBodyLen, elen, framingElen)
	}
	return m, err
}

func (req *MCRequest) populateFlexBodyInternal(buf []byte, totalBodyLen, elen, framingElen int) error {
	var halfByteOffset bool
	var err error
	if framingElen > 0 {
		var objs []FrameInfo
		objs, err, halfByteOffset = parseFrameInfoObjects(buf, framingElen)
		if err != nil {
			return err
		}
		req.FramingExtras = objs
	}

	err = req.populateFlexBodyAfterFrames(buf, totalBodyLen, elen, framingElen, halfByteOffset)
	if err != nil {
		return err
	}

	return nil
}

func (req *MCRequest) populateFlexBodyAfterFrames(buf []byte, totalBodyLen, elen, framingElen int, halfByteOffset bool) error {
	var idxCursor int = framingElen
	if req.Opcode >= TAP_MUTATION && req.Opcode <= TAP_CHECKPOINT_END && len(buf[idxCursor:]) > 1 {
		// In these commands there is "engine private"
		// data at the end of the extras.  The first 2
		// bytes of extra data give its length.
		if !halfByteOffset {
			elen += int(binary.BigEndian.Uint16(buf[idxCursor:]))
		} else {
			// 0 1 2 3 4 .... 19 20 21 22 ... 32
			// ^-----^ ^-------^  ^------------^
			//  offset   data       do not care
			var buffer uint32 = binary.BigEndian.Uint32(buf[idxCursor:])
			buffer &= 0xffff000
			elen += int(buffer >> 12)
		}
	}

	// Get the extras
	if !halfByteOffset {
		req.Extras = buf[idxCursor : idxCursor+elen]
	} else {
		preShift := buf[idxCursor : idxCursor+elen+1]
		req.Extras = ShiftByteSliceLeft4Bits(preShift)
	}
	idxCursor += elen

	// Get the Key
	if !halfByteOffset {
		req.Key = buf[idxCursor : idxCursor+req.Keylen]
	} else {
		preShift := buf[idxCursor : idxCursor+req.Keylen+1]
		req.Key = ShiftByteSliceLeft4Bits(preShift)
	}
	idxCursor += req.Keylen

	// get the length of extended metadata
	extMetaLen := 0
	if elen > 29 {
		extMetaLen = int(binary.BigEndian.Uint16(req.Extras[28:30]))
	}
	idxCursor += extMetaLen

	bodyLen := totalBodyLen - req.Keylen - elen - extMetaLen - framingElen
	if bodyLen > MaxBodyLen {
		return fmt.Errorf("%d is too big (max %d)",
			bodyLen, MaxBodyLen)
	}

	if !halfByteOffset {
		req.Body = buf[idxCursor : idxCursor+bodyLen]
		idxCursor += bodyLen
	} else {
		preShift := buf[idxCursor : idxCursor+bodyLen+1]
		req.Body = ShiftByteSliceLeft4Bits(preShift)
		idxCursor += bodyLen
	}

	if extMetaLen > 0 {
		if !halfByteOffset {
			req.ExtMeta = buf[idxCursor:]
		} else {
			preShift := buf[idxCursor:]
			req.ExtMeta = ShiftByteSliceLeft4Bits(preShift)
		}
	}

	return nil
}

// Receive will fill this MCRequest with the data from a reader.
func (req *MCRequest) Receive(r io.Reader, hdrBytes []byte) (int, error) {
	if len(hdrBytes) < HDR_LEN {
		hdrBytes = []byte{
			0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0}
	}
	n, err := io.ReadFull(r, hdrBytes)
	if err != nil {
		fmt.Printf("Err %v\n", err)
		return n, err
	}

	switch hdrBytes[0] {
	case RES_MAGIC:
		fallthrough
	case REQ_MAGIC:
		elen, totalBodyLen := req.receiveRegHeader(hdrBytes)
		bodyRead, err := req.populateRegularBody(r, totalBodyLen, elen)
		return n + bodyRead, err
	case FLEX_MAGIC:
		elen, totalBodyLen, framingElen := req.receiveFlexibleFramingHeader(hdrBytes)
		bodyRead, err := req.populateFlexBody(r, totalBodyLen, elen, framingElen)
		return n + bodyRead, err
	default:
		return n, fmt.Errorf("bad magic: 0x%02x", hdrBytes[0])
	}
}
