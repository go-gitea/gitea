//  Copyright (c) 2014 Couchbase, Inc.
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

package collector

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
	"github.com/blevesearch/bleve/size"
)

var reflectStaticSizeTopNCollector int

func init() {
	var coll TopNCollector
	reflectStaticSizeTopNCollector = int(reflect.TypeOf(coll).Size())
}

type collectorStore interface {
	// Add the document, and if the new store size exceeds the provided size
	// the last element is removed and returned.  If the size has not been
	// exceeded, nil is returned.
	AddNotExceedingSize(doc *search.DocumentMatch, size int) *search.DocumentMatch

	Final(skip int, fixup collectorFixup) (search.DocumentMatchCollection, error)
}

// PreAllocSizeSkipCap will cap preallocation to this amount when
// size+skip exceeds this value
var PreAllocSizeSkipCap = 1000

type collectorCompare func(i, j *search.DocumentMatch) int

type collectorFixup func(d *search.DocumentMatch) error

// TopNCollector collects the top N hits, optionally skipping some results
type TopNCollector struct {
	size          int
	skip          int
	total         uint64
	maxScore      float64
	took          time.Duration
	sort          search.SortOrder
	results       search.DocumentMatchCollection
	facetsBuilder *search.FacetsBuilder

	store collectorStore

	needDocIds    bool
	neededFields  []string
	cachedScoring []bool
	cachedDesc    []bool

	lowestMatchOutsideResults *search.DocumentMatch
	updateFieldVisitor        index.DocumentFieldTermVisitor
	dvReader                  index.DocValueReader
	searchAfter               *search.DocumentMatch
}

// CheckDoneEvery controls how frequently we check the context deadline
const CheckDoneEvery = uint64(1024)

// NewTopNCollector builds a collector to find the top 'size' hits
// skipping over the first 'skip' hits
// ordering hits by the provided sort order
func NewTopNCollector(size int, skip int, sort search.SortOrder) *TopNCollector {
	return newTopNCollector(size, skip, sort)
}

// NewTopNCollector builds a collector to find the top 'size' hits
// skipping over the first 'skip' hits
// ordering hits by the provided sort order
func NewTopNCollectorAfter(size int, sort search.SortOrder, after []string) *TopNCollector {
	rv := newTopNCollector(size, 0, sort)
	rv.searchAfter = &search.DocumentMatch{
		Sort: after,
	}

	for pos, ss := range sort {
		if ss.RequiresDocID() {
			rv.searchAfter.ID = after[pos]
		}
		if ss.RequiresScoring() {
			if score, err := strconv.ParseFloat(after[pos], 64); err == nil {
				rv.searchAfter.Score = score
			}
		}
	}

	return rv
}

func newTopNCollector(size int, skip int, sort search.SortOrder) *TopNCollector {
	hc := &TopNCollector{size: size, skip: skip, sort: sort}

	// pre-allocate space on the store to avoid reslicing
	// unless the size + skip is too large, then cap it
	// everything should still work, just reslices as necessary
	backingSize := size + skip + 1
	if size+skip > PreAllocSizeSkipCap {
		backingSize = PreAllocSizeSkipCap + 1
	}

	if size+skip > 10 {
		hc.store = newStoreHeap(backingSize, func(i, j *search.DocumentMatch) int {
			return hc.sort.Compare(hc.cachedScoring, hc.cachedDesc, i, j)
		})
	} else {
		hc.store = newStoreSlice(backingSize, func(i, j *search.DocumentMatch) int {
			return hc.sort.Compare(hc.cachedScoring, hc.cachedDesc, i, j)
		})
	}

	// these lookups traverse an interface, so do once up-front
	if sort.RequiresDocID() {
		hc.needDocIds = true
	}
	hc.neededFields = sort.RequiredFields()
	hc.cachedScoring = sort.CacheIsScore()
	hc.cachedDesc = sort.CacheDescending()

	return hc
}

func (hc *TopNCollector) Size() int {
	sizeInBytes := reflectStaticSizeTopNCollector + size.SizeOfPtr

	if hc.facetsBuilder != nil {
		sizeInBytes += hc.facetsBuilder.Size()
	}

	for _, entry := range hc.neededFields {
		sizeInBytes += len(entry) + size.SizeOfString
	}

	sizeInBytes += len(hc.cachedScoring) + len(hc.cachedDesc)

	return sizeInBytes
}

// Collect goes to the index to find the matching documents
func (hc *TopNCollector) Collect(ctx context.Context, searcher search.Searcher, reader index.IndexReader) error {
	startTime := time.Now()
	var err error
	var next *search.DocumentMatch

	// pre-allocate enough space in the DocumentMatchPool
	// unless the size + skip is too large, then cap it
	// everything should still work, just allocates DocumentMatches on demand
	backingSize := hc.size + hc.skip + 1
	if hc.size+hc.skip > PreAllocSizeSkipCap {
		backingSize = PreAllocSizeSkipCap + 1
	}
	searchContext := &search.SearchContext{
		DocumentMatchPool: search.NewDocumentMatchPool(backingSize+searcher.DocumentMatchPoolSize(), len(hc.sort)),
		Collector:         hc,
		IndexReader:       reader,
	}

	hc.dvReader, err = reader.DocValueReader(hc.neededFields)
	if err != nil {
		return err
	}

	hc.updateFieldVisitor = func(field string, term []byte) {
		if hc.facetsBuilder != nil {
			hc.facetsBuilder.UpdateVisitor(field, term)
		}
		hc.sort.UpdateVisitor(field, term)
	}

	dmHandlerMaker := MakeTopNDocumentMatchHandler
	if cv := ctx.Value(search.MakeDocumentMatchHandlerKey); cv != nil {
		dmHandlerMaker = cv.(search.MakeDocumentMatchHandler)
	}
	// use the application given builder for making the custom document match
	// handler and perform callbacks/invocations on the newly made handler.
	dmHandler, loadID, err := dmHandlerMaker(searchContext)
	if err != nil {
		return err
	}

	hc.needDocIds = hc.needDocIds || loadID

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		next, err = searcher.Next(searchContext)
	}
	for err == nil && next != nil {
		if hc.total%CheckDoneEvery == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		err = hc.prepareDocumentMatch(searchContext, reader, next)
		if err != nil {
			break
		}

		err = dmHandler(next)
		if err != nil {
			break
		}

		next, err = searcher.Next(searchContext)
	}

	// help finalize/flush the results in case
	// of custom document match handlers.
	err = dmHandler(nil)
	if err != nil {
		return err
	}

	// compute search duration
	hc.took = time.Since(startTime)
	if err != nil {
		return err
	}
	// finalize actual results
	err = hc.finalizeResults(reader)
	if err != nil {
		return err
	}
	return nil
}

var sortByScoreOpt = []string{"_score"}

func (hc *TopNCollector) prepareDocumentMatch(ctx *search.SearchContext,
	reader index.IndexReader, d *search.DocumentMatch) (err error) {

	// visit field terms for features that require it (sort, facets)
	if len(hc.neededFields) > 0 {
		err = hc.visitFieldTerms(reader, d)
		if err != nil {
			return err
		}
	}

	// increment total hits
	hc.total++
	d.HitNumber = hc.total

	// update max score
	if d.Score > hc.maxScore {
		hc.maxScore = d.Score
	}

	// see if we need to load ID (at this early stage, for example to sort on it)
	if hc.needDocIds {
		d.ID, err = reader.ExternalID(d.IndexInternalID)
		if err != nil {
			return err
		}
	}

	// compute this hits sort value
	if len(hc.sort) == 1 && hc.cachedScoring[0] {
		d.Sort = sortByScoreOpt
	} else {
		hc.sort.Value(d)
	}

	return nil
}

func MakeTopNDocumentMatchHandler(
	ctx *search.SearchContext) (search.DocumentMatchHandler, bool, error) {
	var hc *TopNCollector
	var ok bool
	if hc, ok = ctx.Collector.(*TopNCollector); ok {
		return func(d *search.DocumentMatch) error {
			if d == nil {
				return nil
			}

			// support search after based pagination,
			// if this hit is <= the search after sort key
			// we should skip it
			if hc.searchAfter != nil {
				// exact sort order matches use hit number to break tie
				// but we want to allow for exact match, so we pretend
				hc.searchAfter.HitNumber = d.HitNumber
				if hc.sort.Compare(hc.cachedScoring, hc.cachedDesc, d, hc.searchAfter) <= 0 {
					return nil
				}
			}

			// optimization, we track lowest sorting hit already removed from heap
			// with this one comparison, we can avoid all heap operations if
			// this hit would have been added and then immediately removed
			if hc.lowestMatchOutsideResults != nil {
				cmp := hc.sort.Compare(hc.cachedScoring, hc.cachedDesc, d,
					hc.lowestMatchOutsideResults)
				if cmp >= 0 {
					// this hit can't possibly be in the result set, so avoid heap ops
					ctx.DocumentMatchPool.Put(d)
					return nil
				}
			}

			removed := hc.store.AddNotExceedingSize(d, hc.size+hc.skip)
			if removed != nil {
				if hc.lowestMatchOutsideResults == nil {
					hc.lowestMatchOutsideResults = removed
				} else {
					cmp := hc.sort.Compare(hc.cachedScoring, hc.cachedDesc,
						removed, hc.lowestMatchOutsideResults)
					if cmp < 0 {
						tmp := hc.lowestMatchOutsideResults
						hc.lowestMatchOutsideResults = removed
						ctx.DocumentMatchPool.Put(tmp)
					}
				}
			}
			return nil
		}, false, nil
	}
	return nil, false, nil
}

// visitFieldTerms is responsible for visiting the field terms of the
// search hit, and passing visited terms to the sort and facet builder
func (hc *TopNCollector) visitFieldTerms(reader index.IndexReader, d *search.DocumentMatch) error {
	if hc.facetsBuilder != nil {
		hc.facetsBuilder.StartDoc()
	}

	err := hc.dvReader.VisitDocValues(d.IndexInternalID, hc.updateFieldVisitor)
	if hc.facetsBuilder != nil {
		hc.facetsBuilder.EndDoc()
	}

	return err
}

// SetFacetsBuilder registers a facet builder for this collector
func (hc *TopNCollector) SetFacetsBuilder(facetsBuilder *search.FacetsBuilder) {
	hc.facetsBuilder = facetsBuilder
	hc.neededFields = append(hc.neededFields, hc.facetsBuilder.RequiredFields()...)
}

// finalizeResults starts with the heap containing the final top size+skip
// it now throws away the results to be skipped
// and does final doc id lookup (if necessary)
func (hc *TopNCollector) finalizeResults(r index.IndexReader) error {
	var err error
	hc.results, err = hc.store.Final(hc.skip, func(doc *search.DocumentMatch) error {
		if doc.ID == "" {
			// look up the id since we need it for lookup
			var err error
			doc.ID, err = r.ExternalID(doc.IndexInternalID)
			if err != nil {
				return err
			}
		}
		doc.Complete(nil)
		return nil
	})

	return err
}

// Results returns the collected hits
func (hc *TopNCollector) Results() search.DocumentMatchCollection {
	return hc.results
}

// Total returns the total number of hits
func (hc *TopNCollector) Total() uint64 {
	return hc.total
}

// MaxScore returns the maximum score seen across all the hits
func (hc *TopNCollector) MaxScore() float64 {
	return hc.maxScore
}

// Took returns the time spent collecting hits
func (hc *TopNCollector) Took() time.Duration {
	return hc.took
}

// FacetResults returns the computed facets results
func (hc *TopNCollector) FacetResults() search.FacetResults {
	if hc.facetsBuilder != nil {
		return hc.facetsBuilder.Results()
	}
	return nil
}
