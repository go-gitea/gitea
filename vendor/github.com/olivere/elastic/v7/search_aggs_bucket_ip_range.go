// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// IPRangeAggregation is a range aggregation that is dedicated for
// IP addresses.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-bucket-iprange-aggregation.html
type IPRangeAggregation struct {
	field           string
	subAggregations map[string]Aggregation
	meta            map[string]interface{}
	keyed           *bool
	entries         []IPRangeAggregationEntry
}

type IPRangeAggregationEntry struct {
	Key  string
	Mask string
	From string
	To   string
}

func NewIPRangeAggregation() *IPRangeAggregation {
	return &IPRangeAggregation{
		subAggregations: make(map[string]Aggregation),
		entries:         make([]IPRangeAggregationEntry, 0),
	}
}

func (a *IPRangeAggregation) Field(field string) *IPRangeAggregation {
	a.field = field
	return a
}

func (a *IPRangeAggregation) SubAggregation(name string, subAggregation Aggregation) *IPRangeAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *IPRangeAggregation) Meta(metaData map[string]interface{}) *IPRangeAggregation {
	a.meta = metaData
	return a
}

func (a *IPRangeAggregation) Keyed(keyed bool) *IPRangeAggregation {
	a.keyed = &keyed
	return a
}

func (a *IPRangeAggregation) AddMaskRange(mask string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{Mask: mask})
	return a
}

func (a *IPRangeAggregation) AddMaskRangeWithKey(key, mask string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{Key: key, Mask: mask})
	return a
}

func (a *IPRangeAggregation) AddRange(from, to string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{From: from, To: to})
	return a
}

func (a *IPRangeAggregation) AddRangeWithKey(key, from, to string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{Key: key, From: from, To: to})
	return a
}

func (a *IPRangeAggregation) AddUnboundedTo(from string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{From: from, To: ""})
	return a
}

func (a *IPRangeAggregation) AddUnboundedToWithKey(key, from string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{Key: key, From: from, To: ""})
	return a
}

func (a *IPRangeAggregation) AddUnboundedFrom(to string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{From: "", To: to})
	return a
}

func (a *IPRangeAggregation) AddUnboundedFromWithKey(key, to string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{Key: key, From: "", To: to})
	return a
}

func (a *IPRangeAggregation) Lt(to string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{From: "", To: to})
	return a
}

func (a *IPRangeAggregation) LtWithKey(key, to string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{Key: key, From: "", To: to})
	return a
}

func (a *IPRangeAggregation) Between(from, to string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{From: from, To: to})
	return a
}

func (a *IPRangeAggregation) BetweenWithKey(key, from, to string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{Key: key, From: from, To: to})
	return a
}

func (a *IPRangeAggregation) Gt(from string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{From: from, To: ""})
	return a
}

func (a *IPRangeAggregation) GtWithKey(key, from string) *IPRangeAggregation {
	a.entries = append(a.entries, IPRangeAggregationEntry{Key: key, From: from, To: ""})
	return a
}

func (a *IPRangeAggregation) Source() (interface{}, error) {
	// Example:
	// {
	//     "aggs" : {
	//         "range" : {
	//             "ip_range": {
	//                 "field": "ip",
	//                 "ranges": [
	//                     { "to": "10.0.0.5" },
	//                     { "from": "10.0.0.5" }
	//                 ]
	//             }
	//         }
	//         }
	//     }
	// }
	//
	// This method returns only the { "ip_range" : { ... } } part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["ip_range"] = opts

	// ValuesSourceAggregationBuilder
	if a.field != "" {
		opts["field"] = a.field
	}

	if a.keyed != nil {
		opts["keyed"] = *a.keyed
	}

	var ranges []interface{}
	for _, ent := range a.entries {
		r := make(map[string]interface{})
		if ent.Key != "" {
			r["key"] = ent.Key
		}
		if ent.Mask != "" {
			r["mask"] = ent.Mask
		} else {
			if ent.From != "" {
				r["from"] = ent.From
			}
			if ent.To != "" {
				r["to"] = ent.To
			}
		}
		ranges = append(ranges, r)
	}
	opts["ranges"] = ranges

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
