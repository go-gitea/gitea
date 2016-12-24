package iohelper

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/juju/errors"
)

var (
	cachedItob [][]byte
)

func init() {
	cachedItob = make([][]byte, 1024)
	for i := 0; i < len(cachedItob); i++ {
		var b bytes.Buffer
		writeVLong(&b, int64(i))
		cachedItob[i] = b.Bytes()
	}
}

func itob(i int) ([]byte, error) {
	if i >= 0 && i < len(cachedItob) {
		return cachedItob[i], nil
	}

	var b bytes.Buffer
	err := binary.Write(&b, binary.BigEndian, i)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return b.Bytes(), nil
}

func decodeVIntSize(value byte) int32 {
	if int32(value) >= -112 {
		return int32(1)
	}

	if int32(value) < -120 {
		return -119 - int32(value)
	}

	return -111 - int32(value)
}

func isNegativeVInt(value byte) bool {
	return int32(value) < -120 || int32(value) >= -112 && int32(value) < 0
}

func readVLong(r io.Reader) (int64, error) {
	var firstByte byte
	err := binary.Read(r, binary.BigEndian, &firstByte)
	if err != nil {
		return 0, errors.Trace(err)
	}

	l := decodeVIntSize(firstByte)
	if l == 1 {
		return int64(firstByte), nil
	}

	var (
		i   int64
		idx int32
	)

	for idx = 0; idx < l-1; idx++ {
		var b byte
		err = binary.Read(r, binary.BigEndian, &b)
		if err != nil {
			return 0, errors.Trace(err)
		}

		i <<= 8
		i |= int64(b & 255)
	}

	if isNegativeVInt(firstByte) {
		return ^i, nil
	}

	return i, nil
}

func writeVLong(w io.Writer, i int64) error {
	var err error
	if i >= -112 && i <= 127 {
		err = binary.Write(w, binary.BigEndian, byte(i))
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		var l int32 = -112
		if i < 0 {
			i = ^i
			l = -120
		}
		var tmp int64
		for tmp = i; tmp != 0; l-- {
			tmp >>= 8
		}

		err = binary.Write(w, binary.BigEndian, byte(l))
		if err != nil {
			return errors.Trace(err)
		}

		if l < -120 {
			l = -(l + 120)
		} else {
			l = -(l + 112)
		}

		for idx := l; idx != 0; idx-- {
			var mask int64
			shiftbits := uint((idx - 1) * 8)
			mask = int64(255) << shiftbits
			err = binary.Write(w, binary.BigEndian, byte((i&mask)>>shiftbits))
			if err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}

func ReadVarBytes(r ByteMultiReader) ([]byte, error) {
	sz, err := readVLong(r)
	if err != nil {
		return nil, errors.Trace(err)
	}

	b := make([]byte, sz)
	_, err = r.Read(b)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return b, nil
}

func WriteVarBytes(w io.Writer, b []byte) error {
	lenb, err := itob(len(b))
	if err != nil {
		return errors.Trace(err)
	}

	_, err = w.Write(lenb)
	if err != nil {
		return errors.Trace(err)
	}

	_, err = w.Write(b)
	return errors.Trace(err)
}

func ReadInt32(r io.Reader) (int32, error) {
	var n int32
	err := binary.Read(r, binary.BigEndian, &n)
	return n, errors.Trace(err)
}

func ReadN(r io.Reader, n int32) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, errors.Trace(err)
}

func ReadUint64(r io.Reader) (uint64, error) {
	var n uint64
	err := binary.Read(r, binary.BigEndian, &n)
	return n, errors.Trace(err)
}
