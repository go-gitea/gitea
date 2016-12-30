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
	"strconv"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/terror"
)

// Set sets the string value of the key.
func (t *TxStructure) Set(key []byte, value []byte) error {
	ek := t.encodeStringDataKey(key)

	return t.txn.Set(ek, value)
}

// Get gets the string value of a key.
func (t *TxStructure) Get(key []byte) ([]byte, error) {
	ek := t.encodeStringDataKey(key)
	value, err := t.txn.Get(ek)
	if terror.ErrorEqual(err, kv.ErrNotExist) {
		err = nil
	}
	return value, errors.Trace(err)
}

// GetInt64 gets the int64 value of a key.
func (t *TxStructure) GetInt64(key []byte) (int64, error) {
	v, err := t.Get(key)
	if err != nil || v == nil {
		return 0, errors.Trace(err)
	}

	n, err := strconv.ParseInt(string(v), 10, 64)
	return n, errors.Trace(err)
}

// Inc increments the integer value of a key by step, returns
// the value after the increment.
func (t *TxStructure) Inc(key []byte, step int64) (int64, error) {
	ek := t.encodeStringDataKey(key)
	// txn Inc will lock this key, so we don't lock it here.
	n, err := kv.IncInt64(t.txn, ek, step)
	if terror.ErrorEqual(err, kv.ErrNotExist) {
		err = nil
	}
	return n, errors.Trace(err)
}

// Clear removes the string value of the key.
func (t *TxStructure) Clear(key []byte) error {
	ek := t.encodeStringDataKey(key)
	err := t.txn.Delete(ek)
	if terror.ErrorEqual(err, kv.ErrNotExist) {
		err = nil
	}
	return errors.Trace(err)
}
