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
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/blevesearch/bleve/size"
)

var TermSeparator byte = 0xff

var TermSeparatorSplitSlice = []byte{TermSeparator}

type SegmentSnapshot struct {
	id      uint64
	segment segment.Segment
	deleted *roaring.Bitmap
	creator string

	cachedDocs *cachedDocs
}

func (s *SegmentSnapshot) Segment() segment.Segment {
	return s.segment
}

func (s *SegmentSnapshot) Deleted() *roaring.Bitmap {
	return s.deleted
}

func (s *SegmentSnapshot) Id() uint64 {
	return s.id
}

func (s *SegmentSnapshot) FullSize() int64 {
	return int64(s.segment.Count())
}

func (s SegmentSnapshot) LiveSize() int64 {
	return int64(s.Count())
}

func (s *SegmentSnapshot) Close() error {
	return s.segment.Close()
}

func (s *SegmentSnapshot) VisitDocument(num uint64, visitor segment.DocumentFieldValueVisitor) error {
	return s.segment.VisitDocument(num, visitor)
}

func (s *SegmentSnapshot) DocID(num uint64) ([]byte, error) {
	return s.segment.DocID(num)
}

func (s *SegmentSnapshot) Count() uint64 {
	rv := s.segment.Count()
	if s.deleted != nil {
		rv -= s.deleted.GetCardinality()
	}
	return rv
}

func (s *SegmentSnapshot) DocNumbers(docIDs []string) (*roaring.Bitmap, error) {
	rv, err := s.segment.DocNumbers(docIDs)
	if err != nil {
		return nil, err
	}
	if s.deleted != nil {
		rv.AndNot(s.deleted)
	}
	return rv, nil
}

// DocNumbersLive returns a bitmap containing doc numbers for all live docs
func (s *SegmentSnapshot) DocNumbersLive() *roaring.Bitmap {
	rv := roaring.NewBitmap()
	rv.AddRange(0, s.segment.Count())
	if s.deleted != nil {
		rv.AndNot(s.deleted)
	}
	return rv
}

func (s *SegmentSnapshot) Fields() []string {
	return s.segment.Fields()
}

func (s *SegmentSnapshot) Size() (rv int) {
	rv = s.segment.Size()
	if s.deleted != nil {
		rv += int(s.deleted.GetSizeInBytes())
	}
	rv += s.cachedDocs.Size()
	return
}

type cachedFieldDocs struct {
	m       sync.Mutex
	readyCh chan struct{}     // closed when the cachedFieldDocs.docs is ready to be used.
	err     error             // Non-nil if there was an error when preparing this cachedFieldDocs.
	docs    map[uint64][]byte // Keyed by localDocNum, value is a list of terms delimited by 0xFF.
	size    uint64
}

func (cfd *cachedFieldDocs) Size() int {
	var rv int
	cfd.m.Lock()
	for _, entry := range cfd.docs {
		rv += 8 /* size of uint64 */ + len(entry)
	}
	cfd.m.Unlock()
	return rv
}

func (cfd *cachedFieldDocs) prepareField(field string, ss *SegmentSnapshot) {
	cfd.m.Lock()
	defer func() {
		close(cfd.readyCh)
		cfd.m.Unlock()
	}()

	cfd.size += uint64(size.SizeOfUint64) /* size field */
	dict, err := ss.segment.Dictionary(field)
	if err != nil {
		cfd.err = err
		return
	}

	var postings segment.PostingsList
	var postingsItr segment.PostingsIterator

	dictItr := dict.Iterator()
	next, err := dictItr.Next()
	for err == nil && next != nil {
		var err1 error
		postings, err1 = dict.PostingsList([]byte(next.Term), nil, postings)
		if err1 != nil {
			cfd.err = err1
			return
		}

		cfd.size += uint64(size.SizeOfUint64) /* map key */
		postingsItr = postings.Iterator(false, false, false, postingsItr)
		nextPosting, err2 := postingsItr.Next()
		for err2 == nil && nextPosting != nil {
			docNum := nextPosting.Number()
			cfd.docs[docNum] = append(cfd.docs[docNum], []byte(next.Term)...)
			cfd.docs[docNum] = append(cfd.docs[docNum], TermSeparator)
			cfd.size += uint64(len(next.Term) + 1) // map value
			nextPosting, err2 = postingsItr.Next()
		}

		if err2 != nil {
			cfd.err = err2
			return
		}

		next, err = dictItr.Next()
	}

	if err != nil {
		cfd.err = err
		return
	}
}

type cachedDocs struct {
	size  uint64
	m     sync.Mutex                  // As the cache is asynchronously prepared, need a lock
	cache map[string]*cachedFieldDocs // Keyed by field
}

func (c *cachedDocs) prepareFields(wantedFields []string, ss *SegmentSnapshot) error {
	c.m.Lock()

	if c.cache == nil {
		c.cache = make(map[string]*cachedFieldDocs, len(ss.Fields()))
	}

	for _, field := range wantedFields {
		_, exists := c.cache[field]
		if !exists {
			c.cache[field] = &cachedFieldDocs{
				readyCh: make(chan struct{}),
				docs:    make(map[uint64][]byte),
			}

			go c.cache[field].prepareField(field, ss)
		}
	}

	for _, field := range wantedFields {
		cachedFieldDocs := c.cache[field]
		c.m.Unlock()
		<-cachedFieldDocs.readyCh

		if cachedFieldDocs.err != nil {
			return cachedFieldDocs.err
		}
		c.m.Lock()
	}

	c.updateSizeLOCKED()

	c.m.Unlock()
	return nil
}

// hasFields returns true if the cache has all the given fields
func (c *cachedDocs) hasFields(fields []string) bool {
	c.m.Lock()
	for _, field := range fields {
		if _, exists := c.cache[field]; !exists {
			c.m.Unlock()
			return false // found a field not in cache
		}
	}
	c.m.Unlock()
	return true
}

func (c *cachedDocs) Size() int {
	return int(atomic.LoadUint64(&c.size))
}

func (c *cachedDocs) updateSizeLOCKED() {
	sizeInBytes := 0
	for k, v := range c.cache { // cachedFieldDocs
		sizeInBytes += len(k)
		if v != nil {
			sizeInBytes += v.Size()
		}
	}
	atomic.StoreUint64(&c.size, uint64(sizeInBytes))
}

func (c *cachedDocs) visitDoc(localDocNum uint64,
	fields []string, visitor index.DocumentFieldTermVisitor) {
	c.m.Lock()

	for _, field := range fields {
		if cachedFieldDocs, exists := c.cache[field]; exists {
			c.m.Unlock()
			<-cachedFieldDocs.readyCh
			c.m.Lock()

			if tlist, exists := cachedFieldDocs.docs[localDocNum]; exists {
				for {
					i := bytes.Index(tlist, TermSeparatorSplitSlice)
					if i < 0 {
						break
					}
					visitor(field, tlist[0:i])
					tlist = tlist[i+1:]
				}
			}
		}
	}

	c.m.Unlock()
}
