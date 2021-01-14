//  Copyright (c) 2020 Couchbase, Inc.
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

// Plugin represents the essential functions required by a package to plug in
// it's segment implementation
type Plugin interface {

	// Type is the name for this segment plugin
	Type() string

	// Version is a numeric value identifying a specific version of this type.
	// When incompatible changes are made to a particular type of plugin, the
	// version must be incremented.
	Version() uint32

	// New takes a set of AnalysisResults and turns them into a new Segment
	New(results []*index.AnalysisResult) (Segment, uint64, error)

	// Open attempts to open the file at the specified path and
	// return the corresponding Segment
	Open(path string) (Segment, error)

	// Merge takes a set of Segments, and creates a new segment on disk at
	// the specified path.
	// Drops is a set of bitmaps (one for each segment) indicating which
	// documents can be dropped from the segments during the merge.
	// If the closeCh channel is closed, Merge will cease doing work at
	// the next opportunity, and return an error (closed).
	// StatsReporter can optionally be provided, in which case progress
	// made during the merge is reported while operation continues.
	// Returns:
	// A slice of new document numbers (one for each input segment),
	// this allows the caller to know a particular document's new
	// document number in the newly merged segment.
	// The number of bytes written to the new segment file.
	// An error, if any occurred.
	Merge(segments []Segment, drops []*roaring.Bitmap, path string,
		closeCh chan struct{}, s StatsReporter) (
		[][]uint64, uint64, error)
}
