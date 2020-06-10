package rardecode

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
)

const (
	fileSize = 0x1000000

	vmGlobalAddr      = 0x3C000
	vmGlobalSize      = 0x02000
	vmFixedGlobalSize = 0x40

	maxUint32 = 1<<32 - 1
)

// v3Filter is the interface type for RAR V3 filters.
// v3Filter performs the same function as the filter type, except that it also takes
// the initial register values r, and global data as input for the RAR V3 VM.
type v3Filter func(r map[int]uint32, global, buf []byte, offset int64) ([]byte, error)

var (
	// standardV3Filters is a list of known filters. We can replace the use of a vm
	// filter with a custom filter function.
	standardV3Filters = []struct {
		crc uint32   // crc of code byte slice for filter
		len int      // length of code byte slice for filter
		f   v3Filter // replacement filter function
	}{
		{0xad576887, 53, e8FilterV3},
		{0x3cd7e57e, 57, e8e9FilterV3},
		{0x3769893f, 120, itaniumFilterV3},
		{0x0e06077d, 29, deltaFilterV3},
		{0x1c2c5dc8, 149, filterRGBV3},
		{0xbc85e701, 216, filterAudioV3},
	}

	// itanium filter byte masks
	byteMask = []int{4, 4, 6, 6, 0, 0, 7, 7, 4, 4, 0, 0, 4, 4, 0, 0}
)

func filterE8(c byte, v5 bool, buf []byte, offset int64) ([]byte, error) {
	off := int32(offset)
	for b := buf; len(b) >= 5; {
		ch := b[0]
		b = b[1:]
		off++
		if ch != 0xe8 && ch != c {
			continue
		}
		if v5 {
			off %= fileSize
		}
		addr := int32(binary.LittleEndian.Uint32(b))
		if addr < 0 {
			if addr+off >= 0 {
				binary.LittleEndian.PutUint32(b, uint32(addr+fileSize))
			}
		} else if addr < fileSize {
			binary.LittleEndian.PutUint32(b, uint32(addr-off))
		}
		off += 4
		b = b[4:]
	}
	return buf, nil
}

func e8FilterV3(r map[int]uint32, global, buf []byte, offset int64) ([]byte, error) {
	return filterE8(0xe8, false, buf, offset)
}

func e8e9FilterV3(r map[int]uint32, global, buf []byte, offset int64) ([]byte, error) {
	return filterE8(0xe9, false, buf, offset)
}

func getBits(buf []byte, pos, count uint) uint32 {
	n := binary.LittleEndian.Uint32(buf[pos/8:])
	n >>= pos & 7
	mask := uint32(maxUint32) >> (32 - count)
	return n & mask
}

func setBits(buf []byte, pos, count uint, bits uint32) {
	mask := uint32(maxUint32) >> (32 - count)
	mask <<= pos & 7
	bits <<= pos & 7
	n := binary.LittleEndian.Uint32(buf[pos/8:])
	n = (n & ^mask) | (bits & mask)
	binary.LittleEndian.PutUint32(buf[pos/8:], n)
}

func itaniumFilterV3(r map[int]uint32, global, buf []byte, offset int64) ([]byte, error) {
	fileOffset := uint32(offset) >> 4

	for b := buf; len(b) > 21; b = b[16:] {
		c := int(b[0]&0x1f) - 0x10
		if c >= 0 {
			mask := byteMask[c]
			if mask != 0 {
				for i := uint(0); i <= 2; i++ {
					if mask&(1<<i) == 0 {
						continue
					}
					pos := i*41 + 18
					if getBits(b, pos+24, 4) == 5 {
						n := getBits(b, pos, 20)
						n -= fileOffset
						setBits(b, pos, 20, n)
					}
				}
			}
		}
		fileOffset++
	}
	return buf, nil
}

func filterDelta(n int, buf []byte) ([]byte, error) {
	var res []byte
	l := len(buf)
	if cap(buf) >= 2*l {
		res = buf[l : 2*l] // use unused capacity
	} else {
		res = make([]byte, l, 2*l)
	}

	i := 0
	for j := 0; j < n; j++ {
		var c byte
		for k := j; k < len(res); k += n {
			c -= buf[i]
			i++
			res[k] = c
		}
	}
	return res, nil
}

func deltaFilterV3(r map[int]uint32, global, buf []byte, offset int64) ([]byte, error) {
	return filterDelta(int(r[0]), buf)
}

func abs(n int) int {
	if n < 0 {
		n = -n
	}
	return n
}

func filterRGBV3(r map[int]uint32, global, buf []byte, offset int64) ([]byte, error) {
	width := int(r[0] - 3)
	posR := int(r[1])
	if posR < 0 || width < 0 {
		return buf, nil
	}

	var res []byte
	l := len(buf)
	if cap(buf) >= 2*l {
		res = buf[l : 2*l] // use unused capacity
	} else {
		res = make([]byte, l, 2*l)
	}

	for c := 0; c < 3; c++ {
		var prevByte int
		for i := c; i < len(res); i += 3 {
			var predicted int
			upperPos := i - width
			if upperPos >= 3 {
				upperByte := int(res[upperPos])
				upperLeftByte := int(res[upperPos-3])
				predicted = prevByte + upperByte - upperLeftByte
				pa := abs(predicted - prevByte)
				pb := abs(predicted - upperByte)
				pc := abs(predicted - upperLeftByte)
				if pa <= pb && pa <= pc {
					predicted = prevByte
				} else if pb <= pc {
					predicted = upperByte
				} else {
					predicted = upperLeftByte
				}
			} else {
				predicted = prevByte
			}
			prevByte = (predicted - int(buf[0])) & 0xFF
			res[i] = uint8(prevByte)
			buf = buf[1:]
		}

	}
	for i := posR; i < len(res)-2; i += 3 {
		c := res[i+1]
		res[i] += c
		res[i+2] += c
	}
	return res, nil
}

func filterAudioV3(r map[int]uint32, global, buf []byte, offset int64) ([]byte, error) {
	var res []byte
	l := len(buf)
	if cap(buf) >= 2*l {
		res = buf[l : 2*l] // use unused capacity
	} else {
		res = make([]byte, l, 2*l)
	}

	chans := int(r[0])
	for c := 0; c < chans; c++ {
		var prevByte, byteCount int
		var diff [7]int
		var d, k [3]int

		for i := c; i < len(res); i += chans {
			predicted := prevByte<<3 + k[0]*d[0] + k[1]*d[1] + k[2]*d[2]
			predicted = int(int8(predicted >> 3))

			curByte := int(int8(buf[0]))
			buf = buf[1:]
			predicted -= curByte
			res[i] = uint8(predicted)

			dd := curByte << 3
			diff[0] += abs(dd)
			diff[1] += abs(dd - d[0])
			diff[2] += abs(dd + d[0])
			diff[3] += abs(dd - d[1])
			diff[4] += abs(dd + d[1])
			diff[5] += abs(dd - d[2])
			diff[6] += abs(dd + d[2])

			prevDelta := int(int8(predicted - prevByte))
			prevByte = predicted
			d[2] = d[1]
			d[1] = prevDelta - d[0]
			d[0] = prevDelta

			if byteCount&0x1f == 0 {
				min := diff[0]
				diff[0] = 0
				n := 0
				for j := 1; j < len(diff); j++ {
					if diff[j] < min {
						min = diff[j]
						n = j
					}
					diff[j] = 0
				}
				n--
				if n >= 0 {
					m := n / 2
					if n%2 == 0 {
						if k[m] >= -16 {
							k[m]--
						}
					} else {
						if k[m] < 16 {
							k[m]++
						}
					}
				}
			}
			byteCount++
		}

	}
	return res, nil
}

func filterArm(buf []byte, offset int64) ([]byte, error) {
	for i := 0; len(buf)-i > 3; i += 4 {
		if buf[i+3] == 0xeb {
			n := uint(buf[i])
			n += uint(buf[i+1]) * 0x100
			n += uint(buf[i+2]) * 0x10000
			n -= (uint(offset) + uint(i)) / 4
			buf[i] = byte(n)
			buf[i+1] = byte(n >> 8)
			buf[i+2] = byte(n >> 16)
		}
	}
	return buf, nil
}

type vmFilter struct {
	execCount uint32
	global    []byte
	static    []byte
	code      []command
}

// execute implements v3filter type for VM based RAR 3 filters.
func (f *vmFilter) execute(r map[int]uint32, global, buf []byte, offset int64) ([]byte, error) {
	if len(buf) > vmGlobalAddr {
		return buf, errInvalidFilter
	}
	v := newVM(buf)

	// register setup
	v.r[3] = vmGlobalAddr
	v.r[4] = uint32(len(buf))
	v.r[5] = f.execCount
	for i, n := range r {
		v.r[i] = n
	}

	// vm global data memory block
	vg := v.m[vmGlobalAddr : vmGlobalAddr+vmGlobalSize]

	// initialize fixed global memory
	for i, n := range v.r[:vmRegs-1] {
		binary.LittleEndian.PutUint32(vg[i*4:], n)
	}
	binary.LittleEndian.PutUint32(vg[0x1c:], uint32(len(buf)))
	binary.LittleEndian.PutUint64(vg[0x24:], uint64(offset))
	binary.LittleEndian.PutUint32(vg[0x2c:], f.execCount)

	// registers
	v.r[6] = uint32(offset)

	// copy program global memory
	var n int
	if len(f.global) > 0 {
		n = copy(vg[vmFixedGlobalSize:], f.global) // use saved global instead
	} else {
		n = copy(vg[vmFixedGlobalSize:], global)
	}
	copy(vg[vmFixedGlobalSize+n:], f.static)

	v.execute(f.code)

	f.execCount++

	// keep largest global buffer
	if cap(global) > cap(f.global) {
		f.global = global[:0]
	} else if len(f.global) > 0 {
		f.global = f.global[:0]
	}

	// check for global data to be saved for next program execution
	globalSize := binary.LittleEndian.Uint32(vg[0x30:])
	if globalSize > 0 {
		if globalSize > vmGlobalSize-vmFixedGlobalSize {
			globalSize = vmGlobalSize - vmFixedGlobalSize
		}
		if cap(f.global) < int(globalSize) {
			f.global = make([]byte, globalSize)
		} else {
			f.global = f.global[:globalSize]
		}
		copy(f.global, vg[vmFixedGlobalSize:])
	}

	// find program output
	length := binary.LittleEndian.Uint32(vg[0x1c:]) & vmMask
	start := binary.LittleEndian.Uint32(vg[0x20:]) & vmMask
	if start+length > vmSize {
		// TODO: error
		start = 0
		length = 0
	}
	if start != 0 && cap(v.m) > cap(buf) {
		// Initial buffer was to small for vm.
		// Copy output to beginning of vm memory so that decodeReader
		// will re-use the newly allocated vm memory and we will not
		// have to reallocate again next time.
		copy(v.m, v.m[start:start+length])
		start = 0
	}
	return v.m[start : start+length], nil
}

// getV3Filter returns a V3 filter function from a code byte slice.
func getV3Filter(code []byte) (v3Filter, error) {
	// check if filter is a known standard filter
	c := crc32.ChecksumIEEE(code)
	for _, f := range standardV3Filters {
		if f.crc == c && f.len == len(code) {
			return f.f, nil
		}
	}

	// create new vm filter
	f := new(vmFilter)
	r := newRarBitReader(bytes.NewReader(code[1:])) // skip first xor byte check

	// read static data
	n, err := r.readBits(1)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		m, err := r.readUint32()
		if err != nil {
			return nil, err
		}
		f.static = make([]byte, m+1)
		err = r.readFull(f.static)
		if err != nil {
			return nil, err
		}
	}

	f.code, err = readCommands(r)
	if err == io.EOF {
		err = nil
	}

	return f.execute, err
}
