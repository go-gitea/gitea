// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.
package elastic

// ScriptedMetricAggregation is a a metric aggregation that executes using scripts to provide a metric output.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-metrics-scripted-metric-aggregation.html
type ScriptedMetricAggregation struct {
	initScript    *Script
	mapScript     *Script
	combineScript *Script
	reduceScript  *Script

	params map[string]interface{}
	meta   map[string]interface{}
}

func NewScriptedMetricAggregation() *ScriptedMetricAggregation {
	a := &ScriptedMetricAggregation{}
	return a
}

func (a *ScriptedMetricAggregation) InitScript(script *Script) *ScriptedMetricAggregation {
	a.initScript = script
	return a
}

func (a *ScriptedMetricAggregation) MapScript(script *Script) *ScriptedMetricAggregation {
	a.mapScript = script
	return a
}

func (a *ScriptedMetricAggregation) CombineScript(script *Script) *ScriptedMetricAggregation {
	a.combineScript = script
	return a
}

func (a *ScriptedMetricAggregation) ReduceScript(script *Script) *ScriptedMetricAggregation {
	a.reduceScript = script
	return a
}

func (a *ScriptedMetricAggregation) Params(params map[string]interface{}) *ScriptedMetricAggregation {
	a.params = params
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *ScriptedMetricAggregation) Meta(metaData map[string]interface{}) *ScriptedMetricAggregation {
	a.meta = metaData
	return a
}

func (a *ScriptedMetricAggregation) Source() (interface{}, error) {
	// Example:
	//	{
	//    "aggs" : {
	//      "magic_script" : { "scripted_metric" : {
	//  		"init_script" : "state.transactions = []",
	//		  	"map_script" : "state.transactions.add(doc.type.value == 'sale' ? doc.amount.value : -1 * doc.amount.value)",
	//		  	"combine_script" : "double profit = 0; for (t in state.transactions) { profit += t } return profit",
	//		  	"reduce_script" : "double profit = 0; for (a in states) { profit += a } return profit"
	//      } }
	//    }
	//	}
	// This method returns only the { "scripted_metric" : { ... } } part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["scripted_metric"] = opts

	if a.initScript != nil {
		src, err := a.initScript.Source()
		if err != nil {
			return nil, err
		}
		opts["init_script"] = src
	}
	if a.mapScript != nil {
		src, err := a.mapScript.Source()
		if err != nil {
			return nil, err
		}
		opts["map_script"] = src
	}
	if a.combineScript != nil {
		src, err := a.combineScript.Source()
		if err != nil {
			return nil, err
		}
		opts["combine_script"] = src
	}
	if a.reduceScript != nil {
		src, err := a.reduceScript.Source()
		if err != nil {
			return nil, err
		}
		opts["reduce_script"] = src
	}

	if a.params != nil && len(a.params) > 0 {
		opts["params"] = a.params
	}

	// Add Meta data if available
	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil
}
