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

package search

import (
	"fmt"
	"reflect"

	"github.com/blevesearch/bleve/document"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/size"
)

var reflectStaticSizeDocumentMatch int
var reflectStaticSizeSearchContext int
var reflectStaticSizeLocation int

func init() {
	var dm DocumentMatch
	reflectStaticSizeDocumentMatch = int(reflect.TypeOf(dm).Size())
	var sc SearchContext
	reflectStaticSizeSearchContext = int(reflect.TypeOf(sc).Size())
	var l Location
	reflectStaticSizeLocation = int(reflect.TypeOf(l).Size())
}

type ArrayPositions []uint64

func (ap ArrayPositions) Equals(other ArrayPositions) bool {
	if len(ap) != len(other) {
		return false
	}
	for i := range ap {
		if ap[i] != other[i] {
			return false
		}
	}
	return true
}

type Location struct {
	// Pos is the position of the term within the field, starting at 1
	Pos uint64 `json:"pos"`

	// Start and End are the byte offsets of the term in the field
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`

	// ArrayPositions contains the positions of the term within any elements.
	ArrayPositions ArrayPositions `json:"array_positions"`
}

func (l *Location) Size() int {
	return reflectStaticSizeLocation + size.SizeOfPtr +
		len(l.ArrayPositions)*size.SizeOfUint64
}

type Locations []*Location

type TermLocationMap map[string]Locations

func (t TermLocationMap) AddLocation(term string, location *Location) {
	t[term] = append(t[term], location)
}

type FieldTermLocationMap map[string]TermLocationMap

type FieldTermLocation struct {
	Field    string
	Term     string
	Location Location
}

type FieldFragmentMap map[string][]string

type DocumentMatch struct {
	Index           string                `json:"index,omitempty"`
	ID              string                `json:"id"`
	IndexInternalID index.IndexInternalID `json:"-"`
	Score           float64               `json:"score"`
	Expl            *Explanation          `json:"explanation,omitempty"`
	Locations       FieldTermLocationMap  `json:"locations,omitempty"`
	Fragments       FieldFragmentMap      `json:"fragments,omitempty"`
	Sort            []string              `json:"sort,omitempty"`

	// Fields contains the values for document fields listed in
	// SearchRequest.Fields. Text fields are returned as strings, numeric
	// fields as float64s and date fields as time.RFC3339 formatted strings.
	Fields map[string]interface{} `json:"fields,omitempty"`

	// if we load the document for this hit, remember it so we dont load again
	Document *document.Document `json:"-"`

	// used to maintain natural index order
	HitNumber uint64 `json:"-"`

	// used to temporarily hold field term location information during
	// search processing in an efficient, recycle-friendly manner, to
	// be later incorporated into the Locations map when search
	// results are completed
	FieldTermLocations []FieldTermLocation `json:"-"`
}

func (dm *DocumentMatch) AddFieldValue(name string, value interface{}) {
	if dm.Fields == nil {
		dm.Fields = make(map[string]interface{})
	}
	existingVal, ok := dm.Fields[name]
	if !ok {
		dm.Fields[name] = value
		return
	}

	valSlice, ok := existingVal.([]interface{})
	if ok {
		// already a slice, append to it
		valSlice = append(valSlice, value)
	} else {
		// create a slice
		valSlice = []interface{}{existingVal, value}
	}
	dm.Fields[name] = valSlice
}

// Reset allows an already allocated DocumentMatch to be reused
func (dm *DocumentMatch) Reset() *DocumentMatch {
	// remember the []byte used for the IndexInternalID
	indexInternalID := dm.IndexInternalID
	// remember the []interface{} used for sort
	sort := dm.Sort
	// remember the FieldTermLocations backing array
	ftls := dm.FieldTermLocations
	for i := range ftls { // recycle the ArrayPositions of each location
		ftls[i].Location.ArrayPositions = ftls[i].Location.ArrayPositions[:0]
	}
	// idiom to copy over from empty DocumentMatch (0 allocations)
	*dm = DocumentMatch{}
	// reuse the []byte already allocated (and reset len to 0)
	dm.IndexInternalID = indexInternalID[:0]
	// reuse the []interface{} already allocated (and reset len to 0)
	dm.Sort = sort[:0]
	// reuse the FieldTermLocations already allocated (and reset len to 0)
	dm.FieldTermLocations = ftls[:0]
	return dm
}

func (dm *DocumentMatch) Size() int {
	sizeInBytes := reflectStaticSizeDocumentMatch + size.SizeOfPtr +
		len(dm.Index) +
		len(dm.ID) +
		len(dm.IndexInternalID)

	if dm.Expl != nil {
		sizeInBytes += dm.Expl.Size()
	}

	for k, v := range dm.Locations {
		sizeInBytes += size.SizeOfString + len(k)
		for k1, v1 := range v {
			sizeInBytes += size.SizeOfString + len(k1) +
				size.SizeOfSlice
			for _, entry := range v1 {
				sizeInBytes += entry.Size()
			}
		}
	}

	for k, v := range dm.Fragments {
		sizeInBytes += size.SizeOfString + len(k) +
			size.SizeOfSlice

		for _, entry := range v {
			sizeInBytes += size.SizeOfString + len(entry)
		}
	}

	for _, entry := range dm.Sort {
		sizeInBytes += size.SizeOfString + len(entry)
	}

	for k, _ := range dm.Fields {
		sizeInBytes += size.SizeOfString + len(k) +
			size.SizeOfPtr
	}

	if dm.Document != nil {
		sizeInBytes += dm.Document.Size()
	}

	return sizeInBytes
}

// Complete performs final preparation & transformation of the
// DocumentMatch at the end of search processing, also allowing the
// caller to provide an optional preallocated locations slice
func (dm *DocumentMatch) Complete(prealloc []Location) []Location {
	// transform the FieldTermLocations slice into the Locations map
	nlocs := len(dm.FieldTermLocations)
	if nlocs > 0 {
		if cap(prealloc) < nlocs {
			prealloc = make([]Location, nlocs)
		}
		prealloc = prealloc[:nlocs]

		var lastField string
		var tlm TermLocationMap

		for i, ftl := range dm.FieldTermLocations {
			if lastField != ftl.Field {
				lastField = ftl.Field

				if dm.Locations == nil {
					dm.Locations = make(FieldTermLocationMap)
				}

				tlm = dm.Locations[ftl.Field]
				if tlm == nil {
					tlm = make(TermLocationMap)
					dm.Locations[ftl.Field] = tlm
				}
			}

			loc := &prealloc[i]
			*loc = ftl.Location

			if len(loc.ArrayPositions) > 0 { // copy
				loc.ArrayPositions = append(ArrayPositions(nil), loc.ArrayPositions...)
			}

			tlm[ftl.Term] = append(tlm[ftl.Term], loc)

			dm.FieldTermLocations[i] = FieldTermLocation{ // recycle
				Location: Location{
					ArrayPositions: ftl.Location.ArrayPositions[:0],
				},
			}
		}
	}

	dm.FieldTermLocations = dm.FieldTermLocations[:0] // recycle

	return prealloc
}

func (dm *DocumentMatch) String() string {
	return fmt.Sprintf("[%s-%f]", string(dm.IndexInternalID), dm.Score)
}

type DocumentMatchCollection []*DocumentMatch

func (c DocumentMatchCollection) Len() int           { return len(c) }
func (c DocumentMatchCollection) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c DocumentMatchCollection) Less(i, j int) bool { return c[i].Score > c[j].Score }

type Searcher interface {
	Next(ctx *SearchContext) (*DocumentMatch, error)
	Advance(ctx *SearchContext, ID index.IndexInternalID) (*DocumentMatch, error)
	Close() error
	Weight() float64
	SetQueryNorm(float64)
	Count() uint64
	Min() int
	Size() int

	DocumentMatchPoolSize() int
}

type SearcherOptions struct {
	Explain            bool
	IncludeTermVectors bool
}

// SearchContext represents the context around a single search
type SearchContext struct {
	DocumentMatchPool *DocumentMatchPool
}

func (sc *SearchContext) Size() int {
	sizeInBytes := reflectStaticSizeSearchContext + size.SizeOfPtr +
		reflectStaticSizeDocumentMatchPool + size.SizeOfPtr

	if sc.DocumentMatchPool != nil {
		for _, entry := range sc.DocumentMatchPool.avail {
			if entry != nil {
				sizeInBytes += entry.Size()
			}
		}
	}

	return sizeInBytes
}
