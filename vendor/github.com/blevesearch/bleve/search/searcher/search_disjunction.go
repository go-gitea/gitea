//  Copyright (c) 2018 Couchbase, Inc.
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

package searcher

import (
	"fmt"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
)

// DisjunctionMaxClauseCount is a compile time setting that applications can
// adjust to non-zero value to cause the DisjunctionSearcher to return an
// error instead of exeucting searches when the size exceeds this value.
var DisjunctionMaxClauseCount = 0

// DisjunctionHeapTakeover is a compile time setting that applications can
// adjust to control when the DisjunctionSearcher will switch from a simple
// slice implementation to a heap implementation.
var DisjunctionHeapTakeover = 10

func NewDisjunctionSearcher(indexReader index.IndexReader,
	qsearchers []search.Searcher, min float64, options search.SearcherOptions) (
	search.Searcher, error) {
	return newDisjunctionSearcher(indexReader, qsearchers, min, options, true)
}

func newDisjunctionSearcher(indexReader index.IndexReader,
	qsearchers []search.Searcher, min float64, options search.SearcherOptions,
	limit bool) (search.Searcher, error) {
	if len(qsearchers) > DisjunctionHeapTakeover {
		return newDisjunctionHeapSearcher(indexReader, qsearchers, min, options,
			limit)
	}
	return newDisjunctionSliceSearcher(indexReader, qsearchers, min, options,
		limit)
}

func tooManyClauses(count int) bool {
	if DisjunctionMaxClauseCount != 0 && count > DisjunctionMaxClauseCount {
		return true
	}
	return false
}

func tooManyClausesErr() error {
	return fmt.Errorf("TooManyClauses[maxClauseCount is set to %d]",
		DisjunctionMaxClauseCount)
}
