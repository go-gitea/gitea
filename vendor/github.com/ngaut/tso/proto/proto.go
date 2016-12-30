package proto

import (
	"encoding/binary"
	"io"

	"github.com/juju/errors"
)

// RequestHeader is for tso request proto.
type RequestHeader struct {
}

// Timestamp is for tso timestamp.
type Timestamp struct {
	Physical int64
	Logical  int64
}

// Response is for tso reponse proto.
type Response struct {
	Timestamp
}

// Encode encodes repsonse proto into w.
func (res *Response) Encode(w io.Writer) error {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], uint64(res.Physical))
	binary.BigEndian.PutUint64(buf[8:16], uint64(res.Logical))
	_, err := w.Write(buf[0:16])
	return errors.Trace(err)
}

// Decode decodes reponse proto from r.
func (res *Response) Decode(r io.Reader) error {
	var buf [16]byte
	_, err := io.ReadFull(r, buf[0:16])
	if err != nil {
		return errors.Trace(err)
	}

	res.Physical = int64(binary.BigEndian.Uint64(buf[0:8]))
	res.Logical = int64(binary.BigEndian.Uint64(buf[8:16]))
	return nil
}
