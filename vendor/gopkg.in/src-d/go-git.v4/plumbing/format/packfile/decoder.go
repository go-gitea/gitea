package packfile

import (
	"bytes"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

// Format specifies if the packfile uses ref-deltas or ofs-deltas.
type Format int

// Possible values of the Format type.
const (
	UnknownFormat Format = iota
	OFSDeltaFormat
	REFDeltaFormat
)

var (
	// ErrMaxObjectsLimitReached is returned by Decode when the number
	// of objects in the packfile is higher than
	// Decoder.MaxObjectsLimit.
	ErrMaxObjectsLimitReached = NewError("max. objects limit reached")
	// ErrInvalidObject is returned by Decode when an invalid object is
	// found in the packfile.
	ErrInvalidObject = NewError("invalid git object")
	// ErrPackEntryNotFound is returned by Decode when a reference in
	// the packfile references and unknown object.
	ErrPackEntryNotFound = NewError("can't find a pack entry")
	// ErrZLib is returned by Decode when there was an error unzipping
	// the packfile contents.
	ErrZLib = NewError("zlib reading error")
	// ErrCannotRecall is returned by RecallByOffset or RecallByHash if the object
	// to recall cannot be returned.
	ErrCannotRecall = NewError("cannot recall object")
	// ErrResolveDeltasNotSupported is returned if a NewDecoder is used with a
	// non-seekable scanner and without a plumbing.ObjectStorage
	ErrResolveDeltasNotSupported = NewError("resolve delta is not supported")
	// ErrNonSeekable is returned if a ReadObjectAt method is called without a
	// seekable scanner
	ErrNonSeekable = NewError("non-seekable scanner")
	// ErrRollback error making Rollback over a transaction after an error
	ErrRollback = NewError("rollback error, during set error")
	// ErrAlreadyDecoded is returned if NewDecoder is called for a second time
	ErrAlreadyDecoded = NewError("packfile was already decoded")
)

// Decoder reads and decodes packfiles from an input Scanner, if an ObjectStorer
// was provided the decoded objects are store there. If not the decode object
// is destroyed. The Offsets and CRCs are calculated independand if the an
// ObjectStorer was provided or not.
type Decoder struct {
	s  *Scanner
	o  storer.EncodedObjectStorer
	tx storer.Transaction

	isDecoded    bool
	offsetToHash map[int64]plumbing.Hash
	hashToOffset map[plumbing.Hash]int64
	crcs         map[plumbing.Hash]uint32
}

// NewDecoder returns a new Decoder that decodes a Packfile using the given
// s and store the objects in the provided o. ObjectStorer can be nil, in this
// case the objects are not stored but objects offsets on the Packfile and the
// CRCs are calculated.
//
// If ObjectStorer is nil and the Scanner is not Seekable, ErrNonSeekable is
// returned.
//
// If the ObjectStorer implements storer.Transactioner, a transaction is created
// during the Decode execution, if something fails the Rollback is called
func NewDecoder(s *Scanner, o storer.EncodedObjectStorer) (*Decoder, error) {
	if !canResolveDeltas(s, o) {
		return nil, ErrResolveDeltasNotSupported
	}

	return &Decoder{
		s: s,
		o: o,

		offsetToHash: make(map[int64]plumbing.Hash, 0),
		hashToOffset: make(map[plumbing.Hash]int64, 0),
		crcs:         make(map[plumbing.Hash]uint32, 0),
	}, nil
}

func canResolveDeltas(s *Scanner, o storer.EncodedObjectStorer) bool {
	return s.IsSeekable || o != nil
}

// Decode reads a packfile and stores it in the value pointed to by s. The
// offsets and the CRCs are calculated by this method
func (d *Decoder) Decode() (checksum plumbing.Hash, err error) {
	defer func() { d.isDecoded = true }()

	if d.isDecoded {
		return plumbing.ZeroHash, ErrAlreadyDecoded
	}

	if err := d.doDecode(); err != nil {
		return plumbing.ZeroHash, err
	}

	return d.s.Checksum()
}

func (d *Decoder) doDecode() error {
	_, count, err := d.s.Header()
	if err != nil {
		return err
	}

	_, isTxStorer := d.o.(storer.Transactioner)
	switch {
	case d.o == nil:
		return d.decodeObjects(int(count))
	case isTxStorer:
		return d.decodeObjectsWithObjectStorerTx(int(count))
	default:
		return d.decodeObjectsWithObjectStorer(int(count))
	}
}

func (d *Decoder) decodeObjects(count int) error {
	for i := 0; i < count; i++ {
		if _, err := d.DecodeObject(); err != nil {
			return err
		}
	}

	return nil
}

func (d *Decoder) decodeObjectsWithObjectStorer(count int) error {
	for i := 0; i < count; i++ {
		obj, err := d.DecodeObject()
		if err != nil {
			return err
		}

		if _, err := d.o.SetEncodedObject(obj); err != nil {
			return err
		}
	}

	return nil
}

func (d *Decoder) decodeObjectsWithObjectStorerTx(count int) error {
	d.tx = d.o.(storer.Transactioner).Begin()

	for i := 0; i < count; i++ {
		obj, err := d.DecodeObject()
		if err != nil {
			return err
		}

		if _, err := d.tx.SetEncodedObject(obj); err != nil {
			if rerr := d.tx.Rollback(); rerr != nil {
				return ErrRollback.AddDetails(
					"error: %s, during tx.Set error: %s", rerr, err,
				)
			}

			return err
		}

	}

	return d.tx.Commit()
}

// DecodeObject reads the next object from the scanner and returns it. This
// method can be used in replacement of the Decode method, to work in a
// interative way
func (d *Decoder) DecodeObject() (plumbing.EncodedObject, error) {
	h, err := d.s.NextObjectHeader()
	if err != nil {
		return nil, err
	}

	obj := d.newObject()
	obj.SetSize(h.Length)
	obj.SetType(h.Type)
	var crc uint32
	switch h.Type {
	case plumbing.CommitObject, plumbing.TreeObject, plumbing.BlobObject, plumbing.TagObject:
		crc, err = d.fillRegularObjectContent(obj)
	case plumbing.REFDeltaObject:
		crc, err = d.fillREFDeltaObjectContent(obj, h.Reference)
	case plumbing.OFSDeltaObject:
		crc, err = d.fillOFSDeltaObjectContent(obj, h.OffsetReference)
	default:
		err = ErrInvalidObject.AddDetails("type %q", h.Type)
	}

	if err != nil {
		return obj, err
	}

	hash := obj.Hash()
	d.setOffset(hash, h.Offset)
	d.setCRC(hash, crc)

	return obj, nil
}

func (d *Decoder) newObject() plumbing.EncodedObject {
	if d.o == nil {
		return &plumbing.MemoryObject{}
	}

	return d.o.NewEncodedObject()
}

// DecodeObjectAt reads an object at the given location, if Decode wasn't called
// previously objects offset should provided using the SetOffsets method
func (d *Decoder) DecodeObjectAt(offset int64) (plumbing.EncodedObject, error) {
	if !d.s.IsSeekable {
		return nil, ErrNonSeekable
	}

	beforeJump, err := d.s.Seek(offset)
	if err != nil {
		return nil, err
	}

	defer func() {
		_, seekErr := d.s.Seek(beforeJump)
		if err == nil {
			err = seekErr
		}
	}()

	return d.DecodeObject()
}

func (d *Decoder) fillRegularObjectContent(obj plumbing.EncodedObject) (uint32, error) {
	w, err := obj.Writer()
	if err != nil {
		return 0, err
	}

	_, crc, err := d.s.NextObject(w)
	return crc, err
}

func (d *Decoder) fillREFDeltaObjectContent(obj plumbing.EncodedObject, ref plumbing.Hash) (uint32, error) {
	buf := bytes.NewBuffer(nil)
	_, crc, err := d.s.NextObject(buf)
	if err != nil {
		return 0, err
	}

	base, err := d.recallByHash(ref)
	if err != nil {
		return 0, err
	}

	obj.SetType(base.Type())
	return crc, ApplyDelta(obj, base, buf.Bytes())
}

func (d *Decoder) fillOFSDeltaObjectContent(obj plumbing.EncodedObject, offset int64) (uint32, error) {
	buf := bytes.NewBuffer(nil)
	_, crc, err := d.s.NextObject(buf)
	if err != nil {
		return 0, err
	}

	base, err := d.recallByOffset(offset)
	if err != nil {
		return 0, err
	}

	obj.SetType(base.Type())
	return crc, ApplyDelta(obj, base, buf.Bytes())
}

func (d *Decoder) setOffset(h plumbing.Hash, offset int64) {
	d.offsetToHash[offset] = h
	d.hashToOffset[h] = offset
}

func (d *Decoder) setCRC(h plumbing.Hash, crc uint32) {
	d.crcs[h] = crc
}

func (d *Decoder) recallByOffset(o int64) (plumbing.EncodedObject, error) {
	if d.s.IsSeekable {
		return d.DecodeObjectAt(o)
	}

	if h, ok := d.offsetToHash[o]; ok {
		return d.recallByHashNonSeekable(h)
	}

	return nil, plumbing.ErrObjectNotFound
}

func (d *Decoder) recallByHash(h plumbing.Hash) (plumbing.EncodedObject, error) {
	if d.s.IsSeekable {
		if o, ok := d.hashToOffset[h]; ok {
			return d.DecodeObjectAt(o)
		}
	}

	return d.recallByHashNonSeekable(h)
}

// recallByHashNonSeekable if we are in a transaction the objects are read from
// the transaction, if not are directly read from the ObjectStorer
func (d *Decoder) recallByHashNonSeekable(h plumbing.Hash) (obj plumbing.EncodedObject, err error) {
	if d.tx != nil {
		obj, err = d.tx.EncodedObject(plumbing.AnyObject, h)
	} else {
		obj, err = d.o.EncodedObject(plumbing.AnyObject, h)
	}

	if err != plumbing.ErrObjectNotFound {
		return obj, err
	}

	return nil, plumbing.ErrObjectNotFound
}

// SetOffsets sets the offsets, required when using the method DecodeObjectAt,
// without decoding the full packfile
func (d *Decoder) SetOffsets(offsets map[plumbing.Hash]int64) {
	d.hashToOffset = offsets
}

// Offsets returns the objects read offset, Decode method should be called
// before to calculate the Offsets
func (d *Decoder) Offsets() map[plumbing.Hash]int64 {
	return d.hashToOffset
}

// CRCs returns the CRC-32 for each objected read,Decode method should be called
// before to calculate the CRCs
func (d *Decoder) CRCs() map[plumbing.Hash]uint32 {
	return d.crcs
}

// Close close the Scanner, usually this mean that the whole reader is read and
// discarded
func (d *Decoder) Close() error {
	return d.s.Close()
}
