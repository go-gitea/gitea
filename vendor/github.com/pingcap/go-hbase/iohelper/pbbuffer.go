package iohelper

import (
	"encoding/binary"

	pb "github.com/golang/protobuf/proto"
	"github.com/juju/errors"
)

type PbBuffer struct {
	b []byte
}

func NewPbBuffer() *PbBuffer {
	b := []byte{}
	return &PbBuffer{
		b: b,
	}
}

func (b *PbBuffer) Bytes() []byte {
	return b.b
}

func (b *PbBuffer) Write(d []byte) (int, error) {
	b.b = append(b.b, d...)
	return len(d), nil
}

func (b *PbBuffer) WriteByte(d byte) error {
	return binary.Write(b, binary.BigEndian, d)
}

func (b *PbBuffer) WriteString(d string) error {
	return binary.Write(b, binary.BigEndian, d)
}

func (b *PbBuffer) WriteInt32(d int32) error {
	return binary.Write(b, binary.BigEndian, d)
}

func (b *PbBuffer) WriteInt64(d int64) error {
	return binary.Write(b, binary.BigEndian, d)
}

func (b *PbBuffer) WriteFloat32(d float32) error {
	return binary.Write(b, binary.BigEndian, d)
}

func (b *PbBuffer) WriteFloat64(d float64) error {
	return binary.Write(b, binary.BigEndian, d)
}

func (b *PbBuffer) WritePBMessage(d pb.Message) error {
	buf, err := pb.Marshal(d)
	if err != nil {
		return errors.Trace(err)
	}

	_, err = b.Write(buf)
	return errors.Trace(err)
}

func (b *PbBuffer) WriteDelimitedBuffers(bufs ...*PbBuffer) error {
	totalLength := 0
	lens := make([][]byte, len(bufs))
	for i, v := range bufs {
		n := len(v.Bytes())
		lenb := pb.EncodeVarint(uint64(n))

		totalLength += len(lenb) + n
		lens[i] = lenb
	}

	err := b.WriteInt32(int32(totalLength))
	if err != nil {
		return errors.Trace(err)
	}

	for i, v := range bufs {
		_, err = b.Write(lens[i])
		if err != nil {
			return errors.Trace(err)
		}

		_, err = b.Write(v.Bytes())
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func (b *PbBuffer) PrependSize() error {
	size := int32(len(b.b))
	newBuf := NewPbBuffer()

	err := newBuf.WriteInt32(size)
	if err != nil {
		return errors.Trace(err)
	}

	_, err = newBuf.Write(b.b)
	if err != nil {
		return errors.Trace(err)
	}

	*b = *newBuf
	return nil
}
