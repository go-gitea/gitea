package govarint

import "encoding/binary"
import "io"

type U32VarintEncoder interface {
	PutU32(x uint32) int
	Close()
}

type U32VarintDecoder interface {
	GetU32() (uint32, error)
}

///

type U64VarintEncoder interface {
	PutU64(x uint64) int
	Close()
}

type U64VarintDecoder interface {
	GetU64() (uint64, error)
}

///

type U32GroupVarintEncoder struct {
	w     io.Writer
	index int
	store [4]uint32
	temp  [17]byte
}

func NewU32GroupVarintEncoder(w io.Writer) *U32GroupVarintEncoder { return &U32GroupVarintEncoder{w: w} }

func (b *U32GroupVarintEncoder) Flush() (int, error) {
	// TODO: Is it more efficient to have a tailored version that's called only in Close()?
	// If index is zero, there are no integers to flush
	if b.index == 0 {
		return 0, nil
	}
	// In the case we're flushing (the group isn't of size four), the non-values should be zero
	// This ensures the unused entries are all zero in the sizeByte
	for i := b.index; i < 4; i++ {
		b.store[i] = 0
	}
	length := 1
	// We need to reset the size byte to zero as we only bitwise OR into it, we don't overwrite it
	b.temp[0] = 0
	for i, x := range b.store {
		size := byte(0)
		shifts := []byte{24, 16, 8, 0}
		for _, shift := range shifts {
			// Always writes at least one byte -- the first one (shift = 0)
			// Will write more bytes until the rest of the integer is all zeroes
			if (x>>shift) != 0 || shift == 0 {
				size += 1
				b.temp[length] = byte(x >> shift)
				length += 1
			}
		}
		// We store the size in two of the eight bits in the first byte (sizeByte)
		// 0 means there is one byte in total, hence why we subtract one from size
		b.temp[0] |= (size - 1) << (uint8(3-i) * 2)
	}
	// If we're flushing without a full group of four, remove the unused bytes we computed
	// This enables us to realize it's a partial group on decoding thanks to EOF
	if b.index != 4 {
		length -= 4 - b.index
	}
	_, err := b.w.Write(b.temp[:length])
	return length, err
}

func (b *U32GroupVarintEncoder) PutU32(x uint32) (int, error) {
	bytesWritten := 0
	b.store[b.index] = x
	b.index += 1
	if b.index == 4 {
		n, err := b.Flush()
		if err != nil {
			return n, err
		}
		bytesWritten += n
		b.index = 0
	}
	return bytesWritten, nil
}

func (b *U32GroupVarintEncoder) Close() {
	// On Close, we flush any remaining values that might not have been in a full group
	b.Flush()
}

///

type U32GroupVarintDecoder struct {
	r        io.ByteReader
	group    [4]uint32
	pos      int
	finished bool
	capacity int
}

func NewU32GroupVarintDecoder(r io.ByteReader) *U32GroupVarintDecoder {
	return &U32GroupVarintDecoder{r: r, pos: 4, capacity: 4}
}

func (b *U32GroupVarintDecoder) getGroup() error {
	// We should always receive a sizeByte if there are more values to read
	sizeByte, err := b.r.ReadByte()
	if err != nil {
		return err
	}
	// Calculate the size of the four incoming 32 bit integers
	// 0b00 means 1 byte to read, 0b01 = 2, etc
	b.group[0] = uint32((sizeByte >> 6) & 3)
	b.group[1] = uint32((sizeByte >> 4) & 3)
	b.group[2] = uint32((sizeByte >> 2) & 3)
	b.group[3] = uint32(sizeByte & 3)
	//
	for index, size := range b.group {
		b.group[index] = 0
		// Any error that occurs in earlier byte reads should be repeated at the end one
		// Hence we only catch and report the final ReadByte's error
		var err error
		switch size {
		case 0:
			var x byte
			x, err = b.r.ReadByte()
			b.group[index] = uint32(x)
		case 1:
			var x, y byte
			x, _ = b.r.ReadByte()
			y, err = b.r.ReadByte()
			b.group[index] = uint32(x)<<8 | uint32(y)
		case 2:
			var x, y, z byte
			x, _ = b.r.ReadByte()
			y, _ = b.r.ReadByte()
			z, err = b.r.ReadByte()
			b.group[index] = uint32(x)<<16 | uint32(y)<<8 | uint32(z)
		case 3:
			var x, y, z, zz byte
			x, _ = b.r.ReadByte()
			y, _ = b.r.ReadByte()
			z, _ = b.r.ReadByte()
			zz, err = b.r.ReadByte()
			b.group[index] = uint32(x)<<24 | uint32(y)<<16 | uint32(z)<<8 | uint32(zz)
		}
		if err != nil {
			if err == io.EOF {
				// If we hit EOF here, we have found a partial group
				// We've return any valid entries we have read and return EOF once we run out
				b.capacity = index
				b.finished = true
				break
			} else {
				return err
			}
		}
	}
	// Reset the pos pointer to the beginning of the read values
	b.pos = 0
	return nil
}

func (b *U32GroupVarintDecoder) GetU32() (uint32, error) {
	// Check if we have any more values to give out - if not, let's get them
	if b.pos == b.capacity {
		// If finished is set, there is nothing else to do
		if b.finished {
			return 0, io.EOF
		}
		err := b.getGroup()
		if err != nil {
			return 0, err
		}
	}
	// Increment pointer and return the value stored at that point
	b.pos += 1
	return b.group[b.pos-1], nil
}

///

type Base128Encoder struct {
	w        io.Writer
	tmpBytes []byte
}

func NewU32Base128Encoder(w io.Writer) *Base128Encoder {
	return &Base128Encoder{w: w, tmpBytes: make([]byte, binary.MaxVarintLen32)}
}
func NewU64Base128Encoder(w io.Writer) *Base128Encoder {
	return &Base128Encoder{w: w, tmpBytes: make([]byte, binary.MaxVarintLen64)}
}

func (b *Base128Encoder) PutU32(x uint32) (int, error) {
	writtenBytes := binary.PutUvarint(b.tmpBytes, uint64(x))
	return b.w.Write(b.tmpBytes[:writtenBytes])
}

func (b *Base128Encoder) PutU64(x uint64) (int, error) {
	writtenBytes := binary.PutUvarint(b.tmpBytes, x)
	return b.w.Write(b.tmpBytes[:writtenBytes])
}

func (b *Base128Encoder) Close() {
}

///

type Base128Decoder struct {
	r io.ByteReader
}

func NewU32Base128Decoder(r io.ByteReader) *Base128Decoder { return &Base128Decoder{r: r} }
func NewU64Base128Decoder(r io.ByteReader) *Base128Decoder { return &Base128Decoder{r: r} }

func (b *Base128Decoder) GetU32() (uint32, error) {
	v, err := binary.ReadUvarint(b.r)
	return uint32(v), err
}

func (b *Base128Decoder) GetU64() (uint64, error) {
	return binary.ReadUvarint(b.r)
}
