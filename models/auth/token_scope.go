// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"strings"
)

// AccessTokenScope represents the scope for an access token.
type AccessTokenScope string

const (
	AccessTokenScopeAll AccessTokenScope = "all"

	AccessTokenScopeRepo       AccessTokenScope = "repo"
	AccessTokenScopeRepoStatus AccessTokenScope = "repo:status"
	AccessTokenScopePublicRepo AccessTokenScope = "public_repo"

	AccessTokenScopeAdminOrg AccessTokenScope = "admin:org"
	AccessTokenScopeWriteOrg AccessTokenScope = "write:org"
	AccessTokenScopeReadOrg  AccessTokenScope = "read:org"

	AccessTokenScopeAdminPublicKey AccessTokenScope = "admin:public_key"
	AccessTokenScopeWritePublicKey AccessTokenScope = "write:public_key"
	AccessTokenScopeReadPublicKey  AccessTokenScope = "read:public_key"

	AccessTokenScopeAdminRepoHook AccessTokenScope = "admin:repo_hook"
	AccessTokenScopeWriteRepoHook AccessTokenScope = "write:repo_hook"
	AccessTokenScopeReadRepoHook  AccessTokenScope = "read:repo_hook"

	AccessTokenScopeAdminOrgHook AccessTokenScope = "admin:org_hook"

	AccessTokenScopeNotification AccessTokenScope = "notification"

	AccessTokenScopeUser       AccessTokenScope = "user"
	AccessTokenScopeReadUser   AccessTokenScope = "read:user"
	AccessTokenScopeUserEmail  AccessTokenScope = "user:email"
	AccessTokenScopeUserFollow AccessTokenScope = "user:follow"

	AccessTokenScopeDeleteRepo AccessTokenScope = "delete_repo"

	AccessTokenScopePackage       AccessTokenScope = "package"
	AccessTokenScopeWritePackage  AccessTokenScope = "write:package"
	AccessTokenScopeReadPackage   AccessTokenScope = "read:package"
	AccessTokenScopeDeletePackage AccessTokenScope = "delete:package"

	AccessTokenScopeAdminGPGKey AccessTokenScope = "admin:gpg_key"
	AccessTokenScopeWriteGPGKey AccessTokenScope = "write:gpg_key"
	AccessTokenScopeReadGPGKey  AccessTokenScope = "read:gpg_key"

	AccessTokenScopeAdminApplication AccessTokenScope = "admin:application"
	AccessTokenScopeWriteApplication AccessTokenScope = "write:application"
	AccessTokenScopeReadApplication  AccessTokenScope = "read:application"

	AccessTokenScopeSudo AccessTokenScope = "sudo"
)

// AccessTokenScopeBitmap represents a bitmap of access token scopes.
type AccessTokenScopeBitmap uint64

// Bitmap of each scope, including the child scopes.
const (
	// AccessTokenScopeAllBits is the bitmap of all access token scopes, except `sudo`.
	AccessTokenScopeAllBits AccessTokenScopeBitmap = AccessTokenScopeRepoBits |
		AccessTokenScopeAdminOrgBits | AccessTokenScopeAdminPublicKeyBits | AccessTokenScopeAdminOrgHookBits |
		AccessTokenScopeNotificationBits | AccessTokenScopeUserBits | AccessTokenScopeDeleteRepoBits |
		AccessTokenScopePackageBits | AccessTokenScopeAdminGPGKeyBits | AccessTokenScopeAdminApplicationBits

	AccessTokenScopeRepoBits       AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeRepoStatusBits | AccessTokenScopePublicRepoBits | AccessTokenScopeAdminRepoHookBits
	AccessTokenScopeRepoStatusBits AccessTokenScopeBitmap = 1 << iota
	AccessTokenScopePublicRepoBits AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeAdminOrgBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeWriteOrgBits
	AccessTokenScopeWriteOrgBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeReadOrgBits
	AccessTokenScopeReadOrgBits  AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeAdminPublicKeyBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeWritePublicKeyBits
	AccessTokenScopeWritePublicKeyBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeReadPublicKeyBits
	AccessTokenScopeReadPublicKeyBits  AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeAdminRepoHookBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeWriteRepoHookBits
	AccessTokenScopeWriteRepoHookBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeReadRepoHookBits
	AccessTokenScopeReadRepoHookBits  AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeAdminOrgHookBits AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeNotificationBits AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeUserBits       AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeReadUserBits | AccessTokenScopeUserEmailBits | AccessTokenScopeUserFollowBits
	AccessTokenScopeReadUserBits   AccessTokenScopeBitmap = 1 << iota
	AccessTokenScopeUserEmailBits  AccessTokenScopeBitmap = 1 << iota
	AccessTokenScopeUserFollowBits AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeDeleteRepoBits AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopePackageBits       AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeWritePackageBits | AccessTokenScopeDeletePackageBits
	AccessTokenScopeWritePackageBits  AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeReadPackageBits
	AccessTokenScopeReadPackageBits   AccessTokenScopeBitmap = 1 << iota
	AccessTokenScopeDeletePackageBits AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeAdminGPGKeyBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeWriteGPGKeyBits
	AccessTokenScopeWriteGPGKeyBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeReadGPGKeyBits
	AccessTokenScopeReadGPGKeyBits  AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeAdminApplicationBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeWriteApplicationBits
	AccessTokenScopeWriteApplicationBits AccessTokenScopeBitmap = 1<<iota | AccessTokenScopeReadApplicationBits
	AccessTokenScopeReadApplicationBits  AccessTokenScopeBitmap = 1 << iota

	AccessTokenScopeSudoBits AccessTokenScopeBitmap = 1 << iota

	// The current implementation only supports up to 64 token scopes.
	// If we need to support > 64 scopes,
	// refactoring the whole implementation in this file (and only this file) is needed.
)

// allAccessTokenScopes contains all access token scopes.
// The order is important: parent scope must precedes child scopes.
var allAccessTokenScopes = []AccessTokenScope{
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

// allAccessTokenScopeBits contains all access token scopes.
var allAccessTokenScopeBits = map[AccessTokenScope]AccessTokenScopeBitmap{
	AccessTokenScopeRepo:             AccessTokenScopeRepoBits,
	AccessTokenScopeRepoStatus:       AccessTokenScopeRepoStatusBits,
	AccessTokenScopePublicRepo:       AccessTokenScopePublicRepoBits,
	AccessTokenScopeAdminOrg:         AccessTokenScopeAdminOrgBits,
	AccessTokenScopeWriteOrg:         AccessTokenScopeWriteOrgBits,
	AccessTokenScopeReadOrg:          AccessTokenScopeReadOrgBits,
	AccessTokenScopeAdminPublicKey:   AccessTokenScopeAdminPublicKeyBits,
	AccessTokenScopeWritePublicKey:   AccessTokenScopeWritePublicKeyBits,
	AccessTokenScopeReadPublicKey:    AccessTokenScopeReadPublicKeyBits,
	AccessTokenScopeAdminRepoHook:    AccessTokenScopeAdminRepoHookBits,
	AccessTokenScopeWriteRepoHook:    AccessTokenScopeWriteRepoHookBits,
	AccessTokenScopeReadRepoHook:     AccessTokenScopeReadRepoHookBits,
	AccessTokenScopeAdminOrgHook:     AccessTokenScopeAdminOrgHookBits,
	AccessTokenScopeNotification:     AccessTokenScopeNotificationBits,
	AccessTokenScopeUser:             AccessTokenScopeUserBits,
	AccessTokenScopeReadUser:         AccessTokenScopeReadUserBits,
	AccessTokenScopeUserEmail:        AccessTokenScopeUserEmailBits,
	AccessTokenScopeUserFollow:       AccessTokenScopeUserFollowBits,
	AccessTokenScopeDeleteRepo:       AccessTokenScopeDeleteRepoBits,
	AccessTokenScopePackage:          AccessTokenScopePackageBits,
	AccessTokenScopeWritePackage:     AccessTokenScopeWritePackageBits,
	AccessTokenScopeReadPackage:      AccessTokenScopeReadPackageBits,
	AccessTokenScopeDeletePackage:    AccessTokenScopeDeletePackageBits,
	AccessTokenScopeAdminGPGKey:      AccessTokenScopeAdminGPGKeyBits,
	AccessTokenScopeWriteGPGKey:      AccessTokenScopeWriteGPGKeyBits,
	AccessTokenScopeReadGPGKey:       AccessTokenScopeReadGPGKeyBits,
	AccessTokenScopeAdminApplication: AccessTokenScopeAdminApplicationBits,
	AccessTokenScopeWriteApplication: AccessTokenScopeWriteApplicationBits,
	AccessTokenScopeReadApplication:  AccessTokenScopeReadApplicationBits,
	AccessTokenScopeSudo:             AccessTokenScopeSudoBits,
}

// Parse parses the scope string into a bitmap, thus removing possible duplicates.
func (s AccessTokenScope) Parse() (AccessTokenScopeBitmap, error) {
	list := strings.Split(string(s), ",")

	var bitmap AccessTokenScopeBitmap
	for _, v := range list {
		singleScope := AccessTokenScope(v)
		if singleScope == "" {
			continue
		}
		if singleScope == AccessTokenScopeAll {
			bitmap |= AccessTokenScopeAllBits
			continue
		}

		bits, ok := allAccessTokenScopeBits[singleScope]
		if !ok {
			return 0, fmt.Errorf("invalid access token scope: %s", singleScope)
		}
		bitmap |= bits
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
func (s AccessTokenScope) HasScope(scope AccessTokenScope) (bool, error) {
	bitmap, err := s.Parse()
	if err != nil {
		return false, err
	}

	return bitmap.HasScope(scope)
}

// HasScope returns true if the string has the given scope
func (bitmap AccessTokenScopeBitmap) HasScope(scope AccessTokenScope) (bool, error) {
	expectedBits, ok := allAccessTokenScopeBits[scope]
	if !ok {
		return false, fmt.Errorf("invalid access token scope: %s", scope)
	}

	return bitmap&expectedBits == expectedBits, nil
}

// ToScope returns a normalized scope string without any duplicates.
func (bitmap AccessTokenScopeBitmap) ToScope() AccessTokenScope {
	var scopes []string

	// iterate over all scopes, and reconstruct the bitmap
	// if the reconstructed bitmap doesn't change, then the scope is already included
	var reconstruct AccessTokenScopeBitmap

	for _, singleScope := range allAccessTokenScopes {
		// no need for error checking here, since we know the scope is valid
		if ok, _ := bitmap.HasScope(singleScope); ok {
			current := reconstruct | allAccessTokenScopeBits[singleScope]
			if current == reconstruct {
				continue
			}

			reconstruct = current
			scopes = append(scopes, string(singleScope))
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
