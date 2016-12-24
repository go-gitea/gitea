// Copyright 2016 PingCAP, Inc.
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

package executor

import (
	"strconv"
	"strings"

	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/optimizer/plan"
	"github.com/pingcap/tidb/parser/opcode"
	"github.com/pingcap/tidb/util/types"
)

type explainEntry struct {
	ID           int64
	selectType   string
	table        string
	joinType     string
	possibleKeys string
	key          string
	keyLen       string
	ref          string
	rows         int64
	extra        []string
}

func (e *explainEntry) setJoinTypeForTableScan(p *plan.TableScan) {
	if len(p.AccessConditions) == 0 {
		e.joinType = "ALL"
		return
	}
	if p.RefAccess {
		e.joinType = "eq_ref"
		return
	}
	for _, con := range p.AccessConditions {
		if x, ok := con.(*ast.BinaryOperationExpr); ok {
			if x.Op == opcode.EQ {
				e.joinType = "const"
				return
			}
		}
	}
	e.joinType = "range"
}

func (e *explainEntry) setJoinTypeForIndexScan(p *plan.IndexScan) {
	if len(p.AccessConditions) == 0 {
		e.joinType = "index"
		return
	}
	if len(p.AccessConditions) == p.AccessEqualCount {
		if p.RefAccess {
			if p.Index.Unique {
				e.joinType = "eq_ref"
			} else {
				e.joinType = "ref"
			}
		} else {
			if p.Index.Unique {
				e.joinType = "const"
			} else {
				e.joinType = "range"
			}
		}
		return
	}
	e.joinType = "range"
}

// ExplainExec represents an explain executor.
// See: https://dev.mysql.com/doc/refman/5.7/en/explain-output.html
type ExplainExec struct {
	StmtPlan plan.Plan
	fields   []*ast.ResultField
	rows     []*Row
	cursor   int
}

// Fields implements Executor Fields interface.
func (e *ExplainExec) Fields() []*ast.ResultField {
	return e.fields
}

// Next implements Execution Next interface.
func (e *ExplainExec) Next() (*Row, error) {
	if e.rows == nil {
		e.fetchRows()
	}
	if e.cursor >= len(e.rows) {
		return nil, nil
	}
	row := e.rows[e.cursor]
	e.cursor++
	return row, nil
}

func (e *ExplainExec) fetchRows() {
	visitor := &explainVisitor{id: 1}
	e.StmtPlan.Accept(visitor)
	for _, entry := range visitor.entries {
		row := &Row{}
		row.Data = types.MakeDatums(
			entry.ID,
			entry.selectType,
			entry.table,
			entry.joinType,
			entry.key,
			entry.key,
			entry.keyLen,
			entry.ref,
			entry.rows,
			strings.Join(entry.extra, "; "),
		)
		for i := range row.Data {
			if row.Data[i].Kind() == types.KindString && row.Data[i].GetString() == "" {
				row.Data[i].SetNull()
			}
		}
		e.rows = append(e.rows, row)
	}
}

// Close implements Executor Close interface.
func (e *ExplainExec) Close() error {
	return nil
}

type explainVisitor struct {
	id int64

	// Sort extra should be appended in the first table in a join.
	sort    bool
	entries []*explainEntry
}

func (v *explainVisitor) Enter(p plan.Plan) (plan.Plan, bool) {
	switch x := p.(type) {
	case *plan.TableScan:
		v.entries = append(v.entries, v.newEntryForTableScan(x))
	case *plan.IndexScan:
		v.entries = append(v.entries, v.newEntryForIndexScan(x))
	case *plan.Sort:
		v.sort = true
	}
	return p, false
}

func (v *explainVisitor) Leave(p plan.Plan) (plan.Plan, bool) {
	return p, true
}

func (v *explainVisitor) newEntryForTableScan(p *plan.TableScan) *explainEntry {
	entry := &explainEntry{
		ID:         v.id,
		selectType: "SIMPLE",
		table:      p.Table.Name.O,
	}
	entry.setJoinTypeForTableScan(p)
	if entry.joinType != "ALL" {
		entry.key = "PRIMARY"
		entry.keyLen = "8"
	}
	if len(p.AccessConditions)+len(p.FilterConditions) > 0 {
		entry.extra = append(entry.extra, "Using where")
	}

	v.setSortExtra(entry)
	return entry
}

func (v *explainVisitor) newEntryForIndexScan(p *plan.IndexScan) *explainEntry {
	entry := &explainEntry{
		ID:         v.id,
		selectType: "SIMPLE",
		table:      p.Table.Name.O,
		key:        p.Index.Name.O,
	}
	if len(p.AccessConditions) != 0 {
		keyLen := 0
		for i := 0; i < len(p.Index.Columns); i++ {
			if i < p.AccessEqualCount {
				keyLen += p.Index.Columns[i].Length
			} else if i < len(p.AccessConditions) {
				keyLen += p.Index.Columns[i].Length
				break
			}
		}
		entry.keyLen = strconv.Itoa(keyLen)
	}
	entry.setJoinTypeForIndexScan(p)
	if len(p.AccessConditions)+len(p.FilterConditions) > 0 {
		entry.extra = append(entry.extra, "Using where")
	}

	v.setSortExtra(entry)
	return entry
}

func (v *explainVisitor) setSortExtra(entry *explainEntry) {
	if v.sort {
		entry.extra = append(entry.extra, "Using filesort")
		v.sort = false
	}
}
