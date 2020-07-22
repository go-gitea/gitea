package graphql

import (
	"github.com/graphql-go/graphql/gqlerrors"
)

// type Schema interface{}

// Result has the response, errors and extensions from the resolved schema
type Result struct {
	Data       interface{}                `json:"data"`
	Errors     []gqlerrors.FormattedError `json:"errors,omitempty"`
	Extensions map[string]interface{}     `json:"extensions,omitempty"`
}

// HasErrors just a simple function to help you decide if the result has errors or not
func (r *Result) HasErrors() bool {
	return len(r.Errors) > 0
}
