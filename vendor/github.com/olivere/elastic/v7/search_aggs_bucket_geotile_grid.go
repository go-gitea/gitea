package elastic

import "errors"

type GeoTileGridAggregation struct {
	field           string
	precision       int
	size            int
	shardSize       int
	bounds          *BoundingBox
	subAggregations map[string]Aggregation
	meta            map[string]interface{}
}

// NewGeoTileGridAggregation Create new bucket aggregation of Geotile grid type
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-bucket-geotilegrid-aggregation.html
func NewGeoTileGridAggregation() *GeoTileGridAggregation {
	return &GeoTileGridAggregation{
		precision:       -1,
		size:            -1,
		shardSize:       -1,
		subAggregations: make(map[string]Aggregation),
	}
}

// Field The name of the field indexed with GeoPoints. Mandatory.
func (a *GeoTileGridAggregation) Field(field string) *GeoTileGridAggregation {
	a.field = field
	return a
}

// Precision The integer zoom of the key used to define cells/buckets in the results. Defaults to 7. Values outside of [0,29] will be rejected. Optional.
func (a *GeoTileGridAggregation) Precision(precision int) *GeoTileGridAggregation {
	a.precision = precision
	return a
}

// Size The maximum number of buckets to return in the result structure. Optional.
func (a *GeoTileGridAggregation) Size(size int) *GeoTileGridAggregation {
	a.size = size
	return a
}

// ShardSize The maximum number of buckets to return from each shard. Optional.
func (a *GeoTileGridAggregation) ShardSize(shardSize int) *GeoTileGridAggregation {
	a.shardSize = shardSize
	return a
}

// Bounds The bounding box to filter the points in the bucket. Optional.
func (a *GeoTileGridAggregation) Bounds(boundingBox BoundingBox) *GeoTileGridAggregation {
	a.bounds = &boundingBox
	return a
}

// SubAggregation Adds a sub-aggregation to this aggregation.
func (a *GeoTileGridAggregation) SubAggregation(name string, subAggregation Aggregation) *GeoTileGridAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta Sets the meta data to be included in the aggregation response.
func (a *GeoTileGridAggregation) Meta(metaData map[string]interface{}) *GeoTileGridAggregation {
	a.meta = metaData
	return a
}

// Source returns the a JSON-serializable interface.
func (a *GeoTileGridAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["geotile_grid"] = opts

	if a.field == "" {
		return nil, errors.New("elastic: 'field' is a mandatory parameter")
	}
	opts["field"] = a.field

	if a.precision != -1 {
		opts["precision"] = a.precision
	}

	if a.size != -1 {
		opts["size"] = a.size
	}

	if a.shardSize != -1 {
		opts["shard_size"] = a.shardSize
	}

	if a.bounds != nil {
		opts["bounds"] = *a.bounds
	}

	if len(a.subAggregations) > 0 {
		aggsMap := make(map[string]interface{})
		source["aggregations"] = aggsMap
		for name, aggregate := range a.subAggregations {
			src, err := aggregate.Source()
			if err != nil {
				return nil, err
			}
			aggsMap[name] = src
		}
	}

	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil

}

// BoundingBox bounding box
type BoundingBox struct {
	TopLeft     GeoPoint `json:"top_left"`
	BottomRight GeoPoint `json:"bottom_right"`
}
