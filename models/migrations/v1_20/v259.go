// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 // nolint

import (
	"fmt"
	"strings"

	"xorm.io/xorm"

	auth_model "code.gitea.io/gitea/models/auth"
)

// OldAccessTokenScope represents the scope for an access token.
type OldAccessTokenScope string

const (
	OldAccessTokenScopeAll OldAccessTokenScope = "all"

	OldAccessTokenScopeRepo       OldAccessTokenScope = "repo"
	OldAccessTokenScopeRepoStatus OldAccessTokenScope = "repo:status"
	OldAccessTokenScopePublicRepo OldAccessTokenScope = "public_repo"

	OldAccessTokenScopeAdminOrg OldAccessTokenScope = "admin:org"
	OldAccessTokenScopeWriteOrg OldAccessTokenScope = "write:org"
	OldAccessTokenScopeReadOrg  OldAccessTokenScope = "read:org"

	OldAccessTokenScopeAdminPublicKey OldAccessTokenScope = "admin:public_key"
	OldAccessTokenScopeWritePublicKey OldAccessTokenScope = "write:public_key"
	OldAccessTokenScopeReadPublicKey  OldAccessTokenScope = "read:public_key"

	OldAccessTokenScopeAdminRepoHook OldAccessTokenScope = "admin:repo_hook"
	OldAccessTokenScopeWriteRepoHook OldAccessTokenScope = "write:repo_hook"
	OldAccessTokenScopeReadRepoHook  OldAccessTokenScope = "read:repo_hook"

	OldAccessTokenScopeAdminOrgHook OldAccessTokenScope = "admin:org_hook"

	OldAccessTokenScopeNotification OldAccessTokenScope = "notification"

	OldAccessTokenScopeUser       OldAccessTokenScope = "user"
	OldAccessTokenScopeReadUser   OldAccessTokenScope = "read:user"
	OldAccessTokenScopeUserEmail  OldAccessTokenScope = "user:email"
	OldAccessTokenScopeUserFollow OldAccessTokenScope = "user:follow"

	OldAccessTokenScopeDeleteRepo OldAccessTokenScope = "delete_repo"

	OldAccessTokenScopePackage       OldAccessTokenScope = "package"
	OldAccessTokenScopeWritePackage  OldAccessTokenScope = "write:package"
	OldAccessTokenScopeReadPackage   OldAccessTokenScope = "read:package"
	OldAccessTokenScopeDeletePackage OldAccessTokenScope = "delete:package"

	OldAccessTokenScopeAdminGPGKey OldAccessTokenScope = "admin:gpg_key"
	OldAccessTokenScopeWriteGPGKey OldAccessTokenScope = "write:gpg_key"
	OldAccessTokenScopeReadGPGKey  OldAccessTokenScope = "read:gpg_key"

	OldAccessTokenScopeAdminApplication OldAccessTokenScope = "admin:application"
	OldAccessTokenScopeWriteApplication OldAccessTokenScope = "write:application"
	OldAccessTokenScopeReadApplication  OldAccessTokenScope = "read:application"

	OldAccessTokenScopeSudo OldAccessTokenScope = "sudo"
)

var accessTokenScopeMap = map[OldAccessTokenScope][]auth_model.AccessTokenScope{
	OldAccessTokenScopeAll:              {auth_model.AccessTokenScopeAll},
	OldAccessTokenScopeRepo:             {auth_model.AccessTokenScopeDeleteRepository},
	OldAccessTokenScopeRepoStatus:       {auth_model.AccessTokenScopeWriteRepository},
	OldAccessTokenScopePublicRepo:       {auth_model.AccessTokenScopeDeleteRepository, auth_model.AccessTokenScopePublicOnly},
	OldAccessTokenScopeAdminOrg:         {auth_model.AccessTokenScopeDeleteOrganization},
	OldAccessTokenScopeWriteOrg:         {auth_model.AccessTokenScopeWriteOrganization},
	OldAccessTokenScopeReadOrg:          {auth_model.AccessTokenScopeReadOrganization},
	OldAccessTokenScopeAdminPublicKey:   {auth_model.AccessTokenScopeDeleteUser},
	OldAccessTokenScopeWritePublicKey:   {auth_model.AccessTokenScopeWriteUser},
	OldAccessTokenScopeReadPublicKey:    {auth_model.AccessTokenScopeReadUser},
	OldAccessTokenScopeAdminRepoHook:    {auth_model.AccessTokenScopeDeleteRepository},
	OldAccessTokenScopeWriteRepoHook:    {auth_model.AccessTokenScopeWriteRepository},
	OldAccessTokenScopeReadRepoHook:     {auth_model.AccessTokenScopeReadRepository},
	OldAccessTokenScopeAdminOrgHook:     {auth_model.AccessTokenScopeWriteOrganization},
	OldAccessTokenScopeNotification:     {auth_model.AccessTokenScopeDeleteNotification},
	OldAccessTokenScopeUser:             {auth_model.AccessTokenScopeDeleteUser},
	OldAccessTokenScopeReadUser:         {auth_model.AccessTokenScopeReadUser},
	OldAccessTokenScopeUserEmail:        {auth_model.AccessTokenScopeWriteUser},
	OldAccessTokenScopeUserFollow:       {auth_model.AccessTokenScopeWriteUser},
	OldAccessTokenScopeDeleteRepo:       {auth_model.AccessTokenScopeDeleteRepository},
	OldAccessTokenScopePackage:          {auth_model.AccessTokenScopeDeletePackage},
	OldAccessTokenScopeWritePackage:     {auth_model.AccessTokenScopeWritePackage},
	OldAccessTokenScopeReadPackage:      {auth_model.AccessTokenScopeReadPackage},
	OldAccessTokenScopeDeletePackage:    {auth_model.AccessTokenScopeDeletePackage},
	OldAccessTokenScopeAdminGPGKey:      {auth_model.AccessTokenScopeDeleteUser},
	OldAccessTokenScopeWriteGPGKey:      {auth_model.AccessTokenScopeWriteUser},
	OldAccessTokenScopeReadGPGKey:       {auth_model.AccessTokenScopeReadUser},
	OldAccessTokenScopeAdminApplication: {auth_model.AccessTokenScopeDeleteUser},
	OldAccessTokenScopeWriteApplication: {auth_model.AccessTokenScopeWriteUser},
	OldAccessTokenScopeReadApplication:  {auth_model.AccessTokenScopeReadUser},
	OldAccessTokenScopeSudo:             {auth_model.AccessTokenScopeDeleteAdmin},
}

type AccessToken struct {
	ID    int64 `xorm:"pk autoincr"`
	Scope string
}

func ConvertScopedAccessTokens(x *xorm.Engine) error {
	var tokens []*AccessToken

	if err := x.Find(&tokens); err != nil {
		return err
	}

	for _, token := range tokens {
		allNewScopesMap := make(map[auth_model.AccessTokenScope]bool)
		for _, oldScope := range strings.Split(token.Scope, ",") {
			if newScopes, exists := accessTokenScopeMap[OldAccessTokenScope(oldScope)]; exists {
				for _, newScope := range newScopes {
					allNewScopesMap[newScope] = true
				}
			} else {
				return fmt.Errorf("old access token scope %s does not exist", oldScope)
			}
		}

		scopes := make([]string, 0, len(allNewScopesMap))
		for s := range allNewScopesMap {
			scopes = append(scopes, string(s))
		}
		scope := auth_model.AccessTokenScope(strings.Join(scopes, ","))

		// normalize the scope
		normScope, err := scope.Normalize()
		if err != nil {
			return err
		}

		token.Scope = string(normScope)

		// update the db entry with the new scope
		if _, err = x.Cols("scope").Update(token); err != nil {
			return err
		}
	}

	return nil
}
