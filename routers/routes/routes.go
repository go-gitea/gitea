// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"os"
	"path"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/admin"
	apiv1 "code.gitea.io/gitea/routers/api/v1"
	"code.gitea.io/gitea/routers/dev"
	"code.gitea.io/gitea/routers/org"
	"code.gitea.io/gitea/routers/private"
	"code.gitea.io/gitea/routers/repo"
	"code.gitea.io/gitea/routers/user"

	"github.com/go-macaron/binding"
	"github.com/go-macaron/cache"
	"github.com/go-macaron/captcha"
	"github.com/go-macaron/csrf"
	"github.com/go-macaron/gzip"
	"github.com/go-macaron/i18n"
	"github.com/go-macaron/session"
	"github.com/go-macaron/toolbox"
	"gopkg.in/macaron.v1"
)

// NewMacaron initializes Macaron instance.
func NewMacaron() *macaron.Macaron {
	m := macaron.New()
	if !setting.DisableRouterLog {
		m.Use(macaron.Logger())
	}
	m.Use(macaron.Recovery())
	if setting.EnableGzip {
		m.Use(gzip.Gziper())
	}
	if setting.Protocol == setting.FCGI {
		m.SetURLPrefix(setting.AppSubURL)
	}
	m.Use(public.Custom(
		&public.Options{
			SkipLogging: setting.DisableRouterLog,
		},
	))
	m.Use(public.Static(
		&public.Options{
			Directory:   path.Join(setting.StaticRootPath, "public"),
			SkipLogging: setting.DisableRouterLog,
		},
	))
	m.Use(macaron.Static(
		setting.AvatarUploadPath,
		macaron.StaticOptions{
			Prefix:      "avatars",
			SkipLogging: setting.DisableRouterLog,
			ETag:        true,
		},
	))

	m.Use(templates.Renderer())
	models.InitMailRender(templates.Mailer())

	localeNames, err := options.Dir("locale")

	if err != nil {
		log.Fatal(4, "Failed to list locale files: %v", err)
	}

	localFiles := make(map[string][]byte)

	for _, name := range localeNames {
		localFiles[name], err = options.Locale(name)

		if err != nil {
			log.Fatal(4, "Failed to load %s locale file. %v", name, err)
		}
	}

	m.Use(i18n.I18n(i18n.Options{
		SubURL:      setting.AppSubURL,
		Files:       localFiles,
		Langs:       setting.Langs,
		Names:       setting.Names,
		DefaultLang: "en-US",
		Redirect:    true,
	}))
	m.Use(cache.Cacher(cache.Options{
		Adapter:       setting.CacheAdapter,
		AdapterConfig: setting.CacheConn,
		Interval:      setting.CacheInterval,
	}))
	m.Use(captcha.Captchaer(captcha.Options{
		SubURL: setting.AppSubURL,
	}))
	m.Use(session.Sessioner(setting.SessionConfig))
	m.Use(csrf.Csrfer(csrf.Options{
		Secret:     setting.SecretKey,
		Cookie:     setting.CSRFCookieName,
		SetCookie:  true,
		Header:     "X-Csrf-Token",
		CookiePath: setting.AppSubURL,
	}))
	m.Use(toolbox.Toolboxer(m, toolbox.Options{
		HealthCheckFuncs: []*toolbox.HealthCheckFuncDesc{
			{
				Desc: "Database connection",
				Func: models.Ping,
			},
		},
	}))
	m.Use(context.Contexter())
	return m
}

// RegisterRoutes routes routes to Macaron
func RegisterRoutes(m *macaron.Macaron) {
	reqSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: true})
	ignSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: setting.Service.RequireSignInView})
	ignSignInAndCsrf := context.Toggle(&context.ToggleOptions{DisableCSRF: true})
	reqSignOut := context.Toggle(&context.ToggleOptions{SignOutRequired: true})

	bindIgnErr := binding.BindIgnErr
	validation.AddBindingRules()

	openIDSignInEnabled := func(ctx *context.Context) {
		if !setting.Service.EnableOpenIDSignIn {
			ctx.Error(403)
			return
		}
	}

	openIDSignUpEnabled := func(ctx *context.Context) {
		if !setting.Service.EnableOpenIDSignUp {
			ctx.Error(403)
			return
		}
	}

	m.Use(user.GetNotificationCount)

	// FIXME: not all routes need go through same middlewares.
	// Especially some AJAX requests, we can reduce middleware number to improve performance.
	// Routers.
	// for health check
	m.Head("/", func() string {
		return ""
	})
	m.Get("/", ignSignIn, routers.Home)
	m.Get("/swagger", ignSignIn, routers.Swagger)
	m.Group("/explore", func() {
		m.Get("", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL + "/explore/repos")
		})
		m.Get("/repos", routers.ExploreRepos)
		m.Get("/users", routers.ExploreUsers)
		m.Get("/organizations", routers.ExploreOrganizations)
	}, ignSignIn)
	m.Combo("/install", routers.InstallInit).Get(routers.Install).
		Post(bindIgnErr(auth.InstallForm{}), routers.InstallPost)
	m.Get("/^:type(issues|pulls)$", reqSignIn, user.Issues)

	// ***** START: User *****
	m.Group("/user", func() {
		m.Get("/login", user.SignIn)
		m.Post("/login", bindIgnErr(auth.SignInForm{}), user.SignInPost)
		m.Group("", func() {
			m.Combo("/login/openid").
				Get(user.SignInOpenID).
				Post(bindIgnErr(auth.SignInOpenIDForm{}), user.SignInOpenIDPost)
		}, openIDSignInEnabled)
		m.Group("/openid", func() {
			m.Combo("/connect").
				Get(user.ConnectOpenID).
				Post(bindIgnErr(auth.ConnectOpenIDForm{}), user.ConnectOpenIDPost)
			m.Group("/register", func() {
				m.Combo("").
					Get(user.RegisterOpenID, openIDSignUpEnabled).
					Post(bindIgnErr(auth.SignUpOpenIDForm{}), user.RegisterOpenIDPost)
			}, openIDSignUpEnabled)
		}, openIDSignInEnabled)
		m.Get("/sign_up", user.SignUp)
		m.Post("/sign_up", bindIgnErr(auth.RegisterForm{}), user.SignUpPost)
		m.Get("/reset_password", user.ResetPasswd)
		m.Post("/reset_password", user.ResetPasswdPost)
		m.Group("/oauth2", func() {
			m.Get("/:provider", user.SignInOAuth)
			m.Get("/:provider/callback", user.SignInOAuthCallback)
		})
		m.Get("/link_account", user.LinkAccount)
		m.Post("/link_account_signin", bindIgnErr(auth.SignInForm{}), user.LinkAccountPostSignIn)
		m.Post("/link_account_signup", bindIgnErr(auth.RegisterForm{}), user.LinkAccountPostRegister)
		m.Group("/two_factor", func() {
			m.Get("", user.TwoFactor)
			m.Post("", bindIgnErr(auth.TwoFactorAuthForm{}), user.TwoFactorPost)
			m.Get("/scratch", user.TwoFactorScratch)
			m.Post("/scratch", bindIgnErr(auth.TwoFactorScratchAuthForm{}), user.TwoFactorScratchPost)
		})
	}, reqSignOut)

	m.Group("/user/settings", func() {
		m.Get("", user.Settings)
		m.Post("", bindIgnErr(auth.UpdateProfileForm{}), user.SettingsPost)
		m.Combo("/avatar").Get(user.SettingsAvatar).
			Post(binding.MultipartForm(auth.AvatarForm{}), user.SettingsAvatarPost)
		m.Post("/avatar/delete", user.SettingsDeleteAvatar)
		m.Combo("/email").Get(user.SettingsEmails).
			Post(bindIgnErr(auth.AddEmailForm{}), user.SettingsEmailPost)
		m.Post("/email/delete", user.DeleteEmail)
		m.Get("/password", user.SettingsPassword)
		m.Post("/password", bindIgnErr(auth.ChangePasswordForm{}), user.SettingsPasswordPost)
		m.Group("/openid", func() {
			m.Combo("").Get(user.SettingsOpenID).
				Post(bindIgnErr(auth.AddOpenIDForm{}), user.SettingsOpenIDPost)
			m.Post("/delete", user.DeleteOpenID)
			m.Post("/toggle_visibility", user.ToggleOpenIDVisibility)
		}, openIDSignInEnabled)
		m.Combo("/keys").Get(user.SettingsKeys).
			Post(bindIgnErr(auth.AddKeyForm{}), user.SettingsKeysPost)
		m.Post("/keys/delete", user.DeleteKey)
		m.Combo("/applications").Get(user.SettingsApplications).
			Post(bindIgnErr(auth.NewAccessTokenForm{}), user.SettingsApplicationsPost)
		m.Post("/applications/delete", user.SettingsDeleteApplication)
		m.Route("/delete", "GET,POST", user.SettingsDelete)
		m.Combo("/account_link").Get(user.SettingsAccountLinks).Post(user.SettingsDeleteAccountLink)
		m.Get("/organization", user.SettingsOrganization)
		m.Get("/repos", user.SettingsRepos)
		m.Group("/two_factor", func() {
			m.Get("", user.SettingsTwoFactor)
			m.Post("/regenerate_scratch", user.SettingsTwoFactorRegenerateScratch)
			m.Post("/disable", user.SettingsTwoFactorDisable)
			m.Get("/enroll", user.SettingsTwoFactorEnroll)
			m.Post("/enroll", bindIgnErr(auth.TwoFactorAuthForm{}), user.SettingsTwoFactorEnrollPost)
		})
	}, reqSignIn, func(ctx *context.Context) {
		ctx.Data["PageIsUserSettings"] = true
	})

	m.Group("/user", func() {
		// r.Get("/feeds", binding.Bind(auth.FeedsForm{}), user.Feeds)
		m.Any("/activate", user.Activate)
		m.Any("/activate_email", user.ActivateEmail)
		m.Get("/email2user", user.Email2User)
		m.Get("/forgot_password", user.ForgotPasswd)
		m.Post("/forgot_password", user.ForgotPasswdPost)
		m.Get("/logout", user.SignOut)
	})
	// ***** END: User *****

	adminReq := context.Toggle(&context.ToggleOptions{SignInRequired: true, AdminRequired: true})

	// ***** START: Admin *****
	m.Group("/admin", func() {
		m.Get("", adminReq, admin.Dashboard)
		m.Get("/config", admin.Config)
		m.Post("/config/test_mail", admin.SendTestMail)
		m.Get("/monitor", admin.Monitor)

		m.Group("/users", func() {
			m.Get("", admin.Users)
			m.Combo("/new").Get(admin.NewUser).Post(bindIgnErr(auth.AdminCreateUserForm{}), admin.NewUserPost)
			m.Combo("/:userid").Get(admin.EditUser).Post(bindIgnErr(auth.AdminEditUserForm{}), admin.EditUserPost)
			m.Post("/:userid/delete", admin.DeleteUser)
		})

		m.Group("/orgs", func() {
			m.Get("", admin.Organizations)
		})

		m.Group("/repos", func() {
			m.Get("", admin.Repos)
			m.Post("/delete", admin.DeleteRepo)
		})

		m.Group("/auths", func() {
			m.Get("", admin.Authentications)
			m.Combo("/new").Get(admin.NewAuthSource).Post(bindIgnErr(auth.AuthenticationForm{}), admin.NewAuthSourcePost)
			m.Combo("/:authid").Get(admin.EditAuthSource).
				Post(bindIgnErr(auth.AuthenticationForm{}), admin.EditAuthSourcePost)
			m.Post("/:authid/delete", admin.DeleteAuthSource)
		})

		m.Group("/notices", func() {
			m.Get("", admin.Notices)
			m.Post("/delete", admin.DeleteNotices)
			m.Get("/empty", admin.EmptyNotices)
		})
	}, adminReq)
	// ***** END: Admin *****

	m.Group("", func() {
		m.Group("/:username", func() {
			m.Get("", user.Profile)
			m.Get("/followers", user.Followers)
			m.Get("/following", user.Following)
		})

		m.Get("/attachments/:uuid", func(ctx *context.Context) {
			attach, err := models.GetAttachmentByUUID(ctx.Params(":uuid"))
			if err != nil {
				if models.IsErrAttachmentNotExist(err) {
					ctx.Error(404)
				} else {
					ctx.Handle(500, "GetAttachmentByUUID", err)
				}
				return
			}

			fr, err := os.Open(attach.LocalPath())
			if err != nil {
				ctx.Handle(500, "Open", err)
				return
			}
			defer fr.Close()

			if err := attach.IncreaseDownloadCount(); err != nil {
				ctx.Handle(500, "Update", err)
				return
			}

			if err = repo.ServeData(ctx, attach.Name, fr); err != nil {
				ctx.Handle(500, "ServeData", err)
				return
			}
		})
		m.Post("/attachments", repo.UploadAttachment)
	}, ignSignIn)

	m.Group("/:username", func() {
		m.Get("/action/:action", user.Action)
	}, reqSignIn)

	if macaron.Env == macaron.DEV {
		m.Get("/template/*", dev.TemplatePreview)
	}

	reqRepoAdmin := context.RequireRepoAdmin()
	reqRepoWriter := context.RequireRepoWriter()

	// ***** START: Organization *****
	m.Group("/org", func() {
		m.Group("", func() {
			m.Get("/create", org.Create)
			m.Post("/create", bindIgnErr(auth.CreateOrgForm{}), org.CreatePost)
		}, func(ctx *context.Context) {
			if !ctx.User.CanCreateOrganization() {
				ctx.NotFound()
			}
		})

		m.Group("/:org", func() {
			m.Get("/dashboard", user.Dashboard)
			m.Get("/^:type(issues|pulls)$", user.Issues)
			m.Get("/members", org.Members)
			m.Get("/members/action/:action", org.MembersAction)

			m.Get("/teams", org.Teams)
		}, context.OrgAssignment(true))

		m.Group("/:org", func() {
			m.Get("/teams/:team", org.TeamMembers)
			m.Get("/teams/:team/repositories", org.TeamRepositories)
			m.Route("/teams/:team/action/:action", "GET,POST", org.TeamsAction)
			m.Route("/teams/:team/action/repo/:action", "GET,POST", org.TeamsRepoAction)
		}, context.OrgAssignment(true, false, true))

		m.Group("/:org", func() {
			m.Get("/teams/new", org.NewTeam)
			m.Post("/teams/new", bindIgnErr(auth.CreateTeamForm{}), org.NewTeamPost)
			m.Get("/teams/:team/edit", org.EditTeam)
			m.Post("/teams/:team/edit", bindIgnErr(auth.CreateTeamForm{}), org.EditTeamPost)
			m.Post("/teams/:team/delete", org.DeleteTeam)

			m.Group("/settings", func() {
				m.Combo("").Get(org.Settings).
					Post(bindIgnErr(auth.UpdateOrgSettingForm{}), org.SettingsPost)
				m.Post("/avatar", binding.MultipartForm(auth.AvatarForm{}), org.SettingsAvatar)
				m.Post("/avatar/delete", org.SettingsDeleteAvatar)

				m.Group("/hooks", func() {
					m.Get("", org.Webhooks)
					m.Post("/delete", org.DeleteWebhook)
					m.Get("/:type/new", repo.WebhooksNew)
					m.Post("/gitea/new", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksNewPost)
					m.Post("/gogs/new", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksNewPost)
					m.Post("/slack/new", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksNewPost)
					m.Post("/discord/new", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
					m.Get("/:id", repo.WebHooksEdit)
					m.Post("/gitea/:id", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksEditPost)
					m.Post("/gogs/:id", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksEditPost)
					m.Post("/slack/:id", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksEditPost)
					m.Post("/discord/:id", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
				})

				m.Route("/delete", "GET,POST", org.SettingsDelete)
			})

			m.Route("/invitations/new", "GET,POST", org.Invitation)
		}, context.OrgAssignment(true, true))
	}, reqSignIn)
	// ***** END: Organization *****

	// ***** START: Repository *****
	m.Group("/repo", func() {
		m.Get("/create", repo.Create)
		m.Post("/create", bindIgnErr(auth.CreateRepoForm{}), repo.CreatePost)
		m.Get("/migrate", repo.Migrate)
		m.Post("/migrate", bindIgnErr(auth.MigrateRepoForm{}), repo.MigratePost)
		m.Combo("/fork/:repoid").Get(repo.Fork).
			Post(bindIgnErr(auth.CreateRepoForm{}), repo.ForkPost)
	}, reqSignIn)

	m.Group("/:username/:reponame", func() {
		m.Group("/settings", func() {
			m.Combo("").Get(repo.Settings).
				Post(bindIgnErr(auth.RepoSettingForm{}), repo.SettingsPost)
			m.Group("/collaboration", func() {
				m.Combo("").Get(repo.Collaboration).Post(repo.CollaborationPost)
				m.Post("/access_mode", repo.ChangeCollaborationAccessMode)
				m.Post("/delete", repo.DeleteCollaboration)
			})
			m.Group("/branches", func() {
				m.Combo("").Get(repo.ProtectedBranch).Post(repo.ProtectedBranchPost)
				m.Combo("/*").Get(repo.SettingsProtectedBranch).
					Post(bindIgnErr(auth.ProtectBranchForm{}), repo.SettingsProtectedBranchPost)
			}, repo.MustBeNotBare)

			m.Group("/hooks", func() {
				m.Get("", repo.Webhooks)
				m.Post("/delete", repo.DeleteWebhook)
				m.Get("/:type/new", repo.WebhooksNew)
				m.Post("/gitea/new", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksNewPost)
				m.Post("/gogs/new", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksNewPost)
				m.Post("/slack/new", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksNewPost)
				m.Post("/discord/new", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
				m.Get("/:id", repo.WebHooksEdit)
				m.Post("/:id/test", repo.TestWebhook)
				m.Post("/gitea/:id", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksEditPost)
				m.Post("/gogs/:id", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksNewPost)
				m.Post("/slack/:id", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksEditPost)
				m.Post("/discord/:id", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksEditPost)

				m.Group("/git", func() {
					m.Get("", repo.GitHooks)
					m.Combo("/:name").Get(repo.GitHooksEdit).
						Post(repo.GitHooksEditPost)
				}, context.GitHookService())
			})

			m.Group("/keys", func() {
				m.Combo("").Get(repo.DeployKeys).
					Post(bindIgnErr(auth.AddKeyForm{}), repo.DeployKeysPost)
				m.Post("/delete", repo.DeleteDeployKey)
			})

		}, func(ctx *context.Context) {
			ctx.Data["PageIsSettings"] = true
		})
	}, reqSignIn, context.RepoAssignment(), reqRepoAdmin, context.UnitTypes(), context.LoadRepoUnits(), context.RepoRef())

	m.Get("/:username/:reponame/action/:action", reqSignIn, context.RepoAssignment(), repo.Action)

	m.Group("/:username/:reponame", func() {
		// FIXME: should use different URLs but mostly same logic for comments of issue and pull reuqest.
		// So they can apply their own enable/disable logic on routers.
		m.Group("/issues", func() {
			m.Combo("/new", repo.MustEnableIssues).Get(context.RepoRef(), repo.NewIssue).
				Post(bindIgnErr(auth.CreateIssueForm{}), repo.NewIssuePost)

			m.Group("/:index", func() {
				m.Post("/title", repo.UpdateIssueTitle)
				m.Post("/content", repo.UpdateIssueContent)
				m.Post("/watch", repo.IssueWatch)
				m.Combo("/comments").Post(bindIgnErr(auth.CreateCommentForm{}), repo.NewComment)
				m.Group("/times", func() {
					m.Post("/add", bindIgnErr(auth.AddTimeManuallyForm{}), repo.AddTimeManually)
					m.Group("/stopwatch", func() {
						m.Post("/toggle", repo.IssueStopwatch)
						m.Post("/cancel", repo.CancelStopwatch)
					})

				}, func(ctx *context.Context) {
					if !ctx.Repo.CanUseTimetracker(repo.GetActionIssue(ctx), ctx.User) {
						ctx.Handle(404, ctx.Req.RequestURI, nil)
						return
					}
				})
			})

			m.Post("/labels", repo.UpdateIssueLabel, reqRepoWriter)
			m.Post("/milestone", repo.UpdateIssueMilestone, reqRepoWriter)
			m.Post("/assignee", repo.UpdateIssueAssignee, reqRepoWriter)
			m.Post("/status", repo.UpdateIssueStatus, reqRepoWriter)
		}, context.CheckUnit(models.UnitTypeIssues))
		m.Group("/comments/:id", func() {
			m.Post("", repo.UpdateCommentContent)
			m.Post("/delete", repo.DeleteComment)
		}, context.CheckUnit(models.UnitTypeIssues))
		m.Group("/labels", func() {
			m.Post("/new", bindIgnErr(auth.CreateLabelForm{}), repo.NewLabel)
			m.Post("/edit", bindIgnErr(auth.CreateLabelForm{}), repo.UpdateLabel)
			m.Post("/delete", repo.DeleteLabel)
			m.Post("/initialize", bindIgnErr(auth.InitializeLabelsForm{}), repo.InitializeLabels)
		}, reqRepoWriter, context.RepoRef(), context.CheckUnit(models.UnitTypeIssues))
		m.Group("/milestones", func() {
			m.Combo("/new").Get(repo.NewMilestone).
				Post(bindIgnErr(auth.CreateMilestoneForm{}), repo.NewMilestonePost)
			m.Get("/:id/edit", repo.EditMilestone)
			m.Post("/:id/edit", bindIgnErr(auth.CreateMilestoneForm{}), repo.EditMilestonePost)
			m.Get("/:id/:action", repo.ChangeMilestonStatus)
			m.Post("/delete", repo.DeleteMilestone)
		}, reqRepoWriter, context.RepoRef(), context.CheckUnit(models.UnitTypeIssues))

		m.Combo("/compare/*", repo.MustAllowPulls, repo.SetEditorconfigIfExists).
			Get(repo.CompareAndPullRequest).
			Post(bindIgnErr(auth.CreateIssueForm{}), repo.CompareAndPullRequestPost)

		m.Group("", func() {
			m.Combo("/_edit/*").Get(repo.EditFile).
				Post(bindIgnErr(auth.EditRepoFileForm{}), repo.EditFilePost)
			m.Combo("/_new/*").Get(repo.NewFile).
				Post(bindIgnErr(auth.EditRepoFileForm{}), repo.NewFilePost)
			m.Post("/_preview/*", bindIgnErr(auth.EditPreviewDiffForm{}), repo.DiffPreviewPost)
			m.Combo("/_delete/*").Get(repo.DeleteFile).
				Post(bindIgnErr(auth.DeleteRepoFileForm{}), repo.DeleteFilePost)

			m.Group("", func() {
				m.Combo("/_upload/*").Get(repo.UploadFile).
					Post(bindIgnErr(auth.UploadRepoFileForm{}), repo.UploadFilePost)
				m.Post("/upload-file", repo.UploadFileToServer)
				m.Post("/upload-remove", bindIgnErr(auth.RemoveUploadFileForm{}), repo.RemoveUploadFileFromServer)
			}, func(ctx *context.Context) {
				if !setting.Repository.Upload.Enabled {
					ctx.Handle(404, "", nil)
					return
				}
			})
		}, repo.MustBeNotBare, reqRepoWriter, context.RepoRef(), func(ctx *context.Context) {
			if !ctx.Repo.Repository.CanEnableEditor() || ctx.Repo.IsViewCommit {
				ctx.Handle(404, "", nil)
				return
			}
		})
	}, reqSignIn, context.RepoAssignment(), context.UnitTypes(), context.LoadRepoUnits())

	// Releases
	m.Group("/:username/:reponame", func() {
		m.Group("/releases", func() {
			m.Get("/", repo.MustBeNotBare, repo.Releases)
		}, repo.MustBeNotBare, context.RepoRef())
		m.Group("/releases", func() {
			m.Get("/new", repo.NewRelease)
			m.Post("/new", bindIgnErr(auth.NewReleaseForm{}), repo.NewReleasePost)
			m.Post("/delete", repo.DeleteRelease)
		}, reqSignIn, repo.MustBeNotBare, reqRepoWriter, context.RepoRef())
		m.Group("/releases", func() {
			m.Get("/edit/*", repo.EditRelease)
			m.Post("/edit/*", bindIgnErr(auth.EditReleaseForm{}), repo.EditReleasePost)
		}, reqSignIn, repo.MustBeNotBare, reqRepoWriter, func(ctx *context.Context) {
			var err error
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
			if err != nil {
				ctx.Handle(500, "GetBranchCommit", err)
				return
			}
			ctx.Repo.CommitsCount, err = ctx.Repo.Commit.CommitsCount()
			if err != nil {
				ctx.Handle(500, "CommitsCount", err)
				return
			}
			ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
		})
	}, context.RepoAssignment(), context.UnitTypes(), context.LoadRepoUnits(), context.CheckUnit(models.UnitTypeReleases))

	m.Group("/:username/:reponame", func() {
		m.Group("", func() {
			m.Get("/^:type(issues|pulls)$", repo.RetrieveLabels, repo.Issues)
			m.Get("/^:type(issues|pulls)$/:index", repo.ViewIssue)
			m.Get("/labels/", repo.RetrieveLabels, repo.Labels)
			m.Get("/milestones", repo.Milestones)
		}, context.RepoRef())

		m.Group("/wiki", func() {
			m.Get("/?:page", repo.Wiki)
			m.Get("/_pages", repo.WikiPages)

			m.Group("", func() {
				m.Combo("/_new").Get(repo.NewWiki).
					Post(bindIgnErr(auth.NewWikiForm{}), repo.NewWikiPost)
				m.Combo("/:page/_edit").Get(repo.EditWiki).
					Post(bindIgnErr(auth.NewWikiForm{}), repo.EditWikiPost)
				m.Post("/:page/delete", repo.DeleteWikiPagePost)
			}, reqSignIn, reqRepoWriter)
		}, repo.MustEnableWiki, context.RepoRef(), context.CheckUnit(models.UnitTypeWiki))

		m.Group("/wiki", func() {
			m.Get("/raw/*", repo.WikiRaw)
			m.Get("/*", repo.WikiRaw)
		}, repo.MustEnableWiki, context.CheckUnit(models.UnitTypeWiki), context.CheckUnit(models.UnitTypeWiki))

		m.Get("/archive/*", repo.MustBeNotBare, context.CheckUnit(models.UnitTypeCode), repo.Download)

		m.Group("/pulls/:index", func() {
			m.Get("/commits", context.RepoRef(), repo.ViewPullCommits)
			m.Get("/files", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.ViewPullFiles)
			m.Post("/merge", reqRepoWriter, repo.MergePullRequest)
			m.Post("/cleanup", context.RepoRef(), repo.CleanUpPullRequest)
		}, repo.MustAllowPulls, context.CheckUnit(models.UnitTypePullRequests))

		m.Group("", func() {
			m.Get("/raw/*", repo.SingleDownload)
			m.Get("/commits/*", repo.RefCommits)
			m.Get("/graph", repo.Graph)
			m.Get("/commit/:sha([a-f0-9]{7,40})$", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.Diff)
		}, repo.MustBeNotBare, context.RepoRef(), context.CheckUnit(models.UnitTypeCode))

		m.Group("", func() {
			m.Get("/src/*", repo.SetEditorconfigIfExists, repo.Home)
			m.Get("/forks", repo.Forks)
		}, context.RepoRef(), context.CheckUnit(models.UnitTypeCode))
		m.Get("/commit/:sha([a-f0-9]{7,40})\\.:ext(patch|diff)",
			repo.MustBeNotBare, context.CheckUnit(models.UnitTypeCode), repo.RawDiff)

		m.Get("/compare/:before([a-z0-9]{40})\\.\\.\\.:after([a-z0-9]{40})", repo.SetEditorconfigIfExists,
			repo.SetDiffViewStyle, repo.MustBeNotBare, context.CheckUnit(models.UnitTypeCode), repo.CompareDiff)
	}, ignSignIn, context.RepoAssignment(), context.UnitTypes(), context.LoadRepoUnits())
	m.Group("/:username/:reponame", func() {
		m.Get("/stars", repo.Stars)
		m.Get("/watchers", repo.Watchers)
	}, ignSignIn, context.RepoAssignment(), context.RepoRef(), context.UnitTypes(), context.LoadRepoUnits())

	m.Group("/:username", func() {
		m.Group("/:reponame", func() {
			m.Get("", repo.SetEditorconfigIfExists, repo.Home)
			m.Get("\\.git$", repo.SetEditorconfigIfExists, repo.Home)
		}, ignSignIn, context.RepoAssignment(), context.RepoRef(), context.UnitTypes(), context.LoadRepoUnits())

		m.Group("/:reponame", func() {
			m.Group("/info/lfs", func() {
				m.Post("/objects/batch", lfs.BatchHandler)
				m.Get("/objects/:oid/:filename", lfs.ObjectOidHandler)
				m.Any("/objects/:oid", lfs.ObjectOidHandler)
				m.Post("/objects", lfs.PostHandler)
				m.Any("/*", func(ctx *context.Context) {
					ctx.Handle(404, "", nil)
				})
			}, ignSignInAndCsrf)
			m.Any("/*", ignSignInAndCsrf, repo.HTTP)
			m.Head("/tasks/trigger", repo.TriggerTask)
		})
	})
	// ***** END: Repository *****

	m.Group("/notifications", func() {
		m.Get("", user.Notifications)
		m.Post("/status", user.NotificationStatusPost)
	}, reqSignIn)

	m.Group("/api", func() {
		apiv1.RegisterRoutes(m)
	}, ignSignIn)

	m.Group("/api/internal", func() {
		// package name internal is ideal but Golang is not allowed, so we use private as package name.
		private.RegisterRoutes(m)
	})

	// robots.txt
	m.Get("/robots.txt", func(ctx *context.Context) {
		if setting.HasRobotsTxt {
			ctx.ServeFileContent(path.Join(setting.CustomPath, "robots.txt"))
		} else {
			ctx.Handle(404, "", nil)
		}
	})

	// Not found handler.
	m.NotFound(routers.NotFound)
}
