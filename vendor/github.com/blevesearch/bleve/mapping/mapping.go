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

package mapping

import (
	"io/ioutil"
	"log"

	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/document"
)

// A Classifier is an interface describing any object which knows how to
// identify its own type.  Alternatively, if a struct already has a Type
// field or method in conflict, one can use BleveType instead.
type Classifier interface {
	Type() string
}

// A bleveClassifier is an interface describing any object which knows how
// to identify its own type.  This is introduced as an alternative to the
// Classifier interface which often has naming conflicts with existing
// structures.
type bleveClassifier interface {
	BleveType() string
}

var logger = log.New(ioutil.Discard, "bleve mapping ", log.LstdFlags)

// SetLog sets the logger used for logging
// by default log messages are sent to ioutil.Discard
func SetLog(l *log.Logger) {
	logger = l
}

type IndexMapping interface {
	MapDocument(doc *document.Document, data interface{}) error
	Validate() error

	DateTimeParserNamed(name string) analysis.DateTimeParser

	DefaultSearchField() string

	AnalyzerNameForPath(path string) string
	AnalyzerNamed(name string) *analysis.Analyzer
}
