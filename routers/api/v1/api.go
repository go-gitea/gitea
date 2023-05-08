// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package v1 Gitea API.
//
// This documentation describes the Gitea API.
//
//	Schemes: http, https
//	BasePath: /api/v1
//	Version: {{AppVer | JSEscape | Safe}}
//	License: MIT http://opensource.org/licenses/MIT
//
//	Consumes:
//	- application/json
//	- text/plain
//
//	Produces:
//	- application/json
//	- text/html
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
//	AccessToken:
//	     type: apiKey
//	     name: access_token
//	     in: query
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
	"fmt"
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/activitypub"
	"code.gitea.io/gitea/routers/api/v1/admin"
	"code.gitea.io/gitea/routers/api/v1/misc"
	"code.gitea.io/gitea/routers/api/v1/notify"
	"code.gitea.io/gitea/routers/api/v1/org"
	"code.gitea.io/gitea/routers/api/v1/packages"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"code.gitea.io/gitea/routers/api/v1/settings"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/services/auth"
	context_service "code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"

	_ "code.gitea.io/gitea/routers/api/v1/swagger" // for swagger generation

	"gitea.com/go-chi/binding"
	"github.com/go-chi/cors"
)

func sudo() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		sudo := ctx.FormString("sudo")
		if len(sudo) == 0 {
			sudo = ctx.Req.Header.Get("Sudo")
		}

		if len(sudo) > 0 {
			if ctx.IsSigned && ctx.Doer.IsAdmin {
				user, err := user_model.GetUserByName(ctx, sudo)
				if err != nil {
					if user_model.IsErrUserNotExist(err) {
						ctx.NotFound()
					} else {
						ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
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
		userName := ctx.Params("username")
		repoName := ctx.Params("reponame")

		var (
			owner *user_model.User
			err   error
		)

		// Check if the user is the same as the repository owner.
		if ctx.IsSigned && ctx.Doer.LowerName == strings.ToLower(userName) {
			owner = ctx.Doer
		} else {
			owner, err = user_model.GetUserByName(ctx, userName)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					if redirectUserID, err := user_model.LookupUserRedirect(userName); err == nil {
						context.RedirectToUser(ctx.Context, userName, redirectUserID)
					} else if user_model.IsErrUserRedirectNotExist(err) {
						ctx.NotFound("GetUserByName", err)
					} else {
						ctx.Error(http.StatusInternalServerError, "LookupUserRedirect", err)
					}
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner
		ctx.ContextUser = owner

		// Get repository.
		repo, err := repo_model.GetRepositoryByName(owner.ID, repoName)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				redirectRepoID, err := repo_model.LookupRedirect(owner.ID, repoName)
				if err == nil {
					context.RedirectToRepo(ctx.Context, redirectRepoID)
				} else if repo_model.IsErrRedirectNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "LookupRepoRedirect", err)
				}
			} else {
				ctx.Error(http.StatusInternalServerError, "GetRepositoryByName", err)
			}
			return
		}

		repo.Owner = owner
		ctx.Repo.Repository = repo

		if ctx.Doer != nil && ctx.Doer.ID == user_model.ActionsUserID {
			taskID := ctx.Data["ActionsTaskID"].(int64)
			task, err := actions_model.GetTaskByID(ctx, taskID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "actions_model.GetTaskByID", err)
				return
			}
			if task.RepoID != repo.ID {
				ctx.NotFound()
				return
			}

			if task.IsForkPullRequest {
				ctx.Repo.Permission.AccessMode = perm.AccessModeRead
			} else {
				ctx.Repo.Permission.AccessMode = perm.AccessModeWrite
			}

			if err := ctx.Repo.Repository.LoadUnits(ctx); err != nil {
				ctx.Error(http.StatusInternalServerError, "LoadUnits", err)
				return
			}
			ctx.Repo.Permission.Units = ctx.Repo.Repository.Units
			ctx.Repo.Permission.UnitsMode = make(map[unit.Type]perm.AccessMode)
			for _, u := range ctx.Repo.Repository.Units {
				ctx.Repo.Permission.UnitsMode[u.Type] = ctx.Repo.Permission.AccessMode
			}
		} else {
			ctx.Repo.Permission, err = access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
				return
			}
		}

		if !ctx.Repo.HasAccess() {
			ctx.NotFound()
			return
		}
	}
}

func reqPackageAccess(accessMode perm.AccessMode) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqPackageAccess", "user should have specific permission or be a site admin")
			return
		}
	}
}

// Contexter middleware already checks token for user sign in process.
func reqToken(requiredScope auth_model.AccessTokenScope) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		// If actions token is present
		if true == ctx.Data["IsActionsToken"] {
			return
		}

		// If OAuth2 token is present
		if _, ok := ctx.Data["ApiTokenScope"]; ctx.Data["IsApiToken"] == true && ok {
			// no scope required
			if requiredScope == "" {
				return
			}

			// check scope
			scope := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
			allow, err := scope.HasScope(requiredScope)
			if err != nil {
				ctx.Error(http.StatusForbidden, "reqToken", "parsing token failed: "+err.Error())
				return
			}
			if allow {
				return
			}

			// if requires 'repo' scope, but only has 'public_repo' scope, allow it only if the repo is public
			if requiredScope == auth_model.AccessTokenScopeRepo {
				if allowPublicRepo, err := scope.HasScope(auth_model.AccessTokenScopePublicRepo); err == nil && allowPublicRepo {
					if ctx.Repo.Repository != nil && !ctx.Repo.Repository.IsPrivate {
						return
					}
				}
			}

			ctx.Error(http.StatusForbidden, "reqToken", "token does not have required scope: "+requiredScope)
			return
		}
		if ctx.Context.IsBasicAuth {
			ctx.CheckForOTP()
			return
		}
		if ctx.IsSigned {
			return
		}
		ctx.Error(http.StatusUnauthorized, "reqToken", "token is required")
	}
}

func reqExploreSignIn() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if setting.Service.Explore.RequireSigninView && !ctx.IsSigned {
			ctx.Error(http.StatusUnauthorized, "reqExploreSignIn", "you must be signed in to search for users")
		}
	}
}

func reqBasicAuth() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Context.IsBasicAuth {
			ctx.Error(http.StatusUnauthorized, "reqBasicAuth", "auth required")
			return
		}
		ctx.CheckForOTP()
	}
}

// reqSiteAdmin user should be the site admin
func reqSiteAdmin() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqSiteAdmin", "user should be the site admin")
			return
		}
	}
}

// reqOwner user should be the owner of the repo or site admin.
func reqOwner() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserRepoOwner() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqOwner", "user should be the owner of the repo")
			return
		}
	}
}

// reqAdmin user should be an owner or a collaborator with admin write of a repository, or site admin
func reqAdmin() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqAdmin", "user should be an owner or a collaborator with admin write of a repository")
			return
		}
	}
}

// reqRepoWriter user should have a permission to write to a repo, or be a site admin
func reqRepoWriter(unitTypes ...unit.Type) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserRepoWriter(unitTypes) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqRepoWriter", "user should have a permission to write to a repo")
			return
		}
	}
}

// reqRepoBranchWriter user should have a permission to write to a branch, or be a site admin
func reqRepoBranchWriter(ctx *context.APIContext) {
	options, ok := web.GetForm(ctx).(api.FileOptionInterface)
	if !ok || (!ctx.Repo.CanWriteToBranch(ctx.Doer, options.Branch()) && !ctx.IsUserSiteAdmin()) {
		ctx.Error(http.StatusForbidden, "reqRepoBranchWriter", "user should have a permission to write to this branch")
		return
	}
}

// reqRepoReader user should have specific read permission or be a repo admin or a site admin
func reqRepoReader(unitType unit.Type) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserRepoReaderSpecific(unitType) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqRepoReader", "user should have specific read permission or be a repo admin or a site admin")
			return
		}
	}
}

// reqAnyRepoReader user should have any permission to read repository or permissions of site admin
func reqAnyRepoReader() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserRepoReaderAny() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqAnyRepoReader", "user should have any permission to read repository or permissions of site admin")
			return
		}
	}
}

// reqOrgOwnership user should be an organization owner, or a site admin
func reqOrgOwnership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.Context.IsUserSiteAdmin() {
			return
		}

		var orgID int64
		if ctx.Org.Organization != nil {
			orgID = ctx.Org.Organization.ID
		} else if ctx.Org.Team != nil {
			orgID = ctx.Org.Team.OrgID
		} else {
			ctx.Error(http.StatusInternalServerError, "", "reqOrgOwnership: unprepared context")
			return
		}

		isOwner, err := organization.IsOrganizationOwner(ctx, orgID, ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationOwner", err)
			return
		} else if !isOwner {
			if ctx.Org.Organization != nil {
				ctx.Error(http.StatusForbidden, "", "Must be an organization owner")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

// reqTeamMembership user should be an team member, or a site admin
func reqTeamMembership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.Context.IsUserSiteAdmin() {
			return
		}
		if ctx.Org.Team == nil {
			ctx.Error(http.StatusInternalServerError, "", "reqTeamMembership: unprepared context")
			return
		}

		orgID := ctx.Org.Team.OrgID
		isOwner, err := organization.IsOrganizationOwner(ctx, orgID, ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationOwner", err)
			return
		} else if isOwner {
			return
		}

		if isTeamMember, err := organization.IsTeamMember(ctx, orgID, ctx.Org.Team.ID, ctx.Doer.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "IsTeamMember", err)
			return
		} else if !isTeamMember {
			isOrgMember, err := organization.IsOrganizationMember(ctx, orgID, ctx.Doer.ID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "IsOrganizationMember", err)
			} else if isOrgMember {
				ctx.Error(http.StatusForbidden, "", "Must be a team member")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

// reqOrgMembership user should be an organization member, or a site admin
func reqOrgMembership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.Context.IsUserSiteAdmin() {
			return
		}

		var orgID int64
		if ctx.Org.Organization != nil {
			orgID = ctx.Org.Organization.ID
		} else if ctx.Org.Team != nil {
			orgID = ctx.Org.Team.OrgID
		} else {
			ctx.Error(http.StatusInternalServerError, "", "reqOrgMembership: unprepared context")
			return
		}

		if isMember, err := organization.IsOrganizationMember(ctx, orgID, ctx.Doer.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationMember", err)
			return
		} else if !isMember {
			if ctx.Org.Organization != nil {
				ctx.Error(http.StatusForbidden, "", "Must be an organization member")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

func reqGitHook() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Doer.CanEditGitHook() {
			ctx.Error(http.StatusForbidden, "", "must be allowed to edit Git hooks")
			return
		}
	}
}

// reqWebhooksEnabled requires webhooks to be enabled by admin.
func reqWebhooksEnabled() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if setting.DisableWebhooks {
			ctx.Error(http.StatusForbidden, "", "webhooks disabled by administrator")
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
			ctx.Org.Organization, err = organization.GetOrgByName(ctx, ctx.Params(":org"))
			if err != nil {
				if organization.IsErrOrgNotExist(err) {
					redirectUserID, err := user_model.LookupUserRedirect(ctx.Params(":org"))
					if err == nil {
						context.RedirectToUser(ctx.Context, ctx.Params(":org"), redirectUserID)
					} else if user_model.IsErrUserRedirectNotExist(err) {
						ctx.NotFound("GetOrgByName", err)
					} else {
						ctx.Error(http.StatusInternalServerError, "LookupUserRedirect", err)
					}
				} else {
					ctx.Error(http.StatusInternalServerError, "GetOrgByName", err)
				}
				return
			}
			ctx.ContextUser = ctx.Org.Organization.AsUser()
		}

		if assignTeam {
			ctx.Org.Team, err = organization.GetTeamByID(ctx, ctx.ParamsInt64(":teamid"))
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "GetTeamById", err)
				}
				return
			}
		}
	}
}

func mustEnableIssues(ctx *context.APIContext) {
	if !ctx.Repo.CanRead(unit.TypeIssues) {
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
		ctx.NotFound()
		return
	}
}

func mustAllowPulls(ctx *context.APIContext) {
	if !(ctx.Repo.Repository.CanEnablePulls() && ctx.Repo.CanRead(unit.TypePullRequests)) {
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
		ctx.NotFound()
		return
	}
}

func mustEnableIssuesOrPulls(ctx *context.APIContext) {
	if !ctx.Repo.CanRead(unit.TypeIssues) &&
		!(ctx.Repo.Repository.CanEnablePulls() && ctx.Repo.CanRead(unit.TypePullRequests)) {
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
		ctx.NotFound()
		return
	}
}

func mustEnableWiki(ctx *context.APIContext) {
	if !(ctx.Repo.CanRead(unit.TypeWiki)) {
		ctx.NotFound()
		return
	}
}

func mustNotBeArchived(ctx *context.APIContext) {
	if ctx.Repo.Repository.IsArchived {
		ctx.NotFound()
		return
	}
}

func mustEnableAttachments(ctx *context.APIContext) {
	if !setting.Attachment.Enabled {
		ctx.NotFound()
		return
	}
}

// bind binding an obj to a func(ctx *context.APIContext)
func bind[T any](_ T) any {
	return func(ctx *context.APIContext) {
		theObj := new(T) // create a new form obj for every request but not use obj directly
		errs := binding.Bind(ctx.Req, theObj)
		if len(errs) > 0 {
			ctx.Error(http.StatusUnprocessableEntity, "validationError", fmt.Sprintf("%s: %s", errs[0].FieldNames, errs[0].Error()))
			return
		}
		web.SetForm(ctx, theObj)
	}
}

// The OAuth2 plugin is expected to be executed first, as it must ignore the user id stored
// in the session (if there is a user id stored in session other plugins might return the user
// object for that id).
//
// The Session plugin is expected to be executed second, in order to skip authentication
// for users that have already signed in.
func buildAuthGroup() *auth.Group {
	group := auth.NewGroup(
		&auth.OAuth2{},
		&auth.HTTPSign{},
		&auth.Basic{}, // FIXME: this should be removed once we don't allow basic auth in API
	)
	specialAdd(group)

	return group
}

// Routes registers all v1 APIs routes to web application.
func Routes(ctx gocontext.Context) *web.Route {
	m := web.NewRoute()

	m.Use(securityHeaders())
	if setting.CORSConfig.Enabled {
		m.Use(cors.Handler(cors.Options{
			// Scheme:           setting.CORSConfig.Scheme, // FIXME: the cors middleware needs scheme option
			AllowedOrigins: setting.CORSConfig.AllowDomain,
			// setting.CORSConfig.AllowSubdomain // FIXME: the cors middleware needs allowSubdomain option
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			AllowedHeaders:   append([]string{"Authorization", "X-Gitea-OTP"}, setting.CORSConfig.Headers...),
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		}))
	}
	m.Use(context.APIContexter())

	group := buildAuthGroup()
	if err := group.Init(ctx); err != nil {
		log.Error("Could not initialize '%s' auth method, error: %s", group.Name(), err)
	}

	// Get user from session if logged in.
	m.Use(auth.APIAuth(group))

	m.Use(auth.VerifyAuthWithOptionsAPI(&auth.VerifyOptions{
		SignInRequired: setting.Service.RequireSignInView,
	}))

	m.Group("", func() {
		// Miscellaneous (no scope required)
		if setting.API.EnableSwagger {
			m.Get("/swagger", func(ctx *context.APIContext) {
				ctx.Redirect(setting.AppSubURL + "/api/swagger")
			})
		}
		m.Get("/version", misc.Version)
		if setting.Federation.Enabled {
			m.Get("/nodeinfo", misc.NodeInfo)
			m.Group("/activitypub", func() {
				// deprecated, remove in 1.20, use /user-id/{user-id} instead
				m.Group("/user/{username}", func() {
					m.Get("", activitypub.Person)
					m.Post("/inbox", activitypub.ReqHTTPSignature(), activitypub.PersonInbox)
				}, context_service.UserAssignmentAPI())
				m.Group("/user-id/{user-id}", func() {
					m.Get("", activitypub.Person)
					m.Post("/inbox", activitypub.ReqHTTPSignature(), activitypub.PersonInbox)
				}, context_service.UserIDAssignmentAPI())
			})
		}
		m.Get("/signing-key.gpg", misc.SigningKey)
		m.Post("/markup", bind(api.MarkupOption{}), misc.Markup)
		m.Post("/markdown", bind(api.MarkdownOption{}), misc.Markdown)
		m.Post("/markdown/raw", misc.MarkdownRaw)
		m.Get("/gitignore/templates", misc.ListGitignoresTemplates)
		m.Get("/gitignore/templates/{name}", misc.GetGitignoreTemplateInfo)
		m.Get("/licenses", misc.ListLicenseTemplates)
		m.Get("/licenses/{name}", misc.GetLicenseTemplateInfo)
		m.Group("/settings", func() {
			m.Get("/ui", settings.GetGeneralUISettings)
			m.Get("/api", settings.GetGeneralAPISettings)
			m.Get("/attachment", settings.GetGeneralAttachmentSettings)
			m.Get("/repository", settings.GetGeneralRepoSettings)
		})

		// Notifications (requires 'notification' scope)
		m.Group("/notifications", func() {
			m.Combo("").
				Get(notify.ListNotifications).
				Put(notify.ReadNotifications)
			m.Get("/new", notify.NewAvailable)
			m.Combo("/threads/{id}").
				Get(notify.GetThread).
				Patch(notify.ReadThread)
		}, reqToken(auth_model.AccessTokenScopeNotification))

		// Users (no scope required)
		m.Group("/users", func() {
			m.Get("/search", reqExploreSignIn(), user.Search)

			m.Group("/{username}", func() {
				m.Get("", reqExploreSignIn(), user.GetInfo)

				if setting.Service.EnableUserHeatmap {
					m.Get("/heatmap", user.GetUserHeatmapData)
				}

				m.Get("/repos", reqExploreSignIn(), user.ListUserRepos)
				m.Group("/tokens", func() {
					m.Combo("").Get(user.ListAccessTokens).
						Post(bind(api.CreateAccessTokenOption{}), user.CreateAccessToken)
					m.Combo("/{id}").Delete(user.DeleteAccessToken)
				}, reqBasicAuth())

				m.Get("/activities/feeds", user.ListUserActivityFeeds)
			}, context_service.UserAssignmentAPI())
		})

		// (no scope required)
		m.Group("/users", func() {
			m.Group("/{username}", func() {
				m.Get("/keys", user.ListPublicKeys)
				m.Get("/gpg_keys", user.ListGPGKeys)

				m.Get("/followers", user.ListFollowers)
				m.Group("/following", func() {
					m.Get("", user.ListFollowing)
					m.Get("/{target}", user.CheckFollowing)
				})

				m.Get("/starred", user.GetStarredRepos)

				m.Get("/subscriptions", user.GetWatchedRepos)
			}, context_service.UserAssignmentAPI())
		}, reqToken(""))

		m.Group("/user", func() {
			m.Get("", user.GetAuthenticatedUser)
			m.Group("/settings", func() {
				m.Get("", reqToken(auth_model.AccessTokenScopeReadUser), user.GetUserSettings)
				m.Patch("", reqToken(auth_model.AccessTokenScopeUser), bind(api.UserSettingsOptions{}), user.UpdateUserSettings)
			})
			m.Combo("/emails").Get(reqToken(auth_model.AccessTokenScopeReadUser), user.ListEmails).
				Post(reqToken(auth_model.AccessTokenScopeUser), bind(api.CreateEmailOption{}), user.AddEmail).
				Delete(reqToken(auth_model.AccessTokenScopeUser), bind(api.DeleteEmailOption{}), user.DeleteEmail)

			m.Get("/followers", user.ListMyFollowers)
			m.Group("/following", func() {
				m.Get("", user.ListMyFollowing)
				m.Group("/{username}", func() {
					m.Get("", user.CheckMyFollowing)
					m.Put("", reqToken(auth_model.AccessTokenScopeUserFollow), user.Follow)      // requires 'user:follow' scope
					m.Delete("", reqToken(auth_model.AccessTokenScopeUserFollow), user.Unfollow) // requires 'user:follow' scope
				}, context_service.UserAssignmentAPI())
			})

			// (admin:public_key scope)
			m.Group("/keys", func() {
				m.Combo("").Get(reqToken(auth_model.AccessTokenScopeReadPublicKey), user.ListMyPublicKeys).
					Post(reqToken(auth_model.AccessTokenScopeWritePublicKey), bind(api.CreateKeyOption{}), user.CreatePublicKey)
				m.Combo("/{id}").Get(reqToken(auth_model.AccessTokenScopeReadPublicKey), user.GetPublicKey).
					Delete(reqToken(auth_model.AccessTokenScopeWritePublicKey), user.DeletePublicKey)
			})

			// (admin:application scope)
			m.Group("/applications", func() {
				m.Combo("/oauth2").
					Get(reqToken(auth_model.AccessTokenScopeReadApplication), user.ListOauth2Applications).
					Post(reqToken(auth_model.AccessTokenScopeWriteApplication), bind(api.CreateOAuth2ApplicationOptions{}), user.CreateOauth2Application)
				m.Combo("/oauth2/{id}").
					Delete(reqToken(auth_model.AccessTokenScopeWriteApplication), user.DeleteOauth2Application).
					Patch(reqToken(auth_model.AccessTokenScopeWriteApplication), bind(api.CreateOAuth2ApplicationOptions{}), user.UpdateOauth2Application).
					Get(reqToken(auth_model.AccessTokenScopeReadApplication), user.GetOauth2Application)
			})

			// (admin:gpg_key scope)
			m.Group("/gpg_keys", func() {
				m.Combo("").Get(reqToken(auth_model.AccessTokenScopeReadGPGKey), user.ListMyGPGKeys).
					Post(reqToken(auth_model.AccessTokenScopeWriteGPGKey), bind(api.CreateGPGKeyOption{}), user.CreateGPGKey)
				m.Combo("/{id}").Get(reqToken(auth_model.AccessTokenScopeReadGPGKey), user.GetGPGKey).
					Delete(reqToken(auth_model.AccessTokenScopeWriteGPGKey), user.DeleteGPGKey)
			})
			m.Get("/gpg_key_token", reqToken(auth_model.AccessTokenScopeReadGPGKey), user.GetVerificationToken)
			m.Post("/gpg_key_verify", reqToken(auth_model.AccessTokenScopeReadGPGKey), bind(api.VerifyGPGKeyOption{}), user.VerifyUserGPGKey)

			// (repo scope)
			m.Combo("/repos", reqToken(auth_model.AccessTokenScopeRepo)).Get(user.ListMyRepos).
				Post(bind(api.CreateRepoOption{}), repo.Create)

			// (repo scope)
			m.Group("/starred", func() {
				m.Get("", user.GetMyStarredRepos)
				m.Group("/{username}/{reponame}", func() {
					m.Get("", user.IsStarring)
					m.Put("", user.Star)
					m.Delete("", user.Unstar)
				}, repoAssignment())
			}, reqToken(auth_model.AccessTokenScopeRepo))
			m.Get("/times", reqToken(auth_model.AccessTokenScopeRepo), repo.ListMyTrackedTimes)
			m.Get("/stopwatches", reqToken(auth_model.AccessTokenScopeRepo), repo.GetStopwatches)
			m.Get("/subscriptions", reqToken(auth_model.AccessTokenScopeRepo), user.GetMyWatchedRepos)
			m.Get("/teams", reqToken(auth_model.AccessTokenScopeRepo), org.ListUserTeams)
			m.Group("/hooks", func() {
				m.Combo("").Get(user.ListHooks).
					Post(bind(api.CreateHookOption{}), user.CreateHook)
				m.Combo("/{id}").Get(user.GetHook).
					Patch(bind(api.EditHookOption{}), user.EditHook).
					Delete(user.DeleteHook)
			}, reqToken(auth_model.AccessTokenScopeAdminUserHook), reqWebhooksEnabled())
		}, reqToken(""))

		// Repositories
		m.Post("/org/{org}/repos", reqToken(auth_model.AccessTokenScopeAdminOrg), bind(api.CreateRepoOption{}), repo.CreateOrgRepoDeprecated)

		m.Combo("/repositories/{id}", reqToken(auth_model.AccessTokenScopeRepo)).Get(repo.GetByID)

		m.Group("/repos", func() {
			m.Get("/search", repo.Search)

			m.Get("/issues/search", repo.SearchIssues)

			// (repo scope)
			m.Post("/migrate", reqToken(auth_model.AccessTokenScopeRepo), bind(api.MigrateRepoOptions{}), repo.Migrate)

			m.Group("/{username}/{reponame}", func() {
				m.Combo("").Get(reqAnyRepoReader(), repo.Get).
					Delete(reqToken(auth_model.AccessTokenScopeDeleteRepo), reqOwner(), repo.Delete).
					Patch(reqToken(auth_model.AccessTokenScopeRepo), reqAdmin(), bind(api.EditRepoOption{}), repo.Edit)
				m.Post("/generate", reqToken(auth_model.AccessTokenScopeRepo), reqRepoReader(unit.TypeCode), bind(api.GenerateRepoOption{}), repo.Generate)
				m.Group("/transfer", func() {
					m.Post("", reqOwner(), bind(api.TransferRepoOption{}), repo.Transfer)
					m.Post("/accept", repo.AcceptTransfer)
					m.Post("/reject", repo.RejectTransfer)
				}, reqToken(auth_model.AccessTokenScopeRepo))
				m.Combo("/notifications", reqToken(auth_model.AccessTokenScopeNotification)).
					Get(notify.ListRepoNotifications).
					Put(notify.ReadRepoNotifications)
				m.Group("/hooks/git", func() {
					m.Combo("").Get(reqToken(auth_model.AccessTokenScopeReadRepoHook), repo.ListGitHooks)
					m.Group("/{id}", func() {
						m.Combo("").Get(reqToken(auth_model.AccessTokenScopeReadRepoHook), repo.GetGitHook).
							Patch(reqToken(auth_model.AccessTokenScopeWriteRepoHook), bind(api.EditGitHookOption{}), repo.EditGitHook).
							Delete(reqToken(auth_model.AccessTokenScopeWriteRepoHook), repo.DeleteGitHook)
					})
				}, reqAdmin(), reqGitHook(), context.ReferencesGitRepo(true))
				m.Group("/hooks", func() {
					m.Combo("").Get(reqToken(auth_model.AccessTokenScopeReadRepoHook), repo.ListHooks).
						Post(reqToken(auth_model.AccessTokenScopeWriteRepoHook), bind(api.CreateHookOption{}), repo.CreateHook)
					m.Group("/{id}", func() {
						m.Combo("").Get(reqToken(auth_model.AccessTokenScopeReadRepoHook), repo.GetHook).
							Patch(reqToken(auth_model.AccessTokenScopeWriteRepoHook), bind(api.EditHookOption{}), repo.EditHook).
							Delete(reqToken(auth_model.AccessTokenScopeWriteRepoHook), repo.DeleteHook)
						m.Post("/tests", reqToken(auth_model.AccessTokenScopeReadRepoHook), context.ReferencesGitRepo(), context.RepoRefForAPI, repo.TestHook)
					})
				}, reqAdmin(), reqWebhooksEnabled())
				m.Group("/collaborators", func() {
					m.Get("", reqAnyRepoReader(), repo.ListCollaborators)
					m.Group("/{collaborator}", func() {
						m.Combo("").Get(reqAnyRepoReader(), repo.IsCollaborator).
							Put(reqAdmin(), bind(api.AddCollaboratorOption{}), repo.AddCollaborator).
							Delete(reqAdmin(), repo.DeleteCollaborator)
						m.Get("/permission", repo.GetRepoPermissions)
					})
				}, reqToken(auth_model.AccessTokenScopeRepo))
				m.Get("/assignees", reqToken(auth_model.AccessTokenScopeRepo), reqAnyRepoReader(), repo.GetAssignees)
				m.Get("/reviewers", reqToken(auth_model.AccessTokenScopeRepo), reqAnyRepoReader(), repo.GetReviewers)
				m.Group("/teams", func() {
					m.Get("", reqAnyRepoReader(), repo.ListTeams)
					m.Combo("/{team}").Get(reqAnyRepoReader(), repo.IsTeam).
						Put(reqAdmin(), repo.AddTeam).
						Delete(reqAdmin(), repo.DeleteTeam)
				}, reqToken(auth_model.AccessTokenScopeRepo))
				m.Get("/raw/*", context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetRawFile)
				m.Get("/media/*", context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetRawFileOrLFS)
				m.Get("/archive/*", reqRepoReader(unit.TypeCode), repo.GetArchive)
				m.Combo("/forks").Get(repo.ListForks).
					Post(reqToken(auth_model.AccessTokenScopeRepo), reqRepoReader(unit.TypeCode), bind(api.CreateForkOption{}), repo.CreateFork)
				m.Group("/branches", func() {
					m.Get("", repo.ListBranches)
					m.Get("/*", repo.GetBranch)
					m.Delete("/*", reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeCode), repo.DeleteBranch)
					m.Post("", reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeCode), bind(api.CreateBranchRepoOption{}), repo.CreateBranch)
				}, context.ReferencesGitRepo(), reqRepoReader(unit.TypeCode))
				m.Group("/branch_protections", func() {
					m.Get("", repo.ListBranchProtections)
					m.Post("", bind(api.CreateBranchProtectionOption{}), repo.CreateBranchProtection)
					m.Group("/{name}", func() {
						m.Get("", repo.GetBranchProtection)
						m.Patch("", bind(api.EditBranchProtectionOption{}), repo.EditBranchProtection)
						m.Delete("", repo.DeleteBranchProtection)
					})
				}, reqToken(auth_model.AccessTokenScopeRepo), reqAdmin())
				m.Group("/tags", func() {
					m.Get("", repo.ListTags)
					m.Get("/*", repo.GetTag)
					m.Post("", reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeCode), bind(api.CreateTagOption{}), repo.CreateTag)
					m.Delete("/*", reqToken(auth_model.AccessTokenScopeRepo), repo.DeleteTag)
				}, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(true))
				m.Group("/keys", func() {
					m.Combo("").Get(repo.ListDeployKeys).
						Post(bind(api.CreateKeyOption{}), repo.CreateDeployKey)
					m.Combo("/{id}").Get(repo.GetDeployKey).
						Delete(repo.DeleteDeploykey)
				}, reqToken(auth_model.AccessTokenScopeRepo), reqAdmin())
				m.Group("/times", func() {
					m.Combo("").Get(repo.ListTrackedTimesByRepository)
					m.Combo("/{timetrackingusername}").Get(repo.ListTrackedTimesByUser)
				}, mustEnableIssues, reqToken(auth_model.AccessTokenScopeRepo))
				m.Group("/wiki", func() {
					m.Combo("/page/{pageName}").
						Get(repo.GetWikiPage).
						Patch(mustNotBeArchived, reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeWiki), bind(api.CreateWikiPageOptions{}), repo.EditWikiPage).
						Delete(mustNotBeArchived, reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeWiki), repo.DeleteWikiPage)
					m.Get("/revisions/{pageName}", repo.ListPageRevisions)
					m.Post("/new", mustNotBeArchived, reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeWiki), bind(api.CreateWikiPageOptions{}), repo.NewWikiPage)
					m.Get("/pages", repo.ListWikiPages)
				}, mustEnableWiki)
				m.Group("/issues", func() {
					m.Combo("").Get(repo.ListIssues).
						Post(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, bind(api.CreateIssueOption{}), repo.CreateIssue)
					m.Group("/comments", func() {
						m.Get("", repo.ListRepoIssueComments)
						m.Group("/{id}", func() {
							m.Combo("").
								Get(repo.GetIssueComment).
								Patch(mustNotBeArchived, reqToken(auth_model.AccessTokenScopeRepo), bind(api.EditIssueCommentOption{}), repo.EditIssueComment).
								Delete(reqToken(auth_model.AccessTokenScopeRepo), repo.DeleteIssueComment)
							m.Combo("/reactions").
								Get(repo.GetIssueCommentReactions).
								Post(reqToken(auth_model.AccessTokenScopeRepo), bind(api.EditReactionOption{}), repo.PostIssueCommentReaction).
								Delete(reqToken(auth_model.AccessTokenScopeRepo), bind(api.EditReactionOption{}), repo.DeleteIssueCommentReaction)
							m.Group("/assets", func() {
								m.Combo("").
									Get(repo.ListIssueCommentAttachments).
									Post(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, repo.CreateIssueCommentAttachment)
								m.Combo("/{asset}").
									Get(repo.GetIssueCommentAttachment).
									Patch(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, bind(api.EditAttachmentOptions{}), repo.EditIssueCommentAttachment).
									Delete(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, repo.DeleteIssueCommentAttachment)
							}, mustEnableAttachments)
						})
					})
					m.Group("/{index}", func() {
						m.Combo("").Get(repo.GetIssue).
							Patch(reqToken(auth_model.AccessTokenScopeRepo), bind(api.EditIssueOption{}), repo.EditIssue).
							Delete(reqToken(auth_model.AccessTokenScopeRepo), reqAdmin(), context.ReferencesGitRepo(), repo.DeleteIssue)
						m.Group("/comments", func() {
							m.Combo("").Get(repo.ListIssueComments).
								Post(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, bind(api.CreateIssueCommentOption{}), repo.CreateIssueComment)
							m.Combo("/{id}", reqToken(auth_model.AccessTokenScopeRepo)).Patch(bind(api.EditIssueCommentOption{}), repo.EditIssueCommentDeprecated).
								Delete(repo.DeleteIssueCommentDeprecated)
						})
						m.Get("/timeline", repo.ListIssueCommentsAndTimeline)
						m.Group("/labels", func() {
							m.Combo("").Get(repo.ListIssueLabels).
								Post(reqToken(auth_model.AccessTokenScopeRepo), bind(api.IssueLabelsOption{}), repo.AddIssueLabels).
								Put(reqToken(auth_model.AccessTokenScopeRepo), bind(api.IssueLabelsOption{}), repo.ReplaceIssueLabels).
								Delete(reqToken(auth_model.AccessTokenScopeRepo), repo.ClearIssueLabels)
							m.Delete("/{id}", reqToken(auth_model.AccessTokenScopeRepo), repo.DeleteIssueLabel)
						})
						m.Group("/times", func() {
							m.Combo("").
								Get(repo.ListTrackedTimes).
								Post(bind(api.AddTimeOption{}), repo.AddTime).
								Delete(repo.ResetIssueTime)
							m.Delete("/{id}", repo.DeleteTime)
						}, reqToken(auth_model.AccessTokenScopeRepo))
						m.Combo("/deadline").Post(reqToken(auth_model.AccessTokenScopeRepo), bind(api.EditDeadlineOption{}), repo.UpdateIssueDeadline)
						m.Group("/stopwatch", func() {
							m.Post("/start", reqToken(auth_model.AccessTokenScopeRepo), repo.StartIssueStopwatch)
							m.Post("/stop", reqToken(auth_model.AccessTokenScopeRepo), repo.StopIssueStopwatch)
							m.Delete("/delete", reqToken(auth_model.AccessTokenScopeRepo), repo.DeleteIssueStopwatch)
						})
						m.Group("/subscriptions", func() {
							m.Get("", repo.GetIssueSubscribers)
							m.Get("/check", reqToken(auth_model.AccessTokenScopeRepo), repo.CheckIssueSubscription)
							m.Put("/{user}", reqToken(auth_model.AccessTokenScopeRepo), repo.AddIssueSubscription)
							m.Delete("/{user}", reqToken(auth_model.AccessTokenScopeRepo), repo.DelIssueSubscription)
						})
						m.Combo("/reactions").
							Get(repo.GetIssueReactions).
							Post(reqToken(auth_model.AccessTokenScopeRepo), bind(api.EditReactionOption{}), repo.PostIssueReaction).
							Delete(reqToken(auth_model.AccessTokenScopeRepo), bind(api.EditReactionOption{}), repo.DeleteIssueReaction)
						m.Group("/assets", func() {
							m.Combo("").
								Get(repo.ListIssueAttachments).
								Post(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, repo.CreateIssueAttachment)
							m.Combo("/{asset}").
								Get(repo.GetIssueAttachment).
								Patch(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, bind(api.EditAttachmentOptions{}), repo.EditIssueAttachment).
								Delete(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, repo.DeleteIssueAttachment)
						}, mustEnableAttachments)
						m.Combo("/dependencies").
							Get(repo.GetIssueDependencies).
							Post(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, bind(api.IssueMeta{}), repo.CreateIssueDependency).
							Delete(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, bind(api.IssueMeta{}), repo.RemoveIssueDependency)
						m.Combo("/blocks").
							Get(repo.GetIssueBlocks).
							Post(reqToken(auth_model.AccessTokenScopeRepo), bind(api.IssueMeta{}), repo.CreateIssueBlocking).
							Delete(reqToken(auth_model.AccessTokenScopeRepo), bind(api.IssueMeta{}), repo.RemoveIssueBlocking)
					})
				}, mustEnableIssuesOrPulls)
				m.Group("/labels", func() {
					m.Combo("").Get(repo.ListLabels).
						Post(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.CreateLabelOption{}), repo.CreateLabel)
					m.Combo("/{id}").Get(repo.GetLabel).
						Patch(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.EditLabelOption{}), repo.EditLabel).
						Delete(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), repo.DeleteLabel)
				})
				m.Post("/markup", reqToken(auth_model.AccessTokenScopeRepo), bind(api.MarkupOption{}), misc.Markup)
				m.Post("/markdown", reqToken(auth_model.AccessTokenScopeRepo), bind(api.MarkdownOption{}), misc.Markdown)
				m.Post("/markdown/raw", reqToken(auth_model.AccessTokenScopeRepo), misc.MarkdownRaw)
				m.Group("/milestones", func() {
					m.Combo("").Get(repo.ListMilestones).
						Post(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.CreateMilestoneOption{}), repo.CreateMilestone)
					m.Combo("/{id}").Get(repo.GetMilestone).
						Patch(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.EditMilestoneOption{}), repo.EditMilestone).
						Delete(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), repo.DeleteMilestone)
				})
				m.Get("/stargazers", repo.ListStargazers)
				m.Get("/subscribers", repo.ListSubscribers)
				m.Group("/subscription", func() {
					m.Get("", user.IsWatching)
					m.Put("", reqToken(auth_model.AccessTokenScopeRepo), user.Watch)
					m.Delete("", reqToken(auth_model.AccessTokenScopeRepo), user.Unwatch)
				})
				m.Group("/releases", func() {
					m.Combo("").Get(repo.ListReleases).
						Post(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeReleases), context.ReferencesGitRepo(), bind(api.CreateReleaseOption{}), repo.CreateRelease)
					m.Combo("/latest").Get(repo.GetLatestRelease)
					m.Group("/{id}", func() {
						m.Combo("").Get(repo.GetRelease).
							Patch(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeReleases), context.ReferencesGitRepo(), bind(api.EditReleaseOption{}), repo.EditRelease).
							Delete(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeReleases), repo.DeleteRelease)
						m.Group("/assets", func() {
							m.Combo("").Get(repo.ListReleaseAttachments).
								Post(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeReleases), repo.CreateReleaseAttachment)
							m.Combo("/{asset}").Get(repo.GetReleaseAttachment).
								Patch(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeReleases), bind(api.EditAttachmentOptions{}), repo.EditReleaseAttachment).
								Delete(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeReleases), repo.DeleteReleaseAttachment)
						})
					})
					m.Group("/tags", func() {
						m.Combo("/{tag}").
							Get(repo.GetReleaseByTag).
							Delete(reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeReleases), repo.DeleteReleaseByTag)
					})
				}, reqRepoReader(unit.TypeReleases))
				m.Post("/mirror-sync", reqToken(auth_model.AccessTokenScopeRepo), reqRepoWriter(unit.TypeCode), repo.MirrorSync)
				m.Post("/push_mirrors-sync", reqAdmin(), reqToken(auth_model.AccessTokenScopeRepo), repo.PushMirrorSync)
				m.Group("/push_mirrors", func() {
					m.Combo("").Get(repo.ListPushMirrors).
						Post(bind(api.CreatePushMirrorOption{}), repo.AddPushMirror)
					m.Combo("/{name}").
						Delete(repo.DeletePushMirrorByRemoteName).
						Get(repo.GetPushMirrorByName)
				}, reqAdmin(), reqToken(auth_model.AccessTokenScopeRepo))

				m.Get("/editorconfig/{filename}", context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetEditorconfig)
				m.Group("/pulls", func() {
					m.Combo("").Get(repo.ListPullRequests).
						Post(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, bind(api.CreatePullRequestOption{}), repo.CreatePullRequest)
					m.Group("/{index}", func() {
						m.Combo("").Get(repo.GetPullRequest).
							Patch(reqToken(auth_model.AccessTokenScopeRepo), bind(api.EditPullRequestOption{}), repo.EditPullRequest)
						m.Get(".{diffType:diff|patch}", repo.DownloadPullDiffOrPatch)
						m.Post("/update", reqToken(auth_model.AccessTokenScopeRepo), repo.UpdatePullRequest)
						m.Get("/commits", repo.GetPullRequestCommits)
						m.Get("/files", repo.GetPullRequestFiles)
						m.Combo("/merge").Get(repo.IsPullRequestMerged).
							Post(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, bind(forms.MergePullRequestForm{}), repo.MergePullRequest).
							Delete(reqToken(auth_model.AccessTokenScopeRepo), mustNotBeArchived, repo.CancelScheduledAutoMerge)
						m.Group("/reviews", func() {
							m.Combo("").
								Get(repo.ListPullReviews).
								Post(reqToken(auth_model.AccessTokenScopeRepo), bind(api.CreatePullReviewOptions{}), repo.CreatePullReview)
							m.Group("/{id}", func() {
								m.Combo("").
									Get(repo.GetPullReview).
									Delete(reqToken(auth_model.AccessTokenScopeRepo), repo.DeletePullReview).
									Post(reqToken(auth_model.AccessTokenScopeRepo), bind(api.SubmitPullReviewOptions{}), repo.SubmitPullReview)
								m.Combo("/comments").
									Get(repo.GetPullReviewComments)
								m.Post("/dismissals", reqToken(auth_model.AccessTokenScopeRepo), bind(api.DismissPullReviewOptions{}), repo.DismissPullReview)
								m.Post("/undismissals", reqToken(auth_model.AccessTokenScopeRepo), repo.UnDismissPullReview)
							})
						})
						m.Combo("/requested_reviewers", reqToken(auth_model.AccessTokenScopeRepo)).
							Delete(bind(api.PullReviewRequestOptions{}), repo.DeleteReviewRequests).
							Post(bind(api.PullReviewRequestOptions{}), repo.CreateReviewRequests)
					})
				}, mustAllowPulls, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo())
				m.Group("/statuses", func() {
					m.Combo("/{sha}").Get(repo.GetCommitStatuses).
						Post(reqToken(auth_model.AccessTokenScopeRepoStatus), reqRepoWriter(unit.TypeCode), bind(api.CreateStatusOption{}), repo.NewCommitStatus)
				}, reqRepoReader(unit.TypeCode))
				m.Group("/commits", func() {
					m.Get("", context.ReferencesGitRepo(), repo.GetAllCommits)
					m.Group("/{ref}", func() {
						m.Get("/status", repo.GetCombinedCommitStatusByRef)
						m.Get("/statuses", repo.GetCommitStatusesByRef)
					}, context.ReferencesGitRepo())
				}, reqRepoReader(unit.TypeCode))
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
				}, context.ReferencesGitRepo(true), reqRepoReader(unit.TypeCode))
				m.Post("/diffpatch", reqRepoWriter(unit.TypeCode), reqToken(auth_model.AccessTokenScopeRepo), bind(api.ApplyDiffPatchFileOptions{}), repo.ApplyDiffPatch)
				m.Group("/contents", func() {
					m.Get("", repo.GetContentsList)
					m.Get("/*", repo.GetContents)
					m.Group("/*", func() {
						m.Post("", bind(api.CreateFileOptions{}), reqRepoBranchWriter, repo.CreateFile)
						m.Put("", bind(api.UpdateFileOptions{}), reqRepoBranchWriter, repo.UpdateFile)
						m.Delete("", bind(api.DeleteFileOptions{}), reqRepoBranchWriter, repo.DeleteFile)
					}, reqToken(auth_model.AccessTokenScopeRepo))
				}, reqRepoReader(unit.TypeCode))
				m.Get("/signing-key.gpg", misc.SigningKey)
				m.Group("/topics", func() {
					m.Combo("").Get(repo.ListTopics).
						Put(reqToken(auth_model.AccessTokenScopeRepo), reqAdmin(), bind(api.RepoTopicOptions{}), repo.UpdateTopics)
					m.Group("/{topic}", func() {
						m.Combo("").Put(reqToken(auth_model.AccessTokenScopeRepo), repo.AddTopic).
							Delete(reqToken(auth_model.AccessTokenScopeRepo), repo.DeleteTopic)
					}, reqAdmin())
				}, reqAnyRepoReader())
				m.Get("/issue_templates", context.ReferencesGitRepo(), repo.GetIssueTemplates)
				m.Get("/issue_config", context.ReferencesGitRepo(), repo.GetIssueConfig)
				m.Get("/issue_config/validate", context.ReferencesGitRepo(), repo.ValidateIssueConfig)
				m.Get("/languages", reqRepoReader(unit.TypeCode), repo.GetLanguages)
				m.Get("/activities/feeds", repo.ListRepoActivityFeeds)
			}, repoAssignment())
		})

		// NOTE: these are Gitea package management API - see packages.CommonRoutes and packages.DockerContainerRoutes for endpoints that implement package manager APIs
		m.Group("/packages/{username}", func() {
			m.Group("/{type}/{name}/{version}", func() {
				m.Get("", reqToken(auth_model.AccessTokenScopeReadPackage), packages.GetPackage)
				m.Delete("", reqToken(auth_model.AccessTokenScopeDeletePackage), reqPackageAccess(perm.AccessModeWrite), packages.DeletePackage)
				m.Get("/files", reqToken(auth_model.AccessTokenScopeReadPackage), packages.ListPackageFiles)
			})
			m.Get("/", reqToken(auth_model.AccessTokenScopeReadPackage), packages.ListPackages)
		}, context_service.UserAssignmentAPI(), context.PackageAssignmentAPI(), reqPackageAccess(perm.AccessModeRead))

		// Organizations
		m.Get("/user/orgs", reqToken(auth_model.AccessTokenScopeReadOrg), org.ListMyOrgs)
		m.Group("/users/{username}/orgs", func() {
			m.Get("", reqToken(auth_model.AccessTokenScopeReadOrg), org.ListUserOrgs)
			m.Get("/{org}/permissions", reqToken(auth_model.AccessTokenScopeReadOrg), org.GetUserOrgsPermissions)
		}, context_service.UserAssignmentAPI())
		m.Post("/orgs", reqToken(auth_model.AccessTokenScopeWriteOrg), bind(api.CreateOrgOption{}), org.Create)
		m.Get("/orgs", org.GetAll)
		m.Group("/orgs/{org}", func() {
			m.Combo("").Get(org.Get).
				Patch(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), bind(api.EditOrgOption{}), org.Edit).
				Delete(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), org.Delete)
			m.Combo("/repos").Get(user.ListOrgRepos).
				Post(reqToken(auth_model.AccessTokenScopeWriteOrg), bind(api.CreateRepoOption{}), repo.CreateOrgRepo)
			m.Group("/members", func() {
				m.Get("", reqToken(auth_model.AccessTokenScopeReadOrg), org.ListMembers)
				m.Combo("/{username}").Get(reqToken(auth_model.AccessTokenScopeReadOrg), org.IsMember).
					Delete(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), org.DeleteMember)
			})
			m.Group("/public_members", func() {
				m.Get("", org.ListPublicMembers)
				m.Combo("/{username}").Get(org.IsPublicMember).
					Put(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgMembership(), org.PublicizeMember).
					Delete(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgMembership(), org.ConcealMember)
			})
			m.Group("/teams", func() {
				m.Get("", reqToken(auth_model.AccessTokenScopeReadOrg), org.ListTeams)
				m.Post("", reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), bind(api.CreateTeamOption{}), org.CreateTeam)
				m.Get("/search", reqToken(auth_model.AccessTokenScopeReadOrg), org.SearchTeam)
			}, reqOrgMembership())
			m.Group("/labels", func() {
				m.Get("", org.ListLabels)
				m.Post("", reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), bind(api.CreateLabelOption{}), org.CreateLabel)
				m.Combo("/{id}").Get(reqToken(auth_model.AccessTokenScopeReadOrg), org.GetLabel).
					Patch(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), bind(api.EditLabelOption{}), org.EditLabel).
					Delete(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), org.DeleteLabel)
			})
			m.Group("/hooks", func() {
				m.Combo("").Get(org.ListHooks).
					Post(bind(api.CreateHookOption{}), org.CreateHook)
				m.Combo("/{id}").Get(org.GetHook).
					Patch(bind(api.EditHookOption{}), org.EditHook).
					Delete(org.DeleteHook)
			}, reqToken(auth_model.AccessTokenScopeAdminOrgHook), reqOrgOwnership(), reqWebhooksEnabled())
			m.Get("/activities/feeds", org.ListOrgActivityFeeds)
		}, orgAssignment(true))
		m.Group("/teams/{teamid}", func() {
			m.Combo("").Get(reqToken(auth_model.AccessTokenScopeReadOrg), org.GetTeam).
				Patch(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), bind(api.EditTeamOption{}), org.EditTeam).
				Delete(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), org.DeleteTeam)
			m.Group("/members", func() {
				m.Get("", reqToken(auth_model.AccessTokenScopeReadOrg), org.GetTeamMembers)
				m.Combo("/{username}").
					Get(reqToken(auth_model.AccessTokenScopeReadOrg), org.GetTeamMember).
					Put(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), org.AddTeamMember).
					Delete(reqToken(auth_model.AccessTokenScopeWriteOrg), reqOrgOwnership(), org.RemoveTeamMember)
			})
			m.Group("/repos", func() {
				m.Get("", reqToken(auth_model.AccessTokenScopeReadOrg), org.GetTeamRepos)
				m.Combo("/{org}/{reponame}").
					Put(reqToken(auth_model.AccessTokenScopeWriteOrg), org.AddTeamRepository).
					Delete(reqToken(auth_model.AccessTokenScopeWriteOrg), org.RemoveTeamRepository).
					Get(reqToken(auth_model.AccessTokenScopeReadOrg), org.GetTeamRepo)
			})
			m.Get("/activities/feeds", org.ListTeamActivityFeeds)
		}, orgAssignment(false, true), reqToken(""), reqTeamMembership())

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
				}, context_service.UserAssignmentAPI())
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
		}, reqToken(auth_model.AccessTokenScopeSudo), reqSiteAdmin())

		m.Group("/topics", func() {
			m.Get("/search", repo.TopicSearch)
		})
	}, sudo())

	return m
}

func securityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// CORB: https://www.chromium.org/Home/chromium-security/corb-for-developers
			// http://stackoverflow.com/a/3146618/244009
			resp.Header().Set("x-content-type-options", "nosniff")
			next.ServeHTTP(resp, req)
		})
	}
}
