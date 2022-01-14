// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/x448/float16"
)

// Unmarshal parses the CBOR-encoded data and stores the result in the value
// pointed to by v using the default decoding options.  If v is nil or not a
// pointer, Unmarshal returns an error.
//
// Unmarshal uses the inverse of the encodings that Marshal uses, allocating
// maps, slices, and pointers as necessary, with the following additional rules:
//
// To unmarshal CBOR into a pointer, Unmarshal first handles the case of the
// CBOR being the CBOR literal null.  In that case, Unmarshal sets the pointer
// to nil.  Otherwise, Unmarshal unmarshals the CBOR into the value pointed at
// by the pointer.  If the pointer is nil, Unmarshal allocates a new value for
// it to point to.
//
// To unmarshal CBOR into an interface value, Unmarshal stores one of these in
// the interface value:
//
//     bool, for CBOR booleans
//     uint64, for CBOR positive integers
//     int64, for CBOR negative integers
//     float64, for CBOR floating points
//     []byte, for CBOR byte strings
//     string, for CBOR text strings
//     []interface{}, for CBOR arrays
//     map[interface{}]interface{}, for CBOR maps
//     nil, for CBOR null
//
// To unmarshal a CBOR array into a slice, Unmarshal allocates a new slice only
// if the CBOR array is empty or slice capacity is less than CBOR array length.
// Otherwise Unmarshal reuses the existing slice, overwriting existing elements.
// Unmarshal sets the slice length to CBOR array length.
//
// To ummarshal a CBOR array into a Go array, Unmarshal decodes CBOR array
// elements into corresponding Go array elements.  If the Go array is smaller
// than the CBOR array, the additional CBOR array elements are discarded.  If
// the CBOR array is smaller than the Go array, the additional Go array elements
// are set to zero values.
//
// To unmarshal a CBOR map into a map, Unmarshal allocates a new map only if the
// map is nil.  Otherwise Unmarshal reuses the existing map, keeping existing
// entries.  Unmarshal stores key-value pairs from the CBOR map into Go map.
//
// To unmarshal a CBOR map into a struct, Unmarshal matches CBOR map keys to the
// keys in the following priority:
//
//     1. "cbor" key in struct field tag,
//     2. "json" key in struct field tag,
//     3. struct field name.
//
// Unmarshal prefers an exact match but also accepts a case-insensitive match.
// Map keys which don't have a corresponding struct field are ignored.
//
// To unmarshal a CBOR text string into a time.Time value, Unmarshal parses text
// string formatted in RFC3339.  To unmarshal a CBOR integer/float into a
// time.Time value, Unmarshal creates an unix time with integer/float as seconds
// and fractional seconds since January 1, 1970 UTC.
//
// To unmarshal CBOR into a value implementing the Unmarshaler interface,
// Unmarshal calls that value's UnmarshalCBOR method.
//
// Unmarshal decodes a CBOR byte string into a value implementing
// encoding.BinaryUnmarshaler.
//
// If a CBOR value is not appropriate for a given Go type, or if a CBOR number
// overflows the Go type, Unmarshal skips that field and completes the
// unmarshalling as best as it can.  If no more serious errors are encountered,
// unmarshal returns an UnmarshalTypeError describing the earliest such error.
// In any case, it's not guaranteed that all the remaining fields following the
// problematic one will be unmarshaled into the target object.
//
// The CBOR null value unmarshals into a slice/map/pointer/interface by setting
// that Go value to nil.  Because null is often used to mean "not present",
// unmarshalling a CBOR null into any other Go type has no effect on the value
// produces no error.
//
// Unmarshal ignores CBOR tag data and parses tagged data following CBOR tag.
func Unmarshal(data []byte, v interface{}) error {
	return defaultDecMode.Unmarshal(data, v)
}

// Unmarshaler is the interface implemented by types that can unmarshal a CBOR
// representation of themselves.  The input can be assumed to be a valid encoding
// of a CBOR value. UnmarshalCBOR must copy the CBOR data if it wishes to retain
// the data after returning.
type Unmarshaler interface {
	UnmarshalCBOR([]byte) error
}

// InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "cbor: Unmarshal(nil)"
	}
	if e.Type.Kind() != reflect.Ptr {
		return "cbor: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "cbor: Unmarshal(nil " + e.Type.String() + ")"
}

// UnmarshalTypeError describes a CBOR value that was not appropriate for a Go type.
type UnmarshalTypeError struct {
	Value  string       // description of CBOR value
	Type   reflect.Type // type of Go value it could not be assigned to
	Struct string       // struct type containing the field
	Field  string       // name of the field holding the Go value
	errMsg string       // additional error message (optional)
}

func (e *UnmarshalTypeError) Error() string {
	var s string
	if e.Struct != "" || e.Field != "" {
		s = "cbor: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
	} else {
		s = "cbor: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
	}
	if e.errMsg != "" {
		s += " (" + e.errMsg + ")"
	}
	return s
}

// DupMapKeyError describes detected duplicate map key in CBOR map.
type DupMapKeyError struct {
	Key   interface{}
	Index int
}

func (e *DupMapKeyError) Error() string {
	return fmt.Sprintf("cbor: found duplicate map key \"%v\" at map element index %d", e.Key, e.Index)
}

// DupMapKeyMode specifies how to enforce duplicate map key.
type DupMapKeyMode int

const (
	// DupMapKeyQuiet doesn't enforce duplicate map key. Decoder quietly (no error)
	// uses faster of "keep first" or "keep last" depending on Go data type and other factors.
	DupMapKeyQuiet DupMapKeyMode = iota

	// DupMapKeyEnforcedAPF enforces detection and rejection of duplicate map keys.
	// APF means "Allow Partial Fill" and the destination map or struct can be partially filled.
	// If a duplicate map key is detected, DupMapKeyError is returned without further decoding
	// of the map. It's the caller's responsibility to respond to DupMapKeyError by
	// discarding the partially filled result if their protocol requires it.
	// WARNING: using DupMapKeyEnforcedAPF will decrease performance and increase memory use.
	DupMapKeyEnforcedAPF

	maxDupMapKeyMode
)

func (dmkm DupMapKeyMode) valid() bool {
	return dmkm < maxDupMapKeyMode
}

// IndefLengthMode specifies whether to allow indefinite length items.
type IndefLengthMode int

const (
	// IndefLengthAllowed allows indefinite length items.
	IndefLengthAllowed IndefLengthMode = iota

	// IndefLengthForbidden disallows indefinite length items.
	IndefLengthForbidden

	maxIndefLengthMode
)

func (m IndefLengthMode) valid() bool {
	return m < maxIndefLengthMode
}

// TagsMode specifies whether to allow CBOR tags.
type TagsMode int

const (
	// TagsAllowed allows CBOR tags.
	TagsAllowed TagsMode = iota

	// TagsForbidden disallows CBOR tags.
	TagsForbidden

	maxTagsMode
)

func (tm TagsMode) valid() bool {
	return tm < maxTagsMode
}

// DecOptions specifies decoding options.
type DecOptions struct {
	// DupMapKey specifies whether to enforce duplicate map key.
	DupMapKey DupMapKeyMode

	// TimeTag specifies whether to check validity of time.Time (e.g. valid tag number and tag content type).
	// For now, valid tag number means 0 or 1 as specified in RFC 7049 if the Go type is time.Time.
	TimeTag DecTagMode

	// MaxNestedLevels specifies the max nested levels allowed for any combination of CBOR array, maps, and tags.
	// Default is 32 levels and it can be set to [4, 256].
	MaxNestedLevels int

	// MaxArrayElements specifies the max number of elements for CBOR arrays.
	// Default is 128*1024=131072 and it can be set to [16, 134217728]
	MaxArrayElements int

	// MaxMapPairs specifies the max number of key-value pairs for CBOR maps.
	// Default is 128*1024=131072 and it can be set to [16, 134217728]
	MaxMapPairs int

	// IndefLength specifies whether to allow indefinite length CBOR items.
	IndefLength IndefLengthMode

	// TagsMd specifies whether to allow CBOR tags (major type 6).
	TagsMd TagsMode
}

// DecMode returns DecMode with immutable options and no tags (safe for concurrency).
func (opts DecOptions) DecMode() (DecMode, error) {
	return opts.decMode()
}

// DecModeWithTags returns DecMode with options and tags that are both immutable (safe for concurrency).
func (opts DecOptions) DecModeWithTags(tags TagSet) (DecMode, error) {
	if opts.TagsMd == TagsForbidden {
		return nil, errors.New("cbor: cannot create DecMode with TagSet when TagsMd is TagsForbidden")
	}
	if tags == nil {
		return nil, errors.New("cbor: cannot create DecMode with nil value as TagSet")
	}

	dm, err := opts.decMode()
	if err != nil {
		return nil, err
	}

	// Copy tags
	ts := tagSet(make(map[reflect.Type]*tagItem))
	syncTags := tags.(*syncTagSet)
	syncTags.RLock()
	for contentType, tag := range syncTags.t {
		if tag.opts.DecTag != DecTagIgnored {
			ts[contentType] = tag
		}
	}
	syncTags.RUnlock()

	if len(ts) > 0 {
		dm.tags = ts
	}

	return dm, nil
}

// DecModeWithSharedTags returns DecMode with immutable options and mutable shared tags (safe for concurrency).
func (opts DecOptions) DecModeWithSharedTags(tags TagSet) (DecMode, error) {
	if opts.TagsMd == TagsForbidden {
		return nil, errors.New("cbor: cannot create DecMode with TagSet when TagsMd is TagsForbidden")
	}
	if tags == nil {
		return nil, errors.New("cbor: cannot create DecMode with nil value as TagSet")
	}
	dm, err := opts.decMode()
	if err != nil {
		return nil, err
	}
	dm.tags = tags
	return dm, nil
}

const (
	defaultMaxArrayElements = 131072
	minMaxArrayElements     = 16
	maxMaxArrayElements     = 134217728

	defaultMaxMapPairs = 131072
	minMaxMapPairs     = 16
	maxMaxMapPairs     = 134217728
)

func (opts DecOptions) decMode() (*decMode, error) {
	if !opts.DupMapKey.valid() {
		return nil, errors.New("cbor: invalid DupMapKey " + strconv.Itoa(int(opts.DupMapKey)))
	}
	if !opts.TimeTag.valid() {
		return nil, errors.New("cbor: invalid TimeTag " + strconv.Itoa(int(opts.TimeTag)))
	}
	if !opts.IndefLength.valid() {
		return nil, errors.New("cbor: invalid IndefLength " + strconv.Itoa(int(opts.IndefLength)))
	}
	if !opts.TagsMd.valid() {
		return nil, errors.New("cbor: invalid TagsMd " + strconv.Itoa(int(opts.TagsMd)))
	}
	if opts.MaxNestedLevels == 0 {
		opts.MaxNestedLevels = 32
	} else if opts.MaxNestedLevels < 4 || opts.MaxNestedLevels > 256 {
		return nil, errors.New("cbor: invalid MaxNestedLevels " + strconv.Itoa(opts.MaxNestedLevels) + " (range is [4, 256])")
	}
	if opts.MaxArrayElements == 0 {
		opts.MaxArrayElements = defaultMaxArrayElements
	} else if opts.MaxArrayElements < minMaxArrayElements || opts.MaxArrayElements > maxMaxArrayElements {
		return nil, errors.New("cbor: invalid MaxArrayElements " + strconv.Itoa(opts.MaxArrayElements) + " (range is [" + strconv.Itoa(minMaxArrayElements) + ", " + strconv.Itoa(maxMaxArrayElements) + "])")
	}
	if opts.MaxMapPairs == 0 {
		opts.MaxMapPairs = defaultMaxMapPairs
	} else if opts.MaxMapPairs < minMaxMapPairs || opts.MaxMapPairs > maxMaxMapPairs {
		return nil, errors.New("cbor: invalid MaxMapPairs " + strconv.Itoa(opts.MaxMapPairs) + " (range is [" + strconv.Itoa(minMaxMapPairs) + ", " + strconv.Itoa(maxMaxMapPairs) + "])")
	}
	dm := decMode{
		dupMapKey:        opts.DupMapKey,
		timeTag:          opts.TimeTag,
		maxNestedLevels:  opts.MaxNestedLevels,
		maxArrayElements: opts.MaxArrayElements,
		maxMapPairs:      opts.MaxMapPairs,
		indefLength:      opts.IndefLength,
		tagsMd:           opts.TagsMd,
	}
	return &dm, nil
}

// DecMode is the main interface for CBOR decoding.
type DecMode interface {
	Unmarshal(data []byte, v interface{}) error
	NewDecoder(r io.Reader) *Decoder
	DecOptions() DecOptions
}

type decMode struct {
	tags             tagProvider
	dupMapKey        DupMapKeyMode
	timeTag          DecTagMode
	maxNestedLevels  int
	maxArrayElements int
	maxMapPairs      int
	indefLength      IndefLengthMode
	tagsMd           TagsMode
}

var defaultDecMode, _ = DecOptions{}.decMode()

// DecOptions returns user specified options used to create this DecMode.
func (dm *decMode) DecOptions() DecOptions {
	return DecOptions{
		DupMapKey:        dm.dupMapKey,
		TimeTag:          dm.timeTag,
		MaxNestedLevels:  dm.maxNestedLevels,
		MaxArrayElements: dm.maxArrayElements,
		MaxMapPairs:      dm.maxMapPairs,
		IndefLength:      dm.indefLength,
		TagsMd:           dm.tagsMd,
	}
}

// Unmarshal parses the CBOR-encoded data and stores the result in the value
// pointed to by v using dm DecMode.  If v is nil or not a pointer, Unmarshal
// returns an error.
//
// See the documentation for Unmarshal for details.
func (dm *decMode) Unmarshal(data []byte, v interface{}) error {
	d := decodeState{data: data, dm: dm}
	return d.value(v)
}

// NewDecoder returns a new decoder that reads from r using dm DecMode.
func (dm *decMode) NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r, d: decodeState{dm: dm}}
}

type decodeState struct {
	data []byte
	off  int // next read offset in data
	dm   *decMode
}

func (d *decodeState) value(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	off := d.off // Save offset before data validation
	err := d.valid()
	d.off = off // Restore offset
	if err != nil {
		return err
	}

	rv = rv.Elem()

	if rv.Kind() == reflect.Interface && rv.NumMethod() == 0 {
		// Fast path to decode to empty interface without retrieving typeInfo.
		iv, err := d.parse()
		if iv != nil {
			rv.Set(reflect.ValueOf(iv))
		}
		return err
	}

	return d.parseToValue(rv, getTypeInfo(rv.Type()))
}

type cborType uint8

const (
	cborTypePositiveInt cborType = 0x00
	cborTypeNegativeInt cborType = 0x20
	cborTypeByteString  cborType = 0x40
	cborTypeTextString  cborType = 0x60
	cborTypeArray       cborType = 0x80
	cborTypeMap         cborType = 0xa0
	cborTypeTag         cborType = 0xc0
	cborTypePrimitives  cborType = 0xe0
)

func (t cborType) String() string {
	switch t {
	case cborTypePositiveInt:
		return "positive integer"
	case cborTypeNegativeInt:
		return "negative integer"
	case cborTypeByteString:
		return "byte string"
	case cborTypeTextString:
		return "UTF-8 text string"
	case cborTypeArray:
		return "array"
	case cborTypeMap:
		return "map"
	case cborTypeTag:
		return "tag"
	case cborTypePrimitives:
		return "primitives"
	default:
		return "Invalid type " + strconv.Itoa(int(t))
	}
}

// parseToValue assumes data is well-formed, and does not perform bounds checking.
// This function is complicated because it's the main function that decodes CBOR data to reflect.Value.
func (d *decodeState) parseToValue(v reflect.Value, tInfo *typeInfo) error { //nolint:gocyclo
	// Create new value for the pointer v to point to if CBOR value is not nil/undefined.
	if !d.nextCBORNil() {
		for v.Kind() == reflect.Ptr {
			if v.IsNil() {
				if !v.CanSet() {
					d.skip()
					return errors.New("cbor: cannot set new value for " + v.Type().String())
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
	}

	if tInfo.spclType != specialTypeNone {
		switch tInfo.spclType {
		case specialTypeEmptyIface:
			iv, err := d.parse()
			if iv != nil {
				v.Set(reflect.ValueOf(iv))
			}
			return err
		case specialTypeTag:
			return d.parseToTag(v)
		case specialTypeTime:
			return d.parseToTime(v)
		case specialTypeUnmarshalerIface:
			return d.parseToUnmarshaler(v)
		}
	}

	// Check registered tag number
	if tagItem := d.getRegisteredTagItem(tInfo.nonPtrType); tagItem != nil {
		t := d.nextCBORType()
		if t != cborTypeTag {
			if tagItem.opts.DecTag == DecTagRequired {
				d.skip() // Required tag number is absent, skip entire tag
				return &UnmarshalTypeError{Value: t.String(), Type: tInfo.typ, errMsg: "expect CBOR tag value"}
			}
		} else if err := d.validRegisteredTagNums(tInfo.nonPtrType, tagItem.num); err != nil {
			d.skip() // Skip tag content
			return err
		}
	}

	t := d.nextCBORType()

	// Skip tag number(s) here to avoid recursion
	if t == cborTypeTag {
		d.getHead()
		t = d.nextCBORType()
		for t == cborTypeTag {
			d.getHead()
			t = d.nextCBORType()
		}
	}

	switch t {
	case cborTypePositiveInt:
		_, _, val := d.getHead()
		return fillPositiveInt(t, val, v)
	case cborTypeNegativeInt:
		_, _, val := d.getHead()
		if val > math.MaxInt64 {
			return &UnmarshalTypeError{
				Value:  t.String(),
				Type:   tInfo.nonPtrType,
				errMsg: "-1-" + strconv.FormatUint(val, 10) + " overflows Go's int64",
			}
		}
		nValue := int64(-1) ^ int64(val)
		return fillNegativeInt(t, nValue, v)
	case cborTypeByteString:
		b := d.parseByteString()
		return fillByteString(t, b, v)
	case cborTypeTextString:
		b, err := d.parseTextString()
		if err != nil {
			return err
		}
		return fillTextString(t, b, v)
	case cborTypePrimitives:
		_, ai, val := d.getHead()
		if ai < 20 || ai == 24 {
			return fillPositiveInt(t, val, v)
		}
		switch ai {
		case 20, 21:
			return fillBool(t, ai == 21, v)
		case 22, 23:
			return fillNil(t, v)
		case 25:
			f := float64(float16.Frombits(uint16(val)).Float32())
			return fillFloat(t, f, v)
		case 26:
			f := float64(math.Float32frombits(uint32(val)))
			return fillFloat(t, f, v)
		case 27:
			f := math.Float64frombits(val)
			return fillFloat(t, f, v)
		}
	case cborTypeArray:
		if tInfo.nonPtrKind == reflect.Slice {
			return d.parseArrayToSlice(v, tInfo)
		} else if tInfo.nonPtrKind == reflect.Array {
			return d.parseArrayToArray(v, tInfo)
		} else if tInfo.nonPtrKind == reflect.Struct {
			return d.parseArrayToStruct(v, tInfo)
		}
		d.skip()
		return &UnmarshalTypeError{Value: t.String(), Type: tInfo.nonPtrType}
	case cborTypeMap:
		if tInfo.nonPtrKind == reflect.Struct {
			return d.parseMapToStruct(v, tInfo)
		} else if tInfo.nonPtrKind == reflect.Map {
			return d.parseMapToMap(v, tInfo)
		}
		d.skip()
		return &UnmarshalTypeError{Value: t.String(), Type: tInfo.nonPtrType}
	}
	return nil
}

func (d *decodeState) parseToTag(v reflect.Value) error {
	t := d.nextCBORType()
	if t != cborTypeTag {
		d.skip()
		return &UnmarshalTypeError{Value: t.String(), Type: typeTag}
	}

	// Unmarshal tag number
	_, _, num := d.getHead()

	// Unmarshal tag content
	content, err := d.parse()
	if err != nil {
		return err
	}

	v.Set(reflect.ValueOf(Tag{num, content}))
	return nil
}

func (d *decodeState) parseToTime(v reflect.Value) error {
	t := d.nextCBORType()

	// Verify that tag number or absent of tag number is acceptable to specified timeTag.
	if t == cborTypeTag {
		if d.dm.timeTag == DecTagIgnored {
			// Skip tag number
			d.getHead()
			t = d.nextCBORType()
			for t == cborTypeTag {
				d.getHead()
				t = d.nextCBORType()
			}
		} else {
			// Read tag number
			_, _, tagNum := d.getHead()

			// Verify tag number (0 or 1) is followed by appropriate tag content type.
			t = d.nextCBORType()
			switch tagNum {
			case 0:
				// Tag content (date/time text string in RFC 3339 format) must be string type.
				if t != cborTypeTextString {
					d.skip()
					return errors.New("cbor: tag number 0 must be followed by text string, got " + t.String())
				}
			case 1:
				// Tag content (epoch date/time) must be uint, int, or float type.
				if t != cborTypePositiveInt && t != cborTypeNegativeInt && (d.data[d.off] < 0xf9 || d.data[d.off] > 0xfb) {
					d.skip()
					return errors.New("cbor: tag number 1 must be followed by integer or floating-point number, got " + t.String())
				}
			default:
				d.skip()
				return errors.New("cbor: wrong tag number for time.Time, got " + strconv.Itoa(int(tagNum)) + ", expect 0 or 1")
			}
		}
	} else {
		if d.dm.timeTag == DecTagRequired {
			d.skip()
			return &UnmarshalTypeError{Value: t.String(), Type: typeTime, errMsg: "expect CBOR tag value"}
		}
	}

	switch t {
	case cborTypePositiveInt:
		_, _, val := d.getHead()
		tm := time.Unix(int64(val), 0)
		v.Set(reflect.ValueOf(tm))
		return nil
	case cborTypeNegativeInt:
		_, _, val := d.getHead()
		if val > math.MaxInt64 {
			return &UnmarshalTypeError{
				Value:  t.String(),
				Type:   typeTime,
				errMsg: "-1-" + strconv.FormatUint(val, 10) + " overflows Go's int64",
			}
		}
		nValue := int64(-1) ^ int64(val)
		tm := time.Unix(nValue, 0)
		v.Set(reflect.ValueOf(tm))
		return nil
	case cborTypeTextString:
		b, err := d.parseTextString()
		if err != nil {
			return err
		}
		tm, err := time.Parse(time.RFC3339, string(b))
		if err != nil {
			return errors.New("cbor: cannot set " + string(b) + " for time.Time: " + err.Error())
		}
		v.Set(reflect.ValueOf(tm))
		return nil
	case cborTypePrimitives:
		_, ai, val := d.getHead()
		var f float64
		switch ai {
		case 22, 23:
			v.Set(reflect.ValueOf(time.Time{}))
			return nil
		case 25:
			f = float64(float16.Frombits(uint16(val)).Float32())
		case 26:
			f = float64(math.Float32frombits(uint32(val)))
		case 27:
			f = math.Float64frombits(val)
		default:
			return &UnmarshalTypeError{Value: t.String(), Type: typeTime}
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			v.Set(reflect.ValueOf(time.Time{}))
			return nil
		}
		f1, f2 := math.Modf(f)
		tm := time.Unix(int64(f1), int64(f2*1e9))
		v.Set(reflect.ValueOf(tm))
		return nil
	}
	d.skip()
	return &UnmarshalTypeError{Value: t.String(), Type: typeTime}
}

// parseToUnmarshaler assumes data is well-formed, and does not perform bounds checking.
func (d *decodeState) parseToUnmarshaler(v reflect.Value) error {
	if d.nextCBORNil() && v.Kind() == reflect.Ptr && v.IsNil() {
		d.skip()
		return nil
	}

	if v.Kind() != reflect.Ptr && v.CanAddr() {
		v = v.Addr()
	}
	if u, ok := v.Interface().(Unmarshaler); ok {
		start := d.off
		d.skip()
		return u.UnmarshalCBOR(d.data[start:d.off])
	}
	d.skip()
	return errors.New("cbor: failed to assert " + v.Type().String() + " as cbor.Unmarshaler")
}

// parse assumes data is well-formed, and does not perform bounds checking.
func (d *decodeState) parse() (interface{}, error) {
	t := d.nextCBORType()
	switch t {
	case cborTypePositiveInt:
		_, _, val := d.getHead()
		return val, nil
	case cborTypeNegativeInt:
		_, _, val := d.getHead()
		if val > math.MaxInt64 {
			return nil, &UnmarshalTypeError{
				Value:  t.String(),
				Type:   reflect.TypeOf([]interface{}(nil)).Elem(),
				errMsg: "-1-" + strconv.FormatUint(val, 10) + " overflows Go's int64",
			}
		}
		nValue := int64(-1) ^ int64(val)
		return nValue, nil
	case cborTypeByteString:
		return d.parseByteString(), nil
	case cborTypeTextString:
		b, err := d.parseTextString()
		if err != nil {
			return nil, err
		}
		return string(b), nil
	case cborTypeTag:
		_, _, tagNum := d.getHead()
		nt := d.nextCBORType()
		content, err := d.parse()
		if err != nil {
			return nil, err
		}
		switch tagNum {
		case 0:
			// Tag content should be date/time text string in RFC 3339 format.
			s, ok := content.(string)
			if !ok {
				return nil, errors.New("cbor: tag number 0 must be followed by text string, got " + nt.String())
			}
			tm, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return nil, errors.New("cbor: cannot set " + s + " for time.Time: " + err.Error())
			}
			return tm, nil
		case 1:
			// Tag content should be epoch date/time.
			switch content := content.(type) {
			case uint64:
				return time.Unix(int64(content), 0), nil
			case int64:
				return time.Unix(content, 0), nil
			case float64:
				f1, f2 := math.Modf(content)
				return time.Unix(int64(f1), int64(f2*1e9)), nil
			default:
				return nil, errors.New("cbor: tag number 1 must be followed by integer or floating-point number, got " + nt.String())
			}
		}
		return Tag{tagNum, content}, nil
	case cborTypePrimitives:
		_, ai, val := d.getHead()
		if ai < 20 || ai == 24 {
			return val, nil
		}
		switch ai {
		case 20, 21:
			return (ai == 21), nil
		case 22, 23:
			return nil, nil
		case 25:
			f := float64(float16.Frombits(uint16(val)).Float32())
			return f, nil
		case 26:
			f := float64(math.Float32frombits(uint32(val)))
			return f, nil
		case 27:
			f := math.Float64frombits(val)
			return f, nil
		}
	case cborTypeArray:
		return d.parseArray()
	case cborTypeMap:
		return d.parseMap()
	}
	return nil, nil
}

// parseByteString parses CBOR encoded byte string.  It returns a byte slice
// pointing to a copy of parsed data.
func (d *decodeState) parseByteString() []byte {
	_, ai, val := d.getHead()
	if ai != 31 {
		b := make([]byte, int(val))
		copy(b, d.data[d.off:d.off+int(val)])
		d.off += int(val)
		return b
	}
	// Process indefinite length string chunks.
	b := []byte{}
	for !d.foundBreak() {
		_, _, val = d.getHead()
		b = append(b, d.data[d.off:d.off+int(val)]...)
		d.off += int(val)
	}
	return b
}

// parseTextString parses CBOR encoded text string.  It does not return a string
// to prevent creating an extra copy of string.  Caller should wrap returned
// byte slice as string when needed.
//
// parseStruct() uses parseTextString() to improve memory and performance,
// compared with using parse(reflect.Value).  parse(reflect.Value) sets
// reflect.Value with parsed string, while parseTextString() returns zero-copy []byte.
func (d *decodeState) parseTextString() ([]byte, error) {
	_, ai, val := d.getHead()
	if ai != 31 {
		b := d.data[d.off : d.off+int(val)]
		d.off += int(val)
		if !utf8.Valid(b) {
			return nil, &SemanticError{"cbor: invalid UTF-8 string"}
		}
		return b, nil
	}
	// Process indefinite length string chunks.
	b := []byte{}
	for !d.foundBreak() {
		_, _, val = d.getHead()
		x := d.data[d.off : d.off+int(val)]
		d.off += int(val)
		if !utf8.Valid(x) {
			for !d.foundBreak() {
				d.skip() // Skip remaining chunk on error
			}
			return nil, &SemanticError{"cbor: invalid UTF-8 string"}
		}
		b = append(b, x...)
	}
	return b, nil
}

func (d *decodeState) parseArray() ([]interface{}, error) {
	_, ai, val := d.getHead()
	hasSize := (ai != 31)
	count := int(val)
	if !hasSize {
		count = d.numOfItemsUntilBreak() // peek ahead to get array size to preallocate slice for better performance
	}
	v := make([]interface{}, count)
	var e interface{}
	var err, lastErr error
	for i := 0; (hasSize && i < count) || (!hasSize && !d.foundBreak()); i++ {
		if e, lastErr = d.parse(); lastErr != nil {
			if err == nil {
				err = lastErr
			}
			continue
		}
		v[i] = e
	}
	return v, err
}

func (d *decodeState) parseArrayToSlice(v reflect.Value, tInfo *typeInfo) error {
	_, ai, val := d.getHead()
	hasSize := (ai != 31)
	count := int(val)
	if !hasSize {
		count = d.numOfItemsUntilBreak() // peek ahead to get array size to preallocate slice for better performance
	}
	if count == 0 {
		v.Set(reflect.MakeSlice(tInfo.nonPtrType, 0, 0))
	}
	if v.IsNil() || v.Cap() < count {
		v.Set(reflect.MakeSlice(tInfo.nonPtrType, count, count))
	}
	v.SetLen(count)
	var err error
	for i := 0; (hasSize && i < count) || (!hasSize && !d.foundBreak()); i++ {
		if lastErr := d.parseToValue(v.Index(i), tInfo.elemTypeInfo); lastErr != nil {
			if err == nil {
				err = lastErr
			}
		}
	}
	return err
}

func (d *decodeState) parseArrayToArray(v reflect.Value, tInfo *typeInfo) error {
	_, ai, val := d.getHead()
	hasSize := (ai != 31)
	count := int(val)
	gi := 0
	vLen := v.Len()
	var err error
	for ci := 0; (hasSize && ci < count) || (!hasSize && !d.foundBreak()); ci++ {
		if gi < vLen {
			// Read CBOR array element and set array element
			if lastErr := d.parseToValue(v.Index(gi), tInfo.elemTypeInfo); lastErr != nil {
				if err == nil {
					err = lastErr
				}
			}
			gi++
		} else {
			d.skip() // Skip remaining CBOR array element
		}
	}
	// Set remaining Go array elements to zero values.
	if gi < vLen {
		zeroV := reflect.Zero(tInfo.elemTypeInfo.typ)
		for ; gi < vLen; gi++ {
			v.Index(gi).Set(zeroV)
		}
	}
	return err
}

func (d *decodeState) parseMap() (map[interface{}]interface{}, error) {
	_, ai, val := d.getHead()
	hasSize := (ai != 31)
	count := int(val)
	m := make(map[interface{}]interface{})
	var k, e interface{}
	var err, lastErr error
	keyCount := 0
	for i := 0; (hasSize && i < count) || (!hasSize && !d.foundBreak()); i++ {
		// Parse CBOR map key.
		if k, lastErr = d.parse(); lastErr != nil {
			if err == nil {
				err = lastErr
			}
			d.skip()
			continue
		}

		// Detect if CBOR map key can be used as Go map key.
		kkind := reflect.ValueOf(k).Kind()
		if tag, ok := k.(Tag); ok {
			kkind = tag.contentKind()
		}
		if !isHashableKind(kkind) {
			if err == nil {
				err = errors.New("cbor: invalid map key type: " + kkind.String())
			}
			d.skip()
			continue
		}

		// Parse CBOR map value.
		if e, lastErr = d.parse(); lastErr != nil {
			if err == nil {
				err = lastErr
			}
			continue
		}

		// Add key-value pair to Go map.
		m[k] = e

		// Detect duplicate map key.
		if d.dm.dupMapKey == DupMapKeyEnforcedAPF {
			newKeyCount := len(m)
			if newKeyCount == keyCount {
				m[k] = nil
				err = &DupMapKeyError{k, i}
				i++
				// skip the rest of the map
				for ; (hasSize && i < count) || (!hasSize && !d.foundBreak()); i++ {
					d.skip() // Skip map key
					d.skip() // Skip map value
				}
				return m, err
			}
			keyCount = newKeyCount
		}
	}
	return m, err
}

func (d *decodeState) parseMapToMap(v reflect.Value, tInfo *typeInfo) error { //nolint:gocyclo
	_, ai, val := d.getHead()
	hasSize := (ai != 31)
	count := int(val)
	if v.IsNil() {
		mapsize := count
		if !hasSize {
			mapsize = 0
		}
		v.Set(reflect.MakeMapWithSize(tInfo.nonPtrType, mapsize))
	}
	keyType, eleType := tInfo.keyTypeInfo.typ, tInfo.elemTypeInfo.typ
	reuseKey, reuseEle := isImmutableKind(tInfo.keyTypeInfo.kind), isImmutableKind(tInfo.elemTypeInfo.kind)
	var keyValue, eleValue, zeroKeyValue, zeroEleValue reflect.Value
	keyIsInterfaceType := keyType == typeIntf // If key type is interface{}, need to check if key value is hashable.
	var err, lastErr error
	keyCount := v.Len()
	var existingKeys map[interface{}]bool // Store existing map keys, used for detecting duplicate map key.
	if d.dm.dupMapKey == DupMapKeyEnforcedAPF {
		existingKeys = make(map[interface{}]bool, keyCount)
		if keyCount > 0 {
			vKeys := v.MapKeys()
			for i := 0; i < len(vKeys); i++ {
				existingKeys[vKeys[i].Interface()] = true
			}
		}
	}
	for i := 0; (hasSize && i < count) || (!hasSize && !d.foundBreak()); i++ {
		// Parse CBOR map key.
		if !keyValue.IsValid() {
			keyValue = reflect.New(keyType).Elem()
		} else if !reuseKey {
			if !zeroKeyValue.IsValid() {
				zeroKeyValue = reflect.Zero(keyType)
			}
			keyValue.Set(zeroKeyValue)
		}
		if lastErr = d.parseToValue(keyValue, tInfo.keyTypeInfo); lastErr != nil {
			if err == nil {
				err = lastErr
			}
			d.skip()
			continue
		}

		// Detect if CBOR map key can be used as Go map key.
		if keyIsInterfaceType {
			kkind := keyValue.Elem().Kind()
			if keyValue.Elem().IsValid() {
				if tag, ok := keyValue.Elem().Interface().(Tag); ok {
					kkind = tag.contentKind()
				}
			}
			if !isHashableKind(kkind) {
				if err == nil {
					err = errors.New("cbor: invalid map key type: " + kkind.String())
				}
				d.skip()
				continue
			}
		}

		// Parse CBOR map value.
		if !eleValue.IsValid() {
			eleValue = reflect.New(eleType).Elem()
		} else if !reuseEle {
			if !zeroEleValue.IsValid() {
				zeroEleValue = reflect.Zero(eleType)
			}
			eleValue.Set(zeroEleValue)
		}
		if lastErr := d.parseToValue(eleValue, tInfo.elemTypeInfo); lastErr != nil {
			if err == nil {
				err = lastErr
			}
			continue
		}

		// Add key-value pair to Go map.
		v.SetMapIndex(keyValue, eleValue)

		// Detect duplicate map key.
		if d.dm.dupMapKey == DupMapKeyEnforcedAPF {
			newKeyCount := v.Len()
			if newKeyCount == keyCount {
				kvi := keyValue.Interface()
				if !existingKeys[kvi] {
					v.SetMapIndex(keyValue, reflect.New(eleType).Elem())
					err = &DupMapKeyError{kvi, i}
					i++
					// skip the rest of the map
					for ; (hasSize && i < count) || (!hasSize && !d.foundBreak()); i++ {
						d.skip() // skip map key
						d.skip() // skip map value
					}
					return err
				}
				delete(existingKeys, kvi)
			}
			keyCount = newKeyCount
		}
	}
	return err
}

func (d *decodeState) parseArrayToStruct(v reflect.Value, tInfo *typeInfo) error {
	structType := getDecodingStructType(tInfo.nonPtrType)
	if structType.err != nil {
		return structType.err
	}

	if !structType.toArray {
		t := d.nextCBORType()
		d.skip()
		return &UnmarshalTypeError{
			Value:  t.String(),
			Type:   tInfo.nonPtrType,
			errMsg: "cannot decode CBOR array to struct without toarray option",
		}
	}

	start := d.off
	t, ai, val := d.getHead()
	hasSize := (ai != 31)
	count := int(val)
	if !hasSize {
		count = d.numOfItemsUntilBreak() // peek ahead to get array size
	}
	if count != len(structType.fields) {
		d.off = start
		d.skip()
		return &UnmarshalTypeError{
			Value:  t.String(),
			Type:   tInfo.typ,
			errMsg: "cannot decode CBOR array to struct with different number of elements",
		}
	}
	var err error
	for i := 0; (hasSize && i < count) || (!hasSize && !d.foundBreak()); i++ {
		f := structType.fields[i]
		fv, lastErr := fieldByIndex(v, f.idx)
		if lastErr != nil {
			if err == nil {
				err = lastErr
			}
			d.skip()
			continue
		}
		if lastErr := d.parseToValue(fv, f.typInfo); lastErr != nil {
			if err == nil {
				if typeError, ok := lastErr.(*UnmarshalTypeError); ok {
					typeError.Struct = tInfo.typ.String()
					typeError.Field = f.name
					err = typeError
				} else {
					err = lastErr
				}
			}
		}
	}
	return err
}

// parseMapToStruct needs to be fast so gocyclo can be ignored for now.
func (d *decodeState) parseMapToStruct(v reflect.Value, tInfo *typeInfo) error { //nolint:gocyclo
	structType := getDecodingStructType(tInfo.nonPtrType)
	if structType.err != nil {
		return structType.err
	}

	if structType.toArray {
		t := d.nextCBORType()
		d.skip()
		return &UnmarshalTypeError{
			Value:  t.String(),
			Type:   tInfo.nonPtrType,
			errMsg: "cannot decode CBOR map to struct with toarray option",
		}
	}

	foundFldIdx := make([]bool, len(structType.fields))
	_, ai, val := d.getHead()
	hasSize := (ai != 31)
	count := int(val)
	var err, lastErr error
	keyCount := 0
	var mapKeys map[interface{}]struct{} // Store map keys, used for detecting duplicate map key.
	if d.dm.dupMapKey == DupMapKeyEnforcedAPF {
		mapKeys = make(map[interface{}]struct{}, len(structType.fields))
	}
	for j := 0; (hasSize && j < count) || (!hasSize && !d.foundBreak()); j++ {
		var f *field
		var k interface{} // Used by duplicate map key detection

		t := d.nextCBORType()
		if t == cborTypeTextString {
			var keyBytes []byte
			keyBytes, lastErr = d.parseTextString()
			if lastErr != nil {
				if err == nil {
					err = lastErr
				}
				d.skip() // skip value
				continue
			}

			keyLen := len(keyBytes)
			// Find field with exact match
			for i := 0; i < len(structType.fields); i++ {
				fld := structType.fields[i]
				if !foundFldIdx[i] && len(fld.name) == keyLen && fld.name == string(keyBytes) {
					f = fld
					foundFldIdx[i] = true
					break
				}
			}
			// Find field with case-insensitive match
			if f == nil {
				keyString := string(keyBytes)
				for i := 0; i < len(structType.fields); i++ {
					fld := structType.fields[i]
					if !foundFldIdx[i] && len(fld.name) == keyLen && strings.EqualFold(fld.name, keyString) {
						f = fld
						foundFldIdx[i] = true
						break
					}
				}
			}

			if d.dm.dupMapKey == DupMapKeyEnforcedAPF {
				k = string(keyBytes)
			}
		} else if t <= cborTypeNegativeInt { // uint/int
			var nameAsInt int64

			if t == cborTypePositiveInt {
				_, _, val := d.getHead()
				nameAsInt = int64(val)
			} else {
				_, _, val := d.getHead()
				if val > math.MaxInt64 {
					if err == nil {
						err = &UnmarshalTypeError{
							Value:  t.String(),
							Type:   reflect.TypeOf(int64(0)),
							errMsg: "-1-" + strconv.FormatUint(val, 10) + " overflows Go's int64",
						}
					}
					d.skip() // skip value
					continue
				}
				nameAsInt = int64(-1) ^ int64(val)
			}

			// Find field
			for i := 0; i < len(structType.fields); i++ {
				fld := structType.fields[i]
				if !foundFldIdx[i] && fld.keyAsInt && fld.nameAsInt == nameAsInt {
					f = fld
					foundFldIdx[i] = true
					break
				}
			}

			if d.dm.dupMapKey == DupMapKeyEnforcedAPF {
				k = nameAsInt
			}
		} else {
			if err == nil {
				err = &UnmarshalTypeError{
					Value:  t.String(),
					Type:   reflect.TypeOf(""),
					errMsg: "map key is of type " + t.String() + " and cannot be used to match struct field name",
				}
			}
			if d.dm.dupMapKey == DupMapKeyEnforcedAPF {
				// parse key
				k, lastErr = d.parse()
				if lastErr != nil {
					d.skip() // skip value
					continue
				}
				// Detect if CBOR map key can be used as Go map key.
				kkind := reflect.ValueOf(k).Kind()
				if tag, ok := k.(Tag); ok {
					kkind = tag.contentKind()
				}
				if !isHashableKind(kkind) {
					d.skip() // skip value
					continue
				}
			} else {
				d.skip() // skip key
			}
		}

		if d.dm.dupMapKey == DupMapKeyEnforcedAPF {
			mapKeys[k] = struct{}{}
			newKeyCount := len(mapKeys)
			if newKeyCount == keyCount {
				err = &DupMapKeyError{k, j}
				d.skip() // skip value
				j++
				// skip the rest of the map
				for ; (hasSize && j < count) || (!hasSize && !d.foundBreak()); j++ {
					d.skip()
					d.skip()
				}
				return err
			}
			keyCount = newKeyCount
		}

		if f == nil {
			d.skip() // Skip value
			continue
		}
		// reflect.Value.FieldByIndex() panics at nil pointer to unexported
		// anonymous field.  fieldByIndex() returns error.
		fv, lastErr := fieldByIndex(v, f.idx)
		if lastErr != nil {
			if err == nil {
				err = lastErr
			}
			d.skip()
			continue
		}
		if lastErr = d.parseToValue(fv, f.typInfo); lastErr != nil {
			if err == nil {
				if typeError, ok := lastErr.(*UnmarshalTypeError); ok {
					typeError.Struct = tInfo.nonPtrType.String()
					typeError.Field = f.name
					err = typeError
				} else {
					err = lastErr
				}
			}
		}
	}
	return err
}

// validRegisteredTagNums verifies that tag numbers match registered tag numbers of type t.
// validRegisteredTagNums assumes next CBOR data type is tag.  It scans all tag numbers, and stops at tag content.
func (d *decodeState) validRegisteredTagNums(t reflect.Type, registeredTagNums []uint64) error {
	// Scan until next cbor data is tag content.
	tagNums := make([]uint64, 0, 2)
	for d.nextCBORType() == cborTypeTag {
		_, _, val := d.getHead()
		tagNums = append(tagNums, val)
	}

	// Verify that tag numbers match registered tag numbers of type t
	if len(tagNums) != len(registeredTagNums) {
		return &WrongTagError{t, registeredTagNums, tagNums}
	}
	for i, n := range registeredTagNums {
		if n != tagNums[i] {
			return &WrongTagError{t, registeredTagNums, tagNums}
		}
	}
	return nil
}

func (d *decodeState) getRegisteredTagItem(vt reflect.Type) *tagItem {
	if d.dm.tags != nil {
		return d.dm.tags.get(vt)
	}
	return nil
}

// skip moves data offset to the next item.  skip assumes data is well-formed,
// and does not perform bounds checking.
func (d *decodeState) skip() {
	t, ai, val := d.getHead()

	if ai == 31 {
		switch t {
		case cborTypeByteString, cborTypeTextString, cborTypeArray, cborTypeMap:
			for {
				if d.data[d.off] == 0xff {
					d.off++
					return
				}
				d.skip()
			}
		}
	}

	switch t {
	case cborTypeByteString, cborTypeTextString:
		d.off += int(val)
	case cborTypeArray:
		for i := 0; i < int(val); i++ {
			d.skip()
		}
	case cborTypeMap:
		for i := 0; i < int(val)*2; i++ {
			d.skip()
		}
	case cborTypeTag:
		d.skip()
	}
}

// getHead assumes data is well-formed, and does not perform bounds checking.
func (d *decodeState) getHead() (t cborType, ai byte, val uint64) {
	t = cborType(d.data[d.off] & 0xe0)
	ai = d.data[d.off] & 0x1f
	val = uint64(ai)
	d.off++

	if ai < 24 {
		return
	}
	if ai == 24 {
		val = uint64(d.data[d.off])
		d.off++
		return
	}
	if ai == 25 {
		val = uint64(binary.BigEndian.Uint16(d.data[d.off : d.off+2]))
		d.off += 2
		return
	}
	if ai == 26 {
		val = uint64(binary.BigEndian.Uint32(d.data[d.off : d.off+4]))
		d.off += 4
		return
	}
	if ai == 27 {
		val = binary.BigEndian.Uint64(d.data[d.off : d.off+8])
		d.off += 8
		return
	}
	return
}

func (d *decodeState) numOfItemsUntilBreak() int {
	savedOff := d.off
	i := 0
	for !d.foundBreak() {
		d.skip()
		i++
	}
	d.off = savedOff
	return i
}

// foundBreak assumes data is well-formed, and does not perform bounds checking.
func (d *decodeState) foundBreak() bool {
	if d.data[d.off] == 0xff {
		d.off++
		return true
	}
	return false
}

func (d *decodeState) reset(data []byte) {
	d.data = data
	d.off = 0
}

func (d *decodeState) nextCBORType() cborType {
	return cborType(d.data[d.off] & 0xe0)
}

func (d *decodeState) nextCBORNil() bool {
	return d.data[d.off] == 0xf6 || d.data[d.off] == 0xf7
}

var (
	typeIntf              = reflect.TypeOf([]interface{}(nil)).Elem()
	typeTime              = reflect.TypeOf(time.Time{})
	typeUnmarshaler       = reflect.TypeOf((*Unmarshaler)(nil)).Elem()
	typeBinaryUnmarshaler = reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem()
)

func fillNil(t cborType, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Slice, reflect.Map, reflect.Interface, reflect.Ptr:
		v.Set(reflect.Zero(v.Type()))
		return nil
	}
	return nil
}

func fillPositiveInt(t cborType, val uint64, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val > math.MaxInt64 {
			return &UnmarshalTypeError{Value: t.String(), Type: v.Type(), errMsg: strconv.FormatUint(val, 10) + " overflows " + v.Type().String()}
		}
		if v.OverflowInt(int64(val)) {
			return &UnmarshalTypeError{Value: t.String(), Type: v.Type(), errMsg: strconv.FormatUint(val, 10) + " overflows " + v.Type().String()}
		}
		v.SetInt(int64(val))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.OverflowUint(val) {
			return &UnmarshalTypeError{Value: t.String(), Type: v.Type(), errMsg: strconv.FormatUint(val, 10) + " overflows " + v.Type().String()}
		}
		v.SetUint(val)
		return nil
	case reflect.Float32, reflect.Float64:
		f := float64(val)
		v.SetFloat(f)
		return nil
	}
	return &UnmarshalTypeError{Value: t.String(), Type: v.Type()}
}

func fillNegativeInt(t cborType, val int64, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.OverflowInt(val) {
			return &UnmarshalTypeError{Value: t.String(), Type: v.Type(), errMsg: strconv.FormatInt(val, 10) + " overflows " + v.Type().String()}
		}
		v.SetInt(val)
		return nil
	case reflect.Float32, reflect.Float64:
		f := float64(val)
		v.SetFloat(f)
		return nil
	}
	return &UnmarshalTypeError{Value: t.String(), Type: v.Type()}
}

func fillBool(t cborType, val bool, v reflect.Value) error {
	if v.Kind() == reflect.Bool {
		v.SetBool(val)
		return nil
	}
	return &UnmarshalTypeError{Value: t.String(), Type: v.Type()}
}

func fillFloat(t cborType, val float64, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		if v.OverflowFloat(val) {
			return &UnmarshalTypeError{
				Value:  t.String(),
				Type:   v.Type(),
				errMsg: strconv.FormatFloat(val, 'E', -1, 64) + " overflows " + v.Type().String(),
			}
		}
		v.SetFloat(val)
		return nil
	}
	return &UnmarshalTypeError{Value: t.String(), Type: v.Type()}
}

func fillByteString(t cborType, val []byte, v reflect.Value) error {
	if reflect.PtrTo(v.Type()).Implements(typeBinaryUnmarshaler) {
		if v.CanAddr() {
			v = v.Addr()
			if u, ok := v.Interface().(encoding.BinaryUnmarshaler); ok {
				return u.UnmarshalBinary(val)
			}
		}
		return errors.New("cbor: cannot set new value for " + v.Type().String())
	}
	if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
		v.SetBytes(val)
		return nil
	}
	if v.Kind() == reflect.Array && v.Type().Elem().Kind() == reflect.Uint8 {
		vLen := v.Len()
		i := 0
		for ; i < vLen && i < len(val); i++ {
			v.Index(i).SetUint(uint64(val[i]))
		}
		// Set remaining Go array elements to zero values.
		if i < vLen {
			zeroV := reflect.Zero(reflect.TypeOf(byte(0)))
			for ; i < vLen; i++ {
				v.Index(i).Set(zeroV)
			}
		}
		return nil
	}
	return &UnmarshalTypeError{Value: t.String(), Type: v.Type()}
}

func fillTextString(t cborType, val []byte, v reflect.Value) error {
	if v.Kind() == reflect.String {
		v.SetString(string(val))
		return nil
	}
	return &UnmarshalTypeError{Value: t.String(), Type: v.Type()}
}

func isImmutableKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return true
	default:
		return false
	}
}

func isHashableKind(k reflect.Kind) bool {
	switch k {
	case reflect.Slice, reflect.Map, reflect.Func:
		return false
	default:
		return true
	}
}

// fieldByIndex returns the nested field corresponding to the index.  It
// allocates pointer to struct field if it is nil and settable.
// reflect.Value.FieldByIndex() panics at nil pointer to unexported anonymous
// field.  This function returns error.
func fieldByIndex(v reflect.Value, index []int) (reflect.Value, error) {
	for _, i := range index {
		if v.Kind() == reflect.Ptr && v.Type().Elem().Kind() == reflect.Struct {
			if v.IsNil() {
				if !v.CanSet() {
					return reflect.Value{}, errors.New("cbor: cannot set embedded pointer to unexported struct: " + v.Type().String())
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v, nil
}
