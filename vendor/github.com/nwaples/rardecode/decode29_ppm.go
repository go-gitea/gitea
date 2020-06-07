package rardecode

import "io"

type ppm29Decoder struct {
	m   model // ppm model
	esc byte  // escape character
	br  io.ByteReader
}

func (d *ppm29Decoder) init(br *rarBitReader) error {
	maxOrder, err := br.readBits(7)
	if err != nil {
		return err
	}
	reset := maxOrder&0x20 > 0

	// Should have flushed all unread bits from bitReader by now,
	// use underlying ByteReader
	d.br = br.r

	var maxMB int
	if reset {
		c, err := d.br.ReadByte()
		if err != nil {
			return err
		}
		maxMB = int(c) + 1
	}

	if maxOrder&0x40 > 0 {
		d.esc, err = d.br.ReadByte()
		if err != nil {
			return err
		}
	}

	maxOrder = (maxOrder & 0x1f) + 1
	if maxOrder > 16 {
		maxOrder = 16 + (maxOrder-16)*3
	}

	return d.m.init(d.br, reset, maxOrder, maxMB)
}

func (d *ppm29Decoder) reset() {
	d.esc = 2
}

func (d *ppm29Decoder) readFilterData() ([]byte, error) {
	c, err := d.m.ReadByte()
	if err != nil {
		return nil, err
	}
	n := int(c&7) + 1
	if n == 7 {
		b, err := d.m.ReadByte()
		if err != nil {
			return nil, err
		}
		n += int(b)
	} else if n == 8 {
		b, err := d.m.ReadByte()
		if err != nil {
			return nil, err
		}
		n = int(b) << 8
		b, err = d.m.ReadByte()
		if err != nil {
			return nil, err
		}
		n |= int(b)
	}

	n++
	buf := make([]byte, n)
	buf[0] = byte(c)
	for i := 1; i < n; i++ {
		buf[i], err = d.m.ReadByte()
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func (d *ppm29Decoder) decode(w *window) ([]byte, error) {
	c, err := d.m.ReadByte()
	if err != nil {
		return nil, err
	}
	if c != d.esc {
		w.writeByte(c)
		return nil, nil
	}
	c, err = d.m.ReadByte()
	if err != nil {
		return nil, err
	}

	switch c {
	case 0:
		return nil, endOfBlock
	case 2:
		return nil, endOfBlockAndFile
	case 3:
		return d.readFilterData()
	case 4:
		offset := 0
		for i := 0; i < 3; i++ {
			c, err = d.m.ReadByte()
			if err != nil {
				return nil, err
			}
			offset = offset<<8 | int(c)
		}
		len, err := d.m.ReadByte()
		if err != nil {
			return nil, err
		}
		w.copyBytes(int(len)+32, offset+2)
	case 5:
		len, err := d.m.ReadByte()
		if err != nil {
			return nil, err
		}
		w.copyBytes(int(len)+4, 1)
	default:
		w.writeByte(d.esc)
	}
	return nil, nil
}
