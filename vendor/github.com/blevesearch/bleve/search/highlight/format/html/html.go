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

package html

import (
	"github.com/blevesearch/bleve/registry"
	"github.com/blevesearch/bleve/search/highlight"
)

const Name = "html"

const defaultHTMLHighlightBefore = "<mark>"
const defaultHTMLHighlightAfter = "</mark>"

type FragmentFormatter struct {
	before string
	after  string
}

func NewFragmentFormatter(before, after string) *FragmentFormatter {
	return &FragmentFormatter{
		before: before,
		after:  after,
	}
}

func (a *FragmentFormatter) Format(f *highlight.Fragment, orderedTermLocations highlight.TermLocations) string {
	rv := ""
	curr := f.Start
	for _, termLocation := range orderedTermLocations {
		if termLocation == nil {
			continue
		}
		// make sure the array positions match
		if !termLocation.ArrayPositions.Equals(f.ArrayPositions) {
			continue
		}
		if termLocation.Start < curr {
			continue
		}
		if termLocation.End > f.End {
			break
		}
		// add the stuff before this location
		rv += string(f.Orig[curr:termLocation.Start])
		// add the color
		rv += a.before
		// add the term itself
		rv += string(f.Orig[termLocation.Start:termLocation.End])
		// reset the color
		rv += a.after
		// update current
		curr = termLocation.End
	}
	// add any remaining text after the last token
	rv += string(f.Orig[curr:f.End])

	return rv
}

func Constructor(config map[string]interface{}, cache *registry.Cache) (highlight.FragmentFormatter, error) {
	before := defaultHTMLHighlightBefore
	beforeVal, ok := config["before"].(string)
	if ok {
		before = beforeVal
	}
	after := defaultHTMLHighlightAfter
	afterVal, ok := config["after"].(string)
	if ok {
		after = afterVal
	}
	return NewFragmentFormatter(before, after), nil
}

func init() {
	registry.RegisterFragmentFormatter(Name, Constructor)
}
