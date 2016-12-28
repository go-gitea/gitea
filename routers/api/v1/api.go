// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1

import (
	"strings"

	"github.com/go-macaron/binding"
	"gopkg.in/macaron.v1"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/admin"
	"code.gitea.io/gitea/routers/api/v1/misc"
	"code.gitea.io/gitea/routers/api/v1/org"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"code.gitea.io/gitea/routers/api/v1/user"
)

func repoAssignment() macaron.Handler {
	return func(ctx *context.APIContext) {
		userName := ctx.Params(":username")
		repoName := ctx.Params(":reponame")

		var (
			owner *models.User
			err   error
		)

		// Check if the user is the same as the repository owner.
		if ctx.IsSigned && ctx.User.LowerName == strings.ToLower(userName) {
			owner = ctx.User
		} else {
			owner, err = models.GetUserByName(userName)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.Status(404)
				} else {
					ctx.Error(500, "GetUserByName", err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner

		// Get repository.
		repo, err := models.GetRepositoryByName(owner.ID, repoName)
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				ctx.Status(404)
			} else {
				ctx.Error(500, "GetRepositoryByName", err)
			}
			return
		} else if err = repo.GetOwner(); err != nil {
			ctx.Error(500, "GetOwner", err)
			return
		}

		if ctx.IsSigned && ctx.User.IsAdmin {
			ctx.Repo.AccessMode = models.AccessModeOwner
		} else {
			mode, err := models.AccessLevel(ctx.User, repo)
			if err != nil {
				ctx.Error(500, "AccessLevel", err)
				return
			}
			ctx.Repo.AccessMode = mode
		}

		if !ctx.Repo.HasAccess() {
			ctx.Status(404)
			return
		}

		ctx.Repo.Repository = repo
	}
}

// Contexter middleware already checks token for user sign in process.
func reqToken() macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsSigned {
			ctx.Error(401)
			return
		}
	}
}

func reqBasicAuth() macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsBasicAuth {
			ctx.Error(401)
			return
		}
	}
}

func reqAdmin() macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsSigned || !ctx.User.IsAdmin {
			ctx.Error(403)
			return
		}
	}
}

func reqRepoWriter() macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.Repo.IsWriter() {
			ctx.Error(403)
			return
		}
	}
}

func orgAssignment(args ...bool) macaron.Handler {
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
			ctx.Org.Organization, err = models.GetUserByName(ctx.Params(":orgname"))
			if err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.Status(404)
				} else {
					ctx.Error(500, "GetUserByName", err)
				}
				return
			}
		}

		if assignTeam {
			ctx.Org.Team, err = models.GetTeamByID(ctx.ParamsInt64(":teamid"))
			if err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.Status(404)
				} else {
					ctx.Error(500, "GetTeamById", err)
				}
				return
			}
		}
	}
}

func mustEnableIssues(ctx *context.APIContext) {
	if !ctx.Repo.Repository.EnableIssues || ctx.Repo.Repository.EnableExternalTracker {
		ctx.Status(404)
		return
	}
}

func mustAllowPulls(ctx *context.Context) {
	if !ctx.Repo.Repository.AllowsPulls() {
		ctx.Status(404)
		return
	}
}

// RegisterRoutes registers all v1 APIs routes to web application.
// FIXME: custom form error response
func RegisterRoutes(m *macaron.Macaron) {
	bind := binding.Bind

	m.Group("/v1", func() {
		// Miscellaneous
		m.Post("/markdown", bind(api.MarkdownOption{}), misc.Markdown)
		m.Post("/markdown/raw", misc.MarkdownRaw)

		// Users
		m.Group("/users", func() {
			m.Get("/search", user.Search)

			m.Group("/:username", func() {
				m.Get("", user.GetInfo)

				m.Group("/tokens", func() {
					m.Combo("").Get(user.ListAccessTokens).
						Post(bind(api.CreateAccessTokenOption{}), user.CreateAccessToken)
				}, reqBasicAuth())
			})
		})

		m.Group("/users", func() {
			m.Group("/:username", func() {
				m.Get("/keys", user.ListPublicKeys)

				m.Get("/followers", user.ListFollowers)
				m.Group("/following", func() {
					m.Get("", user.ListFollowing)
					m.Get("/:target", user.CheckFollowing)
				})

				m.Get("/starred", user.GetStarredRepos)

				m.Get("/subscriptions", user.GetWatchedRepos)
			})
		}, reqToken())

		m.Group("/user", func() {
			m.Get("", user.GetAuthenticatedUser)
			m.Combo("/emails").Get(user.ListEmails).
				Post(bind(api.CreateEmailOption{}), user.AddEmail).
				Delete(bind(api.CreateEmailOption{}), user.DeleteEmail)

			m.Get("/followers", user.ListMyFollowers)
			m.Group("/following", func() {
				m.Get("", user.ListMyFollowing)
				m.Combo("/:username").Get(user.CheckMyFollowing).Put(user.Follow).Delete(user.Unfollow)
			})

			m.Group("/keys", func() {
				m.Combo("").Get(user.ListMyPublicKeys).
					Post(bind(api.CreateKeyOption{}), user.CreatePublicKey)
				m.Combo("/:id").Get(user.GetPublicKey).
					Delete(user.DeletePublicKey)
			})

			m.Group("/starred", func() {
				m.Get("", user.GetMyStarredRepos)
				m.Group("/:username/:reponame", func() {
					m.Get("", user.IsStarring)
					m.Put("", user.Star)
					m.Delete("", user.Unstar)
				}, context.ExtractOwnerAndRepo())
			})

			m.Get("/subscriptions", user.GetMyWatchedRepos)
		}, reqToken())

		// Repositories
		m.Combo("/user/repos", reqToken()).Get(repo.ListMyRepos).
			Post(bind(api.CreateRepoOption{}), repo.Create)
		m.Post("/org/:org/repos", reqToken(), bind(api.CreateRepoOption{}), repo.CreateOrgRepo)

		m.Group("/repos", func() {
			m.Get("/search", repo.Search)
		})

		m.Combo("/repositories/:id", reqToken()).Get(repo.GetByID)

		m.Group("/repos", func() {
			m.Post("/migrate", bind(auth.MigrateRepoForm{}), repo.Migrate)
			m.Combo("/:username/:reponame", context.ExtractOwnerAndRepo()).
				Get(repo.Get).
				Delete(repo.Delete)

			m.Group("/:username/:reponame", func() {
				m.Group("/hooks", func() {
					m.Combo("").Get(repo.ListHooks).
						Post(bind(api.CreateHookOption{}), repo.CreateHook)
					m.Combo("/:id").Get(repo.GetHook).
						Patch(bind(api.EditHookOption{}), repo.EditHook).
						Delete(repo.DeleteHook)
				})
				m.Group("/collaborators", func() {
					m.Get("", repo.ListCollaborators)
					m.Combo("/:collaborator").Get(repo.IsCollaborator).
						Put(bind(api.AddCollaboratorOption{}), repo.AddCollaborator).
						Delete(repo.DeleteCollaborator)
				})
				m.Get("/raw/*", context.RepoRef(), repo.GetRawFile)
				m.Get("/archive/*", repo.GetArchive)
				m.Group("/branches", func() {
					m.Get("", repo.ListBranches)
					m.Get("/:branchname", repo.GetBranch)
				})
				m.Group("/keys", func() {
					m.Combo("").Get(repo.ListDeployKeys).
						Post(bind(api.CreateKeyOption{}), repo.CreateDeployKey)
					m.Combo("/:id").Get(repo.GetDeployKey).
						Delete(repo.DeleteDeploykey)
				})
				m.Group("/issues", func() {
					m.Combo("").Get(repo.ListIssues).Post(bind(api.CreateIssueOption{}), repo.CreateIssue)
					m.Group("/comments", func() {
						m.Get("", repo.ListRepoIssueComments)
						m.Combo("/:id").Patch(bind(api.EditIssueCommentOption{}), repo.EditIssueComment)
					})
					m.Group("/:index", func() {
						m.Combo("").Get(repo.GetIssue).Patch(bind(api.EditIssueOption{}), repo.EditIssue)

						m.Group("/comments", func() {
							m.Combo("").Get(repo.ListIssueComments).Post(bind(api.CreateIssueCommentOption{}), repo.CreateIssueComment)
							m.Combo("/:id").Patch(bind(api.EditIssueCommentOption{}), repo.EditIssueComment).
								Delete(repo.DeleteIssueComment)
						})

						m.Group("/labels", func() {
							m.Combo("").Get(repo.ListIssueLabels).
								Post(bind(api.IssueLabelsOption{}), repo.AddIssueLabels).
								Put(bind(api.IssueLabelsOption{}), repo.ReplaceIssueLabels).
								Delete(repo.ClearIssueLabels)
							m.Delete("/:id", repo.DeleteIssueLabel)
						})

					})
				}, mustEnableIssues)
				m.Group("/labels", func() {
					m.Combo("").Get(repo.ListLabels).
						Post(bind(api.CreateLabelOption{}), repo.CreateLabel)
					m.Combo("/:id").Get(repo.GetLabel).Patch(bind(api.EditLabelOption{}), repo.EditLabel).
						Delete(repo.DeleteLabel)
				})
				m.Group("/milestones", func() {
					m.Combo("").Get(repo.ListMilestones).
						Post(reqRepoWriter(), bind(api.CreateMilestoneOption{}), repo.CreateMilestone)
					m.Combo("/:id").Get(repo.GetMilestone).
						Patch(reqRepoWriter(), bind(api.EditMilestoneOption{}), repo.EditMilestone).
						Delete(reqRepoWriter(), repo.DeleteMilestone)
				})
				m.Group("/subscription", func() {
					m.Get("", user.IsWatching)
					m.Put("", user.Watch)
					m.Delete("", user.Unwatch)
				}, context.ExtractOwnerAndRepo())
				m.Get("/editorconfig/:filename", context.RepoRef(), repo.GetEditorconfig)
				m.Group("/pulls", func() {
					m.Combo("").Get(bind(api.ListPullRequestsOptions{}), repo.ListPullRequests).Post(reqRepoWriter(), bind(api.CreatePullRequestOption{}), repo.CreatePullRequest)
					m.Group("/:index", func() {
						m.Combo("").Get(repo.GetPullRequest).Patch(reqRepoWriter(), bind(api.EditPullRequestOption{}), repo.EditPullRequest)
						m.Combo("/merge").Get(repo.IsPullRequestMerged).Post(reqRepoWriter(), repo.MergePullRequest)
					})

				}, mustAllowPulls, context.ReferencesGitRepo())
			}, repoAssignment())
		}, reqToken())

		// Organizations
		m.Get("/user/orgs", reqToken(), org.ListMyOrgs)
		m.Get("/users/:username/orgs", org.ListUserOrgs)
		m.Group("/orgs/:orgname", func() {
			m.Combo("").Get(org.Get).Patch(bind(api.EditOrgOption{}), org.Edit)
			m.Combo("/teams").Get(org.ListTeams)
			m.Group("/hooks", func() {
				m.Combo("").Get(org.ListHooks).
					Post(bind(api.CreateHookOption{}), org.CreateHook)
				m.Combo("/:id").Get(org.GetHook).
					Patch(bind(api.EditHookOption{}), org.EditHook).
					Delete(org.DeleteHook)
			})
		}, orgAssignment(true))
		m.Group("/teams/:teamid", func() {
			m.Get("", org.GetTeam)
			m.Get("/members", org.GetTeamMembers)
			m.Get("/repos", org.GetTeamRepos)
		}, orgAssignment(false, true))

		m.Any("/*", func(ctx *context.Context) {
			ctx.Error(404)
		})

		m.Group("/admin", func() {
			m.Group("/users", func() {
				m.Post("", bind(api.CreateUserOption{}), admin.CreateUser)

				m.Group("/:username", func() {
					m.Combo("").Patch(bind(api.EditUserOption{}), admin.EditUser).
						Delete(admin.DeleteUser)
					m.Post("/keys", bind(api.CreateKeyOption{}), admin.CreatePublicKey)
					m.Post("/orgs", bind(api.CreateOrgOption{}), admin.CreateOrg)
					m.Post("/repos", bind(api.CreateRepoOption{}), admin.CreateRepo)
				})
			})

			m.Group("/orgs/:orgname", func() {
				m.Group("/teams", func() {
					m.Post("", orgAssignment(true), bind(api.CreateTeamOption{}), admin.CreateTeam)
				})
			})
			m.Group("/teams", func() {
				m.Group("/:teamid", func() {
					m.Combo("").Patch(bind(api.EditTeamOption{}), admin.EditTeam).
						Delete(admin.DeleteTeam)
					m.Combo("/members/:username").Put(admin.AddTeamMember).Delete(admin.RemoveTeamMember)
					m.Combo("/repos/:reponame").Put(admin.AddTeamRepository).Delete(admin.RemoveTeamRepository)
				}, orgAssignment(false, true))
			})
		}, reqAdmin())
	}, context.APIContexter())
}
