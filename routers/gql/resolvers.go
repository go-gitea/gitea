package gql

import (
	"github.com/graphql-go/graphql"

	api "code.gitea.io/gitea/modules/structs"
)

type Resolver struct {
}

// RepositoryResolver resolves our repository
func (r *Resolver) RepositoryResolver(p graphql.ResolveParams) (interface{}, error) {
	//owner, ownerOk := p.Args["name"].(string)
	name, nameOk := p.Args["name"].(string)
	//if ownerOk && nameOk {
	if nameOk {
		//it would be great here if we could call routers/api/v1/repo/repo.go Search function as
		//that has all the logic. However, you pass a http context type object there and it is ued.
		//repositories := .GetUsersByName(name)
		repositories := []api.Repository{}
		//results[i] = repo.APIFormat(accessMode)
		repositories[0] = api.Repository{
			ID:          0,
			Name:        name,
			FullName:    "full name",
			Description: "description",
		}
		return repositories, nil
	}

	return nil, nil
}
