// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"strings"
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

// AccessTokenScopeBitmap represents a bitmap of access token scopes.
type AccessTokenScopeBitmap uint64

// AccessTokenScopeAllBitmap is the bitmap of all access token scopes.
var AccessTokenScopeAllBitmap AccessTokenScopeBitmap = 1<<uint(len(AllAccessTokenScopes)) - 1

// Parse parses the scope string into a bitmap, thus removing possible duplicates.
func (s AccessTokenScope) Parse() (AccessTokenScopeBitmap, error) {
	list := strings.Split(string(s), ",")

	var bitmap AccessTokenScopeBitmap
	for _, v := range list {
		if v == "" {
			continue
		}
		if v == AccessTokenScopeAll {
			return AccessTokenScopeAllBitmap, nil
		}

		idx := sliceIndex(AllAccessTokenScopes, v)
		if idx < 0 {
			return 0, fmt.Errorf("invalid access token scope: %s", v)
		}
		bitmap |= 1 << uint(idx)

		// take care of child scopes
		switch v {
		case AccessTokenScopeRepo:
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeRepoStatus))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopePublicRepo))
		case AccessTokenScopeAdminOrg:
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeWriteOrg))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeReadOrg))
		case AccessTokenScopeAdminPublicKey:
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeWritePublicKey))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeReadPublicKey))
		case AccessTokenScopeAdminRepoHook:
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeWriteRepoHook))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeReadRepoHook))
		case AccessTokenScopeUser:
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeReadUser))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeUserEmail))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeUserFollow))
		case AccessTokenScopePackage:
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeWritePackage))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeReadPackage))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeDeletePackage))
		case AccessTokenScopeAdminGPGKey:
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeWriteGPGKey))
			bitmap |= 1 << uint(sliceIndex(AllAccessTokenScopes, AccessTokenScopeReadGPGKey))
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
	index := sliceIndex(AllAccessTokenScopes, scope)
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

	scope := AccessTokenScope(strings.Join(scopes, ","))
	if scope == "repo,admin:org,admin:public_key,admin:repo_hook,admin:org_hook,notification,user,delete_repo,package,admin:gpg_key" {
		return AccessTokenScopeAll
	}
	return scope
}

// sliceIndex returns the index of the first instance of str in slice, or -1 if str is not present in slice.
func sliceIndex(slice []string, element string) int {
	for i, v := range slice {
		if v == element {
			return i
		}
	}
	return -1
}
