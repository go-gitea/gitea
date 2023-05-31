// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

// unknownAccessTokenScope represents the scope for an access token that isn't
// known be an old token or a new token.
type unknownAccessTokenScope string

// AccessTokenScope represents the scope for an access token.
type AccessTokenScope string

// for all categories, delete implies write, write implies read
const (
	AccessTokenScopeAll        AccessTokenScope = "all"
	AccessTokenScopePublicOnly AccessTokenScope = "public-only" // limited to public orgs/repos

	AccessTokenScopeReadActivityPub   AccessTokenScope = "read:activitypub"
	AccessTokenScopeWriteActivityPub  AccessTokenScope = "write:activitypub"
	AccessTokenScopeDeleteActivityPub AccessTokenScope = "delete:activitypub"

	AccessTokenScopeReadAdmin   AccessTokenScope = "read:admin"
	AccessTokenScopeWriteAdmin  AccessTokenScope = "write:admin"
	AccessTokenScopeDeleteAdmin AccessTokenScope = "delete:admin"

	AccessTokenScopeReadMisc   AccessTokenScope = "read:misc"
	AccessTokenScopeWriteMisc  AccessTokenScope = "write:misc"
	AccessTokenScopeDeleteMisc AccessTokenScope = "delete:misc"

	AccessTokenScopeReadNotification   AccessTokenScope = "read:notification"
	AccessTokenScopeWriteNotification  AccessTokenScope = "write:notification"
	AccessTokenScopeDeleteNotification AccessTokenScope = "delete:notification"

	AccessTokenScopeReadOrganization   AccessTokenScope = "read:organization"
	AccessTokenScopeWriteOrganization  AccessTokenScope = "write:organization"
	AccessTokenScopeDeleteOrganization AccessTokenScope = "delete:organization"

	AccessTokenScopeReadPackage   AccessTokenScope = "read:package"
	AccessTokenScopeWritePackage  AccessTokenScope = "write:package"
	AccessTokenScopeDeletePackage AccessTokenScope = "delete:package"

	AccessTokenScopeReadIssue   AccessTokenScope = "read:issue"
	AccessTokenScopeWriteIssue  AccessTokenScope = "write:issue"
	AccessTokenScopeDeleteIssue AccessTokenScope = "delete:issue"

	AccessTokenScopeReadRepository   AccessTokenScope = "read:repository"
	AccessTokenScopeWriteRepository  AccessTokenScope = "write:repository"
	AccessTokenScopeDeleteRepository AccessTokenScope = "delete:repository"

	AccessTokenScopeReadUser   AccessTokenScope = "read:user"
	AccessTokenScopeWriteUser  AccessTokenScope = "write:user"
	AccessTokenScopeDeleteUser AccessTokenScope = "delete:user"
)

// accessTokenScopeBitmap represents a bitmap of access token scopes.
type accessTokenScopeBitmap uint64

// Bitmap of each scope, including the child scopes.
const (
	// AccessTokenScopeAllBits is the bitmap of all access token scopes
	accessTokenScopeAllBits accessTokenScopeBitmap = accessTokenScopeDeleteActivityPubBits |
		accessTokenScopeDeleteAdminBits | accessTokenScopeDeleteMiscBits | accessTokenScopeDeleteNotificationBits |
		accessTokenScopeDeleteOrganizationBits | accessTokenScopeDeletePackageBits | accessTokenScopeDeleteIssueBits |
		accessTokenScopeDeleteRepositoryBits | accessTokenScopeDeleteUserBits

	accessTokenScopePublicOnlyBits accessTokenScopeBitmap = 1 << iota

	accessTokenScopeReadActivityPubBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteActivityPubBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadActivityPubBits
	accessTokenScopeDeleteActivityPubBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWriteActivityPubBits

	accessTokenScopeReadAdminBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteAdminBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadAdminBits
	accessTokenScopeDeleteAdminBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWriteAdminBits

	accessTokenScopeReadMiscBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteMiscBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadMiscBits
	accessTokenScopeDeleteMiscBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWriteMiscBits

	accessTokenScopeReadNotificationBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteNotificationBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadNotificationBits
	accessTokenScopeDeleteNotificationBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWriteNotificationBits

	accessTokenScopeReadOrganizationBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteOrganizationBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadOrganizationBits
	accessTokenScopeDeleteOrganizationBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWriteOrganizationBits

	accessTokenScopeReadPackageBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWritePackageBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadPackageBits
	accessTokenScopeDeletePackageBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWritePackageBits

	accessTokenScopeReadIssueBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteIssueBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadIssueBits
	accessTokenScopeDeleteIssueBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWriteIssueBits

	accessTokenScopeReadRepositoryBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteRepositoryBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadRepositoryBits
	accessTokenScopeDeleteRepositoryBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWriteRepositoryBits

	accessTokenScopeReadUserBits   accessTokenScopeBitmap = 1 << iota
	accessTokenScopeWriteUserBits  accessTokenScopeBitmap = 1<<iota | accessTokenScopeReadUserBits
	accessTokenScopeDeleteUserBits accessTokenScopeBitmap = 1<<iota | accessTokenScopeWriteUserBits

	// The current implementation only supports up to 64 token scopes.
	// If we need to support > 64 scopes,
	// refactoring the whole implementation in this file (and only this file) is needed.
)

// allAccessTokenScopes contains all access token scopes.
// The order is important: parent scope must precede child scopes.
var allAccessTokenScopes = []AccessTokenScope{
	AccessTokenScopePublicOnly,
	AccessTokenScopeDeleteActivityPub, AccessTokenScopeWriteActivityPub, AccessTokenScopeReadActivityPub,
	AccessTokenScopeDeleteAdmin, AccessTokenScopeWriteAdmin, AccessTokenScopeReadAdmin,
	AccessTokenScopeDeleteMisc, AccessTokenScopeWriteMisc, AccessTokenScopeReadMisc,
	AccessTokenScopeDeleteNotification, AccessTokenScopeWriteNotification, AccessTokenScopeReadNotification,
	AccessTokenScopeDeleteOrganization, AccessTokenScopeWriteOrganization, AccessTokenScopeReadOrganization,
	AccessTokenScopeDeletePackage, AccessTokenScopeWritePackage, AccessTokenScopeReadPackage,
	AccessTokenScopeDeleteIssue, AccessTokenScopeWriteIssue, AccessTokenScopeReadIssue,
	AccessTokenScopeDeleteRepository, AccessTokenScopeWriteRepository, AccessTokenScopeReadRepository,
	AccessTokenScopeDeleteUser, AccessTokenScopeWriteUser, AccessTokenScopeReadUser,
}

// allAccessTokenScopeBits contains all access token scopes.
var allAccessTokenScopeBits = map[AccessTokenScope]accessTokenScopeBitmap{
	AccessTokenScopeAll:                accessTokenScopeAllBits,
	AccessTokenScopePublicOnly:         accessTokenScopePublicOnlyBits,
	AccessTokenScopeReadActivityPub:    accessTokenScopeReadActivityPubBits,
	AccessTokenScopeWriteActivityPub:   accessTokenScopeWriteActivityPubBits,
	AccessTokenScopeDeleteActivityPub:  accessTokenScopeDeleteActivityPubBits,
	AccessTokenScopeReadAdmin:          accessTokenScopeReadAdminBits,
	AccessTokenScopeWriteAdmin:         accessTokenScopeWriteAdminBits,
	AccessTokenScopeDeleteAdmin:        accessTokenScopeDeleteAdminBits,
	AccessTokenScopeReadMisc:           accessTokenScopeReadMiscBits,
	AccessTokenScopeWriteMisc:          accessTokenScopeWriteMiscBits,
	AccessTokenScopeDeleteMisc:         accessTokenScopeDeleteMiscBits,
	AccessTokenScopeReadNotification:   accessTokenScopeReadNotificationBits,
	AccessTokenScopeWriteNotification:  accessTokenScopeWriteNotificationBits,
	AccessTokenScopeDeleteNotification: accessTokenScopeDeleteNotificationBits,
	AccessTokenScopeReadOrganization:   accessTokenScopeReadOrganizationBits,
	AccessTokenScopeWriteOrganization:  accessTokenScopeWriteOrganizationBits,
	AccessTokenScopeDeleteOrganization: accessTokenScopeDeleteOrganizationBits,
	AccessTokenScopeReadPackage:        accessTokenScopeReadPackageBits,
	AccessTokenScopeWritePackage:       accessTokenScopeWritePackageBits,
	AccessTokenScopeDeletePackage:      accessTokenScopeDeletePackageBits,
	AccessTokenScopeReadIssue:          accessTokenScopeReadIssueBits,
	AccessTokenScopeWriteIssue:         accessTokenScopeWriteIssueBits,
	AccessTokenScopeDeleteIssue:        accessTokenScopeDeleteIssueBits,
	AccessTokenScopeReadRepository:     accessTokenScopeReadRepositoryBits,
	AccessTokenScopeWriteRepository:    accessTokenScopeWriteRepositoryBits,
	AccessTokenScopeDeleteRepository:   accessTokenScopeDeleteRepositoryBits,
	AccessTokenScopeReadUser:           accessTokenScopeReadUserBits,
	AccessTokenScopeWriteUser:          accessTokenScopeWriteUserBits,
	AccessTokenScopeDeleteUser:         accessTokenScopeDeleteUserBits,
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
func (bitmap accessTokenScopeBitmap) toScope(unknownScopes *[]unknownAccessTokenScope) AccessTokenScope {
	var scopes []string

	// Preserve unknown scopes, and put them at the beginning so that it's clear
	// when debugging.
	if unknownScopes != nil {
		for _, unknownScope := range *unknownScopes {
			scopes = append(scopes, string(unknownScope))
		}
	}

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
		"delete:activitypub,delete:admin,delete:misc,delete:notification,delete:organization,delete:package,delete:issue,delete:repository,delete:user",
		"all",
	))
	return scope
}

// parse the scope string into a bitmap, thus removing possible duplicates.
func (s AccessTokenScope) parse() (accessTokenScopeBitmap, *[]unknownAccessTokenScope) {
	var bitmap accessTokenScopeBitmap
	var unknownScopes []unknownAccessTokenScope

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
			unknownScopes = append(unknownScopes, unknownAccessTokenScope(string(singleScope)))
		}
		bitmap |= bits
	}

	return bitmap, &unknownScopes
}

// NormalizePreservingUnknown returns a normalized scope string without any
// duplicates.  Unknown scopes are included.
func (s AccessTokenScope) NormalizePreservingUnknown() AccessTokenScope {
	bitmap, unknownScopes := s.parse()

	return bitmap.toScope(unknownScopes)
}

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

var accessTokenScopeMap = map[OldAccessTokenScope][]AccessTokenScope{
	OldAccessTokenScopeAll:              {AccessTokenScopeAll},
	OldAccessTokenScopeRepo:             {AccessTokenScopeDeleteRepository},
	OldAccessTokenScopeRepoStatus:       {AccessTokenScopeWriteRepository},
	OldAccessTokenScopePublicRepo:       {AccessTokenScopePublicOnly, AccessTokenScopeDeleteRepository},
	OldAccessTokenScopeAdminOrg:         {AccessTokenScopeDeleteOrganization},
	OldAccessTokenScopeWriteOrg:         {AccessTokenScopeWriteOrganization},
	OldAccessTokenScopeReadOrg:          {AccessTokenScopeReadOrganization},
	OldAccessTokenScopeAdminPublicKey:   {AccessTokenScopeDeleteUser},
	OldAccessTokenScopeWritePublicKey:   {AccessTokenScopeWriteUser},
	OldAccessTokenScopeReadPublicKey:    {AccessTokenScopeReadUser},
	OldAccessTokenScopeAdminRepoHook:    {AccessTokenScopeDeleteRepository},
	OldAccessTokenScopeWriteRepoHook:    {AccessTokenScopeWriteRepository},
	OldAccessTokenScopeReadRepoHook:     {AccessTokenScopeReadRepository},
	OldAccessTokenScopeAdminOrgHook:     {AccessTokenScopeWriteOrganization},
	OldAccessTokenScopeNotification:     {AccessTokenScopeDeleteNotification},
	OldAccessTokenScopeUser:             {AccessTokenScopeDeleteUser},
	OldAccessTokenScopeReadUser:         {AccessTokenScopeReadUser},
	OldAccessTokenScopeUserEmail:        {AccessTokenScopeWriteUser},
	OldAccessTokenScopeUserFollow:       {AccessTokenScopeWriteUser},
	OldAccessTokenScopeDeleteRepo:       {AccessTokenScopeDeleteRepository},
	OldAccessTokenScopePackage:          {AccessTokenScopeDeletePackage},
	OldAccessTokenScopeWritePackage:     {AccessTokenScopeWritePackage},
	OldAccessTokenScopeReadPackage:      {AccessTokenScopeReadPackage},
	OldAccessTokenScopeDeletePackage:    {AccessTokenScopeDeletePackage},
	OldAccessTokenScopeAdminGPGKey:      {AccessTokenScopeDeleteUser},
	OldAccessTokenScopeWriteGPGKey:      {AccessTokenScopeWriteUser},
	OldAccessTokenScopeReadGPGKey:       {AccessTokenScopeReadUser},
	OldAccessTokenScopeAdminApplication: {AccessTokenScopeDeleteUser},
	OldAccessTokenScopeWriteApplication: {AccessTokenScopeWriteUser},
	OldAccessTokenScopeReadApplication:  {AccessTokenScopeReadUser},
	OldAccessTokenScopeSudo:             {AccessTokenScopeDeleteAdmin},
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
		var scopes []string
		allNewScopesMap := make(map[AccessTokenScope]bool)
		for _, oldScope := range strings.Split(token.Scope, ",") {
			if newScopes, exists := accessTokenScopeMap[OldAccessTokenScope(oldScope)]; exists {
				for _, newScope := range newScopes {
					allNewScopesMap[newScope] = true
				}
			} else {
				log.Debug("access token scope not recognized as old token scope %s; preserving it", oldScope)
				scopes = append(scopes, oldScope)
			}
		}

		for s := range allNewScopesMap {
			scopes = append(scopes, string(s))
		}
		scope := AccessTokenScope(strings.Join(scopes, ","))

		// normalize the scope
		normScope := scope.NormalizePreservingUnknown()

		token.Scope = string(normScope)

		// update the db entry with the new scope
		if _, err := x.Cols("scope").ID(token.ID).Update(token); err != nil {
			return err
		}
	}

	return nil
}
