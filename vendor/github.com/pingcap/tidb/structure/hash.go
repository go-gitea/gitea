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

package structure

import (
	"bytes"
	"encoding/binary"
	"strconv"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/terror"
)

// HashPair is the pair for (field, value) in a hash.
type HashPair struct {
	Field []byte
	Value []byte
}

type hashMeta struct {
	FieldCount int64
}

func (meta hashMeta) Value() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf[0:8], uint64(meta.FieldCount))
	return buf
}

func (meta hashMeta) IsEmpty() bool {
	return meta.FieldCount <= 0
}

// HSet sets the string value of a hash field.
func (t *TxStructure) HSet(key []byte, field []byte, value []byte) error {
	return t.updateHash(key, field, func([]byte) ([]byte, error) {
		return value, nil
	})
}

// HGet gets the value of a hash field.
func (t *TxStructure) HGet(key []byte, field []byte) ([]byte, error) {
	dataKey := t.encodeHashDataKey(key, field)
	value, err := t.txn.Get(dataKey)
	if terror.ErrorEqual(err, kv.ErrNotExist) {
		err = nil
	}
	return value, errors.Trace(err)
}

// HInc increments the integer value of a hash field, by step, returns
// the value after the increment.
func (t *TxStructure) HInc(key []byte, field []byte, step int64) (int64, error) {
	base := int64(0)
	err := t.updateHash(key, field, func(oldValue []byte) ([]byte, error) {
		if oldValue != nil {
			var err error
			base, err = strconv.ParseInt(string(oldValue), 10, 64)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		base += step
		return []byte(strconv.FormatInt(base, 10)), nil
	})

	return base, errors.Trace(err)
}

// HGetInt64 gets int64 value of a hash field.
func (t *TxStructure) HGetInt64(key []byte, field []byte) (int64, error) {
	value, err := t.HGet(key, field)
	if err != nil || value == nil {
		return 0, errors.Trace(err)
	}

	var n int64
	n, err = strconv.ParseInt(string(value), 10, 64)
	return n, errors.Trace(err)
}

func (t *TxStructure) updateHash(key []byte, field []byte, fn func(oldValue []byte) ([]byte, error)) error {
	dataKey := t.encodeHashDataKey(key, field)
	oldValue, err := t.loadHashValue(dataKey)
	if err != nil {
		return errors.Trace(err)
	}

	newValue, err := fn(oldValue)
	if err != nil {
		return errors.Trace(err)
	}

	// Check if new value is equal to old value.
	if bytes.Equal(oldValue, newValue) {
		return nil
	}

	if err = t.txn.Set(dataKey, newValue); err != nil {
		return errors.Trace(err)
	}

	metaKey := t.encodeHashMetaKey(key)
	meta, err := t.loadHashMeta(metaKey)
	if err != nil {
		return errors.Trace(err)
	}

	if oldValue == nil {
		meta.FieldCount++
		if err = t.txn.Set(metaKey, meta.Value()); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

// HLen gets the number of fields in a hash.
func (t *TxStructure) HLen(key []byte) (int64, error) {
	metaKey := t.encodeHashMetaKey(key)
	meta, err := t.loadHashMeta(metaKey)
	if err != nil {
		return 0, errors.Trace(err)
	}
	return meta.FieldCount, nil
}

// HDel deletes one or more hash fields.
func (t *TxStructure) HDel(key []byte, fields ...[]byte) error {
	metaKey := t.encodeHashMetaKey(key)
	meta, err := t.loadHashMeta(metaKey)
	if err != nil || meta.IsEmpty() {
		return errors.Trace(err)
	}

	var value []byte
	for _, field := range fields {
		dataKey := t.encodeHashDataKey(key, field)

		value, err = t.loadHashValue(dataKey)
		if err != nil {
			return errors.Trace(err)
		}

		if value != nil {
			if err = t.txn.Delete(dataKey); err != nil {
				return errors.Trace(err)
			}

			meta.FieldCount--
		}
	}

	if meta.IsEmpty() {
		err = t.txn.Delete(metaKey)
	} else {
		err = t.txn.Set(metaKey, meta.Value())
	}

	return errors.Trace(err)
}

// HKeys gets all the fields in a hash.
func (t *TxStructure) HKeys(key []byte) ([][]byte, error) {
	var keys [][]byte
	err := t.iterateHash(key, func(field []byte, value []byte) error {
		keys = append(keys, append([]byte{}, field...))
		return nil
	})

	return keys, errors.Trace(err)
}

// HGetAll gets all the fields and values in a hash.
func (t *TxStructure) HGetAll(key []byte) ([]HashPair, error) {
	var res []HashPair
	err := t.iterateHash(key, func(field []byte, value []byte) error {
		pair := HashPair{
			Field: append([]byte{}, field...),
			Value: append([]byte{}, value...),
		}
		res = append(res, pair)
		return nil
	})

	return res, errors.Trace(err)
}

// HClear removes the hash value of the key.
func (t *TxStructure) HClear(key []byte) error {
	metaKey := t.encodeHashMetaKey(key)
	meta, err := t.loadHashMeta(metaKey)
	if err != nil || meta.IsEmpty() {
		return errors.Trace(err)
	}

	err = t.iterateHash(key, func(field []byte, value []byte) error {
		k := t.encodeHashDataKey(key, field)
		return errors.Trace(t.txn.Delete(k))
	})

	if err != nil {
		return errors.Trace(err)
	}

	return errors.Trace(t.txn.Delete(metaKey))
}

func (t *TxStructure) iterateHash(key []byte, fn func(k []byte, v []byte) error) error {
	dataPrefix := t.hashDataKeyPrefix(key)
	it, err := t.txn.Seek(dataPrefix)
	if err != nil {
		return errors.Trace(err)
	}

	var field []byte

	for it.Valid() {
		if !it.Key().HasPrefix(dataPrefix) {
			break
		}

		_, field, err = t.decodeHashDataKey(it.Key())
		if err != nil {
			return errors.Trace(err)
		}

		if err = fn(field, it.Value()); err != nil {
			return errors.Trace(err)
		}

		err = it.Next()
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func (t *TxStructure) loadHashMeta(metaKey []byte) (hashMeta, error) {
	v, err := t.txn.Get(metaKey)
	if terror.ErrorEqual(err, kv.ErrNotExist) {
		err = nil
	} else if err != nil {
		return hashMeta{}, errors.Trace(err)
	}

	meta := hashMeta{FieldCount: 0}
	if v == nil {
		return meta, nil
	}

	if len(v) != 8 {
		return meta, errors.New("invalid list meta data")
	}

	meta.FieldCount = int64(binary.BigEndian.Uint64(v[0:8]))
	return meta, nil
}

func (t *TxStructure) loadHashValue(dataKey []byte) ([]byte, error) {
	v, err := t.txn.Get(dataKey)
	if terror.ErrorEqual(err, kv.ErrNotExist) {
		err = nil
		v = nil
	} else if err != nil {
		return nil, errors.Trace(err)
	}

	return v, nil
}
