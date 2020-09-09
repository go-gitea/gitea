// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	giteaCtx "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"

	"github.com/graphql-go/graphql"
)

var (
	schema graphql.Schema
)

type reqBody struct {
	Query string `json:"query"`
}

type contextKeyType string

// Init initializes gql server
func Init(cfg graphql.Schema) {
	schema = cfg
}

// GraphQL entry point to executing graphql query
func GraphQL(ctx *giteaCtx.APIContext) {

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
	result := ExecuteQuery(rBody.Query, schema, ctx)

	ctx.JSON(http.StatusOK, result)
}

// ExecuteQuery runs our graphql queries
func ExecuteQuery(query string, schema graphql.Schema, ctx *giteaCtx.APIContext) *graphql.Result {
	apiContextKey := contextKeyType("giteaApiContext")
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: query,
		Context:       context.WithValue(context.Background(), apiContextKey, ctx),
		RootObject:    make(map[string]interface{}),
	})

	if len(result.Errors) > 0 {
		log.Error("Unexpected errors inside ExecuteQuery: %v", result.Errors)
	}

	return result
}
