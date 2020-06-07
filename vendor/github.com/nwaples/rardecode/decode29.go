package rardecode

import (
	"bytes"
	"errors"
	"io"
)

const (
	maxCodeSize      = 0x10000
	maxUniqueFilters = 1024
)

var (
	// Errors marking the end of the decoding block and/or file
	endOfFile         = errors.New("rardecode: end of file")
	endOfBlock        = errors.New("rardecode: end of block")
	endOfBlockAndFile = errors.New("rardecode: end of block and file")
)

// decoder29 implements the decoder interface for RAR 3.0 compression (unpack version 29)
// Decode input is broken up into 1 or more blocks. The start of each block specifies
// the decoding algorithm (ppm or lz) and optional data to initialize with.
// Block length is not stored, it is determined only after decoding an end of file and/or
// block marker in the data.
type decoder29 struct {
	br      *rarBitReader
	eof     bool       // at file eof
	fnum    int        // current filter number (index into filters)
	flen    []int      // filter block length history
	filters []v3Filter // list of current filters used by archive encoding

	// current decode function (lz or ppm).
	// When called it should perform a single decode operation, and either apply the
	// data to the window or return they raw bytes for a filter.
	decode func(w *window) ([]byte, error)

	lz  lz29Decoder  // lz decoder
	ppm ppm29Decoder // ppm decoder
}

// init intializes the decoder for decoding a new file.
func (d *decoder29) init(r io.ByteReader, reset bool) error {
	if d.br == nil {
		d.br = newRarBitReader(r)
	} else {
		d.br.reset(r)
	}
	d.eof = false
	if reset {
		d.initFilters()
		d.lz.reset()
		d.ppm.reset()
		d.decode = nil
	}
	if d.decode == nil {
		return d.readBlockHeader()
	}
	return nil
}

func (d *decoder29) initFilters() {
	d.fnum = 0
	d.flen = nil
	d.filters = nil
}

// readVMCode reads the raw bytes for the code/commands used in a vm filter
func readVMCode(br *rarBitReader) ([]byte, error) {
	n, err := br.readUint32()
	if err != nil {
		return nil, err
	}
	if n > maxCodeSize || n == 0 {
		return nil, errInvalidFilter
	}
	buf := make([]byte, n)
	err = br.readFull(buf)
	if err != nil {
		return nil, err
	}
	var x byte
	for _, c := range buf[1:] {
		x ^= c
	}
	// simple xor checksum on data
	if x != buf[0] {
		return nil, errInvalidFilter
	}
	return buf, nil
}

func (d *decoder29) parseVMFilter(buf []byte) (*filterBlock, error) {
	flags := buf[0]
	br := newRarBitReader(bytes.NewReader(buf[1:]))
	fb := new(filterBlock)

	// Find the filter number which is an index into d.filters.
	// If filter number == len(d.filters) it is a new filter to be added.
	if flags&0x80 > 0 {
		n, err := br.readUint32()
		if err != nil {
			return nil, err
		}
		if n == 0 {
			d.initFilters()
			fb.reset = true
		} else {
			n--
			if n > maxUniqueFilters {
				return nil, errInvalidFilter
			}
			if int(n) > len(d.filters) {
				return nil, errInvalidFilter
			}
		}
		d.fnum = int(n)
	}

	// filter offset
	n, err := br.readUint32()
	if err != nil {
		return nil, err
	}
	if flags&0x40 > 0 {
		n += 258
	}
	fb.offset = int(n)

	// filter length
	if d.fnum == len(d.flen) {
		d.flen = append(d.flen, 0)
	}
	if flags&0x20 > 0 {
		n, err = br.readUint32()
		if err != nil {
			return nil, err
		}
		//fb.length = int(n)
		d.flen[d.fnum] = int(n)
	}
	fb.length = d.flen[d.fnum]

	// initial register values
	r := make(map[int]uint32)
	if flags&0x10 > 0 {
		bits, err := br.readBits(vmRegs - 1)
		if err != nil {
			return nil, err
		}
		for i := 0; i < vmRegs-1; i++ {
			if bits&1 > 0 {
				r[i], err = br.readUint32()
				if err != nil {
					return nil, err
				}
			}
			bits >>= 1
		}
	}

	// filter is new so read the code for it
	if d.fnum == len(d.filters) {
		code, err := readVMCode(br)
		if err != nil {
			return nil, err
		}
		f, err := getV3Filter(code)
		if err != nil {
			return nil, err
		}
		d.filters = append(d.filters, f)
		d.flen = append(d.flen, fb.length)
	}

	// read global data
	var g []byte
	if flags&0x08 > 0 {
		n, err := br.readUint32()
		if err != nil {
			return nil, err
		}
		if n > vmGlobalSize-vmFixedGlobalSize {
			return nil, errInvalidFilter
		}
		g = make([]byte, n)
		err = br.readFull(g)
		if err != nil {
			return nil, err
		}
	}

	// create filter function
	f := d.filters[d.fnum]
	fb.filter = func(buf []byte, offset int64) ([]byte, error) {
		return f(r, g, buf, offset)
	}

	return fb, nil
}

// readBlockHeader determines and initializes the current decoder for a new decode block.
func (d *decoder29) readBlockHeader() error {
	d.br.alignByte()
	n, err := d.br.readBits(1)
	if err == nil {
		if n > 0 {
			d.decode = d.ppm.decode
			err = d.ppm.init(d.br)
		} else {
			d.decode = d.lz.decode
			err = d.lz.init(d.br)
		}
	}
	if err == io.EOF {
		err = errDecoderOutOfData
	}
	return err

}

func (d *decoder29) fill(w *window) ([]*filterBlock, error) {
	if d.eof {
		return nil, io.EOF
	}

	var fl []*filterBlock

	for w.available() > 0 {
		b, err := d.decode(w) // perform a single decode operation
		if len(b) > 0 && err == nil {
			// parse raw data for filter and add to list of filters
			var f *filterBlock
			f, err = d.parseVMFilter(b)
			if f != nil {
				// make offset relative to read index (from write index)
				f.offset += w.buffered()
				fl = append(fl, f)
			}
		}

		switch err {
		case nil:
			continue
		case endOfBlock:
			err = d.readBlockHeader()
			if err == nil {
				continue
			}
		case endOfFile:
			d.eof = true
			err = io.EOF
		case endOfBlockAndFile:
			d.eof = true
			d.decode = nil // clear decoder, it will be setup by next init()
			err = io.EOF
		case io.EOF:
			err = errDecoderOutOfData
		}
		return fl, err
	}
	// return filters
	return fl, nil
}
