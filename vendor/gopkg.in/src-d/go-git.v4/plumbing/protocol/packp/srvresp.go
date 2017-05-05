package packp

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
)

const ackLineLen = 44

// ServerResponse object acknowledgement from upload-pack service
// TODO: implement support for multi_ack or multi_ack_detailed responses
type ServerResponse struct {
	ACKs []plumbing.Hash
}

// Decode decodes the response into the struct, isMultiACK should be true, if
// the request was done with multi_ack or multi_ack_detailed capabilities
func (r *ServerResponse) Decode(reader io.Reader, isMultiACK bool) error {
	if isMultiACK {
		return errors.New("multi_ack and multi_ack_detailed are not supported")
	}

	s := pktline.NewScanner(reader)

	for s.Scan() {
		line := s.Bytes()

		if err := r.decodeLine(line); err != nil {
			return err
		}

		if !isMultiACK {
			break
		}
	}

	return s.Err()
}

func (r *ServerResponse) decodeLine(line []byte) error {
	if len(line) == 0 {
		return fmt.Errorf("unexpected flush")
	}

	if bytes.Compare(line[0:3], ack) == 0 {
		return r.decodeACKLine(line)
	}

	if bytes.Compare(line[0:3], nak) == 0 {
		return nil
	}

	return fmt.Errorf("unexpected content %q", string(line))
}

func (r *ServerResponse) decodeACKLine(line []byte) error {
	if len(line) < ackLineLen {
		return fmt.Errorf("malformed ACK %q", line)
	}

	sp := bytes.Index(line, []byte(" "))
	h := plumbing.NewHash(string(line[sp+1 : sp+41]))
	r.ACKs = append(r.ACKs, h)
	return nil
}
