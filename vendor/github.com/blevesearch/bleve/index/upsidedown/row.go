//  Copyright (c) 2014 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package upsidedown

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/golang/protobuf/proto"
)

const ByteSeparator byte = 0xff

type UpsideDownCouchRowStream chan UpsideDownCouchRow

type UpsideDownCouchRow interface {
	KeySize() int
	KeyTo([]byte) (int, error)
	Key() []byte
	Value() []byte
	ValueSize() int
	ValueTo([]byte) (int, error)
}

func ParseFromKeyValue(key, value []byte) (UpsideDownCouchRow, error) {
	if len(key) > 0 {
		switch key[0] {
		case 'v':
			return NewVersionRowKV(key, value)
		case 'f':
			return NewFieldRowKV(key, value)
		case 'd':
			return NewDictionaryRowKV(key, value)
		case 't':
			return NewTermFrequencyRowKV(key, value)
		case 'b':
			return NewBackIndexRowKV(key, value)
		case 's':
			return NewStoredRowKV(key, value)
		case 'i':
			return NewInternalRowKV(key, value)
		}
		return nil, fmt.Errorf("Unknown field type '%s'", string(key[0]))
	}
	return nil, fmt.Errorf("Invalid empty key")
}

// VERSION

type VersionRow struct {
	version uint8
}

func (v *VersionRow) Key() []byte {
	return []byte{'v'}
}

func (v *VersionRow) KeySize() int {
	return 1
}

func (v *VersionRow) KeyTo(buf []byte) (int, error) {
	buf[0] = 'v'
	return 1, nil
}

func (v *VersionRow) Value() []byte {
	return []byte{byte(v.version)}
}

func (v *VersionRow) ValueSize() int {
	return 1
}

func (v *VersionRow) ValueTo(buf []byte) (int, error) {
	buf[0] = v.version
	return 1, nil
}

func (v *VersionRow) String() string {
	return fmt.Sprintf("Version: %d", v.version)
}

func NewVersionRow(version uint8) *VersionRow {
	return &VersionRow{
		version: version,
	}
}

func NewVersionRowKV(key, value []byte) (*VersionRow, error) {
	rv := VersionRow{}
	buf := bytes.NewBuffer(value)
	err := binary.Read(buf, binary.LittleEndian, &rv.version)
	if err != nil {
		return nil, err
	}
	return &rv, nil
}

// INTERNAL STORAGE

type InternalRow struct {
	key []byte
	val []byte
}

func (i *InternalRow) Key() []byte {
	buf := make([]byte, i.KeySize())
	size, _ := i.KeyTo(buf)
	return buf[:size]
}

func (i *InternalRow) KeySize() int {
	return len(i.key) + 1
}

func (i *InternalRow) KeyTo(buf []byte) (int, error) {
	buf[0] = 'i'
	actual := copy(buf[1:], i.key)
	return 1 + actual, nil
}

func (i *InternalRow) Value() []byte {
	return i.val
}

func (i *InternalRow) ValueSize() int {
	return len(i.val)
}

func (i *InternalRow) ValueTo(buf []byte) (int, error) {
	actual := copy(buf, i.val)
	return actual, nil
}

func (i *InternalRow) String() string {
	return fmt.Sprintf("InternalStore - Key: %s (% x) Val: %s (% x)", i.key, i.key, i.val, i.val)
}

func NewInternalRow(key, val []byte) *InternalRow {
	return &InternalRow{
		key: key,
		val: val,
	}
}

func NewInternalRowKV(key, value []byte) (*InternalRow, error) {
	rv := InternalRow{}
	rv.key = key[1:]
	rv.val = value
	return &rv, nil
}

// FIELD definition

type FieldRow struct {
	index uint16
	name  string
}

func (f *FieldRow) Key() []byte {
	buf := make([]byte, f.KeySize())
	size, _ := f.KeyTo(buf)
	return buf[:size]
}

func (f *FieldRow) KeySize() int {
	return 3
}

func (f *FieldRow) KeyTo(buf []byte) (int, error) {
	buf[0] = 'f'
	binary.LittleEndian.PutUint16(buf[1:3], f.index)
	return 3, nil
}

func (f *FieldRow) Value() []byte {
	return append([]byte(f.name), ByteSeparator)
}

func (f *FieldRow) ValueSize() int {
	return len(f.name) + 1
}

func (f *FieldRow) ValueTo(buf []byte) (int, error) {
	size := copy(buf, f.name)
	buf[size] = ByteSeparator
	return size + 1, nil
}

func (f *FieldRow) String() string {
	return fmt.Sprintf("Field: %d Name: %s", f.index, f.name)
}

func NewFieldRow(index uint16, name string) *FieldRow {
	return &FieldRow{
		index: index,
		name:  name,
	}
}

func NewFieldRowKV(key, value []byte) (*FieldRow, error) {
	rv := FieldRow{}

	buf := bytes.NewBuffer(key)
	_, err := buf.ReadByte() // type
	if err != nil {
		return nil, err
	}
	err = binary.Read(buf, binary.LittleEndian, &rv.index)
	if err != nil {
		return nil, err
	}

	buf = bytes.NewBuffer(value)
	rv.name, err = buf.ReadString(ByteSeparator)
	if err != nil {
		return nil, err
	}
	rv.name = rv.name[:len(rv.name)-1] // trim off separator byte

	return &rv, nil
}

// DICTIONARY

const DictionaryRowMaxValueSize = binary.MaxVarintLen64

type DictionaryRow struct {
	term  []byte
	count uint64
	field uint16
}

func (dr *DictionaryRow) Key() []byte {
	buf := make([]byte, dr.KeySize())
	size, _ := dr.KeyTo(buf)
	return buf[:size]
}

func (dr *DictionaryRow) KeySize() int {
	return len(dr.term) + 3
}

func (dr *DictionaryRow) KeyTo(buf []byte) (int, error) {
	buf[0] = 'd'
	binary.LittleEndian.PutUint16(buf[1:3], dr.field)
	size := copy(buf[3:], dr.term)
	return size + 3, nil
}

func (dr *DictionaryRow) Value() []byte {
	buf := make([]byte, dr.ValueSize())
	size, _ := dr.ValueTo(buf)
	return buf[:size]
}

func (dr *DictionaryRow) ValueSize() int {
	return DictionaryRowMaxValueSize
}

func (dr *DictionaryRow) ValueTo(buf []byte) (int, error) {
	used := binary.PutUvarint(buf, dr.count)
	return used, nil
}

func (dr *DictionaryRow) String() string {
	return fmt.Sprintf("Dictionary Term: `%s` Field: %d Count: %d ", string(dr.term), dr.field, dr.count)
}

func NewDictionaryRow(term []byte, field uint16, count uint64) *DictionaryRow {
	return &DictionaryRow{
		term:  term,
		field: field,
		count: count,
	}
}

func NewDictionaryRowKV(key, value []byte) (*DictionaryRow, error) {
	rv, err := NewDictionaryRowK(key)
	if err != nil {
		return nil, err
	}

	err = rv.parseDictionaryV(value)
	if err != nil {
		return nil, err
	}
	return rv, nil

}

func NewDictionaryRowK(key []byte) (*DictionaryRow, error) {
	rv := &DictionaryRow{}
	err := rv.parseDictionaryK(key)
	if err != nil {
		return nil, err
	}
	return rv, nil
}

func (dr *DictionaryRow) parseDictionaryK(key []byte) error {
	dr.field = binary.LittleEndian.Uint16(key[1:3])
	if dr.term != nil {
		dr.term = dr.term[:0]
	}
	dr.term = append(dr.term, key[3:]...)
	return nil
}

func (dr *DictionaryRow) parseDictionaryV(value []byte) error {
	count, nread := binary.Uvarint(value)
	if nread <= 0 {
		return fmt.Errorf("DictionaryRow parse Uvarint error, nread: %d", nread)
	}
	dr.count = count
	return nil
}

// TERM FIELD FREQUENCY

type TermVector struct {
	field          uint16
	arrayPositions []uint64
	pos            uint64
	start          uint64
	end            uint64
}

func (tv *TermVector) String() string {
	return fmt.Sprintf("Field: %d Pos: %d Start: %d End %d ArrayPositions: %#v", tv.field, tv.pos, tv.start, tv.end, tv.arrayPositions)
}

type TermFrequencyRow struct {
	term    []byte
	doc     []byte
	freq    uint64
	vectors []*TermVector
	norm    float32
	field   uint16
}

func (tfr *TermFrequencyRow) Term() []byte {
	return tfr.term
}

func (tfr *TermFrequencyRow) Freq() uint64 {
	return tfr.freq
}

func (tfr *TermFrequencyRow) ScanPrefixForField() []byte {
	buf := make([]byte, 3)
	buf[0] = 't'
	binary.LittleEndian.PutUint16(buf[1:3], tfr.field)
	return buf
}

func (tfr *TermFrequencyRow) ScanPrefixForFieldTermPrefix() []byte {
	buf := make([]byte, 3+len(tfr.term))
	buf[0] = 't'
	binary.LittleEndian.PutUint16(buf[1:3], tfr.field)
	copy(buf[3:], tfr.term)
	return buf
}

func (tfr *TermFrequencyRow) ScanPrefixForFieldTerm() []byte {
	buf := make([]byte, 3+len(tfr.term)+1)
	buf[0] = 't'
	binary.LittleEndian.PutUint16(buf[1:3], tfr.field)
	termLen := copy(buf[3:], tfr.term)
	buf[3+termLen] = ByteSeparator
	return buf
}

func (tfr *TermFrequencyRow) Key() []byte {
	buf := make([]byte, tfr.KeySize())
	size, _ := tfr.KeyTo(buf)
	return buf[:size]
}

func (tfr *TermFrequencyRow) KeySize() int {
	return 3 + len(tfr.term) + 1 + len(tfr.doc)
}

func (tfr *TermFrequencyRow) KeyTo(buf []byte) (int, error) {
	buf[0] = 't'
	binary.LittleEndian.PutUint16(buf[1:3], tfr.field)
	termLen := copy(buf[3:], tfr.term)
	buf[3+termLen] = ByteSeparator
	docLen := copy(buf[3+termLen+1:], tfr.doc)
	return 3 + termLen + 1 + docLen, nil
}

func (tfr *TermFrequencyRow) KeyAppendTo(buf []byte) ([]byte, error) {
	keySize := tfr.KeySize()
	if cap(buf) < keySize {
		buf = make([]byte, keySize)
	}
	actualSize, err := tfr.KeyTo(buf[0:keySize])
	return buf[0:actualSize], err
}

func (tfr *TermFrequencyRow) DictionaryRowKey() []byte {
	dr := NewDictionaryRow(tfr.term, tfr.field, 0)
	return dr.Key()
}

func (tfr *TermFrequencyRow) DictionaryRowKeySize() int {
	dr := NewDictionaryRow(tfr.term, tfr.field, 0)
	return dr.KeySize()
}

func (tfr *TermFrequencyRow) DictionaryRowKeyTo(buf []byte) (int, error) {
	dr := NewDictionaryRow(tfr.term, tfr.field, 0)
	return dr.KeyTo(buf)
}

func (tfr *TermFrequencyRow) Value() []byte {
	buf := make([]byte, tfr.ValueSize())
	size, _ := tfr.ValueTo(buf)
	return buf[:size]
}

func (tfr *TermFrequencyRow) ValueSize() int {
	bufLen := binary.MaxVarintLen64 + binary.MaxVarintLen64
	for _, vector := range tfr.vectors {
		bufLen += (binary.MaxVarintLen64 * 4) + (1+len(vector.arrayPositions))*binary.MaxVarintLen64
	}
	return bufLen
}

func (tfr *TermFrequencyRow) ValueTo(buf []byte) (int, error) {
	used := binary.PutUvarint(buf[:binary.MaxVarintLen64], tfr.freq)

	normuint32 := math.Float32bits(tfr.norm)
	newbuf := buf[used : used+binary.MaxVarintLen64]
	used += binary.PutUvarint(newbuf, uint64(normuint32))

	for _, vector := range tfr.vectors {
		used += binary.PutUvarint(buf[used:used+binary.MaxVarintLen64], uint64(vector.field))
		used += binary.PutUvarint(buf[used:used+binary.MaxVarintLen64], vector.pos)
		used += binary.PutUvarint(buf[used:used+binary.MaxVarintLen64], vector.start)
		used += binary.PutUvarint(buf[used:used+binary.MaxVarintLen64], vector.end)
		used += binary.PutUvarint(buf[used:used+binary.MaxVarintLen64], uint64(len(vector.arrayPositions)))
		for _, arrayPosition := range vector.arrayPositions {
			used += binary.PutUvarint(buf[used:used+binary.MaxVarintLen64], arrayPosition)
		}
	}
	return used, nil
}

func (tfr *TermFrequencyRow) String() string {
	return fmt.Sprintf("Term: `%s` Field: %d DocId: `%s` Frequency: %d Norm: %f Vectors: %v", string(tfr.term), tfr.field, string(tfr.doc), tfr.freq, tfr.norm, tfr.vectors)
}

func InitTermFrequencyRow(tfr *TermFrequencyRow, term []byte, field uint16, docID []byte, freq uint64, norm float32) *TermFrequencyRow {
	tfr.term = term
	tfr.field = field
	tfr.doc = docID
	tfr.freq = freq
	tfr.norm = norm
	return tfr
}

func NewTermFrequencyRow(term []byte, field uint16, docID []byte, freq uint64, norm float32) *TermFrequencyRow {
	return &TermFrequencyRow{
		term:  term,
		field: field,
		doc:   docID,
		freq:  freq,
		norm:  norm,
	}
}

func NewTermFrequencyRowWithTermVectors(term []byte, field uint16, docID []byte, freq uint64, norm float32, vectors []*TermVector) *TermFrequencyRow {
	return &TermFrequencyRow{
		term:    term,
		field:   field,
		doc:     docID,
		freq:    freq,
		norm:    norm,
		vectors: vectors,
	}
}

func NewTermFrequencyRowK(key []byte) (*TermFrequencyRow, error) {
	rv := &TermFrequencyRow{}
	err := rv.parseK(key)
	if err != nil {
		return nil, err
	}
	return rv, nil
}

func (tfr *TermFrequencyRow) parseK(key []byte) error {
	keyLen := len(key)
	if keyLen < 3 {
		return fmt.Errorf("invalid term frequency key, no valid field")
	}
	tfr.field = binary.LittleEndian.Uint16(key[1:3])

	termEndPos := bytes.IndexByte(key[3:], ByteSeparator)
	if termEndPos < 0 {
		return fmt.Errorf("invalid term frequency key, no byte separator terminating term")
	}
	tfr.term = key[3 : 3+termEndPos]

	docLen := keyLen - (3 + termEndPos + 1)
	if docLen < 1 {
		return fmt.Errorf("invalid term frequency key, empty docid")
	}
	tfr.doc = key[3+termEndPos+1:]

	return nil
}

func (tfr *TermFrequencyRow) parseKDoc(key []byte, term []byte) error {
	tfr.doc = key[3+len(term)+1:]
	if len(tfr.doc) <= 0 {
		return fmt.Errorf("invalid term frequency key, empty docid")
	}

	return nil
}

func (tfr *TermFrequencyRow) parseV(value []byte) error {
	var bytesRead int
	tfr.freq, bytesRead = binary.Uvarint(value)
	if bytesRead <= 0 {
		return fmt.Errorf("invalid term frequency value, invalid frequency")
	}
	currOffset := bytesRead

	var norm uint64
	norm, bytesRead = binary.Uvarint(value[currOffset:])
	if bytesRead <= 0 {
		return fmt.Errorf("invalid term frequency value, no norm")
	}
	currOffset += bytesRead

	tfr.norm = math.Float32frombits(uint32(norm))

	tfr.vectors = nil
	var field uint64
	field, bytesRead = binary.Uvarint(value[currOffset:])
	for bytesRead > 0 {
		currOffset += bytesRead
		tv := TermVector{}
		tv.field = uint16(field)
		// at this point we expect at least one term vector
		if tfr.vectors == nil {
			tfr.vectors = make([]*TermVector, 0)
		}

		tv.pos, bytesRead = binary.Uvarint(value[currOffset:])
		if bytesRead <= 0 {
			return fmt.Errorf("invalid term frequency value, vector contains no position")
		}
		currOffset += bytesRead

		tv.start, bytesRead = binary.Uvarint(value[currOffset:])
		if bytesRead <= 0 {
			return fmt.Errorf("invalid term frequency value, vector contains no start")
		}
		currOffset += bytesRead

		tv.end, bytesRead = binary.Uvarint(value[currOffset:])
		if bytesRead <= 0 {
			return fmt.Errorf("invalid term frequency value, vector contains no end")
		}
		currOffset += bytesRead

		var arrayPositionsLen uint64 = 0
		arrayPositionsLen, bytesRead = binary.Uvarint(value[currOffset:])
		if bytesRead <= 0 {
			return fmt.Errorf("invalid term frequency value, vector contains no arrayPositionLen")
		}
		currOffset += bytesRead

		if arrayPositionsLen > 0 {
			tv.arrayPositions = make([]uint64, arrayPositionsLen)
			for i := 0; uint64(i) < arrayPositionsLen; i++ {
				tv.arrayPositions[i], bytesRead = binary.Uvarint(value[currOffset:])
				if bytesRead <= 0 {
					return fmt.Errorf("invalid term frequency value, vector contains no arrayPosition of index %d", i)
				}
				currOffset += bytesRead
			}
		}

		tfr.vectors = append(tfr.vectors, &tv)
		// try to read next record (may not exist)
		field, bytesRead = binary.Uvarint(value[currOffset:])
	}
	if len(value[currOffset:]) > 0 && bytesRead <= 0 {
		return fmt.Errorf("invalid term frequency value, vector field invalid")
	}

	return nil
}

func NewTermFrequencyRowKV(key, value []byte) (*TermFrequencyRow, error) {
	rv, err := NewTermFrequencyRowK(key)
	if err != nil {
		return nil, err
	}

	err = rv.parseV(value)
	if err != nil {
		return nil, err
	}
	return rv, nil

}

type BackIndexRow struct {
	doc           []byte
	termEntries   []*BackIndexTermEntry
	storedEntries []*BackIndexStoreEntry
}

func (br *BackIndexRow) AllTermKeys() [][]byte {
	if br == nil {
		return nil
	}
	rv := make([][]byte, len(br.termEntries))
	for i, termEntry := range br.termEntries {
		termRow := NewTermFrequencyRow([]byte(termEntry.GetTerm()), uint16(termEntry.GetField()), br.doc, 0, 0)
		rv[i] = termRow.Key()
	}
	return rv
}

func (br *BackIndexRow) AllStoredKeys() [][]byte {
	if br == nil {
		return nil
	}
	rv := make([][]byte, len(br.storedEntries))
	for i, storedEntry := range br.storedEntries {
		storedRow := NewStoredRow(br.doc, uint16(storedEntry.GetField()), storedEntry.GetArrayPositions(), 'x', []byte{})
		rv[i] = storedRow.Key()
	}
	return rv
}

func (br *BackIndexRow) Key() []byte {
	buf := make([]byte, br.KeySize())
	size, _ := br.KeyTo(buf)
	return buf[:size]
}

func (br *BackIndexRow) KeySize() int {
	return len(br.doc) + 1
}

func (br *BackIndexRow) KeyTo(buf []byte) (int, error) {
	buf[0] = 'b'
	used := copy(buf[1:], br.doc)
	return used + 1, nil
}

func (br *BackIndexRow) Value() []byte {
	buf := make([]byte, br.ValueSize())
	size, _ := br.ValueTo(buf)
	return buf[:size]
}

func (br *BackIndexRow) ValueSize() int {
	birv := &BackIndexRowValue{
		TermEntries:   br.termEntries,
		StoredEntries: br.storedEntries,
	}
	return birv.Size()
}

func (br *BackIndexRow) ValueTo(buf []byte) (int, error) {
	birv := &BackIndexRowValue{
		TermEntries:   br.termEntries,
		StoredEntries: br.storedEntries,
	}
	return birv.MarshalTo(buf)
}

func (br *BackIndexRow) String() string {
	return fmt.Sprintf("Backindex DocId: `%s` Term Entries: %v, Stored Entries: %v", string(br.doc), br.termEntries, br.storedEntries)
}

func NewBackIndexRow(docID []byte, entries []*BackIndexTermEntry, storedFields []*BackIndexStoreEntry) *BackIndexRow {
	return &BackIndexRow{
		doc:           docID,
		termEntries:   entries,
		storedEntries: storedFields,
	}
}

func NewBackIndexRowKV(key, value []byte) (*BackIndexRow, error) {
	rv := BackIndexRow{}

	buf := bytes.NewBuffer(key)
	_, err := buf.ReadByte() // type
	if err != nil {
		return nil, err
	}

	rv.doc, err = buf.ReadBytes(ByteSeparator)
	if err == io.EOF && len(rv.doc) < 1 {
		err = fmt.Errorf("invalid doc length 0 - % x", key)
	}
	if err != nil && err != io.EOF {
		return nil, err
	} else if err == nil {
		rv.doc = rv.doc[:len(rv.doc)-1] // trim off separator byte
	}

	var birv BackIndexRowValue
	err = proto.Unmarshal(value, &birv)
	if err != nil {
		return nil, err
	}
	rv.termEntries = birv.TermEntries
	rv.storedEntries = birv.StoredEntries

	return &rv, nil
}

// STORED

type StoredRow struct {
	doc            []byte
	field          uint16
	arrayPositions []uint64
	typ            byte
	value          []byte
}

func (s *StoredRow) Key() []byte {
	buf := make([]byte, s.KeySize())
	size, _ := s.KeyTo(buf)
	return buf[0:size]
}

func (s *StoredRow) KeySize() int {
	return 1 + len(s.doc) + 1 + 2 + (binary.MaxVarintLen64 * len(s.arrayPositions))
}

func (s *StoredRow) KeyTo(buf []byte) (int, error) {
	docLen := len(s.doc)
	buf[0] = 's'
	copy(buf[1:], s.doc)
	buf[1+docLen] = ByteSeparator
	binary.LittleEndian.PutUint16(buf[1+docLen+1:], s.field)
	bytesUsed := 1 + docLen + 1 + 2
	for _, arrayPosition := range s.arrayPositions {
		varbytes := binary.PutUvarint(buf[bytesUsed:], arrayPosition)
		bytesUsed += varbytes
	}
	return bytesUsed, nil
}

func (s *StoredRow) Value() []byte {
	buf := make([]byte, s.ValueSize())
	size, _ := s.ValueTo(buf)
	return buf[:size]
}

func (s *StoredRow) ValueSize() int {
	return len(s.value) + 1
}

func (s *StoredRow) ValueTo(buf []byte) (int, error) {
	buf[0] = s.typ
	used := copy(buf[1:], s.value)
	return used + 1, nil
}

func (s *StoredRow) String() string {
	return fmt.Sprintf("Document: %s Field %d, Array Positions: %v, Type: %s Value: %s", s.doc, s.field, s.arrayPositions, string(s.typ), s.value)
}

func (s *StoredRow) ScanPrefixForDoc() []byte {
	docLen := len(s.doc)
	buf := make([]byte, 1+docLen+1)
	buf[0] = 's'
	copy(buf[1:], s.doc)
	buf[1+docLen] = ByteSeparator
	return buf
}

func NewStoredRow(docID []byte, field uint16, arrayPositions []uint64, typ byte, value []byte) *StoredRow {
	return &StoredRow{
		doc:            docID,
		field:          field,
		arrayPositions: arrayPositions,
		typ:            typ,
		value:          value,
	}
}

func NewStoredRowK(key []byte) (*StoredRow, error) {
	rv := StoredRow{}

	buf := bytes.NewBuffer(key)
	_, err := buf.ReadByte() // type
	if err != nil {
		return nil, err
	}

	rv.doc, err = buf.ReadBytes(ByteSeparator)
	if len(rv.doc) < 2 { // 1 for min doc id length, 1 for separator
		err = fmt.Errorf("invalid doc length 0")
		return nil, err
	}

	rv.doc = rv.doc[:len(rv.doc)-1] // trim off separator byte

	err = binary.Read(buf, binary.LittleEndian, &rv.field)
	if err != nil {
		return nil, err
	}

	rv.arrayPositions = make([]uint64, 0)
	nextArrayPos, err := binary.ReadUvarint(buf)
	for err == nil {
		rv.arrayPositions = append(rv.arrayPositions, nextArrayPos)
		nextArrayPos, err = binary.ReadUvarint(buf)
	}
	return &rv, nil
}

func NewStoredRowKV(key, value []byte) (*StoredRow, error) {
	rv, err := NewStoredRowK(key)
	if err != nil {
		return nil, err
	}
	rv.typ = value[0]
	rv.value = value[1:]
	return rv, nil
}
