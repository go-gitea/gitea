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

package mem

import (
	"math"
	"sort"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/document"
	"github.com/blevesearch/bleve/index"
)

// NewFromAnalyzedDocs places the analyzed document mutations into a new segment
func NewFromAnalyzedDocs(results []*index.AnalysisResult) *Segment {
	s := New()

	// ensure that _id field get fieldID 0
	s.getOrDefineField("_id")

	// fill Dicts/DictKeys and preallocate memory
	s.initializeDict(results)

	// walk each doc
	for _, result := range results {
		s.processDocument(result)
	}

	// go back and sort the dictKeys
	for _, dict := range s.DictKeys {
		sort.Strings(dict)
	}

	// compute memory usage of segment
	s.updateSizeInBytes()

	// professional debugging
	//
	// log.Printf("fields: %v\n", s.FieldsMap)
	// log.Printf("fieldsInv: %v\n", s.FieldsInv)
	// log.Printf("fieldsLoc: %v\n", s.FieldsLoc)
	// log.Printf("dicts: %v\n", s.Dicts)
	// log.Printf("dict keys: %v\n", s.DictKeys)
	// for i, posting := range s.Postings {
	// 	log.Printf("posting %d: %v\n", i, posting)
	// }
	// for i, freq := range s.Freqs {
	// 	log.Printf("freq %d: %v\n", i, freq)
	// }
	// for i, norm := range s.Norms {
	// 	log.Printf("norm %d: %v\n", i, norm)
	// }
	// for i, field := range s.Locfields {
	// 	log.Printf("field %d: %v\n", i, field)
	// }
	// for i, start := range s.Locstarts {
	// 	log.Printf("start %d: %v\n", i, start)
	// }
	// for i, end := range s.Locends {
	// 	log.Printf("end %d: %v\n", i, end)
	// }
	// for i, pos := range s.Locpos {
	// 	log.Printf("pos %d: %v\n", i, pos)
	// }
	// for i, apos := range s.Locarraypos {
	// 	log.Printf("apos %d: %v\n", i, apos)
	// }
	// log.Printf("stored: %v\n", s.Stored)
	// log.Printf("stored types: %v\n", s.StoredTypes)
	// log.Printf("stored pos: %v\n", s.StoredPos)

	return s
}

// fill Dicts/DictKeys and preallocate memory for postings
func (s *Segment) initializeDict(results []*index.AnalysisResult) {
	var numPostingsLists int

	numTermsPerPostingsList := make([]int, 0, 64) // Keyed by postings list id.
	numLocsPerPostingsList := make([]int, 0, 64)  // Keyed by postings list id.

	var numTokenFrequencies int
	var totLocs int

	// initial scan for all fieldID's to sort them
	for _, result := range results {
		for _, field := range result.Document.CompositeFields {
			s.getOrDefineField(field.Name())
		}
		for _, field := range result.Document.Fields {
			s.getOrDefineField(field.Name())
		}
	}
	sort.Strings(s.FieldsInv[1:]) // keep _id as first field
	s.FieldsMap = make(map[string]uint16, len(s.FieldsInv))
	for fieldID, fieldName := range s.FieldsInv {
		s.FieldsMap[fieldName] = uint16(fieldID + 1)
	}

	processField := func(fieldID uint16, tfs analysis.TokenFrequencies) {
		for term, tf := range tfs {
			pidPlus1, exists := s.Dicts[fieldID][term]
			if !exists {
				numPostingsLists++
				pidPlus1 = uint64(numPostingsLists)
				s.Dicts[fieldID][term] = pidPlus1
				s.DictKeys[fieldID] = append(s.DictKeys[fieldID], term)
				numTermsPerPostingsList = append(numTermsPerPostingsList, 0)
				numLocsPerPostingsList = append(numLocsPerPostingsList, 0)
			}
			pid := pidPlus1 - 1
			numTermsPerPostingsList[pid] += 1
			numLocsPerPostingsList[pid] += len(tf.Locations)
			totLocs += len(tf.Locations)
		}
		numTokenFrequencies += len(tfs)
	}

	for _, result := range results {
		// walk each composite field
		for _, field := range result.Document.CompositeFields {
			fieldID := uint16(s.getOrDefineField(field.Name()))
			_, tf := field.Analyze()
			processField(fieldID, tf)
		}

		// walk each field
		for i, field := range result.Document.Fields {
			fieldID := uint16(s.getOrDefineField(field.Name()))
			tf := result.Analyzed[i]
			processField(fieldID, tf)
		}
	}

	s.Postings = make([]*roaring.Bitmap, numPostingsLists)
	for i := 0; i < numPostingsLists; i++ {
		s.Postings[i] = roaring.New()
	}
	s.PostingsLocs = make([]*roaring.Bitmap, numPostingsLists)
	for i := 0; i < numPostingsLists; i++ {
		s.PostingsLocs[i] = roaring.New()
	}

	// Preallocate big, contiguous backing arrays.
	auint64Backing := make([][]uint64, numPostingsLists*4+totLocs) // For Freqs, Locstarts, Locends, Locpos, sub-Locarraypos.
	uint64Backing := make([]uint64, numTokenFrequencies+totLocs*3) // For sub-Freqs, sub-Locstarts, sub-Locends, sub-Locpos.
	float32Backing := make([]float32, numTokenFrequencies)         // For sub-Norms.
	uint16Backing := make([]uint16, totLocs)                       // For sub-Locfields.

	// Point top-level slices to the backing arrays.
	s.Freqs = auint64Backing[0:numPostingsLists]
	auint64Backing = auint64Backing[numPostingsLists:]

	s.Norms = make([][]float32, numPostingsLists)

	s.Locfields = make([][]uint16, numPostingsLists)

	s.Locstarts = auint64Backing[0:numPostingsLists]
	auint64Backing = auint64Backing[numPostingsLists:]

	s.Locends = auint64Backing[0:numPostingsLists]
	auint64Backing = auint64Backing[numPostingsLists:]

	s.Locpos = auint64Backing[0:numPostingsLists]
	auint64Backing = auint64Backing[numPostingsLists:]

	s.Locarraypos = make([][][]uint64, numPostingsLists)

	// Point sub-slices to the backing arrays.
	for pid, numTerms := range numTermsPerPostingsList {
		s.Freqs[pid] = uint64Backing[0:0]
		uint64Backing = uint64Backing[numTerms:]

		s.Norms[pid] = float32Backing[0:0]
		float32Backing = float32Backing[numTerms:]
	}

	for pid, numLocs := range numLocsPerPostingsList {
		s.Locfields[pid] = uint16Backing[0:0]
		uint16Backing = uint16Backing[numLocs:]

		s.Locstarts[pid] = uint64Backing[0:0]
		uint64Backing = uint64Backing[numLocs:]

		s.Locends[pid] = uint64Backing[0:0]
		uint64Backing = uint64Backing[numLocs:]

		s.Locpos[pid] = uint64Backing[0:0]
		uint64Backing = uint64Backing[numLocs:]

		s.Locarraypos[pid] = auint64Backing[0:0]
		auint64Backing = auint64Backing[numLocs:]
	}
}

func (s *Segment) processDocument(result *index.AnalysisResult) {
	// used to collate information across fields
	docMap := make(map[uint16]analysis.TokenFrequencies, len(s.FieldsMap))
	fieldLens := make(map[uint16]int, len(s.FieldsMap))

	docNum := uint64(s.addDocument())

	processField := func(field uint16, name string, l int, tf analysis.TokenFrequencies) {
		fieldLens[field] += l
		if existingFreqs, ok := docMap[field]; ok {
			existingFreqs.MergeAll(name, tf)
		} else {
			docMap[field] = tf
		}
	}

	storeField := func(docNum uint64, field uint16, typ byte, val []byte, pos []uint64) {
		s.Stored[docNum][field] = append(s.Stored[docNum][field], val)
		s.StoredTypes[docNum][field] = append(s.StoredTypes[docNum][field], typ)
		s.StoredPos[docNum][field] = append(s.StoredPos[docNum][field], pos)
	}

	// walk each composite field
	for _, field := range result.Document.CompositeFields {
		fieldID := uint16(s.getOrDefineField(field.Name()))
		l, tf := field.Analyze()
		processField(fieldID, field.Name(), l, tf)
	}

	// walk each field
	for i, field := range result.Document.Fields {
		fieldID := uint16(s.getOrDefineField(field.Name()))
		l := result.Length[i]
		tf := result.Analyzed[i]
		processField(fieldID, field.Name(), l, tf)
		if field.Options().IsStored() {
			storeField(docNum, fieldID, encodeFieldType(field), field.Value(), field.ArrayPositions())
		}

		if field.Options().IncludeDocValues() {
			s.DocValueFields[fieldID] = true
		}
	}

	// now that its been rolled up into docMap, walk that
	for fieldID, tokenFrequencies := range docMap {
		for term, tokenFreq := range tokenFrequencies {
			pid := s.Dicts[fieldID][term] - 1
			bs := s.Postings[pid]
			bs.AddInt(int(docNum))
			s.Freqs[pid] = append(s.Freqs[pid], uint64(tokenFreq.Frequency()))
			s.Norms[pid] = append(s.Norms[pid], float32(1.0/math.Sqrt(float64(fieldLens[fieldID]))))
			locationBS := s.PostingsLocs[pid]
			if len(tokenFreq.Locations) > 0 {
				locationBS.AddInt(int(docNum))
				for _, loc := range tokenFreq.Locations {
					var locf = fieldID
					if loc.Field != "" {
						locf = uint16(s.getOrDefineField(loc.Field))
					}
					s.Locfields[pid] = append(s.Locfields[pid], locf)
					s.Locstarts[pid] = append(s.Locstarts[pid], uint64(loc.Start))
					s.Locends[pid] = append(s.Locends[pid], uint64(loc.End))
					s.Locpos[pid] = append(s.Locpos[pid], uint64(loc.Position))
					if len(loc.ArrayPositions) > 0 {
						s.Locarraypos[pid] = append(s.Locarraypos[pid], loc.ArrayPositions)
					} else {
						s.Locarraypos[pid] = append(s.Locarraypos[pid], nil)
					}
				}
			}
		}
	}
}

func (s *Segment) getOrDefineField(name string) int {
	fieldIDPlus1, ok := s.FieldsMap[name]
	if !ok {
		fieldIDPlus1 = uint16(len(s.FieldsInv) + 1)
		s.FieldsMap[name] = fieldIDPlus1
		s.FieldsInv = append(s.FieldsInv, name)
		s.Dicts = append(s.Dicts, make(map[string]uint64))
		s.DictKeys = append(s.DictKeys, make([]string, 0))
	}
	return int(fieldIDPlus1 - 1)
}

func (s *Segment) addDocument() int {
	docNum := len(s.Stored)
	s.Stored = append(s.Stored, map[uint16][][]byte{})
	s.StoredTypes = append(s.StoredTypes, map[uint16][]byte{})
	s.StoredPos = append(s.StoredPos, map[uint16][][]uint64{})
	return docNum
}

func encodeFieldType(f document.Field) byte {
	fieldType := byte('x')
	switch f.(type) {
	case *document.TextField:
		fieldType = 't'
	case *document.NumericField:
		fieldType = 'n'
	case *document.DateTimeField:
		fieldType = 'd'
	case *document.BooleanField:
		fieldType = 'b'
	case *document.GeoPointField:
		fieldType = 'g'
	case *document.CompositeField:
		fieldType = 'c'
	}
	return fieldType
}
