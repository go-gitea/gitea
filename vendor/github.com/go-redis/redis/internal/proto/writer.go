package proto

import (
	"bufio"
	"encoding"
	"fmt"
	"io"
	"strconv"

	"github.com/go-redis/redis/internal/util"
)

type Writer struct {
	wr *bufio.Writer

	lenBuf []byte
	numBuf []byte
}

func NewWriter(wr io.Writer) *Writer {
	return &Writer{
		wr: bufio.NewWriter(wr),

		lenBuf: make([]byte, 64),
		numBuf: make([]byte, 64),
	}
}

func (w *Writer) WriteArgs(args []interface{}) error {
	err := w.wr.WriteByte(ArrayReply)
	if err != nil {
		return err
	}

	err = w.writeLen(len(args))
	if err != nil {
		return err
	}

	for _, arg := range args {
		err := w.writeArg(arg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) writeLen(n int) error {
	w.lenBuf = strconv.AppendUint(w.lenBuf[:0], uint64(n), 10)
	w.lenBuf = append(w.lenBuf, '\r', '\n')
	_, err := w.wr.Write(w.lenBuf)
	return err
}

func (w *Writer) writeArg(v interface{}) error {
	switch v := v.(type) {
	case nil:
		return w.string("")
	case string:
		return w.string(v)
	case []byte:
		return w.bytes(v)
	case int:
		return w.int(int64(v))
	case int8:
		return w.int(int64(v))
	case int16:
		return w.int(int64(v))
	case int32:
		return w.int(int64(v))
	case int64:
		return w.int(v)
	case uint:
		return w.uint(uint64(v))
	case uint8:
		return w.uint(uint64(v))
	case uint16:
		return w.uint(uint64(v))
	case uint32:
		return w.uint(uint64(v))
	case uint64:
		return w.uint(v)
	case float32:
		return w.float(float64(v))
	case float64:
		return w.float(v)
	case bool:
		if v {
			return w.int(1)
		} else {
			return w.int(0)
		}
	case encoding.BinaryMarshaler:
		b, err := v.MarshalBinary()
		if err != nil {
			return err
		}
		return w.bytes(b)
	default:
		return fmt.Errorf(
			"redis: can't marshal %T (implement encoding.BinaryMarshaler)", v)
	}
}

func (w *Writer) bytes(b []byte) error {
	err := w.wr.WriteByte(StringReply)
	if err != nil {
		return err
	}

	err = w.writeLen(len(b))
	if err != nil {
		return err
	}

	_, err = w.wr.Write(b)
	if err != nil {
		return err
	}

	return w.crlf()
}

func (w *Writer) string(s string) error {
	return w.bytes(util.StringToBytes(s))
}

func (w *Writer) uint(n uint64) error {
	w.numBuf = strconv.AppendUint(w.numBuf[:0], n, 10)
	return w.bytes(w.numBuf)
}

func (w *Writer) int(n int64) error {
	w.numBuf = strconv.AppendInt(w.numBuf[:0], n, 10)
	return w.bytes(w.numBuf)
}

func (w *Writer) float(f float64) error {
	w.numBuf = strconv.AppendFloat(w.numBuf[:0], f, 'f', -1, 64)
	return w.bytes(w.numBuf)
}

func (w *Writer) crlf() error {
	err := w.wr.WriteByte('\r')
	if err != nil {
		return err
	}
	return w.wr.WriteByte('\n')
}

func (w *Writer) Reset(wr io.Writer) {
	w.wr.Reset(wr)
}

func (w *Writer) Flush() error {
	return w.wr.Flush()
}
