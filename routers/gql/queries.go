// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import (
	"github.com/graphql-go/graphql"
)

// Root holds a pointer to a graphql object
type Root struct {
	Query *graphql.Object
}

// NewRoot returns base query type.
func NewRoot() *Root {
	root := Root{
		Query: graphql.NewObject(
			graphql.ObjectConfig{
				Name: "Query",
				Fields: graphql.Fields{
					"repository": &graphql.Field{
						Type:        repository,
						Description: "A repository",
						Args: graphql.FieldConfigArgument{
							"owner": &graphql.ArgumentConfig{
								Type:        graphql.String,
								Description: "Owner of the repository",
							},
							"name": &graphql.ArgumentConfig{
								Type:        graphql.String,
								Description: "Name of the repository",
							},
						},
						Resolve: RepositoryResolver,
					},
					"node": nodeDefinitions.NodeField,
				},
			},
		),
	}
	return &root
}
