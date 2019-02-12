//  Copyright (c) 2017 Couchbase, Inc.
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

package scorch

import (
	"encoding/json"
	"io/ioutil"
	"sync/atomic"
)

// Stats tracks statistics about the index
type Stats struct {
	updates, deletes, batches, errors uint64
	analysisTime, indexTime           uint64
	termSearchersStarted              uint64
	termSearchersFinished             uint64
	numPlainTextBytesIndexed          uint64
	numItemsIntroduced                uint64
	numItemsPersisted                 uint64
	i                                 *Scorch
}

func (s *Stats) statsMap() (map[string]interface{}, error) {
	m := map[string]interface{}{}
	m["updates"] = atomic.LoadUint64(&s.updates)
	m["deletes"] = atomic.LoadUint64(&s.deletes)
	m["batches"] = atomic.LoadUint64(&s.batches)
	m["errors"] = atomic.LoadUint64(&s.errors)
	m["analysis_time"] = atomic.LoadUint64(&s.analysisTime)
	m["index_time"] = atomic.LoadUint64(&s.indexTime)
	m["term_searchers_started"] = atomic.LoadUint64(&s.termSearchersStarted)
	m["term_searchers_finished"] = atomic.LoadUint64(&s.termSearchersFinished)
	m["num_plain_text_bytes_indexed"] = atomic.LoadUint64(&s.numPlainTextBytesIndexed)
	m["num_items_introduced"] = atomic.LoadUint64(&s.numItemsIntroduced)
	m["num_items_persisted"] = atomic.LoadUint64(&s.numItemsPersisted)

	if s.i.path != "" {
		finfos, err := ioutil.ReadDir(s.i.path)
		if err != nil {
			return nil, err
		}

		var numFilesOnDisk, numBytesUsedDisk uint64

		for _, finfo := range finfos {
			if !finfo.IsDir() {
				numBytesUsedDisk += uint64(finfo.Size())
				numFilesOnDisk++
			}
		}

		m["num_bytes_used_disk"] = numBytesUsedDisk
		m["num_files_on_disk"] = numFilesOnDisk
	}

	return m, nil
}

// MarshalJSON implements json.Marshaler
func (s *Stats) MarshalJSON() ([]byte, error) {
	m, err := s.statsMap()
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}
