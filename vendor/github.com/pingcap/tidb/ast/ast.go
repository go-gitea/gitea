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

// Package ast is the abstract syntax tree parsed from a SQL statement by parser.
// It can be analysed and transformed by optimizer.
package ast

import (
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/util/types"
)

// Node is the basic element of the AST.
// Interfaces embed Node should have 'Node' name suffix.
type Node interface {
	// Accept accepts Visitor to visit itself.
	// The returned node should replace original node.
	// ok returns false to stop visiting.
	//
	// Implementation of this method should first call visitor.Enter,
	// assign the returned node to its method receiver, if skipChildren returns true,
	// children should be skipped. Otherwise, call its children in particular order that
	// later elements depends on former elements. Finally, return visitor.Leave.
	Accept(v Visitor) (node Node, ok bool)
	// Text returns the original text of the element.
	Text() string
	// SetText sets original text to the Node.
	SetText(text string)
}

// Flags indicates whether an expression contains certain types of expression.
const (
	FlagConstant       uint64 = 0
	FlagHasParamMarker uint64 = 1 << iota
	FlagHasFunc
	FlagHasReference
	FlagHasAggregateFunc
	FlagHasSubquery
	FlagHasVariable
	FlagHasDefault
)

// ExprNode is a node that can be evaluated.
// Name of implementations should have 'Expr' suffix.
type ExprNode interface {
	// Node is embeded in ExprNode.
	Node
	// SetType sets evaluation type to the expression.
	SetType(tp *types.FieldType)
	// GetType gets the evaluation type of the expression.
	GetType() *types.FieldType
	// SetValue sets value to the expression.
	SetValue(val interface{})
	// GetValue gets value of the expression.
	GetValue() interface{}
	// SetDatum sets datum to the expression.
	SetDatum(datum types.Datum)
	// GetDatum gets datum of the expression.
	GetDatum() *types.Datum
	// SetFlag sets flag to the expression.
	// Flag indicates whether the expression contains
	// parameter marker, reference, aggregate function...
	SetFlag(flag uint64)
	// GetFlag returns the flag of the expression.
	GetFlag() uint64
}

// FuncNode represents function call expression node.
type FuncNode interface {
	ExprNode
	functionExpression()
}

// StmtNode represents statement node.
// Name of implementations should have 'Stmt' suffix.
type StmtNode interface {
	Node
	statement()
}

// DDLNode represents DDL statement node.
type DDLNode interface {
	StmtNode
	ddlStatement()
}

// DMLNode represents DML statement node.
type DMLNode interface {
	StmtNode
	dmlStatement()
}

// ResultField represents a result field which can be a column from a table,
// or an expression in select field. It is a generated property during
// binding process. ResultField is the key element to evaluate a ColumnNameExpr.
// After resolving process, every ColumnNameExpr will be resolved to a ResultField.
// During execution, every row retrieved from table will set the row value to
// ResultFields of that table, so ColumnNameExpr resolved to that ResultField can be
// easily evaluated.
type ResultField struct {
	Column       *model.ColumnInfo
	ColumnAsName model.CIStr
	Table        *model.TableInfo
	TableAsName  model.CIStr
	DBName       model.CIStr

	// The expression for the result field. If it is generated from a select field, it would
	// be the expression of that select field, otherwise the type would be ValueExpr and value
	// will be set for every retrieved row.
	Expr      ExprNode
	TableName *TableName
}

// Row represents a single row from Recordset.
type Row struct {
	Data []types.Datum
}

// RecordSet is an abstract result set interface to help get data from Plan.
type RecordSet interface {

	// Fields gets result fields.
	Fields() (fields []*ResultField, err error)

	// Next returns the next row, nil row means there is no more to return.
	Next() (row *Row, err error)

	// Close closes the underlying iterator, call Next after Close will
	// restart the iteration.
	Close() error
}

// ResultSetNode interface has ResultFields property which is computed and set by
// optimizer.InfoBinder during binding process. Implementations include SelectStmt,
// SubqueryExpr, TableSource, TableName and Join.
type ResultSetNode interface {
	Node
	// GetResultFields gets result fields of the result set node.
	GetResultFields() []*ResultField
	// SetResultFields sets result fields of the result set node.
	SetResultFields(fields []*ResultField)
}

// Statement is an interface for SQL execution.
// NOTE: all Statement implementations must be safe for
// concurrent using by multiple goroutines.
// If the Exec method requires any Execution domain local data,
// they must be held out of the implementing instance.
type Statement interface {
	// Explain gets the execution plans.
	//Explain(ctx context.Context, w format.Formatter)

	// IsDDL shows whether the statement is an DDL operation.
	IsDDL() bool

	// OriginText gets the origin SQL text.
	OriginText() string

	// SetText sets the executive SQL text.
	SetText(text string)

	// Exec executes SQL and gets a Recordset.
	Exec(ctx context.Context) (RecordSet, error)
}

// Visitor visits a Node.
type Visitor interface {
	// Enter is called before children nodes are visited.
	// The returned node must be the same type as the input node n.
	// skipChildren returns true means children nodes should be skipped,
	// this is useful when work is done in Enter and there is no need to visit children.
	Enter(n Node) (node Node, skipChildren bool)
	// Leave is called after children nodes have been visited.
	// The returned node's type can be different from the input node if it is a ExprNode,
	// Non-expression node must be the same type as the input node n.
	// ok returns false to stop visiting.
	Leave(n Node) (node Node, ok bool)
}
