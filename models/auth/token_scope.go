// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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

// AccessTokenScopeBitmap represents a bitmap of access token scopes.
type AccessTokenScopeBitmap uint64

// Bitmap of each scope, including the child scopes.
const (
	// AccessTokenScopeAllBits is the bitmap of all access token scopes.
	AccessTokenScopeAllBits = AccessTokenScopeRepoBits |
		AccessTokenScopeAdminOrgBits | AccessTokenScopeAdminPublicKeyBits | AccessTokenScopeAdminOrgHookBits |
		AccessTokenScopeNotificationBits | AccessTokenScopeUserBits | AccessTokenScopeDeleteRepoBits |
		AccessTokenScopePackageBits | AccessTokenScopeAdminGPGKeyBits | AccessTokenScopeAdminApplicationBits

	AccessTokenScopeRepoBits       = 1<<iota | AccessTokenScopeRepoStatusBits | AccessTokenScopePublicRepoBits | AccessTokenScopeAdminRepoHookBits
	AccessTokenScopeRepoStatusBits = 1 << iota
	AccessTokenScopePublicRepoBits = 1 << iota

	AccessTokenScopeAdminOrgBits = 1<<iota | AccessTokenScopeWriteOrgBits
	AccessTokenScopeWriteOrgBits = 1<<iota | AccessTokenScopeReadOrgBits
	AccessTokenScopeReadOrgBits  = 1 << iota

	AccessTokenScopeAdminPublicKeyBits = 1<<iota | AccessTokenScopeWritePublicKeyBits
	AccessTokenScopeWritePublicKeyBits = 1<<iota | AccessTokenScopeReadPublicKeyBits
	AccessTokenScopeReadPublicKeyBits  = 1 << iota

	AccessTokenScopeAdminRepoHookBits = 1<<iota | AccessTokenScopeWriteRepoHookBits
	AccessTokenScopeWriteRepoHookBits = 1<<iota | AccessTokenScopeReadRepoHookBits
	AccessTokenScopeReadRepoHookBits  = 1 << iota

	AccessTokenScopeAdminOrgHookBits = 1 << iota

	AccessTokenScopeNotificationBits = 1 << iota

	AccessTokenScopeUserBits       = 1<<iota | AccessTokenScopeReadUserBits | AccessTokenScopeUserEmailBits | AccessTokenScopeUserFollowBits
	AccessTokenScopeReadUserBits   = 1 << iota
	AccessTokenScopeUserEmailBits  = 1 << iota
	AccessTokenScopeUserFollowBits = 1 << iota

	AccessTokenScopeDeleteRepoBits = 1 << iota

	AccessTokenScopePackageBits       = 1<<iota | AccessTokenScopeWritePackageBits | AccessTokenScopeDeletePackageBits
	AccessTokenScopeWritePackageBits  = 1<<iota | AccessTokenScopeReadPackageBits
	AccessTokenScopeReadPackageBits   = 1 << iota
	AccessTokenScopeDeletePackageBits = 1 << iota

	AccessTokenScopeAdminGPGKeyBits = 1<<iota | AccessTokenScopeWriteGPGKeyBits
	AccessTokenScopeWriteGPGKeyBits = 1<<iota | AccessTokenScopeReadGPGKeyBits
	AccessTokenScopeReadGPGKeyBits  = 1 << iota

	AccessTokenScopeAdminApplicationBits = 1<<iota | AccessTokenScopeWriteApplicationBits
	AccessTokenScopeWriteApplicationBits = 1<<iota | AccessTokenScopeReadApplicationBits
	AccessTokenScopeReadApplicationBits  = 1 << iota

	AccessTokenScopeSudoBits = 1 << iota

	// The current implementation only supports up to 64 token scopes.
	// If we need to support > 64 scopes,
	// refactoring the whole implementation in this file (and only this file) is needed.
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

// AllAccessTokenScopeBits contains all access token scopes.
// The order must be the same as AllAccessTokenScopeBits.
var AllAccessTokenScopeBits = []AccessTokenScopeBitmap{
	AccessTokenScopeRepoBits, AccessTokenScopeRepoStatusBits, AccessTokenScopePublicRepoBits,
	AccessTokenScopeAdminOrgBits, AccessTokenScopeWriteOrgBits, AccessTokenScopeReadOrgBits,
	AccessTokenScopeAdminPublicKeyBits, AccessTokenScopeWritePublicKeyBits, AccessTokenScopeReadPublicKeyBits,
	AccessTokenScopeAdminRepoHookBits, AccessTokenScopeWriteRepoHookBits, AccessTokenScopeReadRepoHookBits,
	AccessTokenScopeAdminOrgHookBits,
	AccessTokenScopeNotificationBits,
	AccessTokenScopeUserBits, AccessTokenScopeReadUserBits, AccessTokenScopeUserEmailBits, AccessTokenScopeUserFollowBits,
	AccessTokenScopeDeleteRepoBits,
	AccessTokenScopePackageBits, AccessTokenScopeWritePackageBits, AccessTokenScopeReadPackageBits, AccessTokenScopeDeletePackageBits,
	AccessTokenScopeAdminGPGKeyBits, AccessTokenScopeWriteGPGKeyBits, AccessTokenScopeReadGPGKeyBits,
	AccessTokenScopeAdminApplicationBits, AccessTokenScopeWriteApplicationBits, AccessTokenScopeReadApplicationBits,
	AccessTokenScopeSudoBits,
}

// Parse parses the scope string into a bitmap, thus removing possible duplicates.
func (s AccessTokenScope) Parse() (AccessTokenScopeBitmap, error) {
	list := strings.Split(string(s), ",")

	var bitmap AccessTokenScopeBitmap
	for _, v := range list {
		if v == "" {
			continue
		}
		if v == AccessTokenScopeAll {
			bitmap |= AccessTokenScopeAllBits
			continue
		}

		idx := util.SliceFindString(v, AllAccessTokenScopes)
		if idx < 0 {
			return 0, fmt.Errorf("invalid access token scope: %s", v)
		}
		bitmap |= AllAccessTokenScopeBits[idx]
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
	bitmap, err := s.Parse()
	if err != nil {
		return false, err
	}

	return bitmap.HasScope(scope)
}

// HasScope returns true if the string has the given scope
func (bitmap AccessTokenScopeBitmap) HasScope(scope string) (bool, error) {
	index := util.SliceFindString(scope, AllAccessTokenScopes)
	if index == -1 {
		return false, fmt.Errorf("invalid access token scope: %s", scope)
	}

	expectedBitmap := AllAccessTokenScopeBits[index]

	return bitmap&expectedBitmap == expectedBitmap, nil
}

// ToScope returns a normalized scope string without any duplicates.
func (bitmap AccessTokenScopeBitmap) ToScope() AccessTokenScope {
	var scopes []string

	// iterate over all scopes, and reconstruct the bitmap
	// if the reconstructed bitmap doesn't change, then the scope is already included
	var reconstruct AccessTokenScopeBitmap

	for i, v := range AllAccessTokenScopes {
		// no need for error checking here, since we know the scope is valid
		if ok, _ := bitmap.HasScope(v); ok {
			current := reconstruct | AllAccessTokenScopeBits[i]
			if current == reconstruct {
				continue
			}

			reconstruct = current
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
