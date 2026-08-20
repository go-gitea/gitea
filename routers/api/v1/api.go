// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package v1 Gitea API
//
// This documentation describes the Gitea API.
//
//	Schemes: https, http
//	License: MIT http://opensource.org/licenses/MIT
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
//	Security:
//	- BasicAuth :
//	- Token :
//	- AccessToken :
//	- AuthorizationHeaderToken :
//	- SudoParam :
//	- SudoHeader :
//	- TOTPHeader :
//
//	SecurityDefinitions:
//	BasicAuth:
//	     type: basic
//	Token:
//	     type: apiKey
//	     name: token
//	     in: query
//	     description: This authentication option is deprecated for removal in Gitea 1.23. Please use AuthorizationHeaderToken instead.
//	AccessToken:
//	     type: apiKey
//	     name: access_token
//	     in: query
//	     description: This authentication option is deprecated for removal in Gitea 1.23. Please use AuthorizationHeaderToken instead.
//	AuthorizationHeaderToken:
//	     type: apiKey
//	     name: Authorization
//	     in: header
//	     description: API tokens must be prepended with "token" followed by a space.
//	SudoParam:
//	     type: apiKey
//	     name: sudo
//	     in: query
//	     description: Sudo API request as the user provided as the key. Admin privileges are required.
//	SudoHeader:
//	     type: apiKey
//	     name: Sudo
//	     in: header
//	     description: Sudo API request as the user provided as the key. Admin privileges are required.
//	TOTPHeader:
//	     type: apiKey
//	     name: X-GITEA-OTP
//	     in: header
//	     description: Must be used in combination with BasicAuth if two-factor authentication is enabled.
//
// swagger:meta
package v1

import (
	gocontext "context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"
	"gitea.dev/modules/web"
	"gitea.dev/modules/web/middleware"
	web_types "gitea.dev/modules/web/types"
	"gitea.dev/routers/api/v1/activitypub"
	"gitea.dev/routers/api/v1/admin"
	"gitea.dev/routers/api/v1/misc"
	"gitea.dev/routers/api/v1/notify"
	"gitea.dev/routers/api/v1/org"
	"gitea.dev/routers/api/v1/packages"
	"gitea.dev/routers/api/v1/repo"
	"gitea.dev/routers/api/v1/settings"
	"gitea.dev/routers/api/v1/shared"
	"gitea.dev/routers/api/v1/token"
	"gitea.dev/routers/api/v1/user"
	"gitea.dev/routers/common"
	"gitea.dev/services/actions"
	"gitea.dev/services/auth"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"

	_ "gitea.dev/routers/api/v1/swagger" // for swagger generation

	chi_middleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

const (
	codespaceTokenRoutePolicyDataKey        = "CodespaceTokenRoutePolicy"
	codespaceTokenRepositoryRouteDataKey    = "CodespaceTokenRepositoryRoute"
	codespaceTokenRoutePolicySelf           = "self"
	codespaceTokenRoutePolicyPublicInfo     = "public_info"
	codespaceTokenRoutePolicySignedArtifact = "signed_artifact"
)

func sudo() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		sudo := ctx.FormString("sudo")
		if len(sudo) == 0 {
			sudo = ctx.Req.Header.Get("Sudo")
		}

		if len(sudo) > 0 {
			if _, ok := ctx.CodespaceTokenRepoID(); ok {
				ctx.APIError(http.StatusForbidden, "codespace token cannot use sudo")
				return
			}
			if ctx.IsSigned && ctx.Doer.IsAdmin {
				user, err := user_model.GetUserByName(ctx, sudo)
				if err != nil {
					if user_model.IsErrUserNotExist(err) {
						ctx.APIErrorNotFound()
					} else {
						ctx.APIErrorInternal(err)
					}
					return
				}
				log.Trace("Sudo from (%s) to: %s", ctx.Doer.Name, user.Name)
				ctx.Doer = user
			} else {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "Only administrators allowed to sudo.",
				})
				return
			}
		}
	}
}

func repoAssignment() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		userName := ctx.PathParam("username")
		repoName := ctx.PathParam("reponame")

		var (
			owner *user_model.User
			err   error
		)

		// Check if the user is the same as the repository owner.
		if ctx.IsSigned && strings.EqualFold(ctx.Doer.LowerName, userName) {
			owner = ctx.Doer
		} else {
			owner, err = user_model.GetUserByName(ctx, userName)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					if redirectUserID, err := user_model.LookupUserRedirect(ctx, userName); err == nil {
						context.RedirectToUser(ctx.Base, ctx.Doer, userName, redirectUserID)
					} else if user_model.IsErrUserRedirectNotExist(err) {
						ctx.APIErrorNotFound()
					} else {
						ctx.APIErrorInternal(err)
					}
				} else {
					ctx.APIErrorInternal(err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner
		ctx.ContextUser = owner

		// Get repository.
		repo, err := repo_model.GetRepositoryByName(ctx, owner.ID, repoName)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				redirectRepoID, err := repo_model.LookupRedirect(ctx, owner.ID, repoName)
				if err == nil {
					context.RedirectToRepo(ctx.Base, redirectRepoID)
				} else if repo_model.IsErrRedirectNotExist(err) {
					ctx.APIErrorNotFound()
				} else {
					ctx.APIErrorInternal(err)
				}
			} else {
				ctx.APIErrorInternal(err)
			}
			return
		}

		repo.Owner = owner
		ctx.Repo.Repository = repo
		ctx.UseAnonymousForPublicCodespaceRead(repo)

		if taskID, ok := user_model.GetActionsUserTaskID(ctx.Doer); ok {
			ctx.Repo.Permission, err = access_model.GetActionsUserRepoPermission(ctx, repo, ctx.Doer, taskID)
			if err != nil {
				ctx.APIErrorInternal(err)
				return
			}
		} else {
			needTwoFactor, err := doerNeedTwoFactorAuth(ctx, ctx.Doer)
			if err != nil {
				ctx.APIErrorInternal(err)
				return
			}
			if needTwoFactor {
				ctx.Repo.Permission = access_model.PermissionNoAccess()
			} else {
				ctx.Repo.Permission, err = access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
				if err != nil {
					ctx.APIErrorInternal(err)
					return
				}
			}
		}

		if !ctx.Repo.Permission.HasAnyUnitAccessOrPublicAccess() {
			ctx.APIErrorNotFound()
			return
		}

		if !ctx.TokenCanAccessRepo(repo) {
			if _, ok := ctx.CodespaceTokenRepoID(); ok {
				ctx.APIError(http.StatusForbidden, "codespace token does not grant access to this repository")
				return
			}
			ctx.APIErrorNotFound()
			return
		}
	}
}

func doerNeedTwoFactorAuth(ctx gocontext.Context, doer *user_model.User) (bool, error) {
	if !setting.TwoFactorAuthEnforced {
		return false, nil
	}
	if doer == nil {
		return false, nil
	}
	has, err := auth_model.HasTwoFactorOrWebAuthn(ctx, doer.ID)
	if err != nil {
		return false, err
	}
	return !has, nil
}

func reqPackageAccess(accessMode perm.AccessMode) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
			ctx.APIError(http.StatusForbidden, "user should have specific permission or be a site admin")
			return
		}
	}
}

func checkTokenPublicOnly() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.PublicOnly {
			return
		}

		requiredScopeCategories, ok := ctx.Data["requiredScopeCategories"].([]auth_model.AccessTokenScopeCategory)
		if !ok || len(requiredScopeCategories) == 0 {
			return
		}

		for _, category := range requiredScopeCategories {
			switch category {
			case auth_model.AccessTokenScopeCategoryRepository:
				if !ctx.TokenCanAccessRepo(ctx.Repo.Repository) {
					ctx.APIError(http.StatusForbidden, "token scope is limited to public repos")
					return
				}
			case auth_model.AccessTokenScopeCategoryIssue:
				if !ctx.TokenCanAccessRepo(ctx.Repo.Repository) {
					ctx.APIError(http.StatusForbidden, "token scope is limited to public issues")
					return
				}
			case auth_model.AccessTokenScopeCategoryOrganization:
				orgPrivate := ctx.Org.Organization != nil && !ctx.Org.Organization.Visibility.IsPublic()
				userOrgPrivate := ctx.ContextUser != nil && ctx.ContextUser.IsOrganization() && !ctx.ContextUser.Visibility.IsPublic()
				if orgPrivate || userOrgPrivate {
					ctx.APIError(http.StatusForbidden, "token scope is limited to public orgs")
					return
				}
			case auth_model.AccessTokenScopeCategoryUser:
				if ctx.ContextUser != nil && ctx.ContextUser.IsTokenAccessAllowed() && !ctx.ContextUser.Visibility.IsPublic() {
					ctx.APIError(http.StatusForbidden, "token scope is limited to public users")
					return
				}
			case auth_model.AccessTokenScopeCategoryActivityPub:
				if ctx.ContextUser != nil && ctx.ContextUser.IsTokenAccessAllowed() && !ctx.ContextUser.Visibility.IsPublic() {
					ctx.APIError(http.StatusForbidden, "token scope is limited to public activitypub")
					return
				}
			case auth_model.AccessTokenScopeCategoryNotification:
				if !ctx.TokenCanAccessRepo(ctx.Repo.Repository) {
					ctx.APIError(http.StatusForbidden, "token scope is limited to public notifications")
					return
				}
			case auth_model.AccessTokenScopeCategoryPackage:
				// a public-only token must not reach limited-visibility owners either,
				// matching the org/user public-only enforcement above
				if ctx.Package != nil && !ctx.Package.Owner.Visibility.IsPublic() {
					ctx.APIError(http.StatusForbidden, "token scope is limited to public packages")
					return
				}
			}
		}
	}
}

func rejectPublicOnly() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.PublicOnly {
			return
		}

		ctx.APIError(http.StatusForbidden, "this endpoint is not available for public-only tokens")
	}
}

func contextAuthenticatedUser() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		ctx.ContextUser = ctx.Doer
	}
}

func codespaceTokenRoute(policy string) web_types.PreMiddlewareProvider {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if store := reqctx.GetRequestDataStore(req.Context()); store != nil {
				store.GetData()[codespaceTokenRoutePolicyDataKey] = policy
			}
			next.ServeHTTP(w, req)
		})
	}
}

var codespaceTokenRepositoryRoute web_types.PreMiddlewareProvider = func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if store := reqctx.GetRequestDataStore(req.Context()); store != nil {
			store.GetData()[codespaceTokenRepositoryRouteDataKey] = true
		}
		next.ServeHTTP(w, req)
	})
}

func codespaceTokenRouteGuard(ctx *context.APIContext) {
	if _, ok := ctx.CodespaceTokenRepoID(); !ok {
		return
	}
	if ctx.GetData()[codespaceTokenRepositoryRouteDataKey] == true {
		return
	}
	policy, _ := ctx.GetData()[codespaceTokenRoutePolicyDataKey].(string)
	switch policy {
	case codespaceTokenRoutePolicySelf,
		codespaceTokenRoutePolicyPublicInfo,
		codespaceTokenRoutePolicySignedArtifact:
		return
	default:
		ctx.APIError(http.StatusForbidden, "codespace token is not allowed for this API route")
	}
}

func codespaceTokenRoutePolicy(ctx *context.APIContext) string {
	policy, _ := ctx.GetData()[codespaceTokenRoutePolicyDataKey].(string)
	return policy
}

// if a token is being used for auth, we check that it contains the required scope
// if a token is not being used, reqToken will enforce other sign in methods
func tokenRequiresScopes(requiredScopeCategories ...auth_model.AccessTokenScopeCategory) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		// no scope required
		if len(requiredScopeCategories) == 0 {
			return
		}

		// Need OAuth2 token to be present.
		scope, scopeExists := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
		if !scopeExists {
			return
		}

		// use the http method to determine the access level
		requiredScopeLevel := auth_model.Read
		if ctx.Req.Method == http.MethodPost || ctx.Req.Method == http.MethodPut || ctx.Req.Method == http.MethodPatch || ctx.Req.Method == http.MethodDelete {
			requiredScopeLevel = auth_model.Write
		}

		// get the required scope for the given access level and category
		requiredScopes := auth_model.GetRequiredScopes(requiredScopeLevel, requiredScopeCategories...)
		allow, err := scope.HasScope(requiredScopes...)
		if err != nil {
			ctx.APIError(http.StatusForbidden, "checking scope failed: "+err.Error())
			return
		}

		if !allow {
			ctx.APIError(http.StatusForbidden, fmt.Sprintf("token does not have at least one of required scope(s), required=%v, token scope=%v", requiredScopes, scope))
			return
		}

		ctx.Data["requiredScopeCategories"] = requiredScopeCategories

		// check if scope only applies to public resources
		publicOnly, err := scope.PublicOnly()
		if err != nil {
			ctx.APIError(http.StatusForbidden, "parsing public resource scope failed: "+err.Error())
			return
		}

		// assign to true so that those searching should only filter public repositories/users/organizations
		ctx.PublicOnly = publicOnly
	}
}

// Contexter middleware already checks token for user sign in process.
func reqToken() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		// if a real user is signed in, or the user is from a Actions task, we are good
		if ctx.IsSigned {
			return
		}
		ctx.APIError(http.StatusUnauthorized, "token is required")
	}
}

func reqExploreSignIn() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if (setting.Service.RequireSignInViewStrict || setting.Service.Explore.RequireSigninView) && !ctx.IsSigned {
			ctx.APIError(http.StatusUnauthorized, "you must be signed in to search for users")
		}
	}
}

func reqUsersExploreEnabled() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if setting.Service.Explore.DisableUsersPage {
			ctx.APIErrorNotFound()
		}
	}
}

func reqBasicOrRevProxyAuth() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.IsSigned && setting.Service.EnableReverseProxyAuthAPI && ctx.Data["AuthedMethod"] == auth.ReverseProxyMethodName {
			return
		}
		if !ctx.IsBasicAuth {
			ctx.APIError(http.StatusUnauthorized, "auth required")
			return
		}
	}
}

// reqSiteAdmin user should be the site admin
func reqSiteAdmin() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserSiteAdmin() {
			ctx.APIError(http.StatusForbidden, "user should be the site admin")
			return
		}
	}
}

// reqOwner user should be the owner of the repo or site admin.
func reqOwner() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Repo.Permission.IsOwner() && !ctx.IsUserSiteAdmin() {
			ctx.APIError(http.StatusForbidden, "user should be the owner of the repo")
			return
		}
	}
}

// reqSelfOrAdmin doer should be the same as the contextUser or site admin
func reqSelfOrAdmin() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserSiteAdmin() && ctx.ContextUser != ctx.Doer {
			ctx.APIError(http.StatusForbidden, "doer should be the site admin or be same as the contextUser")
			return
		}
	}
}

// reqAdmin user should be an owner or a collaborator with admin write of a repository, or site admin
func reqAdmin() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.APIError(http.StatusForbidden, "user should be an owner or a collaborator with admin write of a repository")
			return
		}
	}
}

// reqRepoWriter user should have a permission to write to a repo, or be a site admin
func reqRepoWriter(unitTypes ...unit.Type) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !codespaceTokenAllowsRepositoryUnit(ctx, perm.AccessModeWrite, unitTypes...) {
			ctx.APIError(http.StatusForbidden, "codespace token does not have the required repository permission")
			return
		}
		if !ctx.IsUserRepoWriter(unitTypes) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.APIError(http.StatusForbidden, "user should have a permission to write to a repo")
			return
		}
	}
}

// reqRepoReader user should have specific read permission or be a repo admin or a site admin
func reqRepoReader(unitType unit.Type) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !codespaceTokenAllowsRepositoryUnit(ctx, perm.AccessModeRead, unitType) {
			ctx.APIError(http.StatusForbidden, "codespace token does not have the required repository permission")
			return
		}
		if !ctx.Repo.Permission.CanRead(unitType) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.APIError(http.StatusForbidden, "user should have specific read permission or be a repo admin or a site admin")
			return
		}
	}
}

func codespaceTokenAllowsRepositoryUnit(ctx *context.APIContext, mode perm.AccessMode, unitTypes ...unit.Type) bool {
	if _, ok := ctx.CodespaceTokenRepoID(); !ok {
		return true
	}
	return slices.ContainsFunc(unitTypes, func(unitType unit.Type) bool {
		return ctx.CodespaceTokenAllowsRepository(unitType, mode)
	})
}

func requireCodespaceTokenRepositoryPermission(ctx *context.APIContext, mode perm.AccessMode, unitTypes ...unit.Type) {
	if _, ok := ctx.CodespaceTokenRepoID(); !ok {
		return
	}
	if !slices.ContainsFunc(unitTypes, func(unitType unit.Type) bool {
		if !ctx.CodespaceTokenAllowsRepository(unitType, mode) {
			return false
		}
		if mode == perm.AccessModeWrite {
			return ctx.Repo.Permission.CanWrite(unitType)
		}
		return ctx.Repo.Permission.CanRead(unitType)
	}) {
		ctx.APIError(http.StatusForbidden, "codespace token does not have the required repository permission")
	}
}

func reqCodespaceTokenRepositoryPermission(unitTypes ...unit.Type) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		mode := perm.AccessModeRead
		switch ctx.Req.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			mode = perm.AccessModeWrite
		}
		requireCodespaceTokenRepositoryPermission(ctx, mode, unitTypes...)
	}
}

func reqCodespaceTokenRepositoryRead(unitTypes ...unit.Type) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		requireCodespaceTokenRepositoryPermission(ctx, perm.AccessModeRead, unitTypes...)
	}
}

// reqAnyRepoReader user should have any permission to read repository or permissions of site admin
func reqAnyRepoReader() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Repo.Permission.HasAnyUnitAccess() && !ctx.IsUserSiteAdmin() {
			ctx.APIError(http.StatusForbidden, "user should have any permission to read repository or permissions of site admin")
			return
		}
	}
}

// reqOrgOwnership user should be an organization owner, or a site admin
func reqOrgOwnership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.IsUserSiteAdmin() {
			return
		}

		var orgID int64
		if ctx.Org.Organization != nil {
			orgID = ctx.Org.Organization.ID
		} else if ctx.Org.Team != nil {
			orgID = ctx.Org.Team.OrgID
		} else {
			setting.PanicInDevOrTesting("reqOrgOwnership: unprepared context")
			ctx.APIErrorInternal(errors.New("reqOrgOwnership: unprepared context"))
			return
		}

		isOwner, err := organization.IsOrganizationOwner(ctx, orgID, ctx.Doer.ID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		} else if !isOwner {
			if ctx.Org.Organization != nil {
				ctx.APIError(http.StatusForbidden, "Must be an organization owner")
			} else {
				ctx.APIErrorNotFound()
			}
			return
		}
	}
}

// reqOrgVisible requires the organization to be visible to the doer, or a site admin
func reqOrgVisible() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.Org.Organization == nil {
			setting.PanicInDevOrTesting("reqOrgVisible: unprepared context")
			ctx.APIErrorInternal(errors.New("reqOrgVisible: unprepared context"))
			return
		}
		if !organization.HasOrgOrUserVisible(ctx, ctx.Org.Organization.AsUser(), ctx.Doer) {
			ctx.APIErrorNotFound()
			return
		}
	}
}

func teamAccessPrivileged(ctx *context.APIContext) (orgID int64, privileged, ok bool) {
	if ctx.IsUserSiteAdmin() {
		return 0, true, true
	}
	if ctx.Org.Team == nil {
		setting.PanicInDevOrTesting("teamAccess: unprepared context")
		ctx.APIErrorInternal(errors.New("teamAccess: unprepared context"))
		return 0, false, false
	}

	orgID = ctx.Org.Team.OrgID
	isOwner, err := organization.IsOrganizationOwner(ctx, orgID, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return 0, false, false
	} else if isOwner {
		return orgID, true, true
	}

	isTeamMember, err := organization.IsTeamMember(ctx, orgID, ctx.Org.Team.ID, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return 0, false, false
	}
	return orgID, isTeamMember, true
}

func denyNonTeamMember(ctx *context.APIContext, orgID int64) {
	isOrgMember, err := organization.IsOrganizationMember(ctx, orgID, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
	} else if isOrgMember {
		ctx.APIError(http.StatusForbidden, "Must be a team member")
	} else {
		ctx.APIErrorNotFound()
	}
}

// reqTeamReadAccess allows callers who can list the team to read its metadata.
// Non-members are admitted by the team's visibility tier and parent org visibility.
// Not sufficient for mutations — use reqOrgOwnership() or reqTeamMembership() for those.
func reqTeamReadAccess() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		orgID, privileged, ok := teamAccessPrivileged(ctx)
		if !ok || privileged {
			return
		}
		if ctx.Org.Organization == nil {
			setting.PanicInDevOrTesting("reqTeamReadAccess: organization not loaded")
			ctx.APIErrorInternal(errors.New("reqTeamReadAccess: organization not loaded"))
			return
		}

		visible, err := ctx.Org.Team.CanNonMemberReadMeta(ctx, ctx.Org.Organization.AsUser(), ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if !visible {
			// Not admitted by visibility: 403 for org members, 404 otherwise.
			denyNonTeamMember(ctx, orgID)
		}
	}
}

// reqTeamMembership user should be a team member, or a site admin
func reqTeamMembership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		orgID, privileged, ok := teamAccessPrivileged(ctx)
		if !ok || privileged {
			return
		}
		denyNonTeamMember(ctx, orgID)
	}
}

// reqOrgMembership user should be an organization member, or a site admin
func reqOrgMembership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.IsUserSiteAdmin() {
			return
		}

		var orgID int64
		if ctx.Org.Organization != nil {
			orgID = ctx.Org.Organization.ID
		} else if ctx.Org.Team != nil {
			orgID = ctx.Org.Team.OrgID
		} else {
			setting.PanicInDevOrTesting("reqOrgMembership: unprepared context")
			ctx.APIErrorInternal(errors.New("reqOrgMembership: unprepared context"))
			return
		}

		if isMember, err := organization.IsOrganizationMember(ctx, orgID, ctx.Doer.ID); err != nil {
			ctx.APIErrorInternal(err)
			return
		} else if !isMember {
			if ctx.Org.Organization != nil {
				ctx.APIError(http.StatusForbidden, "Must be an organization member")
			} else {
				ctx.APIErrorNotFound()
			}
			return
		}
	}
}

func reqGitHook() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Doer.CanEditGitHook() {
			ctx.APIError(http.StatusForbidden, "must be allowed to edit Git hooks")
			return
		}
	}
}

// reqWebhooksEnabled requires webhooks to be enabled by admin.
func reqWebhooksEnabled() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if setting.DisableWebhooks {
			ctx.APIError(http.StatusForbidden, "webhooks disabled by administrator")
			return
		}
	}
}

// reqStarsEnabled requires Starring to be enabled in the config.
func reqStarsEnabled() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if setting.Repository.DisableStars {
			ctx.APIError(http.StatusForbidden, "stars disabled by administrator")
			return
		}
	}
}

func orgAssignment(args ...bool) func(ctx *context.APIContext) {
	var (
		assignOrg  bool
		assignTeam bool
	)
	if len(args) > 0 {
		assignOrg = args[0]
	}
	if len(args) > 1 {
		assignTeam = args[1]
	}
	return func(ctx *context.APIContext) {
		ctx.Org = new(context.APIOrganization)

		var err error
		if assignOrg {
			ctx.Org.Organization, err = organization.GetOrgByName(ctx, ctx.PathParam("org"))
			if err != nil {
				if organization.IsErrOrgNotExist(err) {
					redirectUserID, err := user_model.LookupUserRedirect(ctx, ctx.PathParam("org"))
					if err == nil {
						context.RedirectToUser(ctx.Base, ctx.Doer, ctx.PathParam("org"), redirectUserID)
					} else if user_model.IsErrUserRedirectNotExist(err) {
						ctx.APIErrorNotFound()
					} else {
						ctx.APIErrorInternal(err)
					}
				} else {
					ctx.APIErrorInternal(err)
				}
				return
			}
			ctx.ContextUser = ctx.Org.Organization.AsUser()
		}

		if assignTeam {
			ctx.Org.Team, err = organization.GetTeamByID(ctx, ctx.PathParamInt64("teamid"))
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.APIErrorNotFound()
				} else {
					ctx.APIErrorInternal(err)
				}
				return
			}
			if ctx.Org.Organization == nil {
				ctx.Org.Organization, err = organization.GetOrgByID(ctx, ctx.Org.Team.OrgID)
				if err != nil {
					if organization.IsErrOrgNotExist(err) {
						ctx.APIErrorNotFound()
					} else {
						ctx.APIErrorInternal(err)
					}
					return
				}
			}
		}
	}
}

func mustEnableIssues(ctx *context.APIContext) {
	if !ctx.Repo.Permission.CanRead(unit.TypeIssues) {
		if log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					unit.TypeIssues,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					unit.TypeIssues,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.APIErrorNotFound()
		return
	}
}

func mustAllowPulls(ctx *context.APIContext) {
	if !(ctx.Repo.Repository.CanEnablePulls() && ctx.Repo.Permission.CanRead(unit.TypePullRequests)) {
		if ctx.Repo.Repository.CanEnablePulls() && log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					unit.TypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					unit.TypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.APIErrorNotFound()
		return
	}
}

func mustEnableIssuesOrPulls(ctx *context.APIContext) {
	if !ctx.Repo.Permission.CanRead(unit.TypeIssues) &&
		!(ctx.Repo.Repository.CanEnablePulls() && ctx.Repo.Permission.CanRead(unit.TypePullRequests)) {
		if ctx.Repo.Repository.CanEnablePulls() && log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v and %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					unit.TypeIssues,
					unit.TypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v and %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					unit.TypeIssues,
					unit.TypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.APIErrorNotFound()
		return
	}
}

func mustEnableWiki(ctx *context.APIContext) {
	if !(ctx.Repo.Permission.CanRead(unit.TypeWiki)) {
		ctx.APIErrorNotFound()
		return
	}
}

// reqProjectsUnitAccess mirrors the web's reqUnitAccess for the Projects unit. Org
// visibility is too permissive for reads, org ownership too strict for writes.
func reqProjectsUnitAccess(accessMode perm.AccessMode) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		// "/users/{username}/projects" also accepts an organization, where checkTokenPublicOnly
		// does nothing because IsTokenAccessAllowed is false for orgs. Enforce it here, before
		// the admin bypass, so both spellings of the route answer alike.
		if ctx.PublicOnly && ctx.ContextUser.IsOrganization() && !ctx.ContextUser.Visibility.IsPublic() {
			ctx.APIError(http.StatusForbidden, "token scope is limited to public orgs")
			return
		}
		if ctx.IsUserSiteAdmin() {
			return
		}
		// individual visibility is handled by individualPermsChecker
		if ctx.ContextUser.IsOrganization() &&
			organization.OrgFromUser(ctx.ContextUser).AnyRepoUnitPermission(ctx, ctx.Doer, unit.TypeProjects) < accessMode {
			ctx.APIErrorNotFound()
		}
	}
}

// addProjectRoutes registers a scope's project tree, "writeChecks" guard every mutation.
func addProjectRoutes(m *web.Router, writeChecks ...any) {
	m.Get("", shared.ListProjects)
	m.Group("/{id}", func() {
		m.Get("", shared.GetProject)
		m.Get("/columns", shared.ListProjectColumns)
		m.Group("/columns/{column_id}", func() {
			m.Get("", shared.GetProjectColumn)
			m.Get("/issues", shared.ListProjectColumnIssues)
		})
	})
	m.Group("", func() {
		m.Post("", bind(api.CreateProjectOption{}), shared.CreateProject)
		m.Group("/{id}", func() {
			m.Patch("", bind(api.EditProjectOption{}), shared.EditProject)
			m.Delete("", shared.DeleteProject)
			m.Post("/columns", bind(api.CreateProjectColumnOption{}), shared.CreateProjectColumn)
			m.Post("/columns/move", bind(api.MoveProjectColumnsOption{}), shared.MoveProjectColumns)
			m.Group("/columns/{column_id}", func() {
				m.Patch("", bind(api.EditProjectColumnOption{}), shared.EditProjectColumn)
				m.Delete("", shared.DeleteProjectColumn)
				m.Post("/default", shared.SetDefaultProjectColumn)
				m.Post("/issues/{issue_id}", shared.AddIssueToProjectColumn)
				m.Delete("/issues/{issue_id}", shared.RemoveIssueFromProjectColumn)
			})
			m.Post("/issues/{issue_id}/move", bind(api.MoveProjectIssueOption{}), shared.MoveProjectIssue)
		})
	}, writeChecks...)
}

// mustEnableRepoProjects mirrors repo.MustEnableRepoProjects: the Projects unit can be
// readable while repo-level boards are disallowed, and the web UI then hides them entirely.
func mustEnableRepoProjects(ctx *context.APIContext) {
	projectsUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeProjects)
	if !projectsUnit.ProjectsConfig().IsProjectsAllowed(repo_model.ProjectsModeRepo) {
		ctx.APIErrorNotFound()
	}
}

// FIXME: for consistency, maybe most mustNotBeArchived checks should be replaced with mustEnableEditor
func mustNotBeArchived(ctx *context.APIContext) {
	if ctx.Repo.Repository.IsArchived {
		ctx.APIError(http.StatusLocked, "repo is archived")
		return
	}
}

func mustEnableEditor(ctx *context.APIContext) {
	if !ctx.Repo.Repository.CanEnableEditor() {
		ctx.APIError(http.StatusLocked, "repo is not allowed to edit")
		return
	}
}

func mustEnableAttachments(ctx *context.APIContext) {
	if !setting.Attachment.Enabled {
		ctx.APIErrorNotFound()
		return
	}
}

// bind binding an obj to a func(ctx *context.APIContext)
func bind[T any](tmpl T) any {
	return func(ctx *context.APIContext) {
		form, errs := middleware.BindFormAny(ctx.Req, validation.Binder(), tmpl)
		if len(errs) > 0 {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("%s: %s", errs[0].FieldNames, errs[0].Error()))
			return
		}
		web.SetForm(ctx, form)
	}
}

func buildAuthGroup() *auth.Group {
	group := auth.NewGroup(
		&auth.OAuth2{},
		&auth.HTTPSign{},
		&auth.Basic{}, // FIXME: this should be removed once we don't allow basic auth in API
	)
	if setting.Service.EnableReverseProxyAuthAPI {
		group.Add(&auth.ReverseProxy{}) // TODO: does it still make sense to support reverse proxy auth in API?
	}
	// others: API doesn't support SSPI auth because the caller should use token
	return group
}

func apiAuth(authMethod auth.Method) func(*context.APIContext) {
	return func(ctx *context.APIContext) {
		if codespaceTokenRoutePolicy(ctx) == codespaceTokenRoutePolicySignedArtifact {
			return
		}
		ar, err := common.AuthShared(ctx.Base, nil, authMethod)
		if err != nil {
			if auth.IsCodespaceTokenForbidden(err) {
				ctx.APIError(http.StatusForbidden, "codespace token is not allowed for this request")
				return
			}
			msg, ok := auth.ErrAsUserAuthMessage(err)
			msg = util.Iif(ok, msg, "invalid username, password or token")
			ctx.APIError(http.StatusUnauthorized, msg)
			return
		}
		ctx.Doer = ar.Doer
		ctx.IsSigned = ar.Doer != nil
		ctx.IsBasicAuth = ar.IsBasicAuth
	}
}

// verifyAuthWithOptions checks authentication according to options
func verifyAuthWithOptions(options *common.VerifyOptions) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if codespaceTokenRoutePolicy(ctx) == codespaceTokenRoutePolicySignedArtifact {
			return
		}
		// Check prohibit login users.
		if ctx.IsSigned {
			if !ctx.Doer.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "This account is not activated.",
				})
				return
			}
			if !ctx.Doer.IsActive || ctx.Doer.ProhibitLogin {
				log.Info("Failed authentication attempt for %s from %s", ctx.Doer.Name, ctx.RemoteAddr())
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "This account is prohibited from signing in, please contact your site administrator.",
				})
				return
			}

			if ctx.Doer.MustChangePassword {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "You must change your password. Change it at: " + setting.AppURL + "/user/change_password",
				})
				return
			}
		}

		// Redirect to dashboard if user tries to visit any non-login page.
		if options.SignOutRequired && ctx.IsSigned && ctx.Req.URL.RequestURI() != "/" {
			ctx.Redirect(setting.AppSubURL + "/")
			return
		}

		if options.SignInRequired {
			if !ctx.IsSigned {
				// Restrict API calls with error message.
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "Only signed in user is allowed to call APIs.",
				})
				return
			} else if !ctx.Doer.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "This account is not activated.",
				})
				return
			}
		}

		if options.AdminRequired {
			if !ctx.Doer.IsAdmin {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "You have no permission to request for this.",
				})
				return
			}
		}
	}
}

func individualPermsChecker(ctx *context.APIContext) {
	// org permissions have been checked in context.OrgAssignment(), but individual permissions haven't been checked.
	if ctx.ContextUser.IsIndividual() {
		switch ctx.ContextUser.Visibility {
		case api.VisibleTypePrivate:
			if ctx.Doer == nil || (ctx.ContextUser.ID != ctx.Doer.ID && !ctx.Doer.IsAdmin) {
				ctx.APIErrorNotFound()
				return
			}
		case api.VisibleTypeLimited:
			if ctx.Doer == nil {
				ctx.APIErrorNotFound()
				return
			}
		}
	}
}

// check for and warn against deprecated authentication options
func checkDeprecatedAuthMethods(ctx *context.APIContext) {
	if ctx.FormString("token") != "" || ctx.FormString("access_token") != "" {
		ctx.Resp.Header().Set("X-Gitea-Warning", "token and access_token API authentication is deprecated and will be removed in gitea 1.23. Please use AuthorizationHeaderToken instead. Existing queries will continue to work but without authorization.")
	}
}

// Routes registers all v1 APIs routes to web application.
func Routes() *web.Router {
	m := web.NewRouter()

	// redirect HEAD requests to GET if no HEAD handler is defined (RFC 9110 §9.3.2)
	m.BeforeRouting(chi_middleware.GetHead)

	if setting.CORSConfig.Enabled {
		m.BeforeRouting(cors.Handler(cors.Options{
			AllowedOrigins:   setting.CORSConfig.AllowDomain,
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			AllowedHeaders:   append([]string{"Authorization", "X-Gitea-OTP"}, setting.CORSConfig.Headers...),
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		}))
	}

	m.AfterRouting(context.APIContexter())
	m.AfterRouting(checkDeprecatedAuthMethods)

	// Get user from session if logged in.
	m.AfterRouting(apiAuth(buildAuthGroup()))
	m.AfterRouting(codespaceTokenRouteGuard)

	m.AfterRouting(verifyAuthWithOptions(&common.VerifyOptions{
		SignInRequired: setting.Service.RequireSignInViewStrict,
	}))

	addActionsManagementRoutes := func(
		m *web.Router,
		reqOwnerCheck func(ctx *context.APIContext),
		act actions.API,
	) {
		m.Group("/actions", func() {
			m.Group("/secrets", func() {
				m.Get("", reqToken(), reqOwnerCheck, act.ListActionsSecrets)
				m.Combo("/{secretname}").
					Put(reqToken(), reqOwnerCheck, bind(api.CreateOrUpdateSecretOption{}), act.CreateOrUpdateSecret).
					Delete(reqToken(), reqOwnerCheck, act.DeleteSecret)
			})

			m.Group("/variables", func() {
				m.Get("", reqToken(), reqOwnerCheck, act.ListVariables)
				m.Combo("/{variablename}").
					Get(reqToken(), reqOwnerCheck, act.GetVariable).
					Delete(reqToken(), reqOwnerCheck, act.DeleteVariable).
					Post(reqToken(), reqOwnerCheck, bind(api.CreateVariableOption{}), act.CreateVariable).
					Put(reqToken(), reqOwnerCheck, bind(api.UpdateVariableOption{}), act.UpdateVariable)
			})

			m.Group("/runners", func() {
				m.Get("", reqToken(), reqOwnerCheck, act.ListRunners)
				m.Post("/registration-token", reqToken(), reqOwnerCheck, act.CreateRegistrationToken)
				m.Get("/{runner_id}", reqToken(), reqOwnerCheck, act.GetRunner)
				m.Delete("/{runner_id}", reqToken(), reqOwnerCheck, act.DeleteRunner)
				m.Patch("/{runner_id}", reqToken(), reqOwnerCheck, bind(api.EditActionRunnerOption{}), act.UpdateRunner)
			})
		})
	}
	addActionsReaderRoutes := func(
		m *web.Router,
		reqReaderCheck func(ctx *context.APIContext),
		act actions.API,
	) {
		m.Group("/actions", func() {
			m.Get("/runs", reqToken(), reqReaderCheck, act.ListWorkflowRuns)
			m.Get("/jobs", reqToken(), reqReaderCheck, act.ListWorkflowJobs)
		})
	}

	m.Group("", func() {
		// Miscellaneous (no scope required)
		if setting.API.EnableSwagger {
			m.Get("/swagger", func(ctx *context.APIContext) {
				ctx.Redirect(setting.AppSubURL + "/api/swagger")
			})
		}

		if setting.Federation.Enabled {
			m.Get("/nodeinfo", activitypub.NotImplemented)
			m.Any("/activitypub/*", tokenRequiresScopes(auth_model.AccessTokenScopeCategoryActivityPub), activitypub.NotImplemented)
		}

		// Misc (public accessible)
		m.Group("", func() {
			m.Get("/version", codespaceTokenRoute(codespaceTokenRoutePolicyPublicInfo), misc.Version)
			m.Get("/signing-key.gpg", codespaceTokenRoute(codespaceTokenRoutePolicyPublicInfo), misc.SigningKeyGPG)
			m.Get("/signing-key.pub", codespaceTokenRoute(codespaceTokenRoutePolicyPublicInfo), misc.SigningKeySSH)
			m.Post("/markup", reqToken(), bind(api.MarkupOption{}), misc.Markup)
			m.Post("/markdown", reqToken(), bind(api.MarkdownOption{}), misc.Markdown)
			m.Post("/markdown/raw", reqToken(), misc.MarkdownRaw)
			m.Get("/gitignore/templates", misc.ListGitignoresTemplates)
			m.Get("/gitignore/templates/{name}", misc.GetGitignoreTemplateInfo)
			m.Get("/licenses", misc.ListLicenseTemplates)
			m.Get("/licenses/{name}", misc.GetLicenseTemplateInfo)
			m.Get("/label/templates", misc.ListLabelTemplates)
			m.Get("/label/templates/{name}", misc.GetLabelTemplate)

			m.Group("/settings", func() {
				m.Get("/ui", settings.GetGeneralUISettings)
				m.Get("/api", settings.GetGeneralAPISettings)
				m.Get("/attachment", settings.GetGeneralAttachmentSettings)
				m.Get("/repository", settings.GetGeneralRepoSettings)
			})
		})

		// Token introspection and deletion endpoint
		m.Combo("/token").
			Get(reqToken(), token.GetCurrentToken).
			Delete(reqToken(), token.DeleteCurrentToken)

		// Notifications (requires 'notifications' scope)
		// The notifications API is not available for public-only tokens because a user's notifications mix
		// public and private repository events in the same mailbox.
		m.Group("/notifications", func() {
			m.Combo("").
				Get(reqToken(), notify.ListNotifications).
				Put(reqToken(), notify.ReadNotifications)
			m.Get("/new", reqToken(), notify.NewAvailable)
			m.Combo("/threads/{id}").
				Get(reqToken(), notify.GetThread).
				Patch(reqToken(), notify.ReadThread)
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryNotification), rejectPublicOnly())

		// Users (requires user scope)
		m.Group("/users", func() {
			m.Get("/search", reqExploreSignIn(), reqUsersExploreEnabled(), user.Search)

			m.Group("/{username}", func() {
				m.Get("", reqExploreSignIn(), user.GetInfo)

				if setting.Service.EnableUserHeatmap {
					m.Get("/heatmap", user.GetUserHeatmapData)
				}

				m.Get("/repos", tokenRequiresScopes(auth_model.AccessTokenScopeCategoryRepository), reqExploreSignIn(), user.ListUserRepos)
				m.Get("/projects", tokenRequiresScopes(auth_model.AccessTokenScopeCategoryIssue), reqExploreSignIn(),
					reqProjectsUnitAccess(perm.AccessModeRead), shared.ListProjects)
				m.Group("/tokens", func() {
					m.Combo("").Get(user.ListAccessTokens).
						Post(bind(api.CreateAccessTokenOption{}), reqToken(), user.CreateAccessToken)
					m.Combo("/{id}").Delete(reqToken(), user.DeleteAccessToken)
				}, reqSelfOrAdmin(), reqBasicOrRevProxyAuth())

				m.Get("/activities/feeds", user.ListUserActivityFeeds)
			}, context.UserAssignmentAPI(), checkTokenPublicOnly(), individualPermsChecker)
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryUser))

		// Users (requires user scope)
		m.Group("/users", func() {
			m.Group("/{username}", func() {
				m.Get("/keys", user.ListPublicKeys)
				m.Get("/gpg_keys", user.ListGPGKeys)

				m.Get("/followers", user.ListFollowers)
				m.Group("/following", func() {
					m.Get("", user.ListFollowing)
					m.Get("/{target}", user.CheckFollowing)
				})

				m.Get("/starred", reqStarsEnabled(), user.GetStarredRepos)

				m.Get("/subscriptions", user.GetWatchedRepos)
			}, context.UserAssignmentAPI(), checkTokenPublicOnly())
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryUser), reqToken())

		// Users (requires user scope)
		m.Group("/user", func() {
			m.Get("", codespaceTokenRoute(codespaceTokenRoutePolicySelf), user.GetAuthenticatedUser)
			m.Group("/settings", func() {
				m.Get("", user.GetUserSettings)
				m.Patch("", bind(api.UserSettingsOptions{}), user.UpdateUserSettings)
			}, rejectPublicOnly())
			m.Group("/projects", func() {
				addProjectRoutes(m, reqToken())
			}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryIssue))
			// Email addresses are always private account data.
			m.Combo("/emails", rejectPublicOnly()).
				Get(user.ListEmails).
				Post(bind(api.CreateEmailOption{}), user.AddEmail).
				Delete(bind(api.DeleteEmailOption{}), user.DeleteEmail)

			// manage user-level actions features
			m.Group("/actions", func() {
				m.Group("/secrets", func() {
					m.Combo("/{secretname}").
						Put(bind(api.CreateOrUpdateSecretOption{}), user.CreateOrUpdateSecret).
						Delete(user.DeleteSecret)
				})

				m.Group("/variables", func() {
					m.Get("", user.ListVariables)
					m.Combo("/{variablename}").
						Get(user.GetVariable).
						Delete(user.DeleteVariable).
						Post(bind(api.CreateVariableOption{}), user.CreateVariable).
						Put(bind(api.UpdateVariableOption{}), user.UpdateVariable)
				})

				m.Group("/runners", func() {
					m.Get("", reqToken(), user.ListRunners)
					m.Post("/registration-token", reqToken(), user.CreateRegistrationToken)
					m.Get("/{runner_id}", reqToken(), user.GetRunner)
					m.Delete("/{runner_id}", reqToken(), user.DeleteRunner)
					m.Patch("/{runner_id}", reqToken(), bind(api.EditActionRunnerOption{}), user.UpdateRunner)
				})

				m.Get("/runs", reqToken(), user.ListWorkflowRuns)
				m.Get("/jobs", reqToken(), user.ListWorkflowJobs)
			}, rejectPublicOnly())

			m.Get("/followers", user.ListMyFollowers)
			m.Group("/following", func() {
				m.Get("", user.ListMyFollowing)
				m.Group("/{username}", func() {
					m.Get("", user.CheckMyFollowing)
					m.Put("", user.Follow)
					m.Delete("", user.Unfollow)
				}, context.UserAssignmentAPI())
			})

			// (admin:public_key scope)
			m.Group("/keys", func() {
				m.Combo("").Get(user.ListMyPublicKeys).
					Post(bind(api.CreateKeyOption{}), user.CreatePublicKey)
				m.Combo("/{id}").Get(user.GetPublicKey).
					Delete(user.DeletePublicKey)
			}, rejectPublicOnly())

			// (admin:application scope)
			m.Group("/applications", func() {
				m.Combo("/oauth2").
					Get(user.ListOauth2Applications).
					Post(bind(api.CreateOAuth2ApplicationOptions{}), user.CreateOauth2Application)
				m.Combo("/oauth2/{id}").
					Delete(user.DeleteOauth2Application).
					Patch(bind(api.CreateOAuth2ApplicationOptions{}), user.UpdateOauth2Application).
					Get(user.GetOauth2Application)
			}, rejectPublicOnly())

			// (admin:gpg_key scope)
			m.Group("/gpg_keys", func() {
				m.Combo("").Get(user.ListMyGPGKeys).
					Post(bind(api.CreateGPGKeyOption{}), user.CreateGPGKey)
				m.Combo("/{id}").Get(user.GetGPGKey).
					Delete(user.DeleteGPGKey)
			}, rejectPublicOnly())
			m.Get("/gpg_key_token", rejectPublicOnly(), user.GetVerificationToken)
			m.Post("/gpg_key_verify", rejectPublicOnly(), bind(api.VerifyGPGKeyOption{}), user.VerifyUserGPGKey)

			// (repo scope)
			m.Combo("/repos", tokenRequiresScopes(auth_model.AccessTokenScopeCategoryRepository)).Get(user.ListMyRepos).
				Post(rejectPublicOnly(), bind(api.CreateRepoOption{}), repo.Create)

			// (repo scope)
			m.Group("/starred", func() {
				m.Get("", user.GetMyStarredRepos)
				m.Group("/{username}/{reponame}", func() {
					m.Get("", user.IsStarring)
					m.Put("", user.Star)
					m.Delete("", user.Unstar)
				}, repoAssignment(), checkTokenPublicOnly())
			}, reqStarsEnabled(), tokenRequiresScopes(auth_model.AccessTokenScopeCategoryRepository))
			m.Get("/times", rejectPublicOnly(), repo.ListMyTrackedTimes)
			m.Get("/stopwatches", rejectPublicOnly(), repo.GetStopwatches)
			m.Get("/subscriptions", user.GetMyWatchedRepos)
			m.Get("/teams", rejectPublicOnly(), org.ListUserTeams)
			m.Group("/hooks", func() {
				m.Combo("").Get(user.ListHooks).
					Post(bind(api.CreateHookOption{}), user.CreateHook)
				m.Combo("/{id}").Get(user.GetHook).
					Patch(bind(api.EditHookOption{}), user.EditHook).
					Delete(user.DeleteHook)
			}, reqWebhooksEnabled(), rejectPublicOnly())

			m.Group("/avatar", func() {
				m.Post("", bind(api.UpdateUserAvatarOption{}), user.UpdateAvatar)
				m.Delete("", user.DeleteAvatar)
			}, rejectPublicOnly())

			m.Group("/blocks", func() {
				m.Get("", user.ListBlocks)
				m.Group("/{username}", func() {
					m.Get("", user.CheckUserBlock)
					m.Put("", user.BlockUser)
					m.Delete("", user.UnblockUser)
				}, context.UserAssignmentAPI(), checkTokenPublicOnly())
			}, rejectPublicOnly())
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryUser), reqToken(), contextAuthenticatedUser(), checkTokenPublicOnly())

		// Repositories (requires repo scope, org scope)
		m.Post("/org/{org}/repos",
			// FIXME: we need org in context
			tokenRequiresScopes(auth_model.AccessTokenScopeCategoryOrganization, auth_model.AccessTokenScopeCategoryRepository),
			reqToken(),
			bind(api.CreateRepoOption{}),
			repo.CreateOrgRepoDeprecated)

		// requires repo scope
		// FIXME: Don't expose repository id outside of the system
		m.Combo("/repositories/{id}", reqToken(), tokenRequiresScopes(auth_model.AccessTokenScopeCategoryRepository)).Get(repo.GetByID)

		// Repos (requires repo scope)
		m.Group("/repos", func() {
			m.Get("/search", repo.Search)

			// (repo scope)
			m.Post("/migrate", reqToken(), bind(api.MigrateRepoOptions{}), repo.Migrate)

			m.Group("/{username}/{reponame}", func() {
				m.Get("/compare/*", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), repo.CompareDiff)

				m.Get("", codespaceTokenRepositoryRoute, reqAnyRepoReader(), reqCodespaceTokenRepositoryPermission(unit.TypeCode), repo.Get)
				m.Combo("").
					Delete(reqToken(), reqOwner(), repo.Delete).
					Patch(reqToken(), reqAdmin(), bind(api.EditRepoOption{}), repo.Edit)
				m.Post("/generate", reqToken(), reqRepoReader(unit.TypeCode), bind(api.GenerateRepoOption{}), repo.Generate)
				m.Group("/transfer", func() {
					m.Post("", reqOwner(), bind(api.TransferRepoOption{}), repo.Transfer)
					m.Post("/accept", repo.AcceptTransfer)
					m.Post("/reject", repo.RejectTransfer)
				}, reqToken())

				// Adds the routes for secrets/variables and runner management
				repoActionsAPI := repo.NewAction()
				addActionsManagementRoutes(m, reqOwner(), repoActionsAPI)
				m.Group("", func() {
					addActionsReaderRoutes(m, reqRepoReader(unit.TypeActions), repoActionsAPI)
				}, codespaceTokenRepositoryRoute)

				m.Group("/actions/workflows", func() {
					m.Get("", repo.ActionsListRepositoryWorkflows)
					m.Get("/{workflow_id}", repo.ActionsGetWorkflow)
					m.Get("/{workflow_id}/runs", repo.ActionsListWorkflowRuns)
					m.Put("/{workflow_id}/disable", reqRepoWriter(unit.TypeActions), repo.ActionsDisableWorkflow)
					m.Put("/{workflow_id}/enable", reqRepoWriter(unit.TypeActions), repo.ActionsEnableWorkflow)
					m.Post("/{workflow_id}/dispatches", reqRepoWriter(unit.TypeActions), bind(api.CreateActionWorkflowDispatch{}), repo.ActionsDispatchWorkflow)
				}, context.ReferencesGitRepo(), reqToken(), reqRepoReader(unit.TypeActions), codespaceTokenRepositoryRoute)

				m.Group("/actions/jobs", func() {
					m.Get("/{job_id}", repo.GetWorkflowJob)
					m.Get("/{job_id}/logs", repo.DownloadActionsRunJobLogs)
				}, reqToken(), reqRepoReader(unit.TypeActions), codespaceTokenRepositoryRoute)

				m.Group("/hooks/git", func() {
					m.Combo("").Get(repo.ListGitHooks)
					m.Group("/{id}", func() {
						m.Combo("").Get(repo.GetGitHook).
							Patch(bind(api.EditGitHookOption{}), repo.EditGitHook).
							Delete(repo.DeleteGitHook)
					})
				}, reqToken(), reqAdmin(), reqGitHook(), context.ReferencesGitRepo(true))
				m.Group("/hooks", func() {
					m.Combo("").Get(repo.ListHooks).
						Post(bind(api.CreateHookOption{}), repo.CreateHook)
					m.Group("/{id}", func() {
						m.Combo("").Get(repo.GetHook).
							Patch(bind(api.EditHookOption{}), repo.EditHook).
							Delete(repo.DeleteHook)
						m.Post("/tests", context.ReferencesGitRepo(), context.RepoRefForAPI, repo.TestHook)
					})
				}, reqToken(), reqAdmin(), reqWebhooksEnabled())
				m.Group("/collaborators", func() {
					m.Get("", reqAnyRepoReader(), repo.ListCollaborators)
					m.Group("/{collaborator}", func() {
						m.Combo("").Get(reqAnyRepoReader(), repo.IsCollaborator).
							Put(reqAdmin(), bind(api.AddCollaboratorOption{}), repo.AddOrUpdateCollaborator).
							Delete(reqAdmin(), repo.DeleteCollaborator)
						m.Get("/permission", repo.GetRepoPermissions)
					})
				}, reqToken())
				m.Get("/assignees", codespaceTokenRepositoryRoute, reqToken(), reqCodespaceTokenRepositoryPermission(unit.TypeIssues, unit.TypePullRequests), reqAnyRepoReader(), repo.GetAssignees)
				m.Get("/assignees/{assignee}", codespaceTokenRepositoryRoute, reqToken(), reqCodespaceTokenRepositoryPermission(unit.TypeIssues, unit.TypePullRequests), reqAnyRepoReader(), repo.CheckRepoIssueAssignee)
				m.Get("/reviewers", codespaceTokenRepositoryRoute, reqToken(), reqCodespaceTokenRepositoryPermission(unit.TypePullRequests), reqAnyRepoReader(), repo.GetReviewers)
				m.Group("/teams", func() {
					m.Get("", reqAnyRepoReader(), repo.ListTeams)
					m.Combo("/{team}").Get(reqAnyRepoReader(), repo.IsTeam).
						Put(reqAdmin(), repo.AddTeam).
						Delete(reqAdmin(), repo.DeleteTeam)
				}, reqToken())
				m.Get("/raw/*", codespaceTokenRepositoryRoute, context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetRawFile)
				m.Get("/media/*", codespaceTokenRepositoryRoute, context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetRawFileOrLFS)
				m.Methods("HEAD,GET", "/archive/*", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(true), repo.GetArchive)
				m.Get("/forks", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), repo.ListForks)
				m.Post("/forks", reqToken(), reqRepoReader(unit.TypeCode), bind(api.CreateForkOption{}), repo.CreateFork)
				m.Post("/merge-upstream", codespaceTokenRepositoryRoute, reqToken(), mustNotBeArchived, reqRepoWriter(unit.TypeCode), bind(api.MergeUpstreamRequest{}), repo.MergeUpstream)
				m.Group("/branches", func() {
					m.Get("", repo.ListBranches)
					m.Get("/*", repo.GetBranch)
					m.Delete("/*", reqToken(), reqRepoWriter(unit.TypeCode), mustNotBeArchived, repo.DeleteBranch)
					m.Post("", reqToken(), reqRepoWriter(unit.TypeCode), mustNotBeArchived, bind(api.CreateBranchRepoOption{}), repo.CreateBranch)
					m.Put("/*", reqToken(), reqRepoWriter(unit.TypeCode), mustNotBeArchived, bind(api.UpdateBranchRepoOption{}), repo.UpdateBranch)
					m.Patch("/*", reqToken(), reqRepoWriter(unit.TypeCode), mustNotBeArchived, bind(api.RenameBranchRepoOption{}), repo.RenameBranch)
				}, context.ReferencesGitRepo(), reqRepoReader(unit.TypeCode), codespaceTokenRepositoryRoute)
				m.Group("/branch_protections", func() {
					m.Get("", repo.ListBranchProtections)
					m.Post("", bind(api.CreateBranchProtectionOption{}), mustNotBeArchived, repo.CreateBranchProtection)
					m.Group("/*", func() {
						m.Get("", repo.GetBranchProtection)
						m.Patch("", bind(api.EditBranchProtectionOption{}), mustNotBeArchived, repo.EditBranchProtection)
						m.Delete("", mustNotBeArchived, repo.DeleteBranchProtection)
					})
					m.Post("/priority", bind(api.UpdateBranchProtectionPriories{}), mustNotBeArchived, repo.UpdateBranchProtectionPriories)
				}, reqToken(), reqAdmin())
				m.Group("/tags", func() {
					m.Get("", repo.ListTags)
					m.Get("/*", repo.GetTag)
					m.Post("", reqToken(), reqRepoWriter(unit.TypeCode), mustNotBeArchived, bind(api.CreateTagOption{}), repo.CreateTag)
					m.Delete("/*", reqToken(), reqRepoWriter(unit.TypeCode), mustNotBeArchived, repo.DeleteTag)
				}, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(true), codespaceTokenRepositoryRoute)
				m.Group("/tag_protections", func() {
					m.Combo("").Get(repo.ListTagProtection).
						Post(bind(api.CreateTagProtectionOption{}), mustNotBeArchived, repo.CreateTagProtection)
					m.Group("/{id}", func() {
						m.Combo("").Get(repo.GetTagProtection).
							Patch(bind(api.EditTagProtectionOption{}), mustNotBeArchived, repo.EditTagProtection).
							Delete(repo.DeleteTagProtection)
					})
				}, reqToken(), reqAdmin())
				m.Group("/actions", func() {
					m.Get("/tasks", repo.ListActionTasks)
					m.Group("/runs", func() {
						m.Group("/{run}", func() {
							m.Get("", repo.GetWorkflowRun)
							m.Group("/attempts/{attempt}", func() {
								m.Get("", repo.GetWorkflowRunAttempt)
								m.Get("/jobs", repo.ListWorkflowRunAttemptJobs)
							})
							m.Delete("", reqToken(), reqRepoWriter(unit.TypeActions), repo.DeleteActionRun)
							m.Post("/rerun", reqToken(), reqRepoWriter(unit.TypeActions), repo.RerunWorkflowRun)
							m.Post("/rerun-failed-jobs", reqToken(), reqRepoWriter(unit.TypeActions), repo.RerunFailedWorkflowRun)
							m.Post("/cancel", reqToken(), reqRepoWriter(unit.TypeActions), repo.CancelWorkflowRun)
							m.Post("/force-cancel", reqToken(), reqRepoWriter(unit.TypeActions), repo.ForceCancelWorkflowRun)
							m.Post("/approve", reqToken(), reqRepoWriter(unit.TypeActions), repo.ApproveWorkflowRun)
							m.Group("/jobs", func() {
								m.Get("", repo.ListWorkflowRunJobs)
								m.Post("/{job_id}/rerun", reqToken(), reqRepoWriter(unit.TypeActions), repo.RerunWorkflowJob)
							})
							m.Get("/logs", reqToken(), repo.GetWorkflowRunLogs)
							m.Get("/artifacts", repo.GetArtifactsOfRun)
						})
					})
					m.Get("/artifacts", repo.GetArtifacts)
					m.Group("/artifacts/{artifact_id}", func() {
						m.Get("", repo.GetArtifact)
						m.Delete("", reqRepoWriter(unit.TypeActions), repo.DeleteArtifact)
					})
					m.Get("/artifacts/{artifact_id}/zip", repo.DownloadArtifact)
				}, reqRepoReader(unit.TypeActions), codespaceTokenRepositoryRoute)
				m.Group("/keys", func() {
					m.Combo("").Get(repo.ListDeployKeys).
						Post(bind(api.CreateKeyOption{}), repo.CreateDeployKey)
					m.Combo("/{id}").Get(repo.GetDeployKey).
						Delete(repo.DeleteDeploykey)
				}, reqToken(), reqAdmin())
				m.Group("/times", func() {
					m.Combo("").Get(repo.ListTrackedTimesByRepository)
					m.Combo("/{timetrackingusername}").Get(repo.ListTrackedTimesByUser)
				}, mustEnableIssues, reqToken(), reqCodespaceTokenRepositoryPermission(unit.TypeIssues, unit.TypePullRequests), codespaceTokenRepositoryRoute)
				m.Group("/wiki", func() {
					m.Combo("/page/{pageName}").
						Get(repo.GetWikiPage).
						Patch(mustNotBeArchived, reqToken(), reqRepoWriter(unit.TypeWiki), bind(api.CreateWikiPageOptions{}), repo.EditWikiPage).
						Delete(mustNotBeArchived, reqToken(), reqRepoWriter(unit.TypeWiki), repo.DeleteWikiPage)
					m.Get("/revisions/{pageName}", repo.ListPageRevisions)
					m.Post("/new", reqToken(), mustNotBeArchived, reqRepoWriter(unit.TypeWiki), bind(api.CreateWikiPageOptions{}), repo.NewWikiPage)
					m.Get("/pages", repo.ListWikiPages)
				}, mustEnableWiki, reqCodespaceTokenRepositoryPermission(unit.TypeWiki), codespaceTokenRepositoryRoute)
				m.Post("/markup", codespaceTokenRepositoryRoute, reqToken(), reqCodespaceTokenRepositoryRead(unit.TypeCode), bind(api.MarkupOption{}), misc.Markup)
				m.Post("/markdown", codespaceTokenRepositoryRoute, reqToken(), reqCodespaceTokenRepositoryRead(unit.TypeCode), bind(api.MarkdownOption{}), misc.Markdown)
				m.Post("/markdown/raw", codespaceTokenRepositoryRoute, reqToken(), reqCodespaceTokenRepositoryRead(unit.TypeCode), misc.MarkdownRaw)
				m.Get("/stargazers", reqStarsEnabled(), repo.ListStargazers)
				m.Get("/subscribers", repo.ListSubscribers)
				m.Group("/subscription", func() {
					m.Get("", user.IsWatching)
					m.Put("", user.Watch)
					m.Delete("", user.Unwatch)
				}, reqToken())
				m.Group("/releases", func() {
					m.Combo("").Get(repo.ListReleases).
						Post(reqToken(), reqRepoWriter(unit.TypeReleases), context.ReferencesGitRepo(), bind(api.CreateReleaseOption{}), repo.CreateRelease)
					m.Combo("/latest").Get(repo.GetLatestRelease)
					m.Group("/{id}", func() {
						m.Combo("").Get(repo.GetRelease).
							Patch(reqToken(), reqRepoWriter(unit.TypeReleases), context.ReferencesGitRepo(), bind(api.EditReleaseOption{}), repo.EditRelease).
							Delete(reqToken(), reqRepoWriter(unit.TypeReleases), repo.DeleteRelease)
						m.Group("/assets", func() {
							m.Combo("").Get(repo.ListReleaseAttachments).
								Post(reqToken(), reqRepoWriter(unit.TypeReleases), repo.CreateReleaseAttachment)
							m.Combo("/{attachment_id}").Get(repo.GetReleaseAttachment).
								Patch(reqToken(), reqRepoWriter(unit.TypeReleases), bind(api.EditAttachmentOptions{}), repo.EditReleaseAttachment).
								Delete(reqToken(), reqRepoWriter(unit.TypeReleases), repo.DeleteReleaseAttachment)
						})
					})
					m.Group("/tags", func() {
						m.Combo("/{tag}").
							Get(repo.GetReleaseByTag).
							Delete(reqToken(), reqRepoWriter(unit.TypeReleases), repo.DeleteReleaseByTag)
					})
				}, reqRepoReader(unit.TypeReleases), codespaceTokenRepositoryRoute)
				m.Post("/mirror-sync", codespaceTokenRepositoryRoute, reqToken(), reqRepoWriter(unit.TypeCode), mustNotBeArchived, repo.MirrorSync)
				m.Post("/push_mirrors-sync", reqAdmin(), reqToken(), mustNotBeArchived, repo.PushMirrorSync)
				m.Group("/push_mirrors", func() {
					m.Combo("").Get(repo.ListPushMirrors).
						Post(mustNotBeArchived, bind(api.CreatePushMirrorOption{}), repo.AddPushMirror)
					m.Combo("/{name}").
						Delete(mustNotBeArchived, repo.DeletePushMirrorByRemoteName).
						Get(repo.GetPushMirrorByName)
				}, reqAdmin(), reqToken())

				m.Get("/editorconfig/{filename}", codespaceTokenRepositoryRoute, context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetEditorconfig)
				m.Group("/pulls", func() {
					m.Combo("").Get(repo.ListPullRequests).
						Post(reqToken(), mustNotBeArchived, bind(api.CreatePullRequestOption{}), repo.CreatePullRequest)
					m.Get("/pinned", repo.ListPinnedPullRequests)
					m.Post("/comments/{id}/resolve", reqToken(), mustNotBeArchived, repo.ResolvePullReviewComment)
					m.Post("/comments/{id}/unresolve", reqToken(), mustNotBeArchived, repo.UnresolvePullReviewComment)
					m.Group("/{index}", func() {
						m.Combo("").Get(repo.GetPullRequest).
							Patch(reqToken(), bind(api.EditPullRequestOption{}), repo.EditPullRequest)
						m.Get(".{diffType:diff|patch}", repo.DownloadPullDiffOrPatch)
						m.Post("/update", reqToken(), repo.UpdatePullRequest)
						m.Get("/commits", repo.GetPullRequestCommits)
						m.Get("/files", repo.GetPullRequestFiles)
						m.Combo("/merge").Get(repo.IsPullRequestMerged).
							Post(reqToken(), mustNotBeArchived, bind(forms.MergePullRequestForm{}), repo.MergePullRequest).
							Delete(reqToken(), mustNotBeArchived, repo.CancelScheduledAutoMerge)
						m.Group("/reviews", func() {
							m.Combo("").
								Get(repo.ListPullReviews).
								Post(reqToken(), bind(api.CreatePullReviewOptions{}), repo.CreatePullReview)
							m.Group("/{id}", func() {
								m.Combo("").
									Get(repo.GetPullReview).
									Delete(reqToken(), repo.DeletePullReview).
									Post(reqToken(), bind(api.SubmitPullReviewOptions{}), repo.SubmitPullReview)
								m.Combo("/comments").
									Get(repo.GetPullReviewComments)
								m.Post("/dismissals", reqToken(), bind(api.DismissPullReviewOptions{}), repo.DismissPullReview)
								m.Post("/undismissals", reqToken(), repo.UnDismissPullReview)
							})
						})
						m.Combo("/requested_reviewers", reqToken()).
							Delete(bind(api.PullReviewRequestOptions{}), repo.DeleteReviewRequests).
							Post(bind(api.PullReviewRequestOptions{}), repo.CreateReviewRequests)
						m.Post("/comments/{id}/replies", reqToken(), mustNotBeArchived, bind(api.CreatePullReviewCommentReplyOptions{}), repo.CreatePullReviewCommentReply)
					})
					m.Get("/{base}/*", repo.GetPullRequestByBaseHead)
				}, mustAllowPulls, reqRepoReader(unit.TypeCode), reqCodespaceTokenRepositoryPermission(unit.TypePullRequests), context.ReferencesGitRepo(), codespaceTokenRepositoryRoute)
				m.Group("/statuses", func() { // "/statuses/{sha}" only accepts commit ID
					m.Combo("/{sha}").Get(repo.GetCommitStatuses).
						Post(reqToken(), reqRepoWriter(unit.TypeCode), bind(api.CreateStatusOption{}), repo.NewCommitStatus)
				}, reqRepoReader(unit.TypeCode), codespaceTokenRepositoryRoute)
				m.Group("/commits", func() {
					m.Group("", func() {
						m.Get("", repo.GetAllCommits)
						m.Get("/{sha}", repo.GetSingleCommit) // GitHub-compatible endpoint
						m.Get("/{sha}.{diffType:diff|patch}", repo.DownloadCommitDiffOrPatch)
					}, context.ReferencesGitRepo(true))
					m.PathGroup("/*", func(g *web.RouterPathGroup) {
						// Mis-configured reverse proxy might decode the `%2F` to slash ahead, so we need to support both formats (escaped, unescaped) here.
						// It also matches GitHub's behavior
						g.MatchPath("GET", "/<ref:*>/status", repo.GetCombinedCommitStatusByRef)
						g.MatchPath("GET", "/<ref:*>/statuses", repo.GetCommitStatusesByRef)
						g.MatchPath("GET", "/<sha>/pull", repo.GetCommitPullRequest)
					})
				}, reqRepoReader(unit.TypeCode), codespaceTokenRepositoryRoute)
				m.Group("/git", func() {
					m.Group("/commits", func() {
						m.Get("/{sha}", repo.GetSingleCommit)
						m.Get("/{sha}.{diffType:diff|patch}", repo.DownloadCommitDiffOrPatch)
					})
					m.Get("/refs", repo.GetGitAllRefs)
					m.Get("/refs/*", repo.GetGitRefs)
					m.Get("/trees/{sha}", repo.GetTree)
					m.Get("/blobs/{sha}", repo.GetBlob)
					m.Get("/tags/{sha}", repo.GetAnnotatedTag)
					m.Get("/notes/{sha}", repo.GetNote)
				}, context.ReferencesGitRepo(true), reqRepoReader(unit.TypeCode), codespaceTokenRepositoryRoute)
				m.Post("/diffpatch", codespaceTokenRepositoryRoute, mustEnableEditor, reqToken(), reqRepoWriter(unit.TypeCode), bind(api.ApplyDiffPatchFileOptions{}), repo.ReqChangeRepoFileOptionsAndCheck, repo.ApplyDiffPatch)
				m.Group("/contents", func() {
					m.Get("", repo.GetContentsList)
					m.Get("/*", repo.GetContents)
					m.Group("", func() {
						// "change file" operations, need permission to write to the target branch provided by the form
						m.Post("", bind(api.ChangeFilesOptions{}), repo.ReqChangeRepoFileOptionsAndCheck, repo.ChangeFiles)
						m.Group("/*", func() {
							m.Post("", bind(api.CreateFileOptions{}), repo.ReqChangeRepoFileOptionsAndCheck, repo.CreateFile)
							m.Put("", bind(api.UpdateFileOptions{}), repo.ReqChangeRepoFileOptionsAndCheck, repo.UpdateFile)
							m.Delete("", bind(api.DeleteFileOptions{}), repo.ReqChangeRepoFileOptionsAndCheck, repo.DeleteFile)
						})
					}, mustEnableEditor, reqToken(), reqRepoWriter(unit.TypeCode))
				}, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(), codespaceTokenRepositoryRoute)
				m.Group("/contents-ext", func() {
					m.Get("", repo.GetContentsExt)
					m.Get("/*", repo.GetContentsExt)
				}, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(), codespaceTokenRepositoryRoute)
				m.Combo("/file-contents", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo()).
					Get(repo.GetFileContentsGet).
					Post(reqCodespaceTokenRepositoryPermission(unit.TypeCode), bind(api.GetFilesOptions{}), repo.GetFileContentsPost) // the POST method requires "write" permission, so we also support "GET" method above
				m.Get("/signing-key.gpg", codespaceTokenRepositoryRoute, reqCodespaceTokenRepositoryPermission(unit.TypeCode), misc.SigningKeyGPG)
				m.Get("/signing-key.pub", codespaceTokenRepositoryRoute, reqCodespaceTokenRepositoryPermission(unit.TypeCode), misc.SigningKeySSH)
				m.Group("/topics", func() {
					m.Combo("").Get(repo.ListTopics).
						Put(reqToken(), reqAdmin(), bind(api.RepoTopicOptions{}), repo.UpdateTopics)
					m.Group("/{topic}", func() {
						m.Combo("").Put(reqToken(), repo.AddTopic).
							Delete(reqToken(), repo.DeleteTopic)
					}, reqAdmin())
				}, reqAnyRepoReader())
				m.Get("/issue_templates", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(), repo.GetIssueTemplates)
				m.Get("/issue_config", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(), repo.GetIssueConfig)
				m.Get("/issue_config/validate", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(), repo.ValidateIssueConfig)
				m.Get("/languages", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), repo.GetLanguages)
				m.Get("/licenses", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), repo.GetLicenses)
				m.Get("/activities/feeds", repo.ListRepoActivityFeeds)
				m.Get("/new_pin_allowed", codespaceTokenRepositoryRoute, reqCodespaceTokenRepositoryPermission(unit.TypeIssues, unit.TypePullRequests), repo.AreNewIssuePinsAllowed)
				m.Group("/avatar", func() {
					m.Post("", bind(api.UpdateRepoAvatarOption{}), repo.UpdateAvatar)
					m.Delete("", repo.DeleteAvatar)
				}, reqAdmin(), reqToken())

				m.Methods("HEAD,GET", "/{ball_type:tarball|zipball|bundle}/*", codespaceTokenRepositoryRoute, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(true), repo.DownloadArchive)
			}, repoAssignment(), checkTokenPublicOnly())
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryRepository))

		// Artifacts direct download endpoint authenticates via signed url
		// it is protected by the "sig" parameter (to help to access private repo), so no need to use other middlewares
		m.Get("/repos/{username}/{reponame}/actions/artifacts/{artifact_id}/zip/raw", codespaceTokenRoute(codespaceTokenRoutePolicySignedArtifact), repo.DownloadArtifactRaw)

		// Notifications (requires notifications scope)
		m.Group("/repos", func() {
			m.Group("/{username}/{reponame}", func() {
				m.Combo("/notifications", reqToken()).
					Get(notify.ListRepoNotifications).
					Put(notify.ReadRepoNotifications)
			}, repoAssignment(), checkTokenPublicOnly())
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryNotification))

		// Issue (requires issue scope)
		m.Group("/repos", func() {
			m.Get("/issues/search", repo.SearchIssues)

			m.Group("/{username}/{reponame}", func() {
				m.Group("/issues", func() {
					m.Combo("").Get(repo.ListIssues).
						Post(reqToken(), mustNotBeArchived, bind(api.CreateIssueOption{}), reqRepoReader(unit.TypeIssues), repo.CreateIssue)
					m.Get("/pinned", reqRepoReader(unit.TypeIssues), repo.ListPinnedIssues)
					m.Group("/comments", func() {
						m.Get("", repo.ListRepoIssueComments)
						m.Group("/{id}", func() {
							m.Combo("").
								Get(repo.GetIssueComment).
								Patch(mustNotBeArchived, reqToken(), bind(api.EditIssueCommentOption{}), repo.EditIssueComment).
								Delete(reqToken(), repo.DeleteIssueComment)
							m.Combo("/reactions").
								Get(repo.GetIssueCommentReactions).
								Post(reqToken(), bind(api.EditReactionOption{}), repo.PostIssueCommentReaction).
								Delete(reqToken(), bind(api.EditReactionOption{}), repo.DeleteIssueCommentReaction)
							m.Group("/assets", func() {
								m.Combo("").
									Get(repo.ListIssueCommentAttachments).
									Post(reqToken(), mustNotBeArchived, repo.CreateIssueCommentAttachment)
								m.Combo("/{attachment_id}").
									Get(repo.GetIssueCommentAttachment).
									Patch(reqToken(), mustNotBeArchived, bind(api.EditAttachmentOptions{}), repo.EditIssueCommentAttachment).
									Delete(reqToken(), mustNotBeArchived, repo.DeleteIssueCommentAttachment)
							}, mustEnableAttachments)
						})
					})
					m.Group("/{index}", func() {
						m.Combo("").Get(repo.GetIssue).
							Patch(reqToken(), bind(api.EditIssueOption{}), repo.EditIssue).
							Delete(reqToken(), reqAdmin(), context.ReferencesGitRepo(), repo.DeleteIssue)
						m.Combo("/assignees").
							Post(reqToken(), mustNotBeArchived, bind(api.IssueAssigneesOption{}), repo.AddIssueAssignees).
							Delete(reqToken(), mustNotBeArchived, bind(api.IssueAssigneesOption{}), repo.DeleteIssueAssignees)
						m.Get("/assignees/{assignee}", repo.CheckIssueAssignee)
						m.Group("/comments", func() {
							m.Combo("").Get(repo.ListIssueComments).
								Post(reqToken(), mustNotBeArchived, bind(api.CreateIssueCommentOption{}), repo.CreateIssueComment)
							m.Combo("/{id}", reqToken()).Patch(bind(api.EditIssueCommentOption{}), repo.EditIssueCommentDeprecated).
								Delete(repo.DeleteIssueCommentDeprecated)
						})
						m.Get("/timeline", repo.ListIssueCommentsAndTimeline)
						m.Group("/labels", func() {
							m.Combo("").Get(repo.ListIssueLabels).
								Post(reqToken(), bind(api.IssueLabelsOption{}), repo.AddIssueLabels).
								Put(reqToken(), bind(api.IssueLabelsOption{}), repo.ReplaceIssueLabels).
								Delete(reqToken(), repo.ClearIssueLabels)
							m.Delete("/{id}", reqToken(), repo.DeleteIssueLabel)
						})
						m.Group("/times", func() {
							m.Combo("").
								Get(repo.ListTrackedTimes).
								Post(bind(api.AddTimeOption{}), repo.AddTime).
								Delete(repo.ResetIssueTime)
							m.Delete("/{id}", repo.DeleteTime)
						}, reqToken())
						m.Combo("/deadline").Post(reqToken(), bind(api.EditDeadlineOption{}), repo.UpdateIssueDeadline)
						m.Group("/stopwatch", func() {
							m.Post("/start", repo.StartIssueStopwatch)
							m.Post("/stop", repo.StopIssueStopwatch)
							m.Delete("/delete", repo.DeleteIssueStopwatch)
						}, reqToken())
						m.Group("/subscriptions", func() {
							m.Get("", repo.GetIssueSubscribers)
							m.Get("/check", reqToken(), repo.CheckIssueSubscription)
							m.Put("/{user}", reqToken(), repo.AddIssueSubscription)
							m.Delete("/{user}", reqToken(), repo.DelIssueSubscription)
						})
						m.Combo("/reactions").
							Get(repo.GetIssueReactions).
							Post(reqToken(), bind(api.EditReactionOption{}), repo.PostIssueReaction).
							Delete(reqToken(), bind(api.EditReactionOption{}), repo.DeleteIssueReaction)
						m.Group("/assets", func() {
							m.Combo("").
								Get(repo.ListIssueAttachments).
								Post(reqToken(), mustNotBeArchived, repo.CreateIssueAttachment)
							m.Combo("/{attachment_id}").
								Get(repo.GetIssueAttachment).
								Patch(reqToken(), mustNotBeArchived, bind(api.EditAttachmentOptions{}), repo.EditIssueAttachment).
								Delete(reqToken(), mustNotBeArchived, repo.DeleteIssueAttachment)
						}, mustEnableAttachments)
						m.Combo("/dependencies").
							Get(repo.GetIssueDependencies).
							Post(reqToken(), mustNotBeArchived, bind(api.IssueMeta{}), repo.CreateIssueDependency).
							Delete(reqToken(), mustNotBeArchived, bind(api.IssueMeta{}), repo.RemoveIssueDependency)
						m.Combo("/blocks").
							Get(repo.GetIssueBlocks).
							Post(reqToken(), bind(api.IssueMeta{}), repo.CreateIssueBlocking).
							Delete(reqToken(), bind(api.IssueMeta{}), repo.RemoveIssueBlocking)
						m.Group("/pin", func() {
							m.Combo("").
								Post(reqToken(), reqAdmin(), repo.PinIssue).
								Delete(reqToken(), reqAdmin(), repo.UnpinIssue)
							m.Patch("/{position}", reqToken(), reqAdmin(), repo.MoveIssuePin)
						})
						m.Group("/lock", func() {
							m.Combo("").
								Put(bind(api.LockIssueOption{}), repo.LockIssue).
								Delete(repo.UnlockIssue)
						}, reqToken(), reqAdmin())
					})
				}, mustEnableIssuesOrPulls)
				m.Group("/labels", func() {
					m.Combo("").Get(repo.ListLabels).
						Post(reqToken(), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.CreateLabelOption{}), repo.CreateLabel)
					m.Combo("/{id}").Get(repo.GetLabel).
						Patch(reqToken(), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.EditLabelOption{}), repo.EditLabel).
						Delete(reqToken(), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), repo.DeleteLabel)
				})
				m.Group("/milestones", func() {
					m.Combo("").Get(repo.ListMilestones).
						Post(reqToken(), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.CreateMilestoneOption{}), repo.CreateMilestone)
					m.Combo("/{id}").Get(repo.GetMilestone).
						Patch(reqToken(), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.EditMilestoneOption{}), repo.EditMilestone).
						Delete(reqToken(), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), repo.DeleteMilestone)
				})
				m.Group("/projects", func() {
					addProjectRoutes(m, reqToken(), reqRepoWriter(unit.TypeProjects), mustNotBeArchived)
				}, reqRepoReader(unit.TypeProjects), mustEnableRepoProjects)
			}, repoAssignment(), checkTokenPublicOnly(), codespaceTokenRepositoryRoute, reqCodespaceTokenRepositoryPermission(unit.TypeIssues, unit.TypePullRequests))
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryIssue))

		// NOTE: these are Gitea package management API - see packages.CommonRoutes and packages.DockerContainerRoutes for endpoints that implement package manager APIs
		m.Group("/packages/{username}", func() {
			m.Group("/{type}/{name}", func() {
				m.Get("/", packages.ListPackageVersions)
				m.Delete("", reqPackageAccess(perm.AccessModeWrite), packages.DeletePackage)

				m.Group("/{version}", func() {
					m.Get("", packages.GetPackage)
					m.Delete("", reqPackageAccess(perm.AccessModeWrite), packages.DeletePackageVersion)
					m.Get("/files", packages.ListPackageFiles)
				})

				m.Group("/-", func() {
					m.Get("/latest", packages.GetLatestPackageVersion)
					m.Post("/link/{repo_name}", reqPackageAccess(perm.AccessModeWrite), packages.LinkPackage)
					m.Post("/unlink", reqPackageAccess(perm.AccessModeWrite), packages.UnlinkPackage)
				})
			})

			m.Get("/", packages.ListPackages)
		}, reqToken(), tokenRequiresScopes(auth_model.AccessTokenScopeCategoryPackage), context.UserAssignmentAPI(), context.PackageAssignmentAPI(), reqPackageAccess(perm.AccessModeRead), checkTokenPublicOnly())

		// Organizations
		m.Get("/user/orgs", reqToken(), tokenRequiresScopes(auth_model.AccessTokenScopeCategoryUser, auth_model.AccessTokenScopeCategoryOrganization), checkTokenPublicOnly(), org.ListMyOrgs)
		m.Group("/users/{username}/orgs", func() {
			m.Get("", reqToken(), org.ListUserOrgs)
			m.Get("/{org}/permissions", reqToken(), org.GetUserOrgsPermissions)
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryUser, auth_model.AccessTokenScopeCategoryOrganization), context.UserAssignmentAPI(), checkTokenPublicOnly())
		m.Post("/orgs", tokenRequiresScopes(auth_model.AccessTokenScopeCategoryOrganization), reqToken(), bind(api.CreateOrgOption{}), org.Create)
		m.Get("/orgs", org.GetAll, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryOrganization))
		m.Group("/orgs/{org}", func() {
			m.Combo("").Get(org.Get).
				Patch(reqToken(), reqOrgOwnership(), bind(api.EditOrgOption{}), org.Edit).
				Delete(reqToken(), reqOrgOwnership(), org.Delete)
			m.Post("/rename", reqToken(), reqOrgOwnership(), bind(api.RenameOrgOption{}), org.Rename)
			m.Combo("/repos").Get(user.ListOrgRepos).
				Post(reqToken(), bind(api.CreateRepoOption{}), repo.CreateOrgRepo).
				Delete(reqToken(), reqOrgOwnership(), tokenRequiresScopes(auth_model.AccessTokenScopeCategoryRepository), org.DeleteOrgRepos)
			m.Group("/members", func() {
				m.Get("", reqToken(), org.ListMembers)
				m.Combo("/{username}").Get(reqToken(), org.IsMember).
					Delete(reqToken(), reqOrgOwnership(), org.DeleteMember)
			}, reqOrgVisible())
			orgActionsAPI := org.NewAction()
			addActionsManagementRoutes(m, reqOrgOwnership(), orgActionsAPI)
			addActionsReaderRoutes(m, reqOrgMembership(), orgActionsAPI)
			m.Group("/public_members", func() {
				m.Get("", org.ListPublicMembers)
				m.Combo("/{username}").Get(org.IsPublicMember).
					Put(reqToken(), reqOrgMembership(), org.PublicizeMember).
					Delete(reqToken(), reqOrgMembership(), org.ConcealMember)
			}, reqOrgVisible())
			m.Group("/teams", func() {
				m.Get("", org.ListTeams)
				m.Post("", reqOrgOwnership(), bind(api.CreateTeamOption{}), org.CreateTeam)
				m.Get("/search", org.SearchTeam)
			}, reqToken(), reqOrgMembership())
			m.Group("/projects", func() {
				addProjectRoutes(m, reqToken(), reqProjectsUnitAccess(perm.AccessModeWrite))
			}, reqProjectsUnitAccess(perm.AccessModeRead), tokenRequiresScopes(auth_model.AccessTokenScopeCategoryIssue))
			m.Group("/labels", func() {
				m.Get("", org.ListLabels)
				m.Post("", reqToken(), reqOrgOwnership(), bind(api.CreateLabelOption{}), org.CreateLabel)
				m.Combo("/{id}").Get(reqToken(), org.GetLabel).
					Patch(reqToken(), reqOrgOwnership(), bind(api.EditLabelOption{}), org.EditLabel).
					Delete(reqToken(), reqOrgOwnership(), org.DeleteLabel)
			}, reqOrgVisible())
			m.Group("/hooks", func() {
				m.Combo("").Get(org.ListHooks).
					Post(bind(api.CreateHookOption{}), org.CreateHook)
				m.Combo("/{id}").Get(org.GetHook).
					Patch(bind(api.EditHookOption{}), org.EditHook).
					Delete(org.DeleteHook)
			}, reqToken(), reqOrgOwnership(), reqWebhooksEnabled())
			m.Group("/avatar", func() {
				m.Post("", bind(api.UpdateUserAvatarOption{}), org.UpdateAvatar)
				m.Delete("", org.DeleteAvatar)
			}, reqToken(), reqOrgOwnership())
			m.Get("/activities/feeds", org.ListOrgActivityFeeds)

			m.Group("/blocks", func() {
				m.Get("", org.ListBlocks)
				m.Group("/{username}", func() {
					m.Get("", org.CheckUserBlock)
					m.Put("", org.BlockUser)
					m.Delete("", org.UnblockUser)
				})
			}, reqToken(), reqOrgOwnership())
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryOrganization), orgAssignment(true), checkTokenPublicOnly())
		m.Group("/teams/{teamid}", func() {
			m.Combo("").Patch(reqToken(), reqOrgOwnership(), bind(api.EditTeamOption{}), org.EditTeam).
				Delete(reqToken(), reqOrgOwnership(), org.DeleteTeam)
			m.Group("", func() {
				m.Get("", org.GetTeam)
				m.Group("/members", func() {
					m.Get("", reqOrgMembership(), org.GetTeamMembers)
					m.Combo("/{username}").Get(reqOrgMembership(), org.GetTeamMember)
				})
				m.Group("/repos", func() {
					m.Get("", org.GetTeamRepos)
					m.Combo("/{org}/{reponame}").Get(org.GetTeamRepo)
				})
				m.Get("/activities/feeds", org.ListTeamActivityFeeds)
			}, reqTeamReadAccess())
			m.Group("/members", func() {
				m.Combo("/{username}").
					Put(reqToken(), reqOrgOwnership(), org.AddTeamMember).
					Delete(reqToken(), reqOrgOwnership(), org.RemoveTeamMember)
			})
			m.Group("/repos", func() {
				m.Combo("/{org}/{reponame}").
					Put(reqToken(), reqTeamMembership(), org.AddTeamRepository).
					Delete(reqToken(), reqTeamMembership(), org.RemoveTeamRepository)
			})
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryOrganization), orgAssignment(false, true), reqToken(), checkTokenPublicOnly())

		m.Group("/admin", func() {
			m.Group("/cron", func() {
				m.Get("", admin.ListCronTasks)
				m.Post("/{task}", admin.PostCronTask)
			})
			m.Get("/orgs", admin.GetAllOrgs)
			m.Group("/users", func() {
				m.Get("", admin.SearchUsers)
				m.Post("", bind(api.CreateUserOption{}), admin.CreateUser)
				m.Group("/{username}", func() {
					m.Combo("").Patch(bind(api.EditUserOption{}), admin.EditUser).
						Delete(admin.DeleteUser)
					m.Group("/keys", func() {
						m.Post("", bind(api.CreateKeyOption{}), admin.CreatePublicKey)
						m.Delete("/{id}", admin.DeleteUserPublicKey)
					})
					m.Get("/orgs", org.ListUserOrgs)
					m.Post("/orgs", bind(api.CreateOrgOption{}), admin.CreateOrg)
					m.Post("/repos", bind(api.CreateRepoOption{}), admin.CreateRepo)
					m.Post("/rename", bind(api.RenameUserOption{}), admin.RenameUser)
					m.Get("/badges", admin.ListUserBadges)
					m.Post("/badges", bind(api.UserBadgeOption{}), admin.AddUserBadges)
					m.Delete("/badges", bind(api.UserBadgeOption{}), admin.DeleteUserBadges)
				}, context.UserAssignmentAPI())
			})
			m.Group("/emails", func() {
				m.Get("", admin.GetAllEmails)
				m.Get("/search", admin.SearchEmail)
			})
			m.Group("/unadopted", func() {
				m.Get("", admin.ListUnadoptedRepositories)
				m.Post("/{username}/{reponame}", admin.AdoptRepository)
				m.Delete("/{username}/{reponame}", admin.DeleteUnadoptedRepository)
			})
			m.Group("/hooks", func() {
				m.Combo("").Get(admin.ListHooks).
					Post(bind(api.CreateHookOption{}), admin.CreateHook)
				m.Combo("/{id}").Get(admin.GetHook).
					Patch(bind(api.EditHookOption{}), admin.EditHook).
					Delete(admin.DeleteHook)
			})
			m.Group("/actions", func() {
				m.Group("/runners", func() {
					m.Get("", admin.ListRunners)
					m.Post("/registration-token", admin.CreateRegistrationToken)
					m.Get("/{runner_id}", admin.GetRunner)
					m.Delete("/{runner_id}", admin.DeleteRunner)
					m.Patch("/{runner_id}", bind(api.EditActionRunnerOption{}), admin.UpdateRunner)
				})
				m.Get("/runs", admin.ListWorkflowRuns)
				m.Get("/jobs", admin.ListWorkflowJobs)
			})
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryAdmin), reqToken(), reqSiteAdmin())

		m.Group("/topics", func() {
			m.Get("/search", repo.TopicSearch)
		}, tokenRequiresScopes(auth_model.AccessTokenScopeCategoryRepository))
	}, sudo())

	return m
}
