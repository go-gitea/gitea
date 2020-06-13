package rardecode

import (
	"errors"
	"io"
)

const (
	mainSize5      = 306
	offsetSize5    = 64
	lowoffsetSize5 = 16
	lengthSize5    = 44
	tableSize5     = mainSize5 + offsetSize5 + lowoffsetSize5 + lengthSize5
)

var (
	errUnknownFilter       = errors.New("rardecode: unknown V5 filter")
	errCorruptDecodeHeader = errors.New("rardecode: corrupt decode header")
)

// decoder50 implements the decoder interface for RAR 5 compression.
// Decode input it broken up into 1 or more blocks. Each block starts with
// a header containing block length and optional code length tables to initialize
// the huffman decoders with.
type decoder50 struct {
	r          io.ByteReader
	br         bitReader // bit reader for current data block
	codeLength [tableSize5]byte

	lastBlock bool // current block is last block in compressed file

	mainDecoder      huffmanDecoder
	offsetDecoder    huffmanDecoder
	lowoffsetDecoder huffmanDecoder
	lengthDecoder    huffmanDecoder

	offset [4]int
	length int
}

func (d *decoder50) init(r io.ByteReader, reset bool) error {
	d.r = r
	d.lastBlock = false

	if reset {
		for i := range d.offset {
			d.offset[i] = 0
		}
		d.length = 0
		for i := range d.codeLength {
			d.codeLength[i] = 0
		}
	}
	err := d.readBlockHeader()
	if err == io.EOF {
		return errDecoderOutOfData
	}
	return err
}

func (d *decoder50) readBlockHeader() error {
	flags, err := d.r.ReadByte()
	if err != nil {
		return err
	}

	bytecount := (flags>>3)&3 + 1
	if bytecount == 4 {
		return errCorruptDecodeHeader
	}

	hsum, err := d.r.ReadByte()
	if err != nil {
		return err
	}

	blockBits := int(flags)&0x07 + 1
	blockBytes := 0
	sum := 0x5a ^ flags
	for i := byte(0); i < bytecount; i++ {
		n, err := d.r.ReadByte()
		if err != nil {
			return err
		}
		sum ^= n
		blockBytes |= int(n) << (i * 8)
	}
	if sum != hsum { // bad header checksum
		return errCorruptDecodeHeader
	}
	blockBits += (blockBytes - 1) * 8

	// create bit reader for block
	d.br = limitBitReader(newRarBitReader(d.r), blockBits, errDecoderOutOfData)
	d.lastBlock = flags&0x40 > 0

	if flags&0x80 > 0 {
		// read new code length tables and reinitialize huffman decoders
		cl := d.codeLength[:]
		err = readCodeLengthTable(d.br, cl, false)
		if err != nil {
			return err
		}
		d.mainDecoder.init(cl[:mainSize5])
		cl = cl[mainSize5:]
		d.offsetDecoder.init(cl[:offsetSize5])
		cl = cl[offsetSize5:]
		d.lowoffsetDecoder.init(cl[:lowoffsetSize5])
		cl = cl[lowoffsetSize5:]
		d.lengthDecoder.init(cl)
	}
	return nil
}

func slotToLength(br bitReader, n int) (int, error) {
	if n >= 8 {
		bits := uint(n/4 - 1)
		n = (4 | (n & 3)) << bits
		if bits > 0 {
			b, err := br.readBits(bits)
			if err != nil {
				return 0, err
			}
			n |= b
		}
	}
	n += 2
	return n, nil
}

// readFilter5Data reads an encoded integer used in V5 filters.
func readFilter5Data(br bitReader) (int, error) {
	// TODO: should data really be uint? (for 32bit ints).
	// It will be masked later anyway by decode window mask.
	bytes, err := br.readBits(2)
	if err != nil {
		return 0, err
	}
	bytes++

	var data int
	for i := 0; i < bytes; i++ {
		n, err := br.readBits(8)
		if err != nil {
			return 0, err
		}
		data |= n << (uint(i) * 8)
	}
	return data, nil
}

func readFilter(br bitReader) (*filterBlock, error) {
	fb := new(filterBlock)
	var err error

	fb.offset, err = readFilter5Data(br)
	if err != nil {
		return nil, err
	}
	fb.length, err = readFilter5Data(br)
	if err != nil {
		return nil, err
	}
	ftype, err := br.readBits(3)
	if err != nil {
		return nil, err
	}
	switch ftype {
	case 0:
		n, err := br.readBits(5)
		if err != nil {
			return nil, err
		}
		fb.filter = func(buf []byte, offset int64) ([]byte, error) { return filterDelta(n+1, buf) }
	case 1:
		fb.filter = func(buf []byte, offset int64) ([]byte, error) { return filterE8(0xe8, true, buf, offset) }
	case 2:
		fb.filter = func(buf []byte, offset int64) ([]byte, error) { return filterE8(0xe9, true, buf, offset) }
	case 3:
		fb.filter = filterArm
	default:
		return nil, errUnknownFilter
	}
	return fb, nil
}

func (d *decoder50) decodeSym(win *window, sym int) (*filterBlock, error) {
	switch {
	case sym < 256:
		// literal
		win.writeByte(byte(sym))
		return nil, nil
	case sym == 256:
		f, err := readFilter(d.br)
		f.offset += win.buffered()
		return f, err
	case sym == 257:
		// use previous offset and length
	case sym < 262:
		i := sym - 258
		offset := d.offset[i]
		copy(d.offset[1:i+1], d.offset[:i])
		d.offset[0] = offset

		sl, err := d.lengthDecoder.readSym(d.br)
		if err != nil {
			return nil, err
		}
		d.length, err = slotToLength(d.br, sl)
		if err != nil {
			return nil, err
		}
	default:
		length, err := slotToLength(d.br, sym-262)
		if err != nil {
			return nil, err
		}

		offset := 1
		slot, err := d.offsetDecoder.readSym(d.br)
		if err != nil {
			return nil, err
		}
		if slot < 4 {
			offset += slot
		} else {
			bits := uint(slot/2 - 1)
			offset += (2 | (slot & 1)) << bits

			if bits >= 4 {
				if bits > 4 {
					n, err := d.br.readBits(bits - 4)
					if err != nil {
						return nil, err
					}
					offset += n << 4
				}
				n, err := d.lowoffsetDecoder.readSym(d.br)
				if err != nil {
					return nil, err
				}
				offset += n
			} else {
				n, err := d.br.readBits(bits)
				if err != nil {
					return nil, err
				}
				offset += n
			}
		}
		if offset > 0x100 {
			length++
			if offset > 0x2000 {
				length++
				if offset > 0x40000 {
					length++
				}
			}
		}
		copy(d.offset[1:], d.offset[:])
		d.offset[0] = offset
		d.length = length
	}
	win.copyBytes(d.length, d.offset[0])
	return nil, nil
}

func (d *decoder50) fill(w *window) ([]*filterBlock, error) {
	var fl []*filterBlock

	for w.available() > 0 {
		sym, err := d.mainDecoder.readSym(d.br)
		if err == nil {
			var f *filterBlock
			f, err = d.decodeSym(w, sym)
			if f != nil {
				fl = append(fl, f)
			}
		} else if err == io.EOF {
			// reached end of the block
			if d.lastBlock {
				return fl, io.EOF
			}
			err = d.readBlockHeader()
		}
		if err != nil {
			if err == io.EOF {
				return fl, errDecoderOutOfData
			}
			return fl, err
		}
	}
	return fl, nil
}
