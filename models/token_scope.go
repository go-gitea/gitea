package models

import (
	"fmt"
	"strings"
)

type AccessTokenScope string

const (
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
}

func (s AccessTokenScope) Parse() (AccessTokenScopeBitmap, error) {
	list := strings.Split(string(s), ",")

	var bitmap AccessTokenScopeBitmap
	for _, v := range list {
		if v == "" {
			continue
		}

		idx := sliceIndex(AllAccessTokenScopes, v)
		if idx < 0 {
			return 0, fmt.Errorf("invalid access token scope: %s", v)
		}
		bitmap |= 1 << uint(idx)
	}
	return bitmap, nil
}

func (s AccessTokenScope) Normalize() (AccessTokenScope, error) {
	bitmap, err := s.Parse()
	if err != nil {
		return "", err
	}

	return bitmap.ToScope(), nil
}

type AccessTokenScopeBitmap uint64

func (bitmap AccessTokenScopeBitmap) ToScope() AccessTokenScope {
	var scopes []string

	groupedScope := make(map[string]struct{})
	for i, v := range AllAccessTokenScopes {
		if bitmap&(1<<uint(i)) != 0 {
			switch v {
			// Parse scopes that contains multiple sub-scopes
			case AccessTokenScopeRepo, AccessTokenScopeAdminOrg, AccessTokenScopeAdminPublicKey,
				AccessTokenScopeAdminRepoHook, AccessTokenScopeUser, AccessTokenScopePackage, AccessTokenScopeAdminGPGKey:
				groupedScope[v] = struct{}{}

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
			}
			scopes = append(scopes, v)
		}
	}

	return AccessTokenScope(strings.Join(scopes, ","))
}

func sliceIndex(slice []string, element string) int {
	for i, v := range slice {
		if v == element {
			return i
		}
	}
	return -1
}
