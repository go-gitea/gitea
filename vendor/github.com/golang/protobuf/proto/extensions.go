// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proto

import (
	"errors"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoiface"
	"google.golang.org/protobuf/runtime/protoimpl"
)

type (
	// ExtensionDesc represents an extension descriptor and
	// is used to interact with an extension field in a message.
	//
	// Variables of this type are generated in code by protoc-gen-go.
	ExtensionDesc = protoimpl.ExtensionInfo

	// ExtensionRange represents a range of message extensions.
	// Used in code generated by protoc-gen-go.
	ExtensionRange = protoiface.ExtensionRangeV1

	// Deprecated: Do not use; this is an internal type.
	Extension = protoimpl.ExtensionFieldV1

	// Deprecated: Do not use; this is an internal type.
	XXX_InternalExtensions = protoimpl.ExtensionFields
)

// ErrMissingExtension reports whether the extension was not present.
var ErrMissingExtension = errors.New("proto: missing extension")

var errNotExtendable = errors.New("proto: not an extendable proto.Message")

// HasExtension reports whether the extension field is present in m
// either as an explicitly populated field or as an unknown field.
func HasExtension(m Message, xt *ExtensionDesc) (has bool) {
	mr := MessageReflect(m)
	if mr == nil || !mr.IsValid() {
		return false
	}

	// Check whether any populated known field matches the field number.
	xtd := xt.TypeDescriptor()
	if isValidExtension(mr.Descriptor(), xtd) {
		has = mr.Has(xtd)
	} else {
		mr.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
			has = int32(fd.Number()) == xt.Field
			return !has
		})
	}

	// Check whether any unknown field matches the field number.
	for b := mr.GetUnknown(); !has && len(b) > 0; {
		num, _, n := protowire.ConsumeField(b)
		has = int32(num) == xt.Field
		b = b[n:]
	}
	return has
}

// ClearExtension removes the the exntesion field from m
// either as an explicitly populated field or as an unknown field.
func ClearExtension(m Message, xt *ExtensionDesc) {
	mr := MessageReflect(m)
	if mr == nil || !mr.IsValid() {
		return
	}

	xtd := xt.TypeDescriptor()
	if isValidExtension(mr.Descriptor(), xtd) {
		mr.Clear(xtd)
	} else {
		mr.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
			if int32(fd.Number()) == xt.Field {
				mr.Clear(fd)
				return false
			}
			return true
		})
	}
	clearUnknown(mr, fieldNum(xt.Field))
}

// ClearAllExtensions clears all extensions from m.
// This includes populated fields and unknown fields in the extension range.
func ClearAllExtensions(m Message) {
	mr := MessageReflect(m)
	if mr == nil || !mr.IsValid() {
		return
	}

	mr.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		if fd.IsExtension() {
			mr.Clear(fd)
		}
		return true
	})
	clearUnknown(mr, mr.Descriptor().ExtensionRanges())
}

// GetExtension retrieves a proto2 extended field from pb.
//
// If the descriptor is type complete (i.e., ExtensionDesc.ExtensionType is non-nil),
// then GetExtension parses the encoded field and returns a Go value of the specified type.
// If the field is not present, then the default value is returned (if one is specified),
// otherwise ErrMissingExtension is reported.
//
// If the descriptor is type incomplete (i.e., ExtensionDesc.ExtensionType is nil),
// then GetExtension returns the raw encoded bytes for the extension field.
func GetExtension(m Message, xt *ExtensionDesc) (interface{}, error) {
	mr := MessageReflect(m)
	if mr == nil || !mr.IsValid() || mr.Descriptor().ExtensionRanges().Len() == 0 {
		return nil, errNotExtendable
	}

	// Retrieve the unknown fields for this extension field.
	var bo protoreflect.RawFields
	for bi := mr.GetUnknown(); len(bi) > 0; {
		num, _, n := protowire.ConsumeField(bi)
		if int32(num) == xt.Field {
			bo = append(bo, bi[:n]...)
		}
		bi = bi[n:]
	}

	// For type incomplete descriptors, only retrieve the unknown fields.
	if xt.ExtensionType == nil {
		return []byte(bo), nil
	}

	// If the extension field only exists as unknown fields, unmarshal it.
	// This is rarely done since proto.Unmarshal eagerly unmarshals extensions.
	xtd := xt.TypeDescriptor()
	if !isValidExtension(mr.Descriptor(), xtd) {
		return nil, fmt.Errorf("proto: bad extended type; %T does not extend %T", xt.ExtendedType, m)
	}
	if !mr.Has(xtd) && len(bo) > 0 {
		m2 := mr.New()
		if err := (proto.UnmarshalOptions{
			Resolver: extensionResolver{xt},
		}.Unmarshal(bo, m2.Interface())); err != nil {
			return nil, err
		}
		if m2.Has(xtd) {
			mr.Set(xtd, m2.Get(xtd))
			clearUnknown(mr, fieldNum(xt.Field))
		}
	}

	// Check whether the message has the extension field set or a default.
	var pv protoreflect.Value
	switch {
	case mr.Has(xtd):
		pv = mr.Get(xtd)
	case xtd.HasDefault():
		pv = xtd.Default()
	default:
		return nil, ErrMissingExtension
	}

	v := xt.InterfaceOf(pv)
	rv := reflect.ValueOf(v)
	if isScalarKind(rv.Kind()) {
		rv2 := reflect.New(rv.Type())
		rv2.Elem().Set(rv)
		v = rv2.Interface()
	}
	return v, nil
}

// extensionResolver is a custom extension resolver that stores a single
// extension type that takes precedence over the global registry.
type extensionResolver struct{ xt protoreflect.ExtensionType }

func (r extensionResolver) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	if xtd := r.xt.TypeDescriptor(); xtd.FullName() == field {
		return r.xt, nil
	}
	return protoregistry.GlobalTypes.FindExtensionByName(field)
}

func (r extensionResolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	if xtd := r.xt.TypeDescriptor(); xtd.ContainingMessage().FullName() == message && xtd.Number() == field {
		return r.xt, nil
	}
	return protoregistry.GlobalTypes.FindExtensionByNumber(message, field)
}

// GetExtensions returns a list of the extensions values present in m,
// corresponding with the provided list of extension descriptors, xts.
// If an extension is missing in m, the corresponding value is nil.
func GetExtensions(m Message, xts []*ExtensionDesc) ([]interface{}, error) {
	mr := MessageReflect(m)
	if mr == nil || !mr.IsValid() {
		return nil, errNotExtendable
	}

	vs := make([]interface{}, len(xts))
	for i, xt := range xts {
		v, err := GetExtension(m, xt)
		if err != nil {
			if err == ErrMissingExtension {
				continue
			}
			return vs, err
		}
		vs[i] = v
	}
	return vs, nil
}

// SetExtension sets an extension field in m to the provided value.
func SetExtension(m Message, xt *ExtensionDesc, v interface{}) error {
	mr := MessageReflect(m)
	if mr == nil || !mr.IsValid() || mr.Descriptor().ExtensionRanges().Len() == 0 {
		return errNotExtendable
	}

	rv := reflect.ValueOf(v)
	if reflect.TypeOf(v) != reflect.TypeOf(xt.ExtensionType) {
		return fmt.Errorf("proto: bad extension value type. got: %T, want: %T", v, xt.ExtensionType)
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return fmt.Errorf("proto: SetExtension called with nil value of type %T", v)
		}
		if isScalarKind(rv.Elem().Kind()) {
			v = rv.Elem().Interface()
		}
	}

	xtd := xt.TypeDescriptor()
	if !isValidExtension(mr.Descriptor(), xtd) {
		return fmt.Errorf("proto: bad extended type; %T does not extend %T", xt.ExtendedType, m)
	}
	mr.Set(xtd, xt.ValueOf(v))
	clearUnknown(mr, fieldNum(xt.Field))
	return nil
}

// SetRawExtension inserts b into the unknown fields of m.
//
// Deprecated: Use Message.ProtoReflect.SetUnknown instead.
func SetRawExtension(m Message, fnum int32, b []byte) {
	mr := MessageReflect(m)
	if mr == nil || !mr.IsValid() {
		return
	}

	// Verify that the raw field is valid.
	for b0 := b; len(b0) > 0; {
		num, _, n := protowire.ConsumeField(b0)
		if int32(num) != fnum {
			panic(fmt.Sprintf("mismatching field number: got %d, want %d", num, fnum))
		}
		b0 = b0[n:]
	}

	ClearExtension(m, &ExtensionDesc{Field: fnum})
	mr.SetUnknown(append(mr.GetUnknown(), b...))
}

// ExtensionDescs returns a list of extension descriptors found in m,
// containing descriptors for both populated extension fields in m and
// also unknown fields of m that are in the extension range.
// For the later case, an type incomplete descriptor is provided where only
// the ExtensionDesc.Field field is populated.
// The order of the extension descriptors is undefined.
func ExtensionDescs(m Message) ([]*ExtensionDesc, error) {
	mr := MessageReflect(m)
	if mr == nil || !mr.IsValid() || mr.Descriptor().ExtensionRanges().Len() == 0 {
		return nil, errNotExtendable
	}

	// Collect a set of known extension descriptors.
	extDescs := make(map[protoreflect.FieldNumber]*ExtensionDesc)
	mr.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsExtension() {
			xt := fd.(protoreflect.ExtensionTypeDescriptor)
			if xd, ok := xt.Type().(*ExtensionDesc); ok {
				extDescs[fd.Number()] = xd
			}
		}
		return true
	})

	// Collect a set of unknown extension descriptors.
	extRanges := mr.Descriptor().ExtensionRanges()
	for b := mr.GetUnknown(); len(b) > 0; {
		num, _, n := protowire.ConsumeField(b)
		if extRanges.Has(num) && extDescs[num] == nil {
			extDescs[num] = nil
		}
		b = b[n:]
	}

	// Transpose the set of descriptors into a list.
	var xts []*ExtensionDesc
	for num, xt := range extDescs {
		if xt == nil {
			xt = &ExtensionDesc{Field: int32(num)}
		}
		xts = append(xts, xt)
	}
	return xts, nil
}

// isValidExtension reports whether xtd is a valid extension descriptor for md.
func isValidExtension(md protoreflect.MessageDescriptor, xtd protoreflect.ExtensionTypeDescriptor) bool {
	return xtd.ContainingMessage() == md && md.ExtensionRanges().Has(xtd.Number())
}

// isScalarKind reports whether k is a protobuf scalar kind (except bytes).
// This function exists for historical reasons since the representation of
// scalars differs between v1 and v2, where v1 uses *T and v2 uses T.
func isScalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.Int32, reflect.Int64, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.String:
		return true
	default:
		return false
	}
}

// clearUnknown removes unknown fields from m where remover.Has reports true.
func clearUnknown(m protoreflect.Message, remover interface {
	Has(protoreflect.FieldNumber) bool
}) {
	var bo protoreflect.RawFields
	for bi := m.GetUnknown(); len(bi) > 0; {
		num, _, n := protowire.ConsumeField(bi)
		if !remover.Has(num) {
			bo = append(bo, bi[:n]...)
		}
		bi = bi[n:]
	}
	if bi := m.GetUnknown(); len(bi) != len(bo) {
		m.SetUnknown(bo)
	}
}

type fieldNum protoreflect.FieldNumber

func (n1 fieldNum) Has(n2 protoreflect.FieldNumber) bool {
	return protoreflect.FieldNumber(n1) == n2
}
