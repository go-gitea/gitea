// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// BucketCountThresholds is used in e.g. terms and significant text aggregations.
type BucketCountThresholds struct {
	MinDocCount      *int64
	ShardMinDocCount *int64
	RequiredSize     *int
	ShardSize        *int
}
