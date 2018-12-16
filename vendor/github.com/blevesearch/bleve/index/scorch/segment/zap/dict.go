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

package zap

import (
	"fmt"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/couchbase/vellum"
	"github.com/couchbase/vellum/regexp"
)

// Dictionary is the zap representation of the term dictionary
type Dictionary struct {
	sb      *SegmentBase
	field   string
	fieldID uint16
	fst     *vellum.FST
}

// PostingsList returns the postings list for the specified term
func (d *Dictionary) PostingsList(term string, except *roaring.Bitmap) (segment.PostingsList, error) {
	return d.postingsList([]byte(term), except, nil)
}

func (d *Dictionary) postingsList(term []byte, except *roaring.Bitmap, rv *PostingsList) (*PostingsList, error) {
	if d.fst == nil {
		return d.postingsListInit(rv, except), nil
	}

	postingsOffset, exists, err := d.fst.Get(term)
	if err != nil {
		return nil, fmt.Errorf("vellum err: %v", err)
	}
	if !exists {
		return d.postingsListInit(rv, except), nil
	}

	return d.postingsListFromOffset(postingsOffset, except, rv)
}

func (d *Dictionary) postingsListFromOffset(postingsOffset uint64, except *roaring.Bitmap, rv *PostingsList) (*PostingsList, error) {
	rv = d.postingsListInit(rv, except)

	err := rv.read(postingsOffset, d)
	if err != nil {
		return nil, err
	}

	return rv, nil
}

func (d *Dictionary) postingsListInit(rv *PostingsList, except *roaring.Bitmap) *PostingsList {
	if rv == nil {
		rv = &PostingsList{}
	} else {
		*rv = PostingsList{} // clear the struct
	}
	rv.sb = d.sb
	rv.except = except
	return rv
}

// Iterator returns an iterator for this dictionary
func (d *Dictionary) Iterator() segment.DictionaryIterator {
	rv := &DictionaryIterator{
		d: d,
	}

	if d.fst != nil {
		itr, err := d.fst.Iterator(nil, nil)
		if err == nil {
			rv.itr = itr
		}
	}

	return rv
}

// PrefixIterator returns an iterator which only visits terms having the
// the specified prefix
func (d *Dictionary) PrefixIterator(prefix string) segment.DictionaryIterator {
	rv := &DictionaryIterator{
		d: d,
	}

	if d.fst != nil {
		r, err := regexp.New(prefix + ".*")
		if err == nil {
			itr, err := d.fst.Search(r, nil, nil)
			if err == nil {
				rv.itr = itr
			}
		}
	}

	return rv
}

// RangeIterator returns an iterator which only visits terms between the
// start and end terms.  NOTE: bleve.index API specifies the end is inclusive.
func (d *Dictionary) RangeIterator(start, end string) segment.DictionaryIterator {
	rv := &DictionaryIterator{
		d: d,
	}

	// need to increment the end position to be inclusive
	endBytes := []byte(end)
	if endBytes[len(endBytes)-1] < 0xff {
		endBytes[len(endBytes)-1]++
	} else {
		endBytes = append(endBytes, 0xff)
	}

	if d.fst != nil {
		itr, err := d.fst.Iterator([]byte(start), endBytes)
		if err == nil {
			rv.itr = itr
		}
	}

	return rv
}

// DictionaryIterator is an iterator for term dictionary
type DictionaryIterator struct {
	d   *Dictionary
	itr vellum.Iterator
	err error
	tmp PostingsList
}

// Next returns the next entry in the dictionary
func (i *DictionaryIterator) Next() (*index.DictEntry, error) {
	if i.itr == nil || i.err == vellum.ErrIteratorDone {
		return nil, nil
	} else if i.err != nil {
		return nil, i.err
	}
	term, postingsOffset := i.itr.Current()
	i.err = i.tmp.read(postingsOffset, i.d)
	if i.err != nil {
		return nil, i.err
	}
	rv := &index.DictEntry{
		Term:  string(term),
		Count: i.tmp.Count(),
	}
	i.err = i.itr.Next()
	return rv, nil
}
