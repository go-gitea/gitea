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

package segmentmap

import (
	"hash/crc32"

	"github.com/juju/errors"
)

// SegmentMap is used for handle a big map slice by slice.
// It's not thread safe.
type SegmentMap struct {
	size int64
	maps []map[string]interface{}

	crcTable *crc32.Table
}

// NewSegmentMap create a new SegmentMap.
func NewSegmentMap(size int64) (*SegmentMap, error) {
	if size <= 0 {
		return nil, errors.Errorf("Invalid size: %d", size)
	}

	sm := &SegmentMap{
		maps: make([]map[string]interface{}, size),
		size: size,
	}
	for i := int64(0); i < size; i++ {
		sm.maps[i] = make(map[string]interface{})
	}

	sm.crcTable = crc32.MakeTable(crc32.Castagnoli)
	return sm, nil
}

// Get is the same as map[k].
func (sm *SegmentMap) Get(key []byte) (interface{}, bool) {
	idx := int64(crc32.Checksum(key, sm.crcTable)) % sm.size
	val, ok := sm.maps[idx][string(key)]
	return val, ok
}

// GetSegment gets the map specific by index.
func (sm *SegmentMap) GetSegment(index int64) (map[string]interface{}, error) {
	if index >= sm.size || index < 0 {
		return nil, errors.Errorf("index out of bound: %d", index)
	}

	return sm.maps[index], nil
}

// Set if key not exists, returns whether already exists.
func (sm *SegmentMap) Set(key []byte, value interface{}, force bool) bool {
	idx := int64(crc32.Checksum(key, sm.crcTable)) % sm.size
	k := string(key)
	_, exist := sm.maps[idx][k]
	if exist && !force {
		return exist
	}

	sm.maps[idx][k] = value
	return exist
}

// SegmentCount returns how many inner segments.
func (sm *SegmentMap) SegmentCount() int64 {
	return sm.size
}
