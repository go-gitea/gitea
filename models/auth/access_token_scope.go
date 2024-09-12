// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/perm"
)

// AccessTokenScopeCategory represents the scope category for an access token
type AccessTokenScopeCategory int

const (
	AccessTokenScopeCategoryActivityPub = iota
	AccessTokenScopeCategoryAdmin
	AccessTokenScopeCategoryMisc // WARN: this is now just a placeholder, don't remove it which will change the following values
	AccessTokenScopeCategoryNotification
	AccessTokenScopeCategoryOrganization
	AccessTokenScopeCategoryPackage
	AccessTokenScopeCategoryIssue
	AccessTokenScopeCategoryRepository
	AccessTokenScopeCategoryUser
)

// AllAccessTokenScopeCategories contains all access token scope categories
var AllAccessTokenScopeCategories = []AccessTokenScopeCategory{
	AccessTokenScopeCategoryActivityPub,
	AccessTokenScopeCategoryAdmin,
	AccessTokenScopeCategoryMisc,
	AccessTokenScopeCategoryNotification,
	AccessTokenScopeCategoryOrganization,
	AccessTokenScopeCategoryPackage,
	AccessTokenScopeCategoryIssue,
	AccessTokenScopeCategoryRepository,
	AccessTokenScopeCategoryUser,
}

// AccessTokenScopeLevel represents the access levels without a given scope category
type AccessTokenScopeLevel int

const (
	NoAccess AccessTokenScopeLevel = iota
	Read
	Write
)

// AccessTokenScope represents the scope for an access token.
type AccessTokenScope string

// for all categories, write implies read
const (
	AccessTokenScopeAll        AccessTokenScope = "all"
	AccessTokenScopePublicOnly AccessTokenScope = "public-only" // limited to public orgs/repos

	AccessTokenScopeReadActivityPub  AccessTokenScope = "read:activitypub"
	AccessTokenScopeWriteActivityPub AccessTokenScope = "write:activitypub"

	AccessTokenScopeReadAdmin  AccessTokenScope = "read:admin"
	AccessTokenScopeWriteAdmin AccessTokenScope = "write:admin"

	AccessTokenScopeReadMisc  AccessTokenScope = "read:misc"
	AccessTokenScopeWriteMisc AccessTokenScope = "write:misc"

	AccessTokenScopeReadNotification  AccessTokenScope = "read:notification"
	AccessTokenScopeWriteNotification AccessTokenScope = "write:notification"

	AccessTokenScopeReadOrganization  AccessTokenScope = "read:organization"
	AccessTokenScopeWriteOrganization AccessTokenScope = "write:organization"

	AccessTokenScopeReadPackage  AccessTokenScope = "read:package"
	AccessTokenScopeWritePackage AccessTokenScope = "write:package"

	AccessTokenScopeReadIssue  AccessTokenScope = "read:issue"
	AccessTokenScopeWriteIssue AccessTokenScope = "write:issue"

	AccessTokenScopeReadRepository  AccessTokenScope = "read:repository"
	AccessTokenScopeWriteRepository AccessTokenScope = "write:repository"

	AccessTokenScopeReadUser  AccessTokenScope = "read:user"
	AccessTokenScopeWriteUser AccessTokenScope = "write:user"
)

// accessTokenScopeBitmap represents a bitmap of access token scopes.
type accessTokenScopeBitmap uint64

// Bitmap of each scope, including the child scopes.
const (
	// AccessTokenScopeAllBits is the bitmap of all access token scopes
	accessTokenScopeAllBits accessTokenScopeBitmap = accessTokenScopeWriteActivityPubBits |
		accessTokenScopeWriteAdminBits | accessTokenScopeWriteMiscBits | accessTokenScopeWriteNotificationBits |
		accessTokenScopeWriteOrganizationBits | accessTokenScopeWritePackageBits | accessTokenScopeWriteIssueBits |
		accessTokenScopeWriteRepositoryBits | accessTokenScopeWriteUserBits

	accessTokenScopePublicOnlyBits accessTokenScopeBitmap = 1 << iota

	accessTokenScopeReadActivityPubBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteActivityPubBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadActivityPubBits

	accessTokenScopeReadAdminBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteAdminBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadAdminBits

	accessTokenScopeReadMiscBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteMiscBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadMiscBits

	accessTokenScopeReadNotificationBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteNotificationBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadNotificationBits

	accessTokenScopeReadOrganizationBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteOrganizationBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadOrganizationBits

	accessTokenScopeReadPackageBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWritePackageBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadPackageBits

	accessTokenScopeReadIssueBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteIssueBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadIssueBits

	accessTokenScopeReadRepositoryBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteRepositoryBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadRepositoryBits

	accessTokenScopeReadUserBits  accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteUserBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadUserBits

	// The current implementation only supports up to 64 token scopes.
	// If we need to support > 64 scopes,
	// refactoring the whole implementation in this file (and only this file) is needed.
)

// allAccessTokenScopes contains all access token scopes.
// The order is important: parent scope must precede child scopes.
var allAccessTokenScopes = []AccessTokenScope{
	AccessTokenScopePublicOnly,
	AccessTokenScopeWriteActivityPub, AccessTokenScopeReadActivityPub,
	AccessTokenScopeWriteAdmin, AccessTokenScopeReadAdmin,
	AccessTokenScopeWriteMisc, AccessTokenScopeReadMisc,
	AccessTokenScopeWriteNotification, AccessTokenScopeReadNotification,
	AccessTokenScopeWriteOrganization, AccessTokenScopeReadOrganization,
	AccessTokenScopeWritePackage, AccessTokenScopeReadPackage,
	AccessTokenScopeWriteIssue, AccessTokenScopeReadIssue,
	AccessTokenScopeWriteRepository, AccessTokenScopeReadRepository,
	AccessTokenScopeWriteUser, AccessTokenScopeReadUser,
}

// allAccessTokenScopeBits contains all access token scopes.
var allAccessTokenScopeBits = map[AccessTokenScope]accessTokenScopeBitmap{
	AccessTokenScopeAll:               accessTokenScopeAllBits,
	AccessTokenScopePublicOnly:        accessTokenScopePublicOnlyBits,
	AccessTokenScopeReadActivityPub:   accessTokenScopeReadActivityPubBits,
	AccessTokenScopeWriteActivityPub:  accessTokenScopeWriteActivityPubBits,
	AccessTokenScopeReadAdmin:         accessTokenScopeReadAdminBits,
	AccessTokenScopeWriteAdmin:        accessTokenScopeWriteAdminBits,
	AccessTokenScopeReadMisc:          accessTokenScopeReadMiscBits,
	AccessTokenScopeWriteMisc:         accessTokenScopeWriteMiscBits,
	AccessTokenScopeReadNotification:  accessTokenScopeReadNotificationBits,
	AccessTokenScopeWriteNotification: accessTokenScopeWriteNotificationBits,
	AccessTokenScopeReadOrganization:  accessTokenScopeReadOrganizationBits,
	AccessTokenScopeWriteOrganization: accessTokenScopeWriteOrganizationBits,
	AccessTokenScopeReadPackage:       accessTokenScopeReadPackageBits,
	AccessTokenScopeWritePackage:      accessTokenScopeWritePackageBits,
	AccessTokenScopeReadIssue:         accessTokenScopeReadIssueBits,
	AccessTokenScopeWriteIssue:        accessTokenScopeWriteIssueBits,
	AccessTokenScopeReadRepository:    accessTokenScopeReadRepositoryBits,
	AccessTokenScopeWriteRepository:   accessTokenScopeWriteRepositoryBits,
	AccessTokenScopeReadUser:          accessTokenScopeReadUserBits,
	AccessTokenScopeWriteUser:         accessTokenScopeWriteUserBits,
}

// readAccessTokenScopes maps a scope category to the read permission scope
var accessTokenScopes = map[AccessTokenScopeLevel]map[AccessTokenScopeCategory]AccessTokenScope{
	Read: {
		AccessTokenScopeCategoryActivityPub:  AccessTokenScopeReadActivityPub,
		AccessTokenScopeCategoryAdmin:        AccessTokenScopeReadAdmin,
		AccessTokenScopeCategoryMisc:         AccessTokenScopeReadMisc,
		AccessTokenScopeCategoryNotification: AccessTokenScopeReadNotification,
		AccessTokenScopeCategoryOrganization: AccessTokenScopeReadOrganization,
		AccessTokenScopeCategoryPackage:      AccessTokenScopeReadPackage,
		AccessTokenScopeCategoryIssue:        AccessTokenScopeReadIssue,
		AccessTokenScopeCategoryRepository:   AccessTokenScopeReadRepository,
		AccessTokenScopeCategoryUser:         AccessTokenScopeReadUser,
	},
	Write: {
		AccessTokenScopeCategoryActivityPub:  AccessTokenScopeWriteActivityPub,
		AccessTokenScopeCategoryAdmin:        AccessTokenScopeWriteAdmin,
		AccessTokenScopeCategoryMisc:         AccessTokenScopeWriteMisc,
		AccessTokenScopeCategoryNotification: AccessTokenScopeWriteNotification,
		AccessTokenScopeCategoryOrganization: AccessTokenScopeWriteOrganization,
		AccessTokenScopeCategoryPackage:      AccessTokenScopeWritePackage,
		AccessTokenScopeCategoryIssue:        AccessTokenScopeWriteIssue,
		AccessTokenScopeCategoryRepository:   AccessTokenScopeWriteRepository,
		AccessTokenScopeCategoryUser:         AccessTokenScopeWriteUser,
	},
}

// GetRequiredScopes gets the specific scopes for a given level and categories
func GetRequiredScopes(level AccessTokenScopeLevel, scopeCategories ...AccessTokenScopeCategory) []AccessTokenScope {
	scopes := make([]AccessTokenScope, 0, len(scopeCategories))
	for _, cat := range scopeCategories {
		scopes = append(scopes, accessTokenScopes[level][cat])
	}
	return scopes
}

// ContainsCategory checks if a list of categories contains a specific category
func ContainsCategory(categories []AccessTokenScopeCategory, category AccessTokenScopeCategory) bool {
	for _, c := range categories {
		if c == category {
			return true
		}
	}
	return false
}

// GetScopeLevelFromAccessMode converts permission access mode to scope level
func GetScopeLevelFromAccessMode(mode perm.AccessMode) AccessTokenScopeLevel {
	switch mode {
	case perm.AccessModeNone:
		return NoAccess
	case perm.AccessModeRead:
		return Read
	case perm.AccessModeWrite:
		return Write
	case perm.AccessModeAdmin:
		return Write
	case perm.AccessModeOwner:
		return Write
	default:
		return NoAccess
	}
}

// parse the scope string into a bitmap, thus removing possible duplicates.
func (s AccessTokenScope) parse() (accessTokenScopeBitmap, error) {
	var bitmap accessTokenScopeBitmap

	// The following is the more performant equivalent of 'for _, v := range strings.Split(remainingScope, ",")' as this is hot code
	remainingScopes := string(s)
	for len(remainingScopes) > 0 {
		i := strings.IndexByte(remainingScopes, ',')
		var v string
		if i < 0 {
			v = remainingScopes
			remainingScopes = ""
		} else if i+1 >= len(remainingScopes) {
			v = remainingScopes[:i]
			remainingScopes = ""
		} else {
			v = remainingScopes[:i]
			remainingScopes = remainingScopes[i+1:]
		}
		singleScope := AccessTokenScope(v)
		if singleScope == "" {
			continue
		}
		if singleScope == AccessTokenScopeAll {
			bitmap |= accessTokenScopeAllBits
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

// StringSlice returns the AccessTokenScope as a []string
func (s AccessTokenScope) StringSlice() []string {
	return strings.Split(string(s), ",")
}

// Normalize returns a normalized scope string without any duplicates.
func (s AccessTokenScope) Normalize() (AccessTokenScope, error) {
	bitmap, err := s.parse()
	if err != nil {
		return "", err
	}

	return bitmap.toScope(), nil
}

// PublicOnly checks if this token scope is limited to public resources
func (s AccessTokenScope) PublicOnly() (bool, error) {
	bitmap, err := s.parse()
	if err != nil {
		return false, err
	}

	return bitmap.hasScope(AccessTokenScopePublicOnly)
}

// HasScope returns true if the string has the given scope
func (s AccessTokenScope) HasScope(scopes ...AccessTokenScope) (bool, error) {
	bitmap, err := s.parse()
	if err != nil {
		return false, err
	}

	for _, s := range scopes {
		if has, err := bitmap.hasScope(s); !has || err != nil {
			return has, err
		}
	}

	return true, nil
}

// HasAnyScope returns true if any of the scopes is contained in the string
func (s AccessTokenScope) HasAnyScope(scopes ...AccessTokenScope) (bool, error) {
	bitmap, err := s.parse()
	if err != nil {
		return false, err
	}

	for _, s := range scopes {
		if has, err := bitmap.hasScope(s); has || err != nil {
			return has, err
		}
	}

	return false, nil
}

// hasScope returns true if the string has the given scope
func (bitmap accessTokenScopeBitmap) hasScope(scope AccessTokenScope) (bool, error) {
	expectedBits, ok := allAccessTokenScopeBits[scope]
	if !ok {
		return false, fmt.Errorf("invalid access token scope: %s", scope)
	}

	return bitmap&expectedBits == expectedBits, nil
}

// toScope returns a normalized scope string without any duplicates.
func (bitmap accessTokenScopeBitmap) toScope() AccessTokenScope {
	var scopes []string

	// iterate over all scopes, and reconstruct the bitmap
	// if the reconstructed bitmap doesn't change, then the scope is already included
	var reconstruct accessTokenScopeBitmap

	for _, singleScope := range allAccessTokenScopes {
		// no need for error checking here, since we know the scope is valid
		if ok, _ := bitmap.hasScope(singleScope); ok {
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
		"write:activitypub,write:admin,write:misc,write:notification,write:organization,write:package,write:issue,write:repository,write:user",
		"all",
	))
	return scope
}
