package gql

import (
	"github.com/graphql-go/graphql"
)

// Root holds a pointer to a graphql object
type Root struct {
	Query *graphql.Object
}

// NewRoot returns base query type. This is where we add all the base queries
func NewRoot() *Root {
	// Create a resolver holding our databse. Resolver can be found in resolvers.go
	resolver := Resolver{}

	// Create a new Root that describes our base query set up. In this
	// example we have a user query that takes one argument called name
	root := Root{
		Query: graphql.NewObject(
			graphql.ObjectConfig{
				Name: "Query",
				Fields: graphql.Fields{
					"repositories": &graphql.Field{
						Type: graphql.NewList(Repository),
						Args: graphql.FieldConfigArgument{
							"Name": &graphql.ArgumentConfig{
								Type: graphql.String,
							},
						},
						Resolve: resolver.RepositoryResolver,
					},
				},
			},
		),
	}
	return &root
}
