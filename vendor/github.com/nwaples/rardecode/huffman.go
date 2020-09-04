package rardecode

import (
	"errors"
	"io"
)

const (
	maxCodeLength = 15 // maximum code length in bits
	maxQuickBits  = 10
	maxQuickSize  = 1 << maxQuickBits
)

var (
	errHuffDecodeFailed   = errors.New("rardecode: huffman decode failed")
	errInvalidLengthTable = errors.New("rardecode: invalid huffman code length table")
)

type huffmanDecoder struct {
	limit     [maxCodeLength + 1]int
	pos       [maxCodeLength + 1]int
	symbol    []int
	min       uint
	quickbits uint
	quicklen  [maxQuickSize]uint
	quicksym  [maxQuickSize]int
}

func (h *huffmanDecoder) init(codeLengths []byte) {
	var count [maxCodeLength + 1]int

	for _, n := range codeLengths {
		if n == 0 {
			continue
		}
		count[n]++
	}

	h.pos[0] = 0
	h.limit[0] = 0
	h.min = 0
	for i := uint(1); i <= maxCodeLength; i++ {
		h.limit[i] = h.limit[i-1] + count[i]<<(maxCodeLength-i)
		h.pos[i] = h.pos[i-1] + count[i-1]
		if h.min == 0 && h.limit[i] > 0 {
			h.min = i
		}
	}

	if cap(h.symbol) >= len(codeLengths) {
		h.symbol = h.symbol[:len(codeLengths)]
		for i := range h.symbol {
			h.symbol[i] = 0
		}
	} else {
		h.symbol = make([]int, len(codeLengths))
	}

	copy(count[:], h.pos[:])
	for i, n := range codeLengths {
		if n != 0 {
			h.symbol[count[n]] = i
			count[n]++
		}
	}

	if len(codeLengths) >= 298 {
		h.quickbits = maxQuickBits
	} else {
		h.quickbits = maxQuickBits - 3
	}

	bits := uint(1)
	for i := 0; i < 1<<h.quickbits; i++ {
		v := i << (maxCodeLength - h.quickbits)

		for v >= h.limit[bits] && bits < maxCodeLength {
			bits++
		}
		h.quicklen[i] = bits

		dist := v - h.limit[bits-1]
		dist >>= (maxCodeLength - bits)

		pos := h.pos[bits] + dist
		if pos < len(h.symbol) {
			h.quicksym[i] = h.symbol[pos]
		} else {
			h.quicksym[i] = 0
		}
	}
}

func (h *huffmanDecoder) readSym(r bitReader) (int, error) {
	bits := uint(maxCodeLength)
	v, err := r.readBits(maxCodeLength)
	if err != nil {
		if err != io.EOF {
			return 0, err
		}
		// fall back to 1 bit at a time if we read past EOF
		for i := uint(1); i <= maxCodeLength; i++ {
			b, err := r.readBits(1)
			if err != nil {
				return 0, err // not enough bits return error
			}
			v |= b << (maxCodeLength - i)
			if v < h.limit[i] {
				bits = i
				break
			}
		}
	} else {
		if v < h.limit[h.quickbits] {
			i := v >> (maxCodeLength - h.quickbits)
			r.unreadBits(maxCodeLength - h.quicklen[i])
			return h.quicksym[i], nil
		}

		for i, n := range h.limit[h.min:] {
			if v < n {
				bits = h.min + uint(i)
				r.unreadBits(maxCodeLength - bits)
				break
			}
		}
	}

	dist := v - h.limit[bits-1]
	dist >>= maxCodeLength - bits

	pos := h.pos[bits] + dist
	if pos > len(h.symbol) {
		return 0, errHuffDecodeFailed
	}

	return h.symbol[pos], nil
}

// readCodeLengthTable reads a new code length table into codeLength from br.
// If addOld is set the old table is added to the new one.
func readCodeLengthTable(br bitReader, codeLength []byte, addOld bool) error {
	var bitlength [20]byte
	for i := 0; i < len(bitlength); i++ {
		n, err := br.readBits(4)
		if err != nil {
			return err
		}
		if n == 0xf {
			cnt, err := br.readBits(4)
			if err != nil {
				return err
			}
			if cnt > 0 {
				// array already zero'd dont need to explicitly set
				i += cnt + 1
				continue
			}
		}
		bitlength[i] = byte(n)
	}

	var bl huffmanDecoder
	bl.init(bitlength[:])

	for i := 0; i < len(codeLength); i++ {
		l, err := bl.readSym(br)
		if err != nil {
			return err
		}

		if l < 16 {
			if addOld {
				codeLength[i] = (codeLength[i] + byte(l)) & 0xf
			} else {
				codeLength[i] = byte(l)
			}
			continue
		}

		var count int
		var value byte

		switch l {
		case 16, 18:
			count, err = br.readBits(3)
			count += 3
		default:
			count, err = br.readBits(7)
			count += 11
		}
		if err != nil {
			return err
		}
		if l < 18 {
			if i == 0 {
				return errInvalidLengthTable
			}
			value = codeLength[i-1]
		}
		for ; count > 0 && i < len(codeLength); i++ {
			codeLength[i] = value
			count--
		}
		i--
	}
	return nil
}
