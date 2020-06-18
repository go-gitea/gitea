package gql

import "github.com/graphql-go/graphql"

// Repository describes a graphql object containing a repository
var Repository = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Repository",
		Fields: graphql.Fields{
			"ID": &graphql.Field{
				Type: graphql.Int,
			},
			"Name": &graphql.Field{
				Type: graphql.String,
			},
			"FullName": &graphql.Field{
				Type: graphql.String,
			},
			"Description": &graphql.Field{
				Type: graphql.String,
			},
		},
	},
)
