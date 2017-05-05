package packfile

import (
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/utils/binary"
)

// Encoder gets the data from the storage and write it into the writer in PACK
// format
type Encoder struct {
	selector     *deltaSelector
	w            *offsetWriter
	zw           *zlib.Writer
	hasher       plumbing.Hasher
	offsets      map[plumbing.Hash]int64
	useRefDeltas bool
}

// NewEncoder creates a new packfile encoder using a specific Writer and
// ObjectStorer
func NewEncoder(w io.Writer, s storer.EncodedObjectStorer, useRefDeltas bool) *Encoder {
	h := plumbing.Hasher{
		Hash: sha1.New(),
	}
	mw := io.MultiWriter(w, h)
	ow := newOffsetWriter(mw)
	zw := zlib.NewWriter(mw)
	return &Encoder{
		selector:     newDeltaSelector(s),
		w:            ow,
		zw:           zw,
		hasher:       h,
		offsets:      make(map[plumbing.Hash]int64),
		useRefDeltas: useRefDeltas,
	}
}

// Encode creates a packfile containing all the objects referenced in hashes
// and writes it to the writer in the Encoder.
func (e *Encoder) Encode(hashes []plumbing.Hash) (plumbing.Hash, error) {
	objects, err := e.selector.ObjectsToPack(hashes)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return e.encode(objects)
}

func (e *Encoder) encode(objects []*ObjectToPack) (plumbing.Hash, error) {
	if err := e.head(len(objects)); err != nil {
		return plumbing.ZeroHash, err
	}

	for _, o := range objects {
		if err := e.entry(o); err != nil {
			return plumbing.ZeroHash, err
		}
	}

	return e.footer()
}

func (e *Encoder) head(numEntries int) error {
	return binary.Write(
		e.w,
		signature,
		int32(VersionSupported),
		int32(numEntries),
	)
}

func (e *Encoder) entry(o *ObjectToPack) error {
	offset := e.w.Offset()

	if o.IsDelta() {
		if err := e.writeDeltaHeader(o, offset); err != nil {
			return err
		}
	} else {
		if err := e.entryHead(o.Object.Type(), o.Object.Size()); err != nil {
			return err
		}
	}

	// Save the position using the original hash, maybe a delta will need it
	e.offsets[o.Original.Hash()] = offset

	e.zw.Reset(e.w)
	or, err := o.Object.Reader()
	if err != nil {
		return err
	}
	_, err = io.Copy(e.zw, or)
	if err != nil {
		return err
	}

	return e.zw.Close()
}

func (e *Encoder) writeDeltaHeader(o *ObjectToPack, offset int64) error {
	// Write offset deltas by default
	t := plumbing.OFSDeltaObject
	if e.useRefDeltas {
		t = plumbing.REFDeltaObject
	}

	if err := e.entryHead(t, o.Object.Size()); err != nil {
		return err
	}

	if e.useRefDeltas {
		return e.writeRefDeltaHeader(o.Base.Original.Hash())
	} else {
		return e.writeOfsDeltaHeader(offset, o.Base.Original.Hash())
	}
}

func (e *Encoder) writeRefDeltaHeader(base plumbing.Hash) error {
	return binary.Write(e.w, base)
}

func (e *Encoder) writeOfsDeltaHeader(deltaOffset int64, base plumbing.Hash) error {
	// because it is an offset delta, we need the base
	// object position
	offset, ok := e.offsets[base]
	if !ok {
		return fmt.Errorf("delta base not found. Hash: %v", base)
	}

	return binary.WriteVariableWidthInt(e.w, deltaOffset-offset)
}

func (e *Encoder) entryHead(typeNum plumbing.ObjectType, size int64) error {
	t := int64(typeNum)
	header := []byte{}
	c := (t << firstLengthBits) | (size & maskFirstLength)
	size >>= firstLengthBits
	for {
		if size == 0 {
			break
		}
		header = append(header, byte(c|maskContinue))
		c = size & int64(maskLength)
		size >>= lengthBits
	}

	header = append(header, byte(c))
	_, err := e.w.Write(header)

	return err
}

func (e *Encoder) footer() (plumbing.Hash, error) {
	h := e.hasher.Sum()
	return h, binary.Write(e.w, h)
}

type offsetWriter struct {
	w      io.Writer
	offset int64
}

func newOffsetWriter(w io.Writer) *offsetWriter {
	return &offsetWriter{w: w}
}

func (ow *offsetWriter) Write(p []byte) (n int, err error) {
	n, err = ow.w.Write(p)
	ow.offset += int64(n)
	return n, err
}

func (ow *offsetWriter) Offset() int64 {
	return ow.offset
}
