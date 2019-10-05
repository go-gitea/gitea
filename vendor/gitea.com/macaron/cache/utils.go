// Copyright 2014 The Macaron Authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package cache

import (
	"bytes"
	"encoding/gob"
	"errors"
)

func EncodeGob(item *Item) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := gob.NewEncoder(buf).Encode(item)
	return buf.Bytes(), err
}

func DecodeGob(data []byte, out *Item) error {
	buf := bytes.NewBuffer(data)
	return gob.NewDecoder(buf).Decode(&out)
}

func Incr(val interface{}) (interface{}, error) {
	switch val.(type) {
	case int:
		val = val.(int) + 1
	case int32:
		val = val.(int32) + 1
	case int64:
		val = val.(int64) + 1
	case uint:
		val = val.(uint) + 1
	case uint32:
		val = val.(uint32) + 1
	case uint64:
		val = val.(uint64) + 1
	default:
		return val, errors.New("item value is not int-type")
	}
	return val, nil
}

func Decr(val interface{}) (interface{}, error) {
	switch val.(type) {
	case int:
		val = val.(int) - 1
	case int32:
		val = val.(int32) - 1
	case int64:
		val = val.(int64) - 1
	case uint:
		if val.(uint) > 0 {
			val = val.(uint) - 1
		} else {
			return val, errors.New("item value is less than 0")
		}
	case uint32:
		if val.(uint32) > 0 {
			val = val.(uint32) - 1
		} else {
			return val, errors.New("item value is less than 0")
		}
	case uint64:
		if val.(uint64) > 0 {
			val = val.(uint64) - 1
		} else {
			return val, errors.New("item value is less than 0")
		}
	default:
		return val, errors.New("item value is not int-type")
	}
	return val, nil
}
