// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"reflect"
	"sort"
	"strings"
)

type field struct {
	name      string
	nameAsInt int64 // used to decoder to match field name with CBOR int
	cborName  []byte
	idx       []int
	typ       reflect.Type
	ef        encodeFunc
	typInfo   *typeInfo // used to decoder to reuse type info
	tagged    bool      // used to choose dominant field (at the same level tagged fields dominate untagged fields)
	omitEmpty bool      // used to skip empty field
	keyAsInt  bool      // used to encode/decode field name as int
}

type fields []*field

// indexFieldSorter sorts fields by field idx at each level, breaking ties with idx depth.
type indexFieldSorter struct {
	fields fields
}

func (x *indexFieldSorter) Len() int {
	return len(x.fields)
}

func (x *indexFieldSorter) Swap(i, j int) {
	x.fields[i], x.fields[j] = x.fields[j], x.fields[i]
}

func (x *indexFieldSorter) Less(i, j int) bool {
	iIdx := x.fields[i].idx
	jIdx := x.fields[j].idx
	for k, d := range iIdx {
		if k >= len(jIdx) {
			// fields[j].idx is a subset of fields[i].idx.
			return false
		}
		if d != jIdx[k] {
			// fields[i].idx and fields[j].idx are different.
			return d < jIdx[k]
		}
	}
	// fields[i].idx is either the same as, or a subset of fields[j].idx.
	return true
}

// nameLevelAndTagFieldSorter sorts fields by field name, idx depth, and presence of tag.
type nameLevelAndTagFieldSorter struct {
	fields fields
}

func (x *nameLevelAndTagFieldSorter) Len() int {
	return len(x.fields)
}

func (x *nameLevelAndTagFieldSorter) Swap(i, j int) {
	x.fields[i], x.fields[j] = x.fields[j], x.fields[i]
}

func (x *nameLevelAndTagFieldSorter) Less(i, j int) bool {
	if x.fields[i].name != x.fields[j].name {
		return x.fields[i].name < x.fields[j].name
	}
	if len(x.fields[i].idx) != len(x.fields[j].idx) {
		return len(x.fields[i].idx) < len(x.fields[j].idx)
	}
	if x.fields[i].tagged != x.fields[j].tagged {
		return x.fields[i].tagged
	}
	return i < j // Field i and j have the same name, depth, and tagged status. Nothing else matters.
}

// getFields returns a list of visible fields of struct type typ following Go
// visibility rules for struct fields.
func getFields(typ reflect.Type) (flds fields, structOptions string) {
	// Inspired by typeFields() in stdlib's encoding/json/encode.go.

	var current map[reflect.Type][][]int // key: struct type, value: field index of this struct type at the same level
	next := map[reflect.Type][][]int{typ: nil}
	visited := map[reflect.Type]bool{} // Inspected struct type at less nested levels.

	for len(next) > 0 {
		current, next = next, map[reflect.Type][][]int{}

		for structType, structIdx := range current {
			if len(structIdx) > 1 {
				continue // Fields of the same embedded struct type at the same level are ignored.
			}

			if visited[structType] {
				continue
			}
			visited[structType] = true

			var fieldIdx []int
			if len(structIdx) > 0 {
				fieldIdx = structIdx[0]
			}

			for i := 0; i < structType.NumField(); i++ {
				f := structType.Field(i)
				ft := f.Type

				if ft.Kind() == reflect.Ptr {
					ft = ft.Elem()
				}

				exportable := f.PkgPath == ""
				if f.Anonymous {
					if !exportable && ft.Kind() != reflect.Struct {
						// Nonexportable anonymous fields of non-struct type are ignored.
						continue
					}
					// Nonexportable anonymous field of struct type can contain exportable fields for serialization.
				} else if !exportable {
					// Get special field "_" struct options
					if f.Name == "_" {
						tag := f.Tag.Get("cbor")
						if tag != "-" {
							structOptions = tag
						}
					}
					// Nonexportable fields are ignored.
					continue
				}

				tag := f.Tag.Get("cbor")
				if tag == "" {
					tag = f.Tag.Get("json")
				}
				if tag == "-" {
					continue
				}

				idx := make([]int, len(fieldIdx)+1)
				copy(idx, fieldIdx)
				idx[len(fieldIdx)] = i

				tagged := len(tag) > 0
				tagFieldName, omitempty, keyasint := getFieldNameAndOptionsFromTag(tag)

				fieldName := tagFieldName
				if tagFieldName == "" {
					fieldName = f.Name
				}

				if !f.Anonymous || ft.Kind() != reflect.Struct || len(tagFieldName) > 0 {
					flds = append(flds, &field{name: fieldName, idx: idx, typ: f.Type, tagged: tagged, omitEmpty: omitempty, keyAsInt: keyasint})
					continue
				}

				// f is anonymous struct of type ft.
				next[ft] = append(next[ft], idx)
			}
		}
	}

	sort.Sort(&nameLevelAndTagFieldSorter{flds})

	// Keep visible fields.
	visibleFields := flds[:0]
	for i, j := 0, 0; i < len(flds); i = j {
		name := flds[i].name
		for j = i + 1; j < len(flds) && flds[j].name == name; j++ {
		}
		if j-i == 1 || len(flds[i].idx) < len(flds[i+1].idx) || (flds[i].tagged && !flds[i+1].tagged) {
			// Keep the field if the field name is unique, or if the first field
			// is at a less nested level, or if the first field is tagged and
			// the second field is not.
			visibleFields = append(visibleFields, flds[i])
		}
	}

	sort.Sort(&indexFieldSorter{visibleFields})

	return visibleFields, structOptions
}

func getFieldNameAndOptionsFromTag(tag string) (name string, omitEmpty bool, keyAsInt bool) {
	if tag == "" {
		return
	}
	idx := strings.Index(tag, ",")
	if idx == -1 {
		return tag, false, false
	}
	if idx > 0 {
		name = tag[:idx]
		tag = tag[idx:]
	}
	s := ",omitempty"
	if idx = strings.Index(tag, s); idx >= 0 && (len(tag) == idx+len(s) || tag[idx+len(s)] == ',') {
		omitEmpty = true
	}
	s = ",keyasint"
	if idx = strings.Index(tag, s); idx >= 0 && (len(tag) == idx+len(s) || tag[idx+len(s)] == ',') {
		keyAsInt = true
	}
	return
}
