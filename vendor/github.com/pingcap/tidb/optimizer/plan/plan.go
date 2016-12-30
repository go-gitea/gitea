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

	"github.com/pingcap/tidb/ast"
)

// Plan is a description of an execution flow.
// It is created from ast.Node first, then optimized by optimizer,
// then used by executor to create a Cursor which executes the statement.
type Plan interface {
	// Accept a visitor, implementation should call Visitor.Enter first,
	// then call children Accept methods, finally call Visitor.Leave.
	Accept(v Visitor) (out Plan, ok bool)
	// Fields returns the result fields of the plan.
	Fields() []*ast.ResultField
	// SetFields sets the results fields of the plan.
	SetFields(fields []*ast.ResultField)
	// The cost before returning fhe first row.
	StartupCost() float64
	// The cost after returning all the rows.
	TotalCost() float64
	// The expected row count.
	RowCount() float64
	// SetLimit is used to push limit to upstream to estimate the cost.
	SetLimit(limit float64)
}

// WithSrcPlan is a Plan has a source Plan.
type WithSrcPlan interface {
	Plan
	Src() Plan
	SetSrc(src Plan)
}

// Visitor visits a Plan.
type Visitor interface {
	// Enter is called before visit children.
	// The out plan should be of exactly the same type as the in plan.
	// if skipChildren is true, the children should not be visited.
	Enter(in Plan) (out Plan, skipChildren bool)

	// Leave is called after children has been visited, the out Plan can
	// be another type, this is different than ast.Visitor Leave, because
	// Plans only contain children plans as Plan interface type, so it is safe
	// to return a different type of plan.
	Leave(in Plan) (out Plan, ok bool)
}

// basePlan implements base Plan interface.
// Should be used as embedded struct in Plan implementations.
type basePlan struct {
	fields      []*ast.ResultField
	startupCost float64
	totalCost   float64
	rowCount    float64
	limit       float64
}

// StartupCost implements Plan StartupCost interface.
func (p *basePlan) StartupCost() float64 {
	return p.startupCost
}

// TotalCost implements Plan TotalCost interface.
func (p *basePlan) TotalCost() float64 {
	return p.totalCost
}

// RowCount implements Plan RowCount interface.
func (p *basePlan) RowCount() float64 {
	if p.limit == 0 {
		return p.rowCount
	}
	return math.Min(p.rowCount, p.limit)
}

// SetLimit implements Plan SetLimit interface.
func (p *basePlan) SetLimit(limit float64) {
	p.limit = limit
}

// Fields implements Plan Fields interface.
func (p *basePlan) Fields() []*ast.ResultField {
	return p.fields
}

// SetFields implements Plan SetFields interface.
func (p *basePlan) SetFields(fields []*ast.ResultField) {
	p.fields = fields
}

// srcPlan implements base PlanWithSrc interface.
type planWithSrc struct {
	basePlan
	src Plan
}

// Src implements PlanWithSrc interface.
func (p *planWithSrc) Src() Plan {
	return p.src
}

// SetSrc implements PlanWithSrc interface.
func (p *planWithSrc) SetSrc(src Plan) {
	p.src = src
}

// SetLimit implements Plan interface.
func (p *planWithSrc) SetLimit(limit float64) {
	p.limit = limit
	p.src.SetLimit(limit)
}
