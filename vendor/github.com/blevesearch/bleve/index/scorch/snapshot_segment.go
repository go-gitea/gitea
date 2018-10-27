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
	"sync"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index/scorch/segment"
)

var TermSeparator byte = 0xff

var TermSeparatorSplitSlice = []byte{TermSeparator}

type SegmentDictionarySnapshot struct {
	s *SegmentSnapshot
	d segment.TermDictionary
}

func (s *SegmentDictionarySnapshot) PostingsList(term string, except *roaring.Bitmap) (segment.PostingsList, error) {
	// TODO: if except is non-nil, perhaps need to OR it with s.s.deleted?
	return s.d.PostingsList(term, s.s.deleted)
}

func (s *SegmentDictionarySnapshot) Iterator() segment.DictionaryIterator {
	return s.d.Iterator()
}

func (s *SegmentDictionarySnapshot) PrefixIterator(prefix string) segment.DictionaryIterator {
	return s.d.PrefixIterator(prefix)
}

func (s *SegmentDictionarySnapshot) RangeIterator(start, end string) segment.DictionaryIterator {
	return s.d.RangeIterator(start, end)
}

type SegmentSnapshot struct {
	id      uint64
	segment segment.Segment
	deleted *roaring.Bitmap

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

func (s *SegmentSnapshot) Count() uint64 {

	rv := s.segment.Count()
	if s.deleted != nil {
		rv -= s.deleted.GetCardinality()
	}
	return rv
}

func (s *SegmentSnapshot) Dictionary(field string) (segment.TermDictionary, error) {
	d, err := s.segment.Dictionary(field)
	if err != nil {
		return nil, err
	}
	return &SegmentDictionarySnapshot{
		s: s,
		d: d,
	}, nil
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

// DocNumbersLive returns bitsit containing doc numbers for all live docs
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

type cachedFieldDocs struct {
	readyCh chan struct{}     // closed when the cachedFieldDocs.docs is ready to be used.
	err     error             // Non-nil if there was an error when preparing this cachedFieldDocs.
	docs    map[uint64][]byte // Keyed by localDocNum, value is a list of terms delimited by 0xFF.
}

func (cfd *cachedFieldDocs) prepareFields(field string, ss *SegmentSnapshot) {
	defer close(cfd.readyCh)

	dict, err := ss.segment.Dictionary(field)
	if err != nil {
		cfd.err = err
		return
	}

	dictItr := dict.Iterator()
	next, err := dictItr.Next()
	for err == nil && next != nil {
		postings, err1 := dict.PostingsList(next.Term, nil)
		if err1 != nil {
			cfd.err = err1
			return
		}

		postingsItr := postings.Iterator()
		nextPosting, err2 := postingsItr.Next()
		for err2 == nil && nextPosting != nil {
			docNum := nextPosting.Number()
			cfd.docs[docNum] = append(cfd.docs[docNum], []byte(next.Term)...)
			cfd.docs[docNum] = append(cfd.docs[docNum], TermSeparator)
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

			go c.cache[field].prepareFields(field, ss)
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

	c.m.Unlock()
	return nil
}

func (c *cachedDocs) sizeInBytes() uint64 {
	sizeInBytes := 0
	c.m.Lock()
	for k, v := range c.cache { // cachedFieldDocs
		sizeInBytes += len(k)
		if v != nil {
			for _, entry := range v.docs { // docs
				sizeInBytes += 8 /* size of uint64 */ + len(entry)
			}
		}
	}
	c.m.Unlock()
	return uint64(sizeInBytes)
}
