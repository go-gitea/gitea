package graphql

import (
	"errors"

	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/language/ast"
)

func NewLocatedError(err interface{}, nodes []ast.Node) *gqlerrors.Error {
	return newLocatedError(err, nodes, nil)
}

func NewLocatedErrorWithPath(err interface{}, nodes []ast.Node, path []interface{}) *gqlerrors.Error {
	return newLocatedError(err, nodes, path)
}

func newLocatedError(err interface{}, nodes []ast.Node, path []interface{}) *gqlerrors.Error {
	if err, ok := err.(*gqlerrors.Error); ok {
		return err
	}

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
	return gqlerrors.NewErrorWithPath(
		message,
		nodes,
		stack,
		nil,
		[]int{},
		path,
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
