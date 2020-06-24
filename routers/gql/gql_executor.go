// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"github.com/graphql-go/graphql"
)

var (
	schema graphql.Schema
)

type reqBody struct {
	Query string `json:"query"`
}

// Init initializes gql server
func Init(cfg graphql.Schema) {
	schema = cfg
}

// GraphQL returns an http.HandlerFunc for our /graphql endpoint
//func (s *Server) GraphQL() http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//	}
//}

// GraphQL I don't really know what it does
func GraphQL(ctx *context.APIContext) {

	// Check to ensure query was provided in the request body
	if ctx.Req.Body() == nil {
		ctx.Error(http.StatusBadRequest, "", "Must provide graphql query in request body")
		return
	}

	var rBody reqBody
	bodyString, err := ctx.Req.Body().String()
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", "Error reading request body")
		return
	}
	// Decode the request body into rBody
	err = json.NewDecoder(strings.NewReader(bodyString)).Decode(&rBody)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", "Error parsing JSON request body")
		return
	}

	// Execute graphql query
	result := ExecuteQuery(rBody.Query, schema)

	ctx.JSON(http.StatusOK, result)
}

// ExecuteQuery runs our graphql queries
func ExecuteQuery(query string, schema graphql.Schema) *graphql.Result {
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: query,
	})

	// Error check
	if len(result.Errors) > 0 {
		fmt.Printf("Unexpected errors inside ExecuteQuery: %v", result.Errors)
	}

	return result
}
