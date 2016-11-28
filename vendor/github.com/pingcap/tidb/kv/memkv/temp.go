// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

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

package memkv

import (
	"github.com/pingcap/tidb/util/types"
)

type btreeIterator interface {
	Next() (k, v []interface{}, err error)
}

// Temp is the interface of a memory kv storage
type Temp interface {
	Drop() (err error)
	Get(k []interface{}) (v []interface{}, err error)
	SeekFirst() (e btreeIterator, err error)
	Set(k, v []interface{}) (err error)
}

// memtemp for join/groupby or any aggregation operation
type memTemp struct {
	// memory btree
	tree *Tree
}

// CreateTemp returns a new empty memory kv
func CreateTemp(asc bool) (_ Temp, err error) {
	return &memTemp{
		tree: NewTree(types.Collators[asc]),
	}, nil
}

func (t *memTemp) Get(k []interface{}) (v []interface{}, err error) {
	v, _ = t.tree.Get(k)
	return
}

func (t *memTemp) Drop() (err error) { return }

func (t *memTemp) Set(k, v []interface{}) (err error) {
	vv, err := types.Clone(v)
	if err != nil {
		return err
	}
	t.tree.Set(append([]interface{}(nil), k...), vv.([]interface{}))
	return
}

func (t *memTemp) SeekFirst() (e btreeIterator, err error) {
	it, err := t.tree.SeekFirst()
	if err != nil {
		return
	}

	return it, nil
}
