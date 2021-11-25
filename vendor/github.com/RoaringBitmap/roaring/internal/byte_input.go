package internal

import (
	"encoding/binary"
	"io"
)

// ByteInput typed interface around io.Reader or raw bytes
type ByteInput interface {
	// Next returns a slice containing the next n bytes from the buffer,
	// advancing the buffer as if the bytes had been returned by Read.
	Next(n int) ([]byte, error)
	// ReadUInt32 reads uint32 with LittleEndian order
	ReadUInt32() (uint32, error)
	// ReadUInt16 reads uint16 with LittleEndian order
	ReadUInt16() (uint16, error)
	// GetReadBytes returns read bytes
	GetReadBytes() int64
	// SkipBytes skips exactly n bytes
	SkipBytes(n int) error
}

// NewByteInputFromReader creates reader wrapper
func NewByteInputFromReader(reader io.Reader) ByteInput {
	return &ByteInputAdapter{
		r:         reader,
		readBytes: 0,
	}
}

// NewByteInput creates raw bytes wrapper
func NewByteInput(buf []byte) ByteInput {
	return &ByteBuffer{
		buf: buf,
		off: 0,
	}
}

// ByteBuffer raw bytes wrapper
type ByteBuffer struct {
	buf []byte
	off int
}

// Next returns a slice containing the next n bytes from the reader
// If there are fewer bytes than the given n, io.ErrUnexpectedEOF will be returned
func (b *ByteBuffer) Next(n int) ([]byte, error) {
	m := len(b.buf) - b.off

	if n > m {
		return nil, io.ErrUnexpectedEOF
	}

	data := b.buf[b.off : b.off+n]
	b.off += n

	return data, nil
}

// ReadUInt32 reads uint32 with LittleEndian order
func (b *ByteBuffer) ReadUInt32() (uint32, error) {
	if len(b.buf)-b.off < 4 {
		return 0, io.ErrUnexpectedEOF
	}

	v := binary.LittleEndian.Uint32(b.buf[b.off:])
	b.off += 4

	return v, nil
}

// ReadUInt16 reads uint16 with LittleEndian order
func (b *ByteBuffer) ReadUInt16() (uint16, error) {
	if len(b.buf)-b.off < 2 {
		return 0, io.ErrUnexpectedEOF
	}

	v := binary.LittleEndian.Uint16(b.buf[b.off:])
	b.off += 2

	return v, nil
}

// GetReadBytes returns read bytes
func (b *ByteBuffer) GetReadBytes() int64 {
	return int64(b.off)
}

// SkipBytes skips exactly n bytes
func (b *ByteBuffer) SkipBytes(n int) error {
	m := len(b.buf) - b.off

	if n > m {
		return io.ErrUnexpectedEOF
	}

	b.off += n

	return nil
}

// Reset resets the given buffer with a new byte slice
func (b *ByteBuffer) Reset(buf []byte) {
	b.buf = buf
	b.off = 0
}

// ByteInputAdapter reader wrapper
type ByteInputAdapter struct {
	r         io.Reader
	readBytes int
}

// Next returns a slice containing the next n bytes from the buffer,
// advancing the buffer as if the bytes had been returned by Read.
func (b *ByteInputAdapter) Next(n int) ([]byte, error) {
	buf := make([]byte, n)
	m, err := io.ReadAtLeast(b.r, buf, n)
	b.readBytes += m

	if err != nil {
		return nil, err
	}

	return buf, nil
}

// ReadUInt32 reads uint32 with LittleEndian order
func (b *ByteInputAdapter) ReadUInt32() (uint32, error) {
	buf, err := b.Next(4)

	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint32(buf), nil
}

// ReadUInt16 reads uint16 with LittleEndian order
func (b *ByteInputAdapter) ReadUInt16() (uint16, error) {
	buf, err := b.Next(2)

	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint16(buf), nil
}

// GetReadBytes returns read bytes
func (b *ByteInputAdapter) GetReadBytes() int64 {
	return int64(b.readBytes)
}

// SkipBytes skips exactly n bytes
func (b *ByteInputAdapter) SkipBytes(n int) error {
	_, err := b.Next(n)

	return err
}

// Reset resets the given buffer with a new stream
func (b *ByteInputAdapter) Reset(stream io.Reader) {
	b.r = stream
	b.readBytes = 0
}
