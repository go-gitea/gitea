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
	"strconv"

	"github.com/juju/errors"
)

// IncInt64 increases the value for key k in kv store by step.
func IncInt64(rm RetrieverMutator, k Key, step int64) (int64, error) {
	val, err := rm.Get(k)
	if IsErrNotFound(err) {
		err = rm.Set(k, []byte(strconv.FormatInt(step, 10)))
		if err != nil {
			return 0, errors.Trace(err)
		}
		return step, nil
	}
	if err != nil {
		return 0, errors.Trace(err)
	}

	intVal, err := strconv.ParseInt(string(val), 10, 0)
	if err != nil {
		return 0, errors.Trace(err)
	}

	intVal += step
	err = rm.Set(k, []byte(strconv.FormatInt(intVal, 10)))
	if err != nil {
		return 0, errors.Trace(err)
	}
	return intVal, nil
}

// GetInt64 get int64 value which created by IncInt64 method.
func GetInt64(r Retriever, k Key) (int64, error) {
	val, err := r.Get(k)
	if IsErrNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, errors.Trace(err)
	}
	intVal, err := strconv.ParseInt(string(val), 10, 0)
	return intVal, errors.Trace(err)
}
