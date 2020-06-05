package rardecode

import "io"

type bitReader interface {
	readBits(n uint) (int, error) // read n bits of data
	unreadBits(n uint)            // revert the reading of the last n bits read
}

type limitedBitReader struct {
	br  bitReader
	n   int
	err error // error to return if br returns EOF before all n bits have been read
}

// limitBitReader returns a bitReader that reads from br and stops with io.EOF after n bits.
// If br returns an io.EOF before reading n bits, err is returned.
func limitBitReader(br bitReader, n int, err error) bitReader {
	return &limitedBitReader{br, n, err}
}

func (l *limitedBitReader) readBits(n uint) (int, error) {
	if int(n) > l.n {
		return 0, io.EOF
	}
	v, err := l.br.readBits(n)
	if err == nil {
		l.n -= int(n)
	} else if err == io.EOF {
		err = l.err
	}
	return v, err
}

func (l *limitedBitReader) unreadBits(n uint) {
	l.n += int(n)
	l.br.unreadBits(n)
}

// rarBitReader wraps an io.ByteReader to perform various bit and byte
// reading utility functions used in RAR file processing.
type rarBitReader struct {
	r io.ByteReader
	v int
	n uint
}

func (r *rarBitReader) reset(br io.ByteReader) {
	r.r = br
	r.n = 0
	r.v = 0
}

func (r *rarBitReader) readBits(n uint) (int, error) {
	for n > r.n {
		c, err := r.r.ReadByte()
		if err != nil {
			return 0, err
		}
		r.v = r.v<<8 | int(c)
		r.n += 8
	}
	r.n -= n
	return (r.v >> r.n) & ((1 << n) - 1), nil
}

func (r *rarBitReader) unreadBits(n uint) {
	r.n += n
}

// alignByte aligns the current bit reading input to the next byte boundary.
func (r *rarBitReader) alignByte() {
	r.n -= r.n % 8
}

// readUint32 reads a RAR V3 encoded uint32
func (r *rarBitReader) readUint32() (uint32, error) {
	n, err := r.readBits(2)
	if err != nil {
		return 0, err
	}
	if n != 1 {
		n, err = r.readBits(4 << uint(n))
		return uint32(n), err
	}
	n, err = r.readBits(4)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		n, err = r.readBits(8)
		n |= -1 << 8
		return uint32(n), err
	}
	nlow, err := r.readBits(4)
	n = n<<4 | nlow
	return uint32(n), err
}

func (r *rarBitReader) ReadByte() (byte, error) {
	n, err := r.readBits(8)
	return byte(n), err
}

// readFull reads len(p) bytes into p. If fewer bytes are read an error is returned.
func (r *rarBitReader) readFull(p []byte) error {
	for i := range p {
		c, err := r.ReadByte()
		if err != nil {
			return err
		}
		p[i] = c
	}
	return nil
}

func newRarBitReader(r io.ByteReader) *rarBitReader {
	return &rarBitReader{r: r}
}
