package idxfile

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/utils/binary"
)

var (
	// ErrUnsupportedVersion is returned by Decode when the idx file version
	// is not supported.
	ErrUnsupportedVersion = errors.New("Unsuported version")
	// ErrMalformedIdxFile is returned by Decode when the idx file is corrupted.
	ErrMalformedIdxFile = errors.New("Malformed IDX file")
)

// A Decoder reads and decodes idx files from an input stream.
type Decoder struct {
	io.Reader
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r}
}

// Decode reads the whole idx object from its input and stores it in the
// value pointed to by idx.
func (d *Decoder) Decode(idx *Idxfile) error {
	if err := validateHeader(d); err != nil {
		return err
	}

	flow := []func(*Idxfile, io.Reader) error{
		readVersion,
		readFanout,
		readObjectNames,
		readCRC32,
		readOffsets,
		readChecksums,
	}

	for _, f := range flow {
		if err := f(idx, d); err != nil {
			return err
		}
	}

	if !idx.isValid() {
		return ErrMalformedIdxFile
	}

	return nil
}

func validateHeader(r io.Reader) error {
	var h = make([]byte, 4)
	if _, err := r.Read(h); err != nil {
		return err
	}

	if !bytes.Equal(h, idxHeader) {
		return ErrMalformedIdxFile
	}

	return nil
}

func readVersion(idx *Idxfile, r io.Reader) error {
	v, err := binary.ReadUint32(r)
	if err != nil {
		return err
	}

	if v > VersionSupported {
		return ErrUnsupportedVersion
	}

	idx.Version = v
	return nil
}

func readFanout(idx *Idxfile, r io.Reader) error {
	var err error
	for i := 0; i < 255; i++ {
		idx.Fanout[i], err = binary.ReadUint32(r)
		if err != nil {
			return err
		}
	}

	idx.ObjectCount, err = binary.ReadUint32(r)
	return err
}

func readObjectNames(idx *Idxfile, r io.Reader) error {
	c := int(idx.ObjectCount)
	for i := 0; i < c; i++ {
		var ref plumbing.Hash
		if _, err := r.Read(ref[:]); err != nil {
			return err
		}

		idx.Entries = append(idx.Entries, Entry{Hash: ref})
	}

	return nil
}

func readCRC32(idx *Idxfile, r io.Reader) error {
	c := int(idx.ObjectCount)
	for i := 0; i < c; i++ {
		if err := binary.Read(r, &idx.Entries[i].CRC32); err != nil {
			return err
		}
	}

	return nil
}

func readOffsets(idx *Idxfile, r io.Reader) error {
	c := int(idx.ObjectCount)
	for i := 0; i < c; i++ {
		o, err := binary.ReadUint32(r)
		if err != nil {
			return err
		}

		idx.Entries[i].Offset = uint64(o)
	}

	return nil
}

func readChecksums(idx *Idxfile, r io.Reader) error {
	if _, err := r.Read(idx.PackfileChecksum[:]); err != nil {
		return err
	}

	if _, err := r.Read(idx.IdxChecksum[:]); err != nil {
		return err
	}

	return nil
}
