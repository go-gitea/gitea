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
	"strings"

	"github.com/ngaut/log"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/parser/opcode"
)

// equalCond represents an equivalent join condition, like "t1.c1 = t2.c1".
type equalCond struct {
	left     *ast.ResultField
	leftIdx  bool
	right    *ast.ResultField
	rightIdx bool
}

func newEqualCond(left, right *ast.ResultField) *equalCond {
	eq := &equalCond{left: left, right: right}
	eq.leftIdx = equivHasIndex(eq.left)
	eq.rightIdx = equivHasIndex(eq.right)
	return eq
}

func equivHasIndex(rf *ast.ResultField) bool {
	if rf.Table.PKIsHandle && mysql.HasPriKeyFlag(rf.Column.Flag) {
		return true
	}
	for _, idx := range rf.Table.Indices {
		if len(idx.Columns) == 1 && idx.Columns[0].Name.L == rf.Column.Name.L {
			return true
		}
	}
	return false
}

// joinPath can be a single table path, inner join or outer join.
type joinPath struct {
	// for table path
	table           *ast.TableName
	totalFilterRate float64

	// for subquery
	subquery ast.Node
	asName   model.CIStr

	neighborCount int // number of neighbor table.
	idxDepCount   int // number of paths this table depends on.
	ordering      *ast.ResultField
	orderingDesc  bool

	// for outer join path
	outer     *joinPath
	inner     *joinPath
	rightJoin bool

	// for inner join path
	inners []*joinPath

	// common
	parent     *joinPath
	filterRate float64
	conditions []ast.ExprNode
	eqConds    []*equalCond
	// The joinPaths that this path's index depends on.
	idxDeps   map[*joinPath]bool
	neighbors map[*joinPath]bool
}

// newTablePath creates a new table join path.
func newTablePath(table *ast.TableName) *joinPath {
	return &joinPath{
		table:      table,
		filterRate: rateFull,
	}
}

// newSubqueryPath creates a new subquery join path.
func newSubqueryPath(node ast.Node, asName model.CIStr) *joinPath {
	return &joinPath{
		subquery:   node,
		asName:     asName,
		filterRate: rateFull,
	}
}

// newOuterJoinPath creates a new outer join path and pushes on condition to children paths.
// The returned joinPath slice has one element.
func newOuterJoinPath(isRightJoin bool, leftPath, rightPath *joinPath, on *ast.OnCondition) *joinPath {
	outerJoin := &joinPath{rightJoin: isRightJoin, outer: leftPath, inner: rightPath, filterRate: 1}
	leftPath.parent = outerJoin
	rightPath.parent = outerJoin
	if isRightJoin {
		outerJoin.outer, outerJoin.inner = outerJoin.inner, outerJoin.outer
	}
	if on != nil {
		conditions := splitWhere(on.Expr)
		availablePaths := []*joinPath{outerJoin.outer}
		for _, con := range conditions {
			if !outerJoin.inner.attachCondition(con, availablePaths) {
				log.Errorf("Inner failed to attach ON condition")
			}
		}
	}
	return outerJoin
}

// newInnerJoinPath creates inner join path and pushes on condition to children paths.
// If left path or right path is also inner join, it will be merged.
func newInnerJoinPath(leftPath, rightPath *joinPath, on *ast.OnCondition) *joinPath {
	var innerJoin *joinPath
	if len(leftPath.inners) != 0 {
		innerJoin = leftPath
	} else {
		innerJoin = &joinPath{filterRate: leftPath.filterRate}
		innerJoin.inners = append(innerJoin.inners, leftPath)
	}
	if len(rightPath.inners) != 0 {
		innerJoin.inners = append(innerJoin.inners, rightPath.inners...)
		innerJoin.conditions = append(innerJoin.conditions, rightPath.conditions...)
	} else {
		innerJoin.inners = append(innerJoin.inners, rightPath)
	}
	innerJoin.filterRate *= rightPath.filterRate

	for _, in := range innerJoin.inners {
		in.parent = innerJoin
	}

	if on != nil {
		conditions := splitWhere(on.Expr)
		for _, con := range conditions {
			if !innerJoin.attachCondition(con, nil) {
				innerJoin.conditions = append(innerJoin.conditions, con)
			}
		}
	}
	return innerJoin
}

func (p *joinPath) resultFields() []*ast.ResultField {
	if p.table != nil {
		return p.table.GetResultFields()
	}
	if p.outer != nil {
		if p.rightJoin {
			return append(p.inner.resultFields(), p.outer.resultFields()...)
		}
		return append(p.outer.resultFields(), p.inner.resultFields()...)
	}
	var rfs []*ast.ResultField
	for _, in := range p.inners {
		rfs = append(rfs, in.resultFields()...)
	}
	return rfs
}

// attachCondition tries to attach a condition as deep as possible.
// availablePaths are paths join before this path.
func (p *joinPath) attachCondition(condition ast.ExprNode, availablePaths []*joinPath) (attached bool) {
	filterRate := guesstimateFilterRate(condition)
	// table
	if p.table != nil || p.subquery != nil {
		attacher := conditionAttachChecker{targetPath: p, availablePaths: availablePaths}
		condition.Accept(&attacher)
		if attacher.invalid {
			return false
		}
		p.conditions = append(p.conditions, condition)
		p.filterRate *= filterRate
		return true
	}
	// inner join
	if len(p.inners) > 0 {
		for _, in := range p.inners {
			if in.attachCondition(condition, availablePaths) {
				p.filterRate *= filterRate
				return true
			}
		}
		attacher := &conditionAttachChecker{targetPath: p, availablePaths: availablePaths}
		condition.Accept(attacher)
		if attacher.invalid {
			return false
		}
		p.conditions = append(p.conditions, condition)
		p.filterRate *= filterRate
		return true
	}

	// outer join
	if p.outer.attachCondition(condition, availablePaths) {
		p.filterRate *= filterRate
		return true
	}
	if p.inner.attachCondition(condition, append(availablePaths, p.outer)) {
		p.filterRate *= filterRate
		return true
	}
	return false
}

func (p *joinPath) containsTable(table *ast.TableName) bool {
	if p.table != nil {
		return p.table == table
	}
	if p.subquery != nil {
		return p.asName.L == table.Name.L
	}
	if len(p.inners) != 0 {
		for _, in := range p.inners {
			if in.containsTable(table) {
				return true
			}
		}
		return false
	}

	return p.outer.containsTable(table) || p.inner.containsTable(table)
}

// attachEqualCond tries to attach an equalCond deep into a table path if applicable.
func (p *joinPath) attachEqualCond(eqCon *equalCond, availablePaths []*joinPath) (attached bool) {
	// table
	if p.table != nil {
		var prevTable *ast.TableName
		var needSwap bool
		if eqCon.left.TableName == p.table {
			prevTable = eqCon.right.TableName
		} else if eqCon.right.TableName == p.table {
			prevTable = eqCon.left.TableName
			needSwap = true
		}
		if prevTable != nil {
			for _, prev := range availablePaths {
				if prev.containsTable(prevTable) {
					if needSwap {
						eqCon.left, eqCon.right = eqCon.right, eqCon.left
						eqCon.leftIdx, eqCon.rightIdx = eqCon.rightIdx, eqCon.leftIdx
					}
					p.eqConds = append(p.eqConds, eqCon)
					return true
				}
			}
		}
		return false
	}

	// inner join
	if len(p.inners) > 0 {
		for _, in := range p.inners {
			if in.attachEqualCond(eqCon, availablePaths) {
				p.filterRate *= rateEqual
				return true
			}
		}
		return false
	}
	// outer join
	if p.outer.attachEqualCond(eqCon, availablePaths) {
		p.filterRate *= rateEqual
		return true
	}
	if p.inner.attachEqualCond(eqCon, append(availablePaths, p.outer)) {
		p.filterRate *= rateEqual
		return true
	}
	return false
}

func (p *joinPath) extractEqualConditon() {
	var equivs []*equalCond
	var cons []ast.ExprNode
	for _, con := range p.conditions {
		eq := equivFromExpr(con)
		if eq != nil {
			equivs = append(equivs, eq)
			if p.table != nil {
				if eq.right.TableName == p.table {
					eq.left, eq.right = eq.right, eq.left
					eq.leftIdx, eq.rightIdx = eq.rightIdx, eq.leftIdx
				}
			}
		} else {
			cons = append(cons, con)
		}
	}
	p.eqConds = equivs
	p.conditions = cons
	for _, in := range p.inners {
		in.extractEqualConditon()
	}
	if p.outer != nil {
		p.outer.extractEqualConditon()
		p.inner.extractEqualConditon()
	}
}

func (p *joinPath) addIndexDependency() {
	if p.outer != nil {
		p.outer.addIndexDependency()
		p.inner.addIndexDependency()
		return
	}
	if p.table != nil {
		return
	}
	for _, eq := range p.eqConds {
		if !eq.leftIdx && !eq.rightIdx {
			continue
		}
		pathLeft := p.findInnerContains(eq.left.TableName)
		if pathLeft == nil {
			continue
		}
		pathRight := p.findInnerContains(eq.right.TableName)
		if pathRight == nil {
			continue
		}
		if eq.leftIdx && eq.rightIdx {
			pathLeft.addNeighbor(pathRight)
			pathRight.addNeighbor(pathLeft)
		} else if eq.leftIdx {
			if !pathLeft.hasOuterIdxEqualCond() {
				pathLeft.addIndexDep(pathRight)
			}
		} else if eq.rightIdx {
			if !pathRight.hasOuterIdxEqualCond() {
				pathRight.addIndexDep(pathLeft)
			}
		}
	}
	for _, in := range p.inners {
		in.removeIndexDepCycle(in)
		in.addIndexDependency()
	}
}

func (p *joinPath) hasOuterIdxEqualCond() bool {
	if p.table != nil {
		for _, eq := range p.eqConds {
			if eq.leftIdx {
				return true
			}
		}
		return false
	}
	if p.outer != nil {
		return p.outer.hasOuterIdxEqualCond()
	}
	for _, in := range p.inners {
		if in.hasOuterIdxEqualCond() {
			return true
		}
	}
	return false
}

func (p *joinPath) findInnerContains(table *ast.TableName) *joinPath {
	for _, in := range p.inners {
		if in.containsTable(table) {
			return in
		}
	}
	return nil
}

func (p *joinPath) addNeighbor(neighbor *joinPath) {
	if p.neighbors == nil {
		p.neighbors = map[*joinPath]bool{}
	}
	p.neighbors[neighbor] = true
	p.neighborCount++
}

func (p *joinPath) addIndexDep(dep *joinPath) {
	if p.idxDeps == nil {
		p.idxDeps = map[*joinPath]bool{}
	}
	p.idxDeps[dep] = true
	p.idxDepCount++
}

func (p *joinPath) removeIndexDepCycle(origin *joinPath) {
	if p.idxDeps == nil {
		return
	}
	for dep := range p.idxDeps {
		if dep == origin {
			delete(p.idxDeps, origin)
			continue
		}
		dep.removeIndexDepCycle(origin)
	}
}

func (p *joinPath) score() float64 {
	return 1 / p.filterRate
}

func (p *joinPath) String() string {
	if p.table != nil {
		return p.table.TableInfo.Name.L
	}
	if p.outer != nil {
		return "outer{" + p.outer.String() + "," + p.inner.String() + "}"
	}
	var innerStrs []string
	for _, in := range p.inners {
		innerStrs = append(innerStrs, in.String())
	}
	return "inner{" + strings.Join(innerStrs, ",") + "}"
}

func (p *joinPath) optimizeJoinOrder(availablePaths []*joinPath) {
	if p.table != nil {
		return
	}
	if p.outer != nil {
		p.outer.optimizeJoinOrder(availablePaths)
		p.inner.optimizeJoinOrder(append(availablePaths, p.outer))
		return
	}
	var ordered []*joinPath
	pathMap := map[*joinPath]bool{}
	for _, in := range p.inners {
		pathMap[in] = true
	}
	for len(pathMap) > 0 {
		next := p.nextPath(pathMap, availablePaths)
		next.optimizeJoinOrder(availablePaths)
		ordered = append(ordered, next)
		delete(pathMap, next)
		availablePaths = append(availablePaths, next)
		for path := range pathMap {
			if path.idxDeps != nil {
				delete(path.idxDeps, next)
			}
			if path.neighbors != nil {
				delete(path.neighbors, next)
			}
		}
		p.reattach(pathMap, availablePaths)
	}
	p.inners = ordered
}

// reattach is called by inner joinPath to retry attach conditions to inner paths
// after an inner path has been added to available paths.
func (p *joinPath) reattach(pathMap map[*joinPath]bool, availablePaths []*joinPath) {
	if len(p.conditions) != 0 {
		remainedConds := make([]ast.ExprNode, 0, len(p.conditions))
		for _, con := range p.conditions {
			var attached bool
			for path := range pathMap {
				if path.attachCondition(con, availablePaths) {
					attached = true
					break
				}
			}
			if !attached {
				remainedConds = append(remainedConds, con)
			}
		}
		p.conditions = remainedConds
	}
	if len(p.eqConds) != 0 {
		remainedEqConds := make([]*equalCond, 0, len(p.eqConds))
		for _, eq := range p.eqConds {
			var attached bool
			for path := range pathMap {
				if path.attachEqualCond(eq, availablePaths) {
					attached = true
					break
				}
			}
			if !attached {
				remainedEqConds = append(remainedEqConds, eq)
			}
		}
		p.eqConds = remainedEqConds
	}
}

func (p *joinPath) nextPath(pathMap map[*joinPath]bool, availablePaths []*joinPath) *joinPath {
	cans := p.candidates(pathMap)
	if len(cans) == 0 {
		var v *joinPath
		for v = range pathMap {
			log.Errorf("index dep %v, prevs %v\n", v.idxDeps, len(availablePaths))
		}
		return v
	}
	indexPath := p.nextIndexPath(cans)
	if indexPath != nil {
		return indexPath
	}
	return p.pickPath(cans)
}

func (p *joinPath) candidates(pathMap map[*joinPath]bool) []*joinPath {
	var cans []*joinPath
	for t := range pathMap {
		if len(t.idxDeps) > 0 {
			continue
		}
		cans = append(cans, t)
	}
	return cans
}

func (p *joinPath) nextIndexPath(candidates []*joinPath) *joinPath {
	var best *joinPath
	for _, can := range candidates {
		// Since we may not have equal conditions attached on the path, we
		// need to check neighborCount and idxDepCount to see if this path
		// can be joined with index.
		neighborIsAvailable := len(can.neighbors) < can.neighborCount
		idxDepIsAvailable := can.idxDepCount > 0
		if can.hasOuterIdxEqualCond() || neighborIsAvailable || idxDepIsAvailable {
			if best == nil {
				best = can
			}
			if can.score() > best.score() {
				best = can
			}
		}
	}
	return best
}

func (p *joinPath) pickPath(candidates []*joinPath) *joinPath {
	var best *joinPath
	for _, path := range candidates {
		if best == nil {
			best = path
		}
		if path.score() > best.score() {
			best = path
		}
	}
	return best
}

// conditionAttachChecker checks if an expression is valid to
// attach to a path. attach is valid only if all the referenced tables in the
// expression are available.
type conditionAttachChecker struct {
	targetPath     *joinPath
	availablePaths []*joinPath
	invalid        bool
}

func (c *conditionAttachChecker) Enter(in ast.Node) (ast.Node, bool) {
	switch x := in.(type) {
	case *ast.ColumnNameExpr:
		table := x.Refer.TableName
		if c.targetPath.containsTable(table) {
			return in, false
		}
		c.invalid = true
		for _, path := range c.availablePaths {
			if path.containsTable(table) {
				c.invalid = false
				return in, false
			}
		}
	}
	return in, false
}

func (c *conditionAttachChecker) Leave(in ast.Node) (ast.Node, bool) {
	return in, !c.invalid
}

func (b *planBuilder) buildJoin(sel *ast.SelectStmt) Plan {
	nrfinder := &nullRejectFinder{nullRejectTables: map[*ast.TableName]bool{}}
	if sel.Where != nil {
		sel.Where.Accept(nrfinder)
	}
	path := b.buildBasicJoinPath(sel.From.TableRefs, nrfinder.nullRejectTables)
	rfs := path.resultFields()

	whereConditions := splitWhere(sel.Where)
	for _, whereCond := range whereConditions {
		if !path.attachCondition(whereCond, nil) {
			// TODO: Find a better way to handle this condition.
			path.conditions = append(path.conditions, whereCond)
			log.Errorf("Failed to attach where condtion.")
		}
	}
	path.extractEqualConditon()
	path.addIndexDependency()
	path.optimizeJoinOrder(nil)
	p := b.buildPlanFromJoinPath(path)
	p.SetFields(rfs)
	return p
}

type nullRejectFinder struct {
	nullRejectTables map[*ast.TableName]bool
}

func (n *nullRejectFinder) Enter(in ast.Node) (ast.Node, bool) {
	switch x := in.(type) {
	case *ast.BinaryOperationExpr:
		if x.Op == opcode.NullEQ || x.Op == opcode.OrOr {
			return in, true
		}
	case *ast.IsNullExpr:
		if !x.Not {
			return in, true
		}
	case *ast.IsTruthExpr:
		if x.Not {
			return in, true
		}
	}
	return in, false
}

func (n *nullRejectFinder) Leave(in ast.Node) (ast.Node, bool) {
	switch x := in.(type) {
	case *ast.ColumnNameExpr:
		n.nullRejectTables[x.Refer.TableName] = true
	}
	return in, true
}

func (b *planBuilder) buildBasicJoinPath(node ast.ResultSetNode, nullRejectTables map[*ast.TableName]bool) *joinPath {
	switch x := node.(type) {
	case nil:
		return nil
	case *ast.Join:
		leftPath := b.buildBasicJoinPath(x.Left, nullRejectTables)
		if x.Right == nil {
			return leftPath
		}
		righPath := b.buildBasicJoinPath(x.Right, nullRejectTables)
		isOuter := b.isOuterJoin(x.Tp, leftPath, righPath, nullRejectTables)
		if isOuter {
			return newOuterJoinPath(x.Tp == ast.RightJoin, leftPath, righPath, x.On)
		}
		return newInnerJoinPath(leftPath, righPath, x.On)
	case *ast.TableSource:
		switch v := x.Source.(type) {
		case *ast.TableName:
			return newTablePath(v)
		case *ast.SelectStmt, *ast.UnionStmt:
			return newSubqueryPath(v, x.AsName)
		default:
			b.err = ErrUnsupportedType.Gen("unsupported table source type %T", x)
			return nil
		}
	default:
		b.err = ErrUnsupportedType.Gen("unsupported table source type %T", x)
		return nil
	}
}

func (b *planBuilder) isOuterJoin(tp ast.JoinType, leftPaths, rightPaths *joinPath,
	nullRejectTables map[*ast.TableName]bool) bool {
	var innerPath *joinPath
	switch tp {
	case ast.LeftJoin:
		innerPath = rightPaths
	case ast.RightJoin:
		innerPath = leftPaths
	default:
		return false
	}
	for table := range nullRejectTables {
		if innerPath.containsTable(table) {
			return false
		}
	}
	return true
}

func equivFromExpr(expr ast.ExprNode) *equalCond {
	binop, ok := expr.(*ast.BinaryOperationExpr)
	if !ok || binop.Op != opcode.EQ {
		return nil
	}
	ln, lOK := binop.L.(*ast.ColumnNameExpr)
	rn, rOK := binop.R.(*ast.ColumnNameExpr)
	if !lOK || !rOK {
		return nil
	}
	if ln.Name.Table.L == "" || rn.Name.Table.L == "" {
		return nil
	}
	if ln.Name.Schema.L == rn.Name.Schema.L && ln.Name.Table.L == rn.Name.Table.L {
		return nil
	}
	return newEqualCond(ln.Refer, rn.Refer)
}

func (b *planBuilder) buildPlanFromJoinPath(path *joinPath) Plan {
	if path.table != nil {
		return b.buildTablePlanFromJoinPath(path)
	}
	if path.subquery != nil {
		return b.buildSubqueryJoinPath(path)
	}
	if path.outer != nil {
		join := &JoinOuter{
			Outer: b.buildPlanFromJoinPath(path.outer),
			Inner: b.buildPlanFromJoinPath(path.inner),
		}
		if path.rightJoin {
			join.SetFields(append(join.Inner.Fields(), join.Outer.Fields()...))
		} else {
			join.SetFields(append(join.Outer.Fields(), join.Inner.Fields()...))
		}
		return join
	}
	join := &JoinInner{}
	for _, in := range path.inners {
		join.Inners = append(join.Inners, b.buildPlanFromJoinPath(in))
		join.fields = append(join.fields, in.resultFields()...)
	}
	join.Conditions = path.conditions
	for _, equiv := range path.eqConds {
		cond := &ast.BinaryOperationExpr{L: equiv.left.Expr, R: equiv.right.Expr, Op: opcode.EQ}
		join.Conditions = append(join.Conditions, cond)
	}
	return join
}

func (b *planBuilder) buildTablePlanFromJoinPath(path *joinPath) Plan {
	for _, equiv := range path.eqConds {
		columnNameExpr := &ast.ColumnNameExpr{}
		columnNameExpr.Name = &ast.ColumnName{}
		columnNameExpr.Name.Name = equiv.left.Column.Name
		columnNameExpr.Name.Table = equiv.left.Table.Name
		columnNameExpr.Refer = equiv.left
		condition := &ast.BinaryOperationExpr{L: columnNameExpr, R: equiv.right.Expr, Op: opcode.EQ}
		ast.SetFlag(condition)
		path.conditions = append(path.conditions, condition)
	}
	candidates := b.buildAllAccessMethodsPlan(path)
	var p Plan
	var lowestCost float64
	for _, can := range candidates {
		cost := EstimateCost(can)
		if p == nil {
			p = can
			lowestCost = cost
		}
		if cost < lowestCost {
			p = can
			lowestCost = cost
		}
	}
	return p
}

// Build subquery join path plan
func (b *planBuilder) buildSubqueryJoinPath(path *joinPath) Plan {
	for _, equiv := range path.eqConds {
		columnNameExpr := &ast.ColumnNameExpr{}
		columnNameExpr.Name = &ast.ColumnName{}
		columnNameExpr.Name.Name = equiv.left.Column.Name
		columnNameExpr.Name.Table = equiv.left.Table.Name
		columnNameExpr.Refer = equiv.left
		condition := &ast.BinaryOperationExpr{L: columnNameExpr, R: equiv.right.Expr, Op: opcode.EQ}
		ast.SetFlag(condition)
		path.conditions = append(path.conditions, condition)
	}
	p := b.build(path.subquery)
	if len(path.conditions) == 0 {
		return p
	}
	filterPlan := &Filter{Conditions: path.conditions}
	filterPlan.SetSrc(p)
	filterPlan.SetFields(p.Fields())
	return filterPlan
}
