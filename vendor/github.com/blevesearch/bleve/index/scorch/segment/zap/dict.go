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
	"bytes"
	"fmt"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/couchbase/vellum"
)

// Dictionary is the zap representation of the term dictionary
type Dictionary struct {
	sb        *SegmentBase
	field     string
	fieldID   uint16
	fst       *vellum.FST
	fstReader *vellum.Reader
}

// PostingsList returns the postings list for the specified term
func (d *Dictionary) PostingsList(term []byte, except *roaring.Bitmap,
	prealloc segment.PostingsList) (segment.PostingsList, error) {
	var preallocPL *PostingsList
	pl, ok := prealloc.(*PostingsList)
	if ok && pl != nil {
		preallocPL = pl
	}
	return d.postingsList(term, except, preallocPL)
}

func (d *Dictionary) postingsList(term []byte, except *roaring.Bitmap, rv *PostingsList) (*PostingsList, error) {
	if d.fstReader == nil {
		if rv == nil || rv == emptyPostingsList {
			return emptyPostingsList, nil
		}
		return d.postingsListInit(rv, except), nil
	}

	postingsOffset, exists, err := d.fstReader.Get(term)
	if err != nil {
		return nil, fmt.Errorf("vellum err: %v", err)
	}
	if !exists {
		if rv == nil || rv == emptyPostingsList {
			return emptyPostingsList, nil
		}
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
	if rv == nil || rv == emptyPostingsList {
		rv = &PostingsList{}
	} else {
		postings := rv.postings
		if postings != nil {
			postings.Clear()
		}

		*rv = PostingsList{} // clear the struct

		rv.postings = postings
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
		} else if err != vellum.ErrIteratorDone {
			rv.err = err
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

	kBeg := []byte(prefix)
	kEnd := segment.IncrementBytes(kBeg)

	if d.fst != nil {
		itr, err := d.fst.Iterator(kBeg, kEnd)
		if err == nil {
			rv.itr = itr
		} else if err != vellum.ErrIteratorDone {
			rv.err = err
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
		} else if err != vellum.ErrIteratorDone {
			rv.err = err
		}
	}

	return rv
}

// AutomatonIterator returns an iterator which only visits terms
// having the the vellum automaton and start/end key range
func (d *Dictionary) AutomatonIterator(a vellum.Automaton,
	startKeyInclusive, endKeyExclusive []byte) segment.DictionaryIterator {
	rv := &DictionaryIterator{
		d: d,
	}

	if d.fst != nil {
		itr, err := d.fst.Search(a, startKeyInclusive, endKeyExclusive)
		if err == nil {
			rv.itr = itr
		} else if err != vellum.ErrIteratorDone {
			rv.err = err
		}
	}

	return rv
}

func (d *Dictionary) OnlyIterator(onlyTerms [][]byte,
	includeCount bool) segment.DictionaryIterator {

	rv := &DictionaryIterator{
		d:         d,
		omitCount: !includeCount,
	}

	var buf bytes.Buffer
	builder, err := vellum.New(&buf, nil)
	if err != nil {
		rv.err = err
		return rv
	}
	for _, term := range onlyTerms {
		err = builder.Insert(term, 0)
		if err != nil {
			rv.err = err
			return rv
		}
	}
	err = builder.Close()
	if err != nil {
		rv.err = err
		return rv
	}

	onlyFST, err := vellum.Load(buf.Bytes())
	if err != nil {
		rv.err = err
		return rv
	}

	itr, err := d.fst.Search(onlyFST, nil, nil)
	if err == nil {
		rv.itr = itr
	} else if err != vellum.ErrIteratorDone {
		rv.err = err
	}

	return rv
}

// DictionaryIterator is an iterator for term dictionary
type DictionaryIterator struct {
	d         *Dictionary
	itr       vellum.Iterator
	err       error
	tmp       PostingsList
	entry     index.DictEntry
	omitCount bool
}

// Next returns the next entry in the dictionary
func (i *DictionaryIterator) Next() (*index.DictEntry, error) {
	if i.err != nil && i.err != vellum.ErrIteratorDone {
		return nil, i.err
	} else if i.itr == nil || i.err == vellum.ErrIteratorDone {
		return nil, nil
	}
	term, postingsOffset := i.itr.Current()
	i.entry.Term = string(term)
	if !i.omitCount {
		i.err = i.tmp.read(postingsOffset, i.d)
		if i.err != nil {
			return nil, i.err
		}
		i.entry.Count = i.tmp.Count()
	}
	i.err = i.itr.Next()
	return &i.entry, nil
}
