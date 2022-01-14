package cbor

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Tag represents CBOR tag data, including tag number and unmarshaled tag content.
type Tag struct {
	Number  uint64
	Content interface{}
}

func (t Tag) contentKind() reflect.Kind {
	c := t.Content
	for {
		t, ok := c.(Tag)
		if !ok {
			break
		}
		c = t.Content
	}
	return reflect.ValueOf(c).Kind()
}

// RawTag represents CBOR tag data, including tag number and raw tag content.
// RawTag implements Unmarshaler and Marshaler interfaces.
type RawTag struct {
	Number  uint64
	Content RawMessage
}

// UnmarshalCBOR sets *t with tag number and raw tag content copied from data.
func (t *RawTag) UnmarshalCBOR(data []byte) error {
	if t == nil {
		return errors.New("cbor.RawTag: UnmarshalCBOR on nil pointer")
	}

	d := decodeState{data: data, dm: defaultDecMode}

	// Unmarshal tag number.
	typ, _, num := d.getHead()
	if typ != cborTypeTag {
		return &UnmarshalTypeError{Value: typ.String(), Type: typeRawTag}
	}
	t.Number = num

	// Unmarshal tag content.
	c := d.data[d.off:]
	t.Content = make([]byte, len(c))
	copy(t.Content, c)
	return nil
}

// MarshalCBOR returns CBOR encoding of t.
func (t RawTag) MarshalCBOR() ([]byte, error) {
	e := getEncodeState()
	encodeHead(e, byte(cborTypeTag), t.Number)

	buf := make([]byte, len(e.Bytes())+len(t.Content))
	n := copy(buf, e.Bytes())
	copy(buf[n:], t.Content)

	putEncodeState(e)
	return buf, nil
}

// DecTagMode specifies how decoder handles tag number.
type DecTagMode int

const (
	// DecTagIgnored makes decoder ignore tag number (skips if present).
	DecTagIgnored DecTagMode = iota

	// DecTagOptional makes decoder verify tag number if it's present.
	DecTagOptional

	// DecTagRequired makes decoder verify tag number and tag number must be present.
	DecTagRequired

	maxDecTagMode
)

func (dtm DecTagMode) valid() bool {
	return dtm < maxDecTagMode
}

// EncTagMode specifies how encoder handles tag number.
type EncTagMode int

const (
	// EncTagNone makes encoder not encode tag number.
	EncTagNone EncTagMode = iota

	// EncTagRequired makes encoder encode tag number.
	EncTagRequired

	maxEncTagMode
)

func (etm EncTagMode) valid() bool {
	return etm < maxEncTagMode
}

// TagOptions specifies how encoder and decoder handle tag number.
type TagOptions struct {
	DecTag DecTagMode
	EncTag EncTagMode
}

// TagSet is an interface to add and remove tag info.  It is used by EncMode and DecMode
// to provide CBOR tag support.
type TagSet interface {
	// Add adds given tag number(s), content type, and tag options to TagSet.
	Add(opts TagOptions, contentType reflect.Type, num uint64, nestedNum ...uint64) error

	// Remove removes given tag content type from TagSet.
	Remove(contentType reflect.Type)

	tagProvider
}

type tagProvider interface {
	get(t reflect.Type) *tagItem
}

type tagItem struct {
	num         []uint64
	cborTagNum  []byte
	contentType reflect.Type
	opts        TagOptions
}

type (
	tagSet map[reflect.Type]*tagItem

	syncTagSet struct {
		sync.RWMutex
		t tagSet
	}
)

func (t tagSet) get(typ reflect.Type) *tagItem {
	return t[typ]
}

// NewTagSet returns TagSet (safe for concurrency).
func NewTagSet() TagSet {
	return &syncTagSet{t: make(map[reflect.Type]*tagItem)}
}

// Add adds given tag number(s), content type, and tag options to TagSet.
func (t *syncTagSet) Add(opts TagOptions, contentType reflect.Type, num uint64, nestedNum ...uint64) error {
	if contentType == nil {
		return errors.New("cbor: cannot add nil content type to TagSet")
	}
	for contentType.Kind() == reflect.Ptr {
		contentType = contentType.Elem()
	}
	tag, err := newTagItem(opts, contentType, num, nestedNum...)
	if err != nil {
		return err
	}
	t.Lock()
	defer t.Unlock()
	if _, ok := t.t[contentType]; ok {
		return errors.New("cbor: content type " + contentType.String() + " already exists in TagSet")
	}
	t.t[contentType] = tag
	return nil
}

// Remove removes given tag content type from TagSet.
func (t *syncTagSet) Remove(contentType reflect.Type) {
	for contentType.Kind() == reflect.Ptr {
		contentType = contentType.Elem()
	}
	t.Lock()
	delete(t.t, contentType)
	t.Unlock()
}

func (t *syncTagSet) get(typ reflect.Type) *tagItem {
	t.RLock()
	ti := t.t[typ]
	t.RUnlock()
	return ti
}

func newTagItem(opts TagOptions, contentType reflect.Type, num uint64, nestedNum ...uint64) (*tagItem, error) {
	if opts.DecTag == DecTagIgnored && opts.EncTag == EncTagNone {
		return nil, errors.New("cbor: cannot add tag with DecTagIgnored and EncTagNone options to TagSet")
	}
	if contentType.PkgPath() == "" || contentType.Kind() == reflect.Interface {
		return nil, errors.New("cbor: can only add named types to TagSet, got " + contentType.String())
	}
	if contentType == typeTime {
		return nil, errors.New("cbor: cannot add time.Time to TagSet, use EncOptions.TimeTag and DecOptions.TimeTag instead")
	}
	if contentType == typeTag {
		return nil, errors.New("cbor: cannot add cbor.Tag to TagSet")
	}
	if contentType == typeRawTag {
		return nil, errors.New("cbor: cannot add cbor.RawTag to TagSet")
	}
	if num == 0 || num == 1 {
		return nil, errors.New("cbor: cannot add tag number 0 or 1 to TagSet, use EncOptions.TimeTag and DecOptions.TimeTag instead")
	}
	if reflect.PtrTo(contentType).Implements(typeMarshaler) && opts.EncTag != EncTagNone {
		return nil, errors.New("cbor: cannot add cbor.Marshaler to TagSet with EncTag != EncTagNone")
	}
	if reflect.PtrTo(contentType).Implements(typeUnmarshaler) && opts.DecTag != DecTagIgnored {
		return nil, errors.New("cbor: cannot add cbor.Unmarshaler to TagSet with DecTag != DecTagIgnored")
	}

	te := tagItem{num: []uint64{num}, opts: opts, contentType: contentType}
	te.num = append(te.num, nestedNum...)

	// Cache encoded tag numbers
	e := getEncodeState()
	for _, n := range te.num {
		encodeHead(e, byte(cborTypeTag), n)
	}
	te.cborTagNum = make([]byte, e.Len())
	copy(te.cborTagNum, e.Bytes())
	putEncodeState(e)

	return &te, nil
}

var (
	typeTag    = reflect.TypeOf(Tag{})
	typeRawTag = reflect.TypeOf(RawTag{})
)

// WrongTagError describes mismatch between CBOR tag and registered tag.
type WrongTagError struct {
	RegisteredType   reflect.Type
	RegisteredTagNum []uint64
	TagNum           []uint64
}

func (e *WrongTagError) Error() string {
	return fmt.Sprintf("cbor: wrong tag number for %s, got %v, expected %v", e.RegisteredType.String(), e.TagNum, e.RegisteredTagNum)
}
