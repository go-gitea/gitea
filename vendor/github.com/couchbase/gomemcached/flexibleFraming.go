package gomemcached

import (
	"encoding/binary"
	"fmt"
)

type FrameObjType int

const (
	FrameBarrier     FrameObjType = iota
	FrameDurability  FrameObjType = iota
	FrameDcpStreamId FrameObjType = iota
	FrameOpenTracing FrameObjType = iota
)

type FrameInfo struct {
	ObjId   FrameObjType
	ObjLen  int
	ObjData []byte
}

var ErrorInvalidOp error = fmt.Errorf("Specified method is not applicable")
var ErrorObjLenNotMatch error = fmt.Errorf("Object length does not match data")

func (f *FrameInfo) Validate() error {
	switch f.ObjId {
	case FrameBarrier:
		if f.ObjLen != 0 {
			return fmt.Errorf("Invalid FrameBarrier - length is %v\n", f.ObjLen)
		} else if f.ObjLen != len(f.ObjData) {
			return ErrorObjLenNotMatch
		}
	case FrameDurability:
		if f.ObjLen != 1 && f.ObjLen != 3 {
			return fmt.Errorf("Invalid FrameDurability - length is %v\n", f.ObjLen)
		} else if f.ObjLen != len(f.ObjData) {
			return ErrorObjLenNotMatch
		}
	case FrameDcpStreamId:
		if f.ObjLen != 2 {
			return fmt.Errorf("Invalid FrameDcpStreamId - length is %v\n", f.ObjLen)
		} else if f.ObjLen != len(f.ObjData) {
			return ErrorObjLenNotMatch
		}
	case FrameOpenTracing:
		if f.ObjLen == 0 {
			return fmt.Errorf("Invalid FrameOpenTracing - length must be > 0")
		} else if f.ObjLen != len(f.ObjData) {
			return ErrorObjLenNotMatch
		}
	default:
		return fmt.Errorf("Unknown FrameInfo type")
	}
	return nil
}

func (f *FrameInfo) GetStreamId() (uint16, error) {
	if f.ObjId != FrameDcpStreamId {
		return 0, ErrorInvalidOp
	}

	var output uint16
	output = uint16(f.ObjData[0])
	output = output << 8
	output |= uint16(f.ObjData[1])
	return output, nil
}

type DurabilityLvl uint8

const (
	DuraInvalid                    DurabilityLvl = iota // Not used (0x0)
	DuraMajority                   DurabilityLvl = iota // (0x01)
	DuraMajorityAndPersistOnMaster DurabilityLvl = iota // (0x02)
	DuraPersistToMajority          DurabilityLvl = iota // (0x03)
)

func (f *FrameInfo) GetDurabilityRequirements() (lvl DurabilityLvl, timeoutProvided bool, timeoutMs uint16, err error) {
	if f.ObjId != FrameDurability {
		err = ErrorInvalidOp
		return
	}
	if f.ObjLen != 1 && f.ObjLen != 3 {
		err = ErrorObjLenNotMatch
		return
	}

	lvl = DurabilityLvl(uint8(f.ObjData[0]))

	if f.ObjLen == 3 {
		timeoutProvided = true
		timeoutMs = binary.BigEndian.Uint16(f.ObjData[1:2])
	}

	return
}

func incrementMarker(bitsToBeIncremented, byteIncrementCnt *int, framingElen, curObjIdx int) (int, error) {
	for *bitsToBeIncremented >= 8 {
		*byteIncrementCnt++
		*bitsToBeIncremented -= 8
	}
	marker := curObjIdx + *byteIncrementCnt
	if marker > framingElen {
		return -1, fmt.Errorf("Out of bounds")
	}
	return marker, nil
}

// Right now, halfByteRemaining will always be false, because ObjID and Len haven't gotten that large yet
func (f *FrameInfo) Bytes() (output []byte, halfByteRemaining bool) {
	// ObjIdentifier - 4 bits + ObjLength - 4 bits
	var idAndLen uint8
	idAndLen |= uint8(f.ObjId) << 4
	idAndLen |= uint8(f.ObjLen)
	output = append(output, byte(idAndLen))

	// Rest is Data
	output = append(output, f.ObjData...)
	return
}

func parseFrameInfoObjects(buf []byte, framingElen int) (objs []FrameInfo, err error, halfByteRemaining bool) {
	var curObjIdx int
	var byteIncrementCnt int
	var bitsToBeIncremented int
	var marker int

	// Parse frameInfo objects
	for curObjIdx = 0; curObjIdx < framingElen; curObjIdx += byteIncrementCnt {
		byteIncrementCnt = 0
		var oneFrameObj FrameInfo

		// First get the objId
		// -------------------------
		var objId int
		var objHeader uint8 = buf[curObjIdx]
		var objIdentifierRaw uint8
		if bitsToBeIncremented == 0 {
			// ObjHeader
			// 0 1 2 3 4 5 6 7
			// ^-----^
			// ObjIdentifierRaw
			objIdentifierRaw = (objHeader & 0xf0) >> 4
		} else {
			// ObjHeader
			// 0 1 2 3 4 5 6 7
			//         ^-----^
			//           ObjIdentifierRaw
			objIdentifierRaw = (objHeader & 0x0f)
		}
		bitsToBeIncremented += 4

		marker, err = incrementMarker(&bitsToBeIncremented, &byteIncrementCnt, framingElen, curObjIdx)
		if err != nil {
			return
		}

		// Value is 0-14
		objId = int(objIdentifierRaw & 0xe)
		// If bit 15 is set, ID is 15 + value of next byte
		if objIdentifierRaw&0x1 > 0 {
			if bitsToBeIncremented > 0 {
				// ObjHeader
				// 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
				// ^-----^ ^---------------^
				// ObjId1    Extension
				// ^ marker
				buffer := uint16(buf[marker])
				buffer = buffer << 8
				buffer |= uint16(buf[marker+1])
				var extension uint8 = uint8(buffer & 0xff0 >> 4)
				objId += int(extension)
			} else {
				// ObjHeader
				// 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
				//         ^-----^ ^-------------------^
				//          ObjId1    extension
				//                 ^ marker
				var extension uint8 = uint8(buf[marker])
				objId += int(extension)
			}
			bitsToBeIncremented += 8
		}

		marker, err = incrementMarker(&bitsToBeIncremented, &byteIncrementCnt, framingElen, curObjIdx)
		if err != nil {
			return
		}
		oneFrameObj.ObjId = FrameObjType(objId)

		// Then get the obj length
		// -------------------------
		var objLenRaw uint8
		var objLen int
		if bitsToBeIncremented > 0 {
			// ObjHeader
			// 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
			//                 ^         ^---------^
			//                 marker       objLen
			objLenRaw = uint8(buf[marker]) & 0x0f
		} else {
			// ObjHeader
			// 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19
			//                                        ^--------^
			//                                          objLen
			//                                        ^ marker
			objLenRaw = uint8(buf[marker]) & 0xf0 >> 4
		}
		bitsToBeIncremented += 4

		marker, err = incrementMarker(&bitsToBeIncremented, &byteIncrementCnt, framingElen, curObjIdx)
		if err != nil {
			return
		}

		// Length is 0-14
		objLen = int(objLenRaw & 0xe)
		// If bit 15 is set, lenghth is 15 + value of next byte
		if objLenRaw&0x1 > 0 {
			if bitsToBeIncremented == 0 {
				// ObjHeader
				// 12 13 14 15 16 17 18 19 20 21 22 23
				// ^---------^ ^--------------------^
				//   objLen        extension
				//             ^ marker
				var extension uint8 = uint8(buf[marker])
				objLen += int(extension)
			} else {
				// ObjHeader
				// 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31
				// ^--------^  ^---------------------^
				//  objLen          extension
				// ^ marker				var buffer uint16
				buffer := uint16(buf[marker])
				buffer = buffer << 8
				buffer |= uint16(buf[marker+1])
				var extension uint8 = uint8(buffer & 0xff0 >> 4)
				objLen += int(extension)
			}
			bitsToBeIncremented += 8
		}

		marker, err = incrementMarker(&bitsToBeIncremented, &byteIncrementCnt, framingElen, curObjIdx)
		if err != nil {
			return
		}
		oneFrameObj.ObjLen = objLen

		// The rest is N-bytes of data based on the length
		if bitsToBeIncremented == 0 {
			// No weird alignment needed
			oneFrameObj.ObjData = buf[marker : marker+objLen]
		} else {
			// 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31
			// ^--------^  ^---------------------^ ^--------->
			//  objLen          extension            data
			//                          ^ marker
			oneFrameObj.ObjData = ShiftByteSliceLeft4Bits(buf[marker : marker+objLen+1])
		}
		err = oneFrameObj.Validate()
		if err != nil {
			return
		}
		objs = append(objs, oneFrameObj)

		bitsToBeIncremented += 8 * objLen
		marker, err = incrementMarker(&bitsToBeIncremented, &byteIncrementCnt, framingElen, curObjIdx)
	}

	if bitsToBeIncremented > 0 {
		halfByteRemaining = true
	}
	return
}

func ShiftByteSliceLeft4Bits(slice []byte) (replacement []byte) {
	var buffer uint16
	var i int
	sliceLen := len(slice)

	if sliceLen < 2 {
		// Let's not shift less than 16 bits
		return
	}

	replacement = make([]byte, sliceLen, cap(slice))

	for i = 0; i < sliceLen-1; i++ {
		// 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
		// ^-----^ ^---------------^ ^-----------
		// garbage   data byte 0       data byte 1
		buffer = uint16(slice[i])
		buffer = buffer << 8
		buffer |= uint16(slice[i+1])
		replacement[i] = uint8(buffer & 0xff0 >> 4)
	}

	if i < sliceLen {
		lastByte := slice[sliceLen-1]
		lastByte = lastByte << 4
		replacement[i] = lastByte
	}
	return
}

// The following is used to theoretically support frameInfo ObjID extensions
// for completeness, but they are not very efficient though
func ShiftByteSliceRight4Bits(slice []byte) (replacement []byte) {
	var buffer uint16
	var i int
	var leftovers uint8 // 4 bits only
	var replacementUnit uint16
	var first bool = true
	var firstLeftovers uint8
	var lastLeftovers uint8
	sliceLen := len(slice)

	if sliceLen < 2 {
		// Let's not shift less than 16 bits
		return
	}

	if slice[sliceLen-1]&0xf == 0 {
		replacement = make([]byte, sliceLen, cap(slice))
	} else {
		replacement = make([]byte, sliceLen+1, cap(slice)+1)
	}

	for i = 0; i < sliceLen-1; i++ {
		buffer = binary.BigEndian.Uint16(slice[i : i+2])
		// (buffer)
		// 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
		// ^-------------^ ^-------------------^
		//     data byte 0        data byte 1
		//
		// into
		//
		// 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23
		// ^-----^ ^---------------^ ^--------------------^ ^----------^
		// zeroes   data byte 0      data byte 1              zeroes

		if first {
			// The leftover OR'ing will overwrite the first 4 bits of data byte 0. Save them
			firstLeftovers = uint8(buffer & 0xf000 >> 12)
			first = false
		}
		replacementUnit = 0
		replacementUnit |= uint16(leftovers) << 12
		replacementUnit |= (buffer & 0xff00) >> 4 // data byte 0
		replacementUnit |= buffer & 0xff >> 4     // data byte 1 first 4 bits
		lastLeftovers = uint8(buffer&0xf) << 4

		replacement[i+1] = byte(replacementUnit)

		leftovers = uint8((buffer & 0x000f) << 4)
	}

	replacement[0] = byte(uint8(replacement[0]) | firstLeftovers)
	if lastLeftovers > 0 {
		replacement[sliceLen] = byte(lastLeftovers)
	}
	return
}

func Merge2HalfByteSlices(src1, src2 []byte) (output []byte) {
	src1Len := len(src1)
	src2Len := len(src2)
	output = make([]byte, src1Len+src2Len-1)

	var mergeByte uint8 = src1[src1Len-1]
	mergeByte |= uint8(src2[0])

	copy(output, src1)
	copy(output[src1Len:], src2[1:])

	output[src1Len-1] = byte(mergeByte)

	return
}
