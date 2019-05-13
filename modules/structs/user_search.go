package structs

type searchUsersResponse struct {
	Users []*User `json:"data"`
}
