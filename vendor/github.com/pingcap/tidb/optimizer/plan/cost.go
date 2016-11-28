// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package plan

import (
	"math"
)

// Pre-defined cost factors.
const (
	FullRangeCount   = 10000
	HalfRangeCount   = 4000
	MiddleRangeCount = 100
	RowCost          = 1.0
	IndexCost        = 2.0
	SortCost         = 2.0
	FilterRate       = 0.5
)

// CostEstimator estimates the cost of a plan.
type costEstimator struct {
}

// Enter implements Visitor Enter interface.
func (c *costEstimator) Enter(p Plan) (Plan, bool) {
	return p, false
}

// Leave implements Visitor Leave interface.
func (c *costEstimator) Leave(p Plan) (Plan, bool) {
	switch v := p.(type) {
	case *IndexScan:
		c.indexScan(v)
	case *Limit:
		v.rowCount = v.Src().RowCount()
		v.startupCost = v.Src().StartupCost()
		v.totalCost = v.Src().TotalCost()
	case *SelectFields:
		if v.Src() != nil {
			v.startupCost = v.Src().StartupCost()
			v.rowCount = v.Src().RowCount()
			v.totalCost = v.Src().TotalCost()
		}
	case *SelectLock:
		v.startupCost = v.Src().StartupCost()
		v.rowCount = v.Src().RowCount()
		v.totalCost = v.Src().TotalCost()
	case *Sort:
		// Sort plan must retrieve all the rows before returns the first row.
		v.startupCost = v.Src().TotalCost() + v.Src().RowCount()*SortCost
		if v.limit == 0 {
			v.rowCount = v.Src().RowCount()
		} else {
			v.rowCount = math.Min(v.Src().RowCount(), v.limit)
		}
		v.totalCost = v.startupCost + v.rowCount*RowCost
	case *TableScan:
		c.tableScan(v)
	}
	return p, true
}

func (c *costEstimator) tableScan(v *TableScan) {
	var rowCount float64 = FullRangeCount
	for _, con := range v.AccessConditions {
		rowCount *= guesstimateFilterRate(con)
	}
	v.startupCost = 0
	if v.limit == 0 {
		// limit is zero means no limit.
		v.rowCount = rowCount
	} else {
		v.rowCount = math.Min(rowCount, v.limit)
	}
	v.totalCost = v.rowCount * RowCost
}

func (c *costEstimator) indexScan(v *IndexScan) {
	var rowCount float64 = FullRangeCount
	for _, con := range v.AccessConditions {
		rowCount *= guesstimateFilterRate(con)
	}
	v.startupCost = 0
	if v.limit == 0 {
		// limit is zero means no limit.
		v.rowCount = rowCount
	} else {
		v.rowCount = math.Min(rowCount, v.limit)
	}
	v.totalCost = v.rowCount * RowCost
}

// EstimateCost estimates the cost of the plan.
func EstimateCost(p Plan) float64 {
	var estimator costEstimator
	p.Accept(&estimator)
	return p.TotalCost()
}
