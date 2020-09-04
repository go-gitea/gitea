package rardecode

const (
	mainSize      = 299
	offsetSize    = 60
	lowOffsetSize = 17
	lengthSize    = 28
	tableSize     = mainSize + offsetSize + lowOffsetSize + lengthSize
)

var (
	lengthBase = [28]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 10, 12, 14, 16, 20,
		24, 28, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224}
	lengthExtraBits = [28]uint{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2,
		2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5}

	offsetBase = [60]int{0, 1, 2, 3, 4, 6, 8, 12, 16, 24, 32, 48, 64, 96,
		128, 192, 256, 384, 512, 768, 1024, 1536, 2048, 3072, 4096,
		6144, 8192, 12288, 16384, 24576, 32768, 49152, 65536, 98304,
		131072, 196608, 262144, 327680, 393216, 458752, 524288,
		589824, 655360, 720896, 786432, 851968, 917504, 983040,
		1048576, 1310720, 1572864, 1835008, 2097152, 2359296, 2621440,
		2883584, 3145728, 3407872, 3670016, 3932160}
	offsetExtraBits = [60]uint{0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6,
		6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14,
		15, 15, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16,
		18, 18, 18, 18, 18, 18, 18, 18, 18, 18, 18, 18}

	shortOffsetBase      = [8]int{0, 4, 8, 16, 32, 64, 128, 192}
	shortOffsetExtraBits = [8]uint{2, 2, 3, 4, 5, 6, 6, 6}
)

type lz29Decoder struct {
	codeLength [tableSize]byte

	mainDecoder      huffmanDecoder
	offsetDecoder    huffmanDecoder
	lowOffsetDecoder huffmanDecoder
	lengthDecoder    huffmanDecoder

	offset           [4]int // history of previous offsets
	length           int    // previous length
	lowOffset        int
	lowOffsetRepeats int

	br *rarBitReader
}

func (d *lz29Decoder) reset() {
	for i := range d.offset {
		d.offset[i] = 0
	}
	d.length = 0
	for i := range d.codeLength {
		d.codeLength[i] = 0
	}
}

func (d *lz29Decoder) init(br *rarBitReader) error {
	d.br = br
	d.lowOffset = 0
	d.lowOffsetRepeats = 0

	n, err := d.br.readBits(1)
	if err != nil {
		return err
	}
	addOld := n > 0

	cl := d.codeLength[:]
	if err = readCodeLengthTable(d.br, cl, addOld); err != nil {
		return err
	}

	d.mainDecoder.init(cl[:mainSize])
	cl = cl[mainSize:]
	d.offsetDecoder.init(cl[:offsetSize])
	cl = cl[offsetSize:]
	d.lowOffsetDecoder.init(cl[:lowOffsetSize])
	cl = cl[lowOffsetSize:]
	d.lengthDecoder.init(cl)

	return nil
}

func (d *lz29Decoder) readFilterData() (b []byte, err error) {
	flags, err := d.br.ReadByte()
	if err != nil {
		return nil, err
	}

	n := (int(flags) & 7) + 1
	switch n {
	case 7:
		n, err = d.br.readBits(8)
		n += 7
		if err != nil {
			return nil, err
		}
	case 8:
		n, err = d.br.readBits(16)
		if err != nil {
			return nil, err
		}
	}

	buf := make([]byte, n+1)
	buf[0] = flags
	err = d.br.readFull(buf[1:])

	return buf, err
}

func (d *lz29Decoder) readEndOfBlock() error {
	n, err := d.br.readBits(1)
	if err != nil {
		return err
	}
	if n > 0 {
		return endOfBlock
	}
	n, err = d.br.readBits(1)
	if err != nil {
		return err
	}
	if n > 0 {
		return endOfBlockAndFile
	}
	return endOfFile
}

func (d *lz29Decoder) decode(win *window) ([]byte, error) {
	sym, err := d.mainDecoder.readSym(d.br)
	if err != nil {
		return nil, err
	}

	switch {
	case sym < 256:
		// literal
		win.writeByte(byte(sym))
		return nil, nil
	case sym == 256:
		return nil, d.readEndOfBlock()
	case sym == 257:
		return d.readFilterData()
	case sym == 258:
		// use previous offset and length
	case sym < 263:
		i := sym - 259
		offset := d.offset[i]
		copy(d.offset[1:i+1], d.offset[:i])
		d.offset[0] = offset

		i, err := d.lengthDecoder.readSym(d.br)
		if err != nil {
			return nil, err
		}
		d.length = lengthBase[i] + 2
		bits := lengthExtraBits[i]
		if bits > 0 {
			n, err := d.br.readBits(bits)
			if err != nil {
				return nil, err
			}
			d.length += n
		}
	case sym < 271:
		i := sym - 263
		copy(d.offset[1:], d.offset[:])
		offset := shortOffsetBase[i] + 1
		bits := shortOffsetExtraBits[i]
		if bits > 0 {
			n, err := d.br.readBits(bits)
			if err != nil {
				return nil, err
			}
			offset += n
		}
		d.offset[0] = offset

		d.length = 2
	default:
		i := sym - 271
		d.length = lengthBase[i] + 3
		bits := lengthExtraBits[i]
		if bits > 0 {
			n, err := d.br.readBits(bits)
			if err != nil {
				return nil, err
			}
			d.length += n
		}

		i, err = d.offsetDecoder.readSym(d.br)
		if err != nil {
			return nil, err
		}
		offset := offsetBase[i] + 1
		bits = offsetExtraBits[i]

		switch {
		case bits >= 4:
			if bits > 4 {
				n, err := d.br.readBits(bits - 4)
				if err != nil {
					return nil, err
				}
				offset += n << 4
			}

			if d.lowOffsetRepeats > 0 {
				d.lowOffsetRepeats--
				offset += d.lowOffset
			} else {
				n, err := d.lowOffsetDecoder.readSym(d.br)
				if err != nil {
					return nil, err
				}
				if n == 16 {
					d.lowOffsetRepeats = 15
					offset += d.lowOffset
				} else {
					offset += n
					d.lowOffset = n
				}
			}
		case bits > 0:
			n, err := d.br.readBits(bits)
			if err != nil {
				return nil, err
			}
			offset += n
		}

		if offset >= 0x2000 {
			d.length++
			if offset >= 0x40000 {
				d.length++
			}
		}
		copy(d.offset[1:], d.offset[:])
		d.offset[0] = offset
	}
	win.copyBytes(d.length, d.offset[0])
	return nil, nil
}
