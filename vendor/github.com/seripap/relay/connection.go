package relay

import "github.com/graphql-go/graphql"

/*
Returns a GraphQLFieldConfigArgumentMap appropriate to include
on a field whose return type is a connection type.
*/
var ConnectionArgs = graphql.FieldConfigArgument{
	"before": &graphql.ArgumentConfig{
		Type: graphql.String,
	},
	"after": &graphql.ArgumentConfig{
		Type: graphql.String,
	},
	"first": &graphql.ArgumentConfig{
		Type: graphql.Int,
	},
	"last": &graphql.ArgumentConfig{
		Type: graphql.Int,
	},
}

func NewConnectionArgs(configMap graphql.FieldConfigArgument) graphql.FieldConfigArgument {
	for fieldName, argConfig := range ConnectionArgs {
		configMap[fieldName] = argConfig
	}
	return configMap
}

type ConnectionConfig struct {
	Name             string         `json:"name"`
	NodeType         graphql.Output `json:"nodeType"`
	EdgeFields       graphql.Fields `json:"edgeFields"`
	ConnectionFields graphql.Fields `json:"connectionFields"`
}

type EdgeType struct {
	Node   interface{}      `json:"node"`
	Cursor ConnectionCursor `json:"cursor"`
}
type GraphQLConnectionDefinitions struct {
	EdgeType       *graphql.Object `json:"edgeType"`
	ConnectionType *graphql.Object `json:"connectionType"`
}

/*
The common page info type used by all connections.
*/
var pageInfoType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "PageInfo",
	Description: "Information about pagination in a connection.",
	Fields: graphql.Fields{
		"hasNextPage": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "When paginating forwards, are there more items?",
		},
		"hasPreviousPage": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "When paginating backwards, are there more items?",
		},
		"startCursor": &graphql.Field{
			Type:        graphql.String,
			Description: "When paginating backwards, the cursor to continue.",
		},
		"endCursor": &graphql.Field{
			Type:        graphql.String,
			Description: "When paginating forwards, the cursor to continue.",
		},
	},
})

/*
Returns a GraphQLObjectType for a connection with the given name,
and whose nodes are of the specified type.
*/

func ConnectionDefinitions(config ConnectionConfig) *GraphQLConnectionDefinitions {

	edgeType := graphql.NewObject(graphql.ObjectConfig{
		Name:        config.Name + "Edge",
		Description: "An edge in a connection",
		Fields: graphql.Fields{
			"node": &graphql.Field{
				Type:        config.NodeType,
				Description: "The item at the end of the edge",
			},
			"cursor": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Description: " cursor for use in pagination",
			},
		},
	})
	for fieldName, fieldConfig := range config.EdgeFields {
		edgeType.AddFieldConfig(fieldName, fieldConfig)
	}

	connectionType := graphql.NewObject(graphql.ObjectConfig{
		Name:        config.Name + "Connection",
		Description: "A connection to a list of items.",

		Fields: graphql.Fields{
			"pageInfo": &graphql.Field{
				Type:        graphql.NewNonNull(pageInfoType),
				Description: "Information to aid in pagination.",
			},
			"edges": &graphql.Field{
				Type:        graphql.NewList(edgeType),
				Description: "Information to aid in pagination.",
			},
			"nodes": &graphql.Field{
				Type:        graphql.NewList(config.NodeType),
				Description: "Information to aid in pagination.",
			},
			"totalCount": &graphql.Field{
				Type:        graphql.Int,
				Description: "Information to aid in pagination.",
			},
		},
	})
	for fieldName, fieldConfig := range config.ConnectionFields {
		connectionType.AddFieldConfig(fieldName, fieldConfig)
	}

	return &GraphQLConnectionDefinitions{
		EdgeType:       edgeType,
		ConnectionType: connectionType,
	}
}
