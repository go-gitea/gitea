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

package segment

import (
	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index"
)

type EmptySegment struct{}

func (e *EmptySegment) Dictionary(field string) (TermDictionary, error) {
	return &EmptyDictionary{}, nil
}

func (e *EmptySegment) VisitDocument(num uint64, visitor DocumentFieldValueVisitor) error {
	return nil
}

func (e *EmptySegment) Count() uint64 {
	return 0
}

func (e *EmptySegment) DocNumbers([]string) (*roaring.Bitmap, error) {
	r := roaring.NewBitmap()
	return r, nil
}

func (e *EmptySegment) Fields() []string {
	return []string{}
}

func (e *EmptySegment) Close() error {
	return nil
}

func (e *EmptySegment) AddRef() {
}

func (e *EmptySegment) DecRef() error {
	return nil
}

type EmptyDictionary struct{}

func (e *EmptyDictionary) PostingsList(term string,
	except *roaring.Bitmap) (PostingsList, error) {
	return &EmptyPostingsList{}, nil
}

func (e *EmptyDictionary) Iterator() DictionaryIterator {
	return &EmptyDictionaryIterator{}
}

func (e *EmptyDictionary) PrefixIterator(prefix string) DictionaryIterator {
	return &EmptyDictionaryIterator{}
}

func (e *EmptyDictionary) RangeIterator(start, end string) DictionaryIterator {
	return &EmptyDictionaryIterator{}
}

type EmptyDictionaryIterator struct{}

func (e *EmptyDictionaryIterator) Next() (*index.DictEntry, error) {
	return nil, nil
}

type EmptyPostingsList struct{}

func (e *EmptyPostingsList) Iterator() PostingsIterator {
	return &EmptyPostingsIterator{}
}

func (e *EmptyPostingsList) Count() uint64 {
	return 0
}

type EmptyPostingsIterator struct{}

func (e *EmptyPostingsIterator) Next() (Posting, error) {
	return nil, nil
}
