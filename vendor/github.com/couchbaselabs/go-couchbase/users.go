package couchbase

import (
	"bytes"
	"fmt"
)

type User struct {
	Name   string
	Id     string
	Domain string
	Roles  []Role
}

type Role struct {
	Role       string
	BucketName string `json:"bucket_name"`
}

// Sample:
// {"role":"admin","name":"Admin","desc":"Can manage ALL cluster features including security.","ce":true}
// {"role":"query_select","bucket_name":"*","name":"Query Select","desc":"Can execute SELECT statement on bucket to retrieve data"}
type RoleDescription struct {
	Role       string
	Name       string
	Desc       string
	Ce         bool
	BucketName string `json:"bucket_name"`
}

// Return user-role data, as parsed JSON.
// Sample:
//   [{"id":"ivanivanov","name":"Ivan Ivanov","roles":[{"role":"cluster_admin"},{"bucket_name":"default","role":"bucket_admin"}]},
//    {"id":"petrpetrov","name":"Petr Petrov","roles":[{"role":"replication_admin"}]}]
func (c *Client) GetUserRoles() ([]interface{}, error) {
	ret := make([]interface{}, 0, 1)
	err := c.parseURLResponse("/settings/rbac/users", &ret)
	if err != nil {
		return nil, err
	}

	// Get the configured administrator.
	// Expected result: {"port":8091,"username":"Administrator"}
	adminInfo := make(map[string]interface{}, 2)
	err = c.parseURLResponse("/settings/web", &adminInfo)
	if err != nil {
		return nil, err
	}

	// Create a special entry for the configured administrator.
	adminResult := map[string]interface{}{
		"name":   adminInfo["username"],
		"id":     adminInfo["username"],
		"domain": "ns_server",
		"roles": []interface{}{
			map[string]interface{}{
				"role": "admin",
			},
		},
	}

	// Add the configured administrator to the list of results.
	ret = append(ret, adminResult)

	return ret, nil
}

func (c *Client) GetUserInfoAll() ([]User, error) {
	ret := make([]User, 0, 16)
	err := c.parseURLResponse("/settings/rbac/users", &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func rolesToParamFormat(roles []Role) string {
	var buffer bytes.Buffer
	for i, role := range roles {
		if i > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString(role.Role)
		if role.BucketName != "" {
			buffer.WriteString("[")
			buffer.WriteString(role.BucketName)
			buffer.WriteString("]")
		}
	}
	return buffer.String()
}

func (c *Client) PutUserInfo(u *User) error {
	params := map[string]interface{}{
		"name":  u.Name,
		"roles": rolesToParamFormat(u.Roles),
	}
	var target string
	switch u.Domain {
	case "external":
		target = "/settings/rbac/users/" + u.Id
	case "local":
		target = "/settings/rbac/users/local/" + u.Id
	default:
		return fmt.Errorf("Unknown user type: %s", u.Domain)
	}
	var ret string // PUT returns an empty string. We ignore it.
	err := c.parsePutURLResponse(target, params, &ret)
	return err
}

func (c *Client) GetRolesAll() ([]RoleDescription, error) {
	ret := make([]RoleDescription, 0, 32)
	err := c.parseURLResponse("/settings/rbac/roles", &ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
