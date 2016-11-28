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

package distinct

import (
	"github.com/juju/errors"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/types"
)

// CreateDistinctChecker creates a new distinct checker.
func CreateDistinctChecker() *Checker {
	return &Checker{
		existingKeys: make(map[string]bool),
	}
}

// Checker stores existing keys and checks if given data is distinct.
type Checker struct {
	existingKeys map[string]bool
}

// Check checks if values is distinct.
func (d *Checker) Check(values []interface{}) (bool, error) {
	bs, err := codec.EncodeValue([]byte{}, types.MakeDatums(values...)...)
	if err != nil {
		return false, errors.Trace(err)
	}
	key := string(bs)
	_, ok := d.existingKeys[key]
	if ok {
		return false, nil
	}
	d.existingKeys[key] = true
	return true, nil
}
