// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// WeightedAvgAggregation is a single-value metrics aggregation that
// computes the weighted average of numeric values that are extracted
// from the aggregated documents. These values can be extracted either
// from specific numeric fields in the documents.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-metrics-weight-avg-aggregation.html
type WeightedAvgAggregation struct {
	fields          map[string]*MultiValuesSourceFieldConfig
	valueType       string
	format          string
	value           *MultiValuesSourceFieldConfig
	weight          *MultiValuesSourceFieldConfig
	subAggregations map[string]Aggregation
	meta            map[string]interface{}
}

func NewWeightedAvgAggregation() *WeightedAvgAggregation {
	return &WeightedAvgAggregation{
		fields:          make(map[string]*MultiValuesSourceFieldConfig),
		subAggregations: make(map[string]Aggregation),
	}
}

func (a *WeightedAvgAggregation) Field(field string, config *MultiValuesSourceFieldConfig) *WeightedAvgAggregation {
	a.fields[field] = config
	return a
}

func (a *WeightedAvgAggregation) ValueType(valueType string) *WeightedAvgAggregation {
	a.valueType = valueType
	return a
}

func (a *WeightedAvgAggregation) Format(format string) *WeightedAvgAggregation {
	a.format = format
	return a
}

func (a *WeightedAvgAggregation) Value(value *MultiValuesSourceFieldConfig) *WeightedAvgAggregation {
	a.value = value
	return a
}

func (a *WeightedAvgAggregation) Weight(weight *MultiValuesSourceFieldConfig) *WeightedAvgAggregation {
	a.weight = weight
	return a
}

func (a *WeightedAvgAggregation) SubAggregation(name string, subAggregation Aggregation) *WeightedAvgAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *WeightedAvgAggregation) Meta(metaData map[string]interface{}) *WeightedAvgAggregation {
	a.meta = metaData
	return a
}

func (a *WeightedAvgAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["weighted_avg"] = opts

	if len(a.fields) > 0 {
		f := make(map[string]interface{})
		for name, config := range a.fields {
			cfg, err := config.Source()
			if err != nil {
				return nil, err
			}
			f[name] = cfg
		}
		opts["fields"] = f
	}

	if v := a.format; v != "" {
		opts["format"] = v
	}

	if v := a.valueType; v != "" {
		opts["value_type"] = v
	}

	if v := a.value; v != nil {
		cfg, err := v.Source()
		if err != nil {
			return nil, err
		}
		opts["value"] = cfg
	}

	if v := a.weight; v != nil {
		cfg, err := v.Source()
		if err != nil {
			return nil, err
		}
		opts["weight"] = cfg
	}

	// AggregationBuilder (SubAggregations)
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

	// Add Meta data if available
	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil
}

// MultiValuesSourceFieldConfig represents a field configuration
// used e.g. in WeightedAvgAggregation.
type MultiValuesSourceFieldConfig struct {
	FieldName string
	Missing   interface{}
	Script    *Script
	TimeZone  string
}

func (f *MultiValuesSourceFieldConfig) Source() (interface{}, error) {
	source := make(map[string]interface{})
	if v := f.Missing; v != nil {
		source["missing"] = v
	}
	if v := f.Script; v != nil {
		src, err := v.Source()
		if err != nil {
			return nil, err
		}
		source["script"] = src
	}
	if v := f.FieldName; v != "" {
		source["field"] = v
	}
	if v := f.TimeZone; v != "" {
		source["time_zone"] = v
	}
	return source, nil
}
