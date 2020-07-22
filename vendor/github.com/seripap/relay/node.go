package relay

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"

	"github.com/graphql-go/graphql"
)

type NodeDefinitions struct {
	NodeInterface *graphql.Interface
	NodeField     *graphql.Field
}

type NodeDefinitionsConfig struct {
	IDFetcher   IDFetcherFn
	TypeResolve graphql.ResolveTypeFn
}
type IDFetcherFn func(id string, info graphql.ResolveInfo, ctx context.Context) (interface{}, error)
type GlobalIDFetcherFn func(obj interface{}, info graphql.ResolveInfo, ctx context.Context) (string, error)

/*
 Given a function to map from an ID to an underlying object, and a function
 to map from an underlying object to the concrete GraphQLObjectType it
 corresponds to, constructs a `Node` interface that objects can implement,
 and a field config for a `node` root field.

 If the typeResolver is omitted, object resolution on the interface will be
 handled with the `isTypeOf` method on object types, as with any GraphQL
interface without a provided `resolveType` method.
*/
func NewNodeDefinitions(config NodeDefinitionsConfig) *NodeDefinitions {
	nodeInterface := graphql.NewInterface(graphql.InterfaceConfig{
		Name:        "Node",
		Description: "An object with an ID",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.ID),
				Description: "The id of the object",
			},
		},
		ResolveType: config.TypeResolve,
	})

	nodeField := &graphql.Field{
		Name:        "Node",
		Description: "Fetches an object given its ID",
		Type:        nodeInterface,
		Args: graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{
				Type:        graphql.NewNonNull(graphql.ID),
				Description: "The ID of an object",
			},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			if config.IDFetcher == nil {
				return nil, nil
			}
			id := ""
			if iid, ok := p.Args["id"]; ok {
				id = fmt.Sprintf("%v", iid)
			}
			return config.IDFetcher(id, p.Info, p.Context)
		},
	}
	return &NodeDefinitions{
		NodeInterface: nodeInterface,
		NodeField:     nodeField,
	}
}

type ResolvedGlobalID struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

/*
Takes a type name and an ID specific to that type name, and returns a
"global ID" that is unique among all types.
*/
func ToGlobalID(ttype string, id string) string {
	str := ttype + ":" + id
	encStr := base64.StdEncoding.EncodeToString([]byte(str))
	return encStr
}

/*
Takes the "global ID" created by toGlobalID, and returns the type name and ID
used to create it.
*/
func FromGlobalID(globalID string) *ResolvedGlobalID {
	strID := ""
	b, err := base64.StdEncoding.DecodeString(globalID)
	if err == nil {
		strID = string(b)
	}
	tokens := strings.Split(strID, ":")
	if len(tokens) < 2 {
		return nil
	}
	return &ResolvedGlobalID{
		Type: tokens[0],
		ID:   tokens[1],
	}
}

/*
Creates the configuration for an id field on a node, using `toGlobalId` to
construct the ID from the provided typename. The type-specific ID is fetcher
by calling idFetcher on the object, or if not provided, by accessing the `id`
property on the object.
*/
func GlobalIDField(typeName string, idFetcher GlobalIDFetcherFn) *graphql.Field {
	return &graphql.Field{
		Name:        "id",
		Description: "The ID of an object",
		Type:        graphql.NewNonNull(graphql.ID),
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			id := ""
			if idFetcher != nil {
				fetched, err := idFetcher(p.Source, p.Info, p.Context)
				id = fmt.Sprintf("%v", fetched)
				if err != nil {
					return id, err
				}
			} else {
				// try to get an ID string from p.Source
				// via reflection on the ID field of the
				// underlying concrete type
				elem := reflect.ValueOf(p.Source)
				if elem.Kind() == reflect.Ptr {
					elem = elem.Elem()
				}
				typeOfElem := elem.Type()
				for i := 0; i < elem.NumField(); i++ {
					ef := elem.Field(i)
					tf := typeOfElem.Field(i)
					if tf.Tag.Get("json") == "id" {
						id = fmt.Sprintf("%v", ef.Interface())
						// We prefer the tagged field, so exit the loop
						break
					}
					if tf.Name == "ID" {
						id = fmt.Sprintf("%v", ef.Interface())
						// We prefer the tagged field, so fall through
						// in case a subsequent field is tagged
					}
				}
			}
			globalID := ToGlobalID(typeName, id)
			return globalID, nil
		},
	}
}
