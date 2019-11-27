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
	"container/heap"
	"encoding/binary"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/document"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/couchbase/vellum"
	lev "github.com/couchbase/vellum/levenshtein"
)

// re usable, threadsafe levenshtein builders
var lb1, lb2 *lev.LevenshteinAutomatonBuilder

type asynchSegmentResult struct {
	dict    segment.TermDictionary
	dictItr segment.DictionaryIterator

	index int
	docs  *roaring.Bitmap

	postings segment.PostingsList

	err error
}

var reflectStaticSizeIndexSnapshot int

func init() {
	var is interface{} = IndexSnapshot{}
	reflectStaticSizeIndexSnapshot = int(reflect.TypeOf(is).Size())
	var err error
	lb1, err = lev.NewLevenshteinAutomatonBuilder(1, true)
	if err != nil {
		panic(fmt.Errorf("Levenshtein automaton ed1 builder err: %v", err))
	}
	lb2, err = lev.NewLevenshteinAutomatonBuilder(2, true)
	if err != nil {
		panic(fmt.Errorf("Levenshtein automaton ed2 builder err: %v", err))
	}
}

type IndexSnapshot struct {
	parent   *Scorch
	segment  []*SegmentSnapshot
	offsets  []uint64
	internal map[string][]byte
	epoch    uint64
	size     uint64
	creator  string

	m    sync.Mutex // Protects the fields that follow.
	refs int64

	m2        sync.Mutex                                 // Protects the fields that follow.
	fieldTFRs map[string][]*IndexSnapshotTermFieldReader // keyed by field, recycled TFR's
}

func (i *IndexSnapshot) Segments() []*SegmentSnapshot {
	return i.segment
}

func (i *IndexSnapshot) Internal() map[string][]byte {
	return i.internal
}

func (i *IndexSnapshot) AddRef() {
	i.m.Lock()
	i.refs++
	i.m.Unlock()
}

func (i *IndexSnapshot) DecRef() (err error) {
	i.m.Lock()
	i.refs--
	if i.refs == 0 {
		for _, s := range i.segment {
			if s != nil {
				err2 := s.segment.DecRef()
				if err == nil {
					err = err2
				}
			}
		}
		if i.parent != nil {
			go i.parent.AddEligibleForRemoval(i.epoch)
		}
	}
	i.m.Unlock()
	return err
}

func (i *IndexSnapshot) Close() error {
	return i.DecRef()
}

func (i *IndexSnapshot) Size() int {
	return int(i.size)
}

func (i *IndexSnapshot) updateSize() {
	i.size += uint64(reflectStaticSizeIndexSnapshot)
	for _, s := range i.segment {
		i.size += uint64(s.Size())
	}
}

func (i *IndexSnapshot) newIndexSnapshotFieldDict(field string,
	makeItr func(i segment.TermDictionary) segment.DictionaryIterator,
	randomLookup bool) (*IndexSnapshotFieldDict, error) {

	results := make(chan *asynchSegmentResult)
	for index, segment := range i.segment {
		go func(index int, segment *SegmentSnapshot) {
			dict, err := segment.segment.Dictionary(field)
			if err != nil {
				results <- &asynchSegmentResult{err: err}
			} else {
				if randomLookup {
					results <- &asynchSegmentResult{dict: dict}
				} else {
					results <- &asynchSegmentResult{dictItr: makeItr(dict)}
				}
			}
		}(index, segment)
	}

	var err error
	rv := &IndexSnapshotFieldDict{
		snapshot: i,
		cursors:  make([]*segmentDictCursor, 0, len(i.segment)),
	}
	for count := 0; count < len(i.segment); count++ {
		asr := <-results
		if asr.err != nil && err == nil {
			err = asr.err
		} else {
			if !randomLookup {
				next, err2 := asr.dictItr.Next()
				if err2 != nil && err == nil {
					err = err2
				}
				if next != nil {
					rv.cursors = append(rv.cursors, &segmentDictCursor{
						itr:  asr.dictItr,
						curr: *next,
					})
				}
			} else {
				rv.cursors = append(rv.cursors, &segmentDictCursor{
					dict: asr.dict,
				})
			}
		}
	}
	// after ensuring we've read all items on channel
	if err != nil {
		return nil, err
	}

	if !randomLookup {
		// prepare heap
		heap.Init(rv)
	}

	return rv, nil
}

func (i *IndexSnapshot) FieldDict(field string) (index.FieldDict, error) {
	return i.newIndexSnapshotFieldDict(field, func(i segment.TermDictionary) segment.DictionaryIterator {
		return i.Iterator()
	}, false)
}

func (i *IndexSnapshot) FieldDictRange(field string, startTerm []byte,
	endTerm []byte) (index.FieldDict, error) {
	return i.newIndexSnapshotFieldDict(field, func(i segment.TermDictionary) segment.DictionaryIterator {
		return i.RangeIterator(string(startTerm), string(endTerm))
	}, false)
}

func (i *IndexSnapshot) FieldDictPrefix(field string,
	termPrefix []byte) (index.FieldDict, error) {
	return i.newIndexSnapshotFieldDict(field, func(i segment.TermDictionary) segment.DictionaryIterator {
		return i.PrefixIterator(string(termPrefix))
	}, false)
}

func (i *IndexSnapshot) FieldDictRegexp(field string,
	termRegex string) (index.FieldDict, error) {
	// TODO: potential optimization where the literal prefix represents the,
	//       entire regexp, allowing us to use PrefixIterator(prefixTerm)?

	a, prefixBeg, prefixEnd, err := segment.ParseRegexp(termRegex)
	if err != nil {
		return nil, err
	}

	return i.newIndexSnapshotFieldDict(field, func(i segment.TermDictionary) segment.DictionaryIterator {
		return i.AutomatonIterator(a, prefixBeg, prefixEnd)
	}, false)
}

func (i *IndexSnapshot) getLevAutomaton(term string,
	fuzziness uint8) (vellum.Automaton, error) {
	if fuzziness == 1 {
		return lb1.BuildDfa(term, fuzziness)
	} else if fuzziness == 2 {
		return lb2.BuildDfa(term, fuzziness)
	}
	return nil, fmt.Errorf("fuzziness exceeds the max limit")
}

func (i *IndexSnapshot) FieldDictFuzzy(field string,
	term string, fuzziness int, prefix string) (index.FieldDict, error) {
	a, err := i.getLevAutomaton(term, uint8(fuzziness))
	if err != nil {
		return nil, err
	}

	var prefixBeg, prefixEnd []byte
	if prefix != "" {
		prefixBeg = []byte(prefix)
		prefixEnd = segment.IncrementBytes(prefixBeg)
	}

	return i.newIndexSnapshotFieldDict(field, func(i segment.TermDictionary) segment.DictionaryIterator {
		return i.AutomatonIterator(a, prefixBeg, prefixEnd)
	}, false)
}

func (i *IndexSnapshot) FieldDictOnly(field string,
	onlyTerms [][]byte, includeCount bool) (index.FieldDict, error) {
	return i.newIndexSnapshotFieldDict(field, func(i segment.TermDictionary) segment.DictionaryIterator {
		return i.OnlyIterator(onlyTerms, includeCount)
	}, false)
}

func (i *IndexSnapshot) FieldDictContains(field string) (index.FieldDictContains, error) {
	return i.newIndexSnapshotFieldDict(field, nil, true)
}

func (i *IndexSnapshot) DocIDReaderAll() (index.DocIDReader, error) {
	results := make(chan *asynchSegmentResult)
	for index, segment := range i.segment {
		go func(index int, segment *SegmentSnapshot) {
			results <- &asynchSegmentResult{
				index: index,
				docs:  segment.DocNumbersLive(),
			}
		}(index, segment)
	}

	return i.newDocIDReader(results)
}

func (i *IndexSnapshot) DocIDReaderOnly(ids []string) (index.DocIDReader, error) {
	results := make(chan *asynchSegmentResult)
	for index, segment := range i.segment {
		go func(index int, segment *SegmentSnapshot) {
			docs, err := segment.DocNumbers(ids)
			if err != nil {
				results <- &asynchSegmentResult{err: err}
			} else {
				results <- &asynchSegmentResult{
					index: index,
					docs:  docs,
				}
			}
		}(index, segment)
	}

	return i.newDocIDReader(results)
}

func (i *IndexSnapshot) newDocIDReader(results chan *asynchSegmentResult) (index.DocIDReader, error) {
	rv := &IndexSnapshotDocIDReader{
		snapshot:  i,
		iterators: make([]roaring.IntIterable, len(i.segment)),
	}
	var err error
	for count := 0; count < len(i.segment); count++ {
		asr := <-results
		if asr.err != nil && err != nil {
			err = asr.err
		} else {
			rv.iterators[asr.index] = asr.docs.Iterator()
		}
	}

	if err != nil {
		return nil, err
	}

	return rv, nil
}

func (i *IndexSnapshot) Fields() ([]string, error) {
	// FIXME not making this concurrent for now as it's not used in hot path
	// of any searches at the moment (just a debug aid)
	fieldsMap := map[string]struct{}{}
	for _, segment := range i.segment {
		fields := segment.Fields()
		for _, field := range fields {
			fieldsMap[field] = struct{}{}
		}
	}
	rv := make([]string, 0, len(fieldsMap))
	for k := range fieldsMap {
		rv = append(rv, k)
	}
	return rv, nil
}

func (i *IndexSnapshot) GetInternal(key []byte) ([]byte, error) {
	return i.internal[string(key)], nil
}

func (i *IndexSnapshot) DocCount() (uint64, error) {
	var rv uint64
	for _, segment := range i.segment {
		rv += segment.Count()
	}
	return rv, nil
}

func (i *IndexSnapshot) Document(id string) (rv *document.Document, err error) {
	// FIXME could be done more efficiently directly, but reusing for simplicity
	tfr, err := i.TermFieldReader([]byte(id), "_id", false, false, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := tfr.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	next, err := tfr.Next(nil)
	if err != nil {
		return nil, err
	}

	if next == nil {
		// no such doc exists
		return nil, nil
	}

	docNum, err := docInternalToNumber(next.ID)
	if err != nil {
		return nil, err
	}
	segmentIndex, localDocNum := i.segmentIndexAndLocalDocNumFromGlobal(docNum)

	rv = document.NewDocument(id)
	err = i.segment[segmentIndex].VisitDocument(localDocNum, func(name string, typ byte, val []byte, pos []uint64) bool {
		if name == "_id" {
			return true
		}

		// copy value, array positions to preserve them beyond the scope of this callback
		value := append([]byte(nil), val...)
		arrayPos := append([]uint64(nil), pos...)

		switch typ {
		case 't':
			rv.AddField(document.NewTextField(name, arrayPos, value))
		case 'n':
			rv.AddField(document.NewNumericFieldFromBytes(name, arrayPos, value))
		case 'd':
			rv.AddField(document.NewDateTimeFieldFromBytes(name, arrayPos, value))
		case 'b':
			rv.AddField(document.NewBooleanFieldFromBytes(name, arrayPos, value))
		case 'g':
			rv.AddField(document.NewGeoPointFieldFromBytes(name, arrayPos, value))
		}

		return true
	})
	if err != nil {
		return nil, err
	}

	return rv, nil
}

func (i *IndexSnapshot) segmentIndexAndLocalDocNumFromGlobal(docNum uint64) (int, uint64) {
	segmentIndex := sort.Search(len(i.offsets),
		func(x int) bool {
			return i.offsets[x] > docNum
		}) - 1

	localDocNum := docNum - i.offsets[segmentIndex]
	return int(segmentIndex), localDocNum
}

func (i *IndexSnapshot) ExternalID(id index.IndexInternalID) (string, error) {
	docNum, err := docInternalToNumber(id)
	if err != nil {
		return "", err
	}
	segmentIndex, localDocNum := i.segmentIndexAndLocalDocNumFromGlobal(docNum)

	v, err := i.segment[segmentIndex].DocID(localDocNum)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", fmt.Errorf("document number %d not found", docNum)
	}

	return string(v), nil
}

func (i *IndexSnapshot) InternalID(id string) (rv index.IndexInternalID, err error) {
	// FIXME could be done more efficiently directly, but reusing for simplicity
	tfr, err := i.TermFieldReader([]byte(id), "_id", false, false, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := tfr.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	next, err := tfr.Next(nil)
	if err != nil || next == nil {
		return nil, err
	}

	return next.ID, nil
}

func (i *IndexSnapshot) TermFieldReader(term []byte, field string, includeFreq,
	includeNorm, includeTermVectors bool) (index.TermFieldReader, error) {
	rv := i.allocTermFieldReaderDicts(field)

	rv.term = term
	rv.field = field
	rv.snapshot = i
	if rv.postings == nil {
		rv.postings = make([]segment.PostingsList, len(i.segment))
	}
	if rv.iterators == nil {
		rv.iterators = make([]segment.PostingsIterator, len(i.segment))
	}
	rv.segmentOffset = 0
	rv.includeFreq = includeFreq
	rv.includeNorm = includeNorm
	rv.includeTermVectors = includeTermVectors
	rv.currPosting = nil
	rv.currID = rv.currID[:0]

	if rv.dicts == nil {
		rv.dicts = make([]segment.TermDictionary, len(i.segment))
		for i, segment := range i.segment {
			dict, err := segment.segment.Dictionary(field)
			if err != nil {
				return nil, err
			}
			rv.dicts[i] = dict
		}
	}

	for i, segment := range i.segment {
		pl, err := rv.dicts[i].PostingsList(term, segment.deleted, rv.postings[i])
		if err != nil {
			return nil, err
		}
		rv.postings[i] = pl
		rv.iterators[i] = pl.Iterator(includeFreq, includeNorm, includeTermVectors, rv.iterators[i])
	}
	atomic.AddUint64(&i.parent.stats.TotTermSearchersStarted, uint64(1))
	return rv, nil
}

func (i *IndexSnapshot) allocTermFieldReaderDicts(field string) (tfr *IndexSnapshotTermFieldReader) {
	i.m2.Lock()
	if i.fieldTFRs != nil {
		tfrs := i.fieldTFRs[field]
		last := len(tfrs) - 1
		if last >= 0 {
			tfr = tfrs[last]
			tfrs[last] = nil
			i.fieldTFRs[field] = tfrs[:last]
			i.m2.Unlock()
			return
		}
	}
	i.m2.Unlock()
	return &IndexSnapshotTermFieldReader{}
}

func (i *IndexSnapshot) recycleTermFieldReader(tfr *IndexSnapshotTermFieldReader) {
	i.parent.rootLock.RLock()
	obsolete := i.parent.root != i
	i.parent.rootLock.RUnlock()
	if obsolete {
		// if we're not the current root (mutations happened), don't bother recycling
		return
	}

	i.m2.Lock()
	if i.fieldTFRs == nil {
		i.fieldTFRs = map[string][]*IndexSnapshotTermFieldReader{}
	}
	i.fieldTFRs[tfr.field] = append(i.fieldTFRs[tfr.field], tfr)
	i.m2.Unlock()
}

func docNumberToBytes(buf []byte, in uint64) []byte {
	if len(buf) != 8 {
		if cap(buf) >= 8 {
			buf = buf[0:8]
		} else {
			buf = make([]byte, 8)
		}
	}
	binary.BigEndian.PutUint64(buf, in)
	return buf
}

func docInternalToNumber(in index.IndexInternalID) (uint64, error) {
	if len(in) != 8 {
		return 0, fmt.Errorf("wrong len for IndexInternalID: %q", in)
	}
	return binary.BigEndian.Uint64(in), nil
}

func (i *IndexSnapshot) DocumentVisitFieldTerms(id index.IndexInternalID,
	fields []string, visitor index.DocumentFieldTermVisitor) error {
	_, err := i.documentVisitFieldTerms(id, fields, visitor, nil)
	return err
}

func (i *IndexSnapshot) documentVisitFieldTerms(id index.IndexInternalID,
	fields []string, visitor index.DocumentFieldTermVisitor,
	dvs segment.DocVisitState) (segment.DocVisitState, error) {
	docNum, err := docInternalToNumber(id)
	if err != nil {
		return nil, err
	}

	segmentIndex, localDocNum := i.segmentIndexAndLocalDocNumFromGlobal(docNum)
	if segmentIndex >= len(i.segment) {
		return nil, nil
	}

	_, dvs, err = i.documentVisitFieldTermsOnSegment(
		segmentIndex, localDocNum, fields, nil, visitor, dvs)

	return dvs, err
}

func (i *IndexSnapshot) documentVisitFieldTermsOnSegment(
	segmentIndex int, localDocNum uint64, fields []string, cFields []string,
	visitor index.DocumentFieldTermVisitor, dvs segment.DocVisitState) (
	cFieldsOut []string, dvsOut segment.DocVisitState, err error) {
	ss := i.segment[segmentIndex]

	var vFields []string // fields that are visitable via the segment

	ssv, ssvOk := ss.segment.(segment.DocumentFieldTermVisitable)
	if ssvOk && ssv != nil {
		vFields, err = ssv.VisitableDocValueFields()
		if err != nil {
			return nil, nil, err
		}
	}

	var errCh chan error

	// cFields represents the fields that we'll need from the
	// cachedDocs, and might be optionally be provided by the caller,
	// if the caller happens to know we're on the same segmentIndex
	// from a previous invocation
	if cFields == nil {
		cFields = subtractStrings(fields, vFields)

		if !ss.cachedDocs.hasFields(cFields) {
			errCh = make(chan error, 1)

			go func() {
				err := ss.cachedDocs.prepareFields(cFields, ss)
				if err != nil {
					errCh <- err
				}
				close(errCh)
			}()
		}
	}

	if ssvOk && ssv != nil && len(vFields) > 0 {
		dvs, err = ssv.VisitDocumentFieldTerms(localDocNum, fields, visitor, dvs)
		if err != nil {
			return nil, nil, err
		}
	}

	if errCh != nil {
		err = <-errCh
		if err != nil {
			return nil, nil, err
		}
	}

	if len(cFields) > 0 {
		ss.cachedDocs.visitDoc(localDocNum, cFields, visitor)
	}

	return cFields, dvs, nil
}

func (i *IndexSnapshot) DocValueReader(fields []string) (
	index.DocValueReader, error) {
	return &DocValueReader{i: i, fields: fields, currSegmentIndex: -1}, nil
}

type DocValueReader struct {
	i      *IndexSnapshot
	fields []string
	dvs    segment.DocVisitState

	currSegmentIndex int
	currCachedFields []string
}

func (dvr *DocValueReader) VisitDocValues(id index.IndexInternalID,
	visitor index.DocumentFieldTermVisitor) (err error) {
	docNum, err := docInternalToNumber(id)
	if err != nil {
		return err
	}

	segmentIndex, localDocNum := dvr.i.segmentIndexAndLocalDocNumFromGlobal(docNum)
	if segmentIndex >= len(dvr.i.segment) {
		return nil
	}

	if dvr.currSegmentIndex != segmentIndex {
		dvr.currSegmentIndex = segmentIndex
		dvr.currCachedFields = nil
	}

	dvr.currCachedFields, dvr.dvs, err = dvr.i.documentVisitFieldTermsOnSegment(
		dvr.currSegmentIndex, localDocNum, dvr.fields, dvr.currCachedFields, visitor, dvr.dvs)

	return err
}

func (i *IndexSnapshot) DumpAll() chan interface{} {
	rv := make(chan interface{})
	go func() {
		close(rv)
	}()
	return rv
}

func (i *IndexSnapshot) DumpDoc(id string) chan interface{} {
	rv := make(chan interface{})
	go func() {
		close(rv)
	}()
	return rv
}

func (i *IndexSnapshot) DumpFields() chan interface{} {
	rv := make(chan interface{})
	go func() {
		close(rv)
	}()
	return rv
}

// subtractStrings returns set a minus elements of set b.
func subtractStrings(a, b []string) []string {
	if len(b) == 0 {
		return a
	}

	rv := make([]string, 0, len(a))
OUTER:
	for _, as := range a {
		for _, bs := range b {
			if as == bs {
				continue OUTER
			}
		}
		rv = append(rv, as)
	}
	return rv
}
