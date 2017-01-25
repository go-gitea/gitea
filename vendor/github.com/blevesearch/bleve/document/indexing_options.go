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

package document

type IndexingOptions int

const (
	IndexField IndexingOptions = 1 << iota
	StoreField
	IncludeTermVectors
)

func (o IndexingOptions) IsIndexed() bool {
	return o&IndexField != 0
}

func (o IndexingOptions) IsStored() bool {
	return o&StoreField != 0
}

func (o IndexingOptions) IncludeTermVectors() bool {
	return o&IncludeTermVectors != 0
}

func (o IndexingOptions) String() string {
	rv := ""
	if o.IsIndexed() {
		rv += "INDEXED"
	}
	if o.IsStored() {
		if rv != "" {
			rv += ", "
		}
		rv += "STORE"
	}
	if o.IncludeTermVectors() {
		if rv != "" {
			rv += ", "
		}
		rv += "TV"
	}
	return rv
}
