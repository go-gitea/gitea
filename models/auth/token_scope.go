// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// AccessTokenScope represents the scope for an access token.
type AccessTokenScope string

const (
	AccessTokenScopeAll = "all"

	AccessTokenScopeRepo       = "repo"
	AccessTokenScopeRepoStatus = "repo:status"
	AccessTokenScopePublicRepo = "public_repo"

	AccessTokenScopeAdminOrg = "admin:org"
	AccessTokenScopeWriteOrg = "write:org"
	AccessTokenScopeReadOrg  = "read:org"

	AccessTokenScopeAdminPublicKey = "admin:public_key"
	AccessTokenScopeWritePublicKey = "write:public_key"
	AccessTokenScopeReadPublicKey  = "read:public_key"

	AccessTokenScopeAdminRepoHook = "admin:repo_hook"
	AccessTokenScopeWriteRepoHook = "write:repo_hook"
	AccessTokenScopeReadRepoHook  = "read:repo_hook"

	AccessTokenScopeAdminOrgHook = "admin:org_hook"

	AccessTokenScopeNotification = "notification"

	AccessTokenScopeUser       = "user"
	AccessTokenScopeReadUser   = "read:user"
	AccessTokenScopeUserEmail  = "user:email"
	AccessTokenScopeUserFollow = "user:follow"

	AccessTokenScopeDeleteRepo = "delete_repo"

	AccessTokenScopePackage       = "package"
	AccessTokenScopeWritePackage  = "write:package"
	AccessTokenScopeReadPackage   = "read:package"
	AccessTokenScopeDeletePackage = "delete:package"

	AccessTokenScopeAdminGPGKey = "admin:gpg_key"
	AccessTokenScopeWriteGPGKey = "write:gpg_key"
	AccessTokenScopeReadGPGKey  = "read:gpg_key"

	AccessTokenScopeAdminApplication = "admin:application"
	AccessTokenScopeWriteApplication = "write:application"
	AccessTokenScopeReadApplication  = "read:application"

	AccessTokenScopeSudo = "sudo"
)

// AllAccessTokenScopes contains all access token scopes.
// The order is important: parent scope must precedes child scopes.
var AllAccessTokenScopes = []string{
	AccessTokenScopeRepo, AccessTokenScopeRepoStatus, AccessTokenScopePublicRepo,
	AccessTokenScopeAdminOrg, AccessTokenScopeWriteOrg, AccessTokenScopeReadOrg,
	AccessTokenScopeAdminPublicKey, AccessTokenScopeWritePublicKey, AccessTokenScopeReadPublicKey,
	AccessTokenScopeAdminRepoHook, AccessTokenScopeWriteRepoHook, AccessTokenScopeReadRepoHook,
	AccessTokenScopeAdminOrgHook,
	AccessTokenScopeNotification,
	AccessTokenScopeUser, AccessTokenScopeReadUser, AccessTokenScopeUserEmail, AccessTokenScopeUserFollow,
	AccessTokenScopeDeleteRepo,
	AccessTokenScopePackage, AccessTokenScopeWritePackage, AccessTokenScopeReadPackage, AccessTokenScopeDeletePackage,
	AccessTokenScopeAdminGPGKey, AccessTokenScopeWriteGPGKey, AccessTokenScopeReadGPGKey,
	AccessTokenScopeAdminApplication, AccessTokenScopeWriteApplication, AccessTokenScopeReadApplication,
	AccessTokenScopeSudo,
}

// AccessTokenScopeBitmap represents a bitmap of access token scopes.
type AccessTokenScopeBitmap uint64

// AccessTokenScopeAllBitmap is the bitmap of all access token scopes.
var AccessTokenScopeAllBitmap AccessTokenScopeBitmap = 1<<uint(len(AllAccessTokenScopes)-1) - 1 // sudo is a special scope to be excluded, so -1 from the length

// Parse parses the scope string into a bitmap, thus removing possible duplicates.
func (s AccessTokenScope) Parse() (AccessTokenScopeBitmap, error) {
	list := strings.Split(string(s), ",")

	var bitmap AccessTokenScopeBitmap
	for _, v := range list {
		if v == "" {
			continue
		}
		if v == AccessTokenScopeAll {
			bitmap |= AccessTokenScopeAllBitmap
			continue
		}

		idx := util.FindStringInSlice(v, AllAccessTokenScopes)
		if idx < 0 {
			return 0, fmt.Errorf("invalid access token scope: %s", v)
		}
		bitmap |= 1 << uint(idx)

		// take care of child scopes
		switch v {
		case AccessTokenScopeRepo:
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeRepoStatus, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopePublicRepo, AllAccessTokenScopes))
			// admin:repo_hook, write:repo_hook, read:repo_hook
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeAdminRepoHook, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeWriteRepoHook, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeReadRepoHook, AllAccessTokenScopes))
		case AccessTokenScopeAdminOrg:
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeWriteOrg, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeReadOrg, AllAccessTokenScopes))
		case AccessTokenScopeAdminPublicKey:
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeWritePublicKey, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeReadPublicKey, AllAccessTokenScopes))
		case AccessTokenScopeAdminRepoHook:
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeWriteRepoHook, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeReadRepoHook, AllAccessTokenScopes))
		case AccessTokenScopeUser:
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeReadUser, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeUserEmail, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeUserFollow, AllAccessTokenScopes))
		case AccessTokenScopePackage:
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeWritePackage, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeReadPackage, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeDeletePackage, AllAccessTokenScopes))
		case AccessTokenScopeAdminGPGKey:
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeWriteGPGKey, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeReadGPGKey, AllAccessTokenScopes))
		case AccessTokenScopeAdminApplication:
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeWriteApplication, AllAccessTokenScopes))
			bitmap |= 1 << uint(util.FindStringInSlice(AccessTokenScopeReadApplication, AllAccessTokenScopes))
		}
	}
	return bitmap, nil
}

// Normalize returns a normalized scope string without any duplicates.
func (s AccessTokenScope) Normalize() (AccessTokenScope, error) {
	bitmap, err := s.Parse()
	if err != nil {
		return "", err
	}

	return bitmap.ToScope(), nil
}

// HasScope returns true if the string has the given scope
func (s AccessTokenScope) HasScope(scope string) (bool, error) {
	index := util.FindStringInSlice(scope, AllAccessTokenScopes)
	if index == -1 {
		return false, fmt.Errorf("invalid access token scope: %s", scope)
	}

	bitmap, err := s.Parse()
	if err != nil {
		return false, err
	}

	return bitmap&(1<<uint(index)) != 0, nil
}

// ToScope returns a normalized scope string without any duplicates.
func (bitmap AccessTokenScopeBitmap) ToScope() AccessTokenScope {
	var scopes []string

	groupedScope := make(map[string]struct{})
	for i, v := range AllAccessTokenScopes {
		if bitmap&(1<<uint(i)) != 0 {
			switch v {
			// Parse scopes that contains multiple sub-scopes
			case AccessTokenScopeRepo, AccessTokenScopeAdminOrg, AccessTokenScopeAdminPublicKey,
				AccessTokenScopeUser, AccessTokenScopePackage, AccessTokenScopeAdminGPGKey:
				groupedScope[v] = struct{}{}
			case AccessTokenScopeAdminRepoHook:
				groupedScope[v] = struct{}{}
				if _, ok := groupedScope[AccessTokenScopeRepo]; ok {
					continue
				}

			// If parent scope is set, all sub-scopes shouldn't be added
			case AccessTokenScopeRepoStatus, AccessTokenScopePublicRepo:
				if _, ok := groupedScope[AccessTokenScopeRepo]; ok {
					continue
				}
			case AccessTokenScopeWriteOrg, AccessTokenScopeReadOrg:
				if _, ok := groupedScope[AccessTokenScopeAdminOrg]; ok {
					continue
				}
			case AccessTokenScopeWritePublicKey, AccessTokenScopeReadPublicKey:
				if _, ok := groupedScope[AccessTokenScopeAdminPublicKey]; ok {
					continue
				}
			case AccessTokenScopeWriteRepoHook, AccessTokenScopeReadRepoHook:
				if _, ok := groupedScope[AccessTokenScopeAdminRepoHook]; ok {
					continue
				}
				if _, ok := groupedScope[AccessTokenScopeRepo]; ok {
					continue
				}
			case AccessTokenScopeReadUser, AccessTokenScopeUserEmail, AccessTokenScopeUserFollow:
				if _, ok := groupedScope[AccessTokenScopeUser]; ok {
					continue
				}
			case AccessTokenScopeWritePackage, AccessTokenScopeReadPackage, AccessTokenScopeDeletePackage:
				if _, ok := groupedScope[AccessTokenScopePackage]; ok {
					continue
				}
			case AccessTokenScopeWriteGPGKey, AccessTokenScopeReadGPGKey:
				if _, ok := groupedScope[AccessTokenScopeAdminGPGKey]; ok {
					continue
				}
			case AccessTokenScopeWriteApplication, AccessTokenScopeReadApplication:
				if _, ok := groupedScope[AccessTokenScopeAdminApplication]; ok {
					continue
				}
			}
			scopes = append(scopes, v)
		}
	}

	scope := AccessTokenScope(strings.Join(scopes, ","))
	scope = AccessTokenScope(strings.ReplaceAll(
		string(scope),
		"repo,admin:org,admin:public_key,admin:org_hook,notification,user,delete_repo,package,admin:gpg_key,admin:application",
		"all",
	))
	return scope
}
