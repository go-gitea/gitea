package swagger

import api "code.gitea.io/gitea/modules/structs"

// Group
// swagger:response Group
type swaggerResponseGroup struct {
	// in:body
	Body api.Group `json:"body"`
}

// GroupList
// swagger:response GroupList
type swaggerResponseGroupList struct {
	// in:body
	Body []api.Group `json:"body"`
}
