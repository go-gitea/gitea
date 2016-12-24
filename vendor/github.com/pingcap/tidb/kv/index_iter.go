// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kv

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/types"
)

var (
	_ Index         = (*kvIndex)(nil)
	_ IndexIterator = (*indexIter)(nil)
)

// IndexIterator is the interface for iterator of index data on KV store.
type IndexIterator interface {
	Next() (k []types.Datum, h int64, err error)
	Close()
}

// Index is the interface for index data on KV store.
type Index interface {
	// Create supports insert into statement.
	Create(rm RetrieverMutator, indexedValues []types.Datum, h int64) error
	// Delete supports delete from statement.
	Delete(m Mutator, indexedValues []types.Datum, h int64) error
	// Drop supports drop table, drop index statements.
	Drop(rm RetrieverMutator) error
	// Exist supports check index exists or not.
	Exist(rm RetrieverMutator, indexedValues []types.Datum, h int64) (bool, int64, error)
	// GenIndexKey generates an index key.
	GenIndexKey(indexedValues []types.Datum, h int64) (key []byte, distinct bool, err error)
	// Seek supports where clause.
	Seek(r Retriever, indexedValues []types.Datum) (iter IndexIterator, hit bool, err error)
	// SeekFirst supports aggregate min and ascend order by.
	SeekFirst(r Retriever) (iter IndexIterator, err error)
}

func encodeHandle(h int64) []byte {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.BigEndian, h)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func decodeHandle(data []byte) (int64, error) {
	var h int64
	buf := bytes.NewBuffer(data)
	err := binary.Read(buf, binary.BigEndian, &h)
	return h, errors.Trace(err)
}

// indexIter is for KV store index iterator.
type indexIter struct {
	it     Iterator
	idx    *kvIndex
	prefix Key
}

// Close does the clean up works when KV store index iterator is closed.
func (c *indexIter) Close() {
	if c.it != nil {
		c.it.Close()
		c.it = nil
	}
}

// Next returns current key and moves iterator to the next step.
func (c *indexIter) Next() (val []types.Datum, h int64, err error) {
	if !c.it.Valid() {
		return nil, 0, errors.Trace(io.EOF)
	}
	if !c.it.Key().HasPrefix(c.prefix) {
		return nil, 0, errors.Trace(io.EOF)
	}
	// get indexedValues
	buf := c.it.Key()[len(c.prefix):]
	vv, err := codec.Decode(buf)
	if err != nil {
		return nil, 0, errors.Trace(err)
	}
	// if index is *not* unique, the handle is in keybuf
	if !c.idx.unique {
		h = vv[len(vv)-1].GetInt64()
		val = vv[0 : len(vv)-1]
	} else {
		// otherwise handle is value
		h, err = decodeHandle(c.it.Value())
		if err != nil {
			return nil, 0, errors.Trace(err)
		}
		val = vv
	}
	// update new iter to next
	err = c.it.Next()
	if err != nil {
		return nil, 0, errors.Trace(err)
	}
	return
}

// kvIndex is the data structure for index data in the KV store.
type kvIndex struct {
	indexName string
	indexID   int64
	unique    bool
	prefix    Key
}

// GenIndexPrefix generates the index prefix.
func GenIndexPrefix(indexPrefix Key, indexID int64) Key {
	buf := make([]byte, 0, len(indexPrefix)+8)
	buf = append(buf, indexPrefix...)
	buf = codec.EncodeInt(buf, indexID)
	return buf
}

// NewKVIndex builds a new kvIndex object.
func NewKVIndex(indexPrefix Key, indexName string, indexID int64, unique bool) Index {
	index := &kvIndex{
		indexName: indexName,
		indexID:   indexID,
		unique:    unique,
		prefix:    GenIndexPrefix(indexPrefix, indexID),
	}

	return index
}

// GenIndexKey generates storage key for index values. Returned distinct indicates whether the
// indexed values should be distinct in storage (i.e. whether handle is encoded in the key).
func (c *kvIndex) GenIndexKey(indexedValues []types.Datum, h int64) (key []byte, distinct bool, err error) {
	if c.unique {
		// See: https://dev.mysql.com/doc/refman/5.7/en/create-index.html
		// A UNIQUE index creates a constraint such that all values in the index must be distinct.
		// An error occurs if you try to add a new row with a key value that matches an existing row.
		// For all engines, a UNIQUE index permits multiple NULL values for columns that can contain NULL.
		distinct = true
		for _, cv := range indexedValues {
			if cv.Kind() == types.KindNull {
				distinct = false
				break
			}
		}
	}

	key = append(key, c.prefix...)
	if distinct {
		key, err = codec.EncodeKey(key, indexedValues...)
	} else {
		key, err = codec.EncodeKey(key, append(indexedValues, types.NewDatum(h))...)
	}
	if err != nil {
		return nil, false, errors.Trace(err)
	}
	return
}

// Create creates a new entry in the kvIndex data.
// If the index is unique and there is an existing entry with the same key, Create will return ErrKeyExists.
func (c *kvIndex) Create(rm RetrieverMutator, indexedValues []types.Datum, h int64) error {
	key, distinct, err := c.GenIndexKey(indexedValues, h)
	if err != nil {
		return errors.Trace(err)
	}
	if !distinct {
		// TODO: reconsider value
		err = rm.Set(key, []byte("timestamp?"))
		return errors.Trace(err)
	}

	_, err = rm.Get(key)
	if IsErrNotFound(err) {
		err = rm.Set(key, encodeHandle(h))
		return errors.Trace(err)
	}

	return errors.Trace(ErrKeyExists)
}

// Delete removes the entry for handle h and indexdValues from KV index.
func (c *kvIndex) Delete(m Mutator, indexedValues []types.Datum, h int64) error {
	key, _, err := c.GenIndexKey(indexedValues, h)
	if err != nil {
		return errors.Trace(err)
	}
	err = m.Delete(key)
	return errors.Trace(err)
}

// Drop removes the KV index from store.
func (c *kvIndex) Drop(rm RetrieverMutator) error {
	it, err := rm.Seek(c.prefix)
	if err != nil {
		return errors.Trace(err)
	}
	defer it.Close()

	// remove all indices
	for it.Valid() {
		if !it.Key().HasPrefix(c.prefix) {
			break
		}
		err := rm.Delete(it.Key())
		if err != nil {
			return errors.Trace(err)
		}
		err = it.Next()
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Seek searches KV index for the entry with indexedValues.
func (c *kvIndex) Seek(r Retriever, indexedValues []types.Datum) (iter IndexIterator, hit bool, err error) {
	key, _, err := c.GenIndexKey(indexedValues, 0)
	if err != nil {
		return nil, false, errors.Trace(err)
	}
	it, err := r.Seek(key)
	if err != nil {
		return nil, false, errors.Trace(err)
	}
	// check if hit
	hit = false
	if it.Valid() && it.Key().Cmp(key) == 0 {
		hit = true
	}
	return &indexIter{it: it, idx: c, prefix: c.prefix}, hit, nil
}

// SeekFirst returns an iterator which points to the first entry of the KV index.
func (c *kvIndex) SeekFirst(r Retriever) (iter IndexIterator, err error) {
	it, err := r.Seek(c.prefix)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &indexIter{it: it, idx: c, prefix: c.prefix}, nil
}

func (c *kvIndex) Exist(rm RetrieverMutator, indexedValues []types.Datum, h int64) (bool, int64, error) {
	key, distinct, err := c.GenIndexKey(indexedValues, h)
	if err != nil {
		return false, 0, errors.Trace(err)
	}

	value, err := rm.Get(key)
	if IsErrNotFound(err) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, errors.Trace(err)
	}

	// For distinct index, the value of key is handle.
	if distinct {
		handle, err := decodeHandle(value)
		if err != nil {
			return false, 0, errors.Trace(err)
		}

		if handle != h {
			return true, handle, errors.Trace(ErrKeyExists)
		}

		return true, handle, nil
	}

	return true, h, nil
}
