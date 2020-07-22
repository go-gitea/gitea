package gqlerrors

import (
	"errors"
	"github.com/graphql-go/graphql/language/ast"
)

// NewLocatedError creates a graphql.Error with location info
// @deprecated 0.4.18
// Already exists in `graphql.NewLocatedError()`
func NewLocatedError(err interface{}, nodes []ast.Node) *Error {
	var origError error
	message := "An unknown error occurred."
	if err, ok := err.(error); ok {
		message = err.Error()
		origError = err
	}
	if err, ok := err.(string); ok {
		message = err
		origError = errors.New(err)
	}
	stack := message
	return NewError(
		message,
		nodes,
		stack,
		nil,
		[]int{},
		origError,
	)
}

func FieldASTsToNodeASTs(fieldASTs []*ast.Field) []ast.Node {
	nodes := []ast.Node{}
	for _, fieldAST := range fieldASTs {
		nodes = append(nodes, fieldAST)
	}
	return nodes
}
