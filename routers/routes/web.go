// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"encoding/gob"
	"net/http"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forms"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/metrics"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/admin"
	apiv1 "code.gitea.io/gitea/routers/api/v1"
	"code.gitea.io/gitea/routers/dev"
	"code.gitea.io/gitea/routers/events"
	"code.gitea.io/gitea/routers/org"
	"code.gitea.io/gitea/routers/private"
	"code.gitea.io/gitea/routers/repo"
	"code.gitea.io/gitea/routers/user"
	userSetting "code.gitea.io/gitea/routers/user/setting"
	"code.gitea.io/gitea/services/mailer"

	// to registers all internal adapters
	_ "code.gitea.io/gitea/modules/session"

	"gitea.com/go-chi/captcha"
	"gitea.com/go-chi/session"
	"github.com/NYTimes/gziphandler"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tstranex/u2f"
)

const (
	// GzipMinSize represents min size to compress for the body size of response
	GzipMinSize = 1400
)

// NormalRoutes represents non install routes
func NormalRoutes() *web.Route {
	r := BaseRoute()
	r.Use(storageHandler(setting.Avatar.Storage, "avatars", storage.Avatars))
	r.Use(storageHandler(setting.RepoAvatar.Storage, "repo-avatars", storage.RepoAvatars))

	gob.Register(&u2f.Challenge{})

	if setting.EnableGzip {
		r.Use(gziphandler.GzipHandlerWithOpts(gziphandler.MinSize(GzipMinSize)))
	}

	if (setting.Protocol == setting.FCGI || setting.Protocol == setting.FCGIUnix) && setting.AppSubURL != "" {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, setting.AppSubURL)
				next.ServeHTTP(resp, req)
			})
		})
	}

	mailer.InitMailRender(templates.Mailer())

	cpt := captcha.NewCaptcha(captcha.Options{
		SubURL: setting.AppSubURL,
	})
	r.Use(captcha.Captchaer(cpt))
	r.Use(session.Sessioner(session.Options{
		Provider:       setting.SessionConfig.Provider,
		ProviderConfig: setting.SessionConfig.ProviderConfig,
		CookieName:     setting.SessionConfig.CookieName,
		CookiePath:     setting.SessionConfig.CookiePath,
		Gclifetime:     setting.SessionConfig.Gclifetime,
		Maxlifetime:    setting.SessionConfig.Maxlifetime,
		Secure:         setting.SessionConfig.Secure,
		Domain:         setting.SessionConfig.Domain,
	}))
	r.Use(context.Csrfer(context.CsrfOptions{
		Secret:         setting.SecretKey,
		Cookie:         setting.CSRFCookieName,
		SetCookie:      true,
		Secure:         setting.SessionConfig.Secure,
		CookieHTTPOnly: setting.CSRFCookieHTTPOnly,
		Header:         "X-Csrf-Token",
		CookieDomain:   setting.SessionConfig.Domain,
		CookiePath:     setting.AppSubURL,
	}))
	/*r.Use(toolbox.Toolboxer(r, toolbox.Options{
		HealthCheckFuncs: []*toolbox.HealthCheckFuncDesc{
			{
				Desc: "Database connection",
				Func: models.Ping,
			},
		},
		DisableDebug: !setting.EnablePprof,
	}))*/
	r.Use(context.Contexter())
	//r.SetAutoHead(true)

	m.Use(user.GetNotificationCount)
	m.Use(repo.GetActiveStopwatch)
	r.Use(func(ctx *context.Context) {
		ctx.Data["UnitWikiGlobalDisabled"] = models.UnitTypeWiki.UnitGlobalDisabled()
		ctx.Data["UnitIssuesGlobalDisabled"] = models.UnitTypeIssues.UnitGlobalDisabled()
		ctx.Data["UnitPullsGlobalDisabled"] = models.UnitTypePullRequests.UnitGlobalDisabled()
		ctx.Data["UnitProjectsGlobalDisabled"] = models.UnitTypeProjects.UnitGlobalDisabled()
	})

	// for health check
	r.Head("/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if setting.HasRobotsTxt {
		r.Get("/robots.txt", func(w http.ResponseWriter, req *http.Request) {
			filePath := path.Join(setting.CustomPath, "robots.txt")
			fi, err := os.Stat(filePath)
			if err == nil && httpcache.HandleTimeCache(req, w, fi) {
				return
			}
			http.ServeFile(w, req, filePath)
		})
	}

	r.Get("/apple-touch-icon.png", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, path.Join(setting.StaticURLPrefix, "img/apple-touch-icon.png"), 301)
	})

	// prometheus metrics endpoint
	if setting.Metrics.Enabled {
		c := metrics.NewCollector()
		prometheus.MustRegister(c)

		r.Get("/metrics", routers.Metrics)
	}

	RegisterRoutes(r)

	return r
}

// RegisterRoutes routes routes to Macaron
func RegisterRoutes(m *web.Route) {
	reqSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: true})
	ignSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: setting.Service.RequireSignInView})
	ignSignInAndCsrf := context.Toggle(&context.ToggleOptions{DisableCSRF: true})
	reqSignOut := context.Toggle(&context.ToggleOptions{SignOutRequired: true})

	//bindIgnErr := binding.BindIgnErr
	bindIgnErr := web.Bind
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

	reqMilestonesDashboardPageEnabled := func(ctx *context.Context) {
		if !setting.Service.ShowMilestonesDashboardPage {
			ctx.Error(403)
			return
		}
	}

	// FIXME: not all routes need go through same middlewares.
	// Especially some AJAX requests, we can reduce middleware number to improve performance.
	// Routers.
	// for health check
	m.Get("/", routers.Home)
	m.Group("/explore", func(m *web.Route) {
		m.Get("/", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL + "/explore/repos")
		})
		m.Get("/repos", routers.ExploreRepos)
		m.Get("/users", routers.ExploreUsers)
		m.Get("/organizations", routers.ExploreOrganizations)
		m.Get("/code", routers.ExploreCode)
	}, ignSignIn)
	m.Get("/issues", reqSignIn, user.Issues)
	m.Get("/pulls", reqSignIn, user.Pulls)
	m.Get("/milestones", reqSignIn, reqMilestonesDashboardPageEnabled, user.Milestones)

	// ***** START: User *****
	m.Group("/user", func(m *web.Route) {
		m.Get("/login", user.SignIn)
		m.Post("/login", bindIgnErr(forms.SignInForm{}), user.SignInPost)
		m.Group("/", func(m *web.Route) {
			m.Combo("/login/openid").
				Get(user.SignInOpenID).
				Post(bindIgnErr(forms.SignInOpenIDForm{}), user.SignInOpenIDPost)
		}, openIDSignInEnabled)
		m.Group("/openid", func(m *web.Route) {
			m.Combo("/connect").
				Get(user.ConnectOpenID).
				Post(bindIgnErr(forms.ConnectOpenIDForm{}), user.ConnectOpenIDPost)
			m.Group("/register", func(m *web.Route) {
				m.Combo("/").
					Get(user.RegisterOpenID, openIDSignUpEnabled).
					Post(bindIgnErr(forms.SignUpOpenIDForm{}), user.RegisterOpenIDPost)
			}, openIDSignUpEnabled)
		}, openIDSignInEnabled)
		m.Get("/sign_up", user.SignUp)
		m.Post("/sign_up", bindIgnErr(forms.RegisterForm{}), user.SignUpPost)
		m.Group("/oauth2", func(m *web.Route) {
			m.Get("/:provider", user.SignInOAuth)
			m.Get("/:provider/callback", user.SignInOAuthCallback)
		})
		m.Get("/link_account", user.LinkAccount)
		m.Post("/link_account_signin", bindIgnErr(forms.SignInForm{}), user.LinkAccountPostSignIn)
		m.Post("/link_account_signup", bindIgnErr(forms.RegisterForm{}), user.LinkAccountPostRegister)
		m.Group("/two_factor", func(m *web.Route) {
			m.Get("/", user.TwoFactor)
			m.Post("/", bindIgnErr(forms.TwoFactorAuthForm{}), user.TwoFactorPost)
			m.Get("/scratch", user.TwoFactorScratch)
			m.Post("/scratch", bindIgnErr(forms.TwoFactorScratchAuthForm{}), user.TwoFactorScratchPost)
		})
		m.Group("/u2f", func(m *web.Route) {
			m.Get("/", user.U2F)
			m.Get("/challenge", user.U2FChallenge)
			m.Post("/sign", bindIgnErr(u2f.SignResponse{}), user.U2FSign)

		})

		// r.Get("/feeds", binding.Bind(forms.FeedsForm{}), user.Feeds)
		m.Any("/activate", user.Activate, reqSignIn)
		m.Any("/activate_email", user.ActivateEmail)
		m.Get("/avatar/:username/:size", user.Avatar)
		m.Get("/email2user", user.Email2User)
		m.Get("/recover_account", user.ResetPasswd)
		m.Post("/recover_account", user.ResetPasswdPost)
		m.Get("/forgot_password", user.ForgotPasswd)
		m.Post("/forgot_password", user.ForgotPasswdPost)
		m.Post("/logout", user.SignOut)
		m.Get("/task/:task", user.TaskStatus)
	}, reqSignOut)

	m.Any("/user/events", reqSignIn, events.Events)

	m.Group("/login/oauth", func(m *web.Route) {
		m.Get("/authorize", bindIgnErr(forms.AuthorizationForm{}), user.AuthorizeOAuth)
		m.Post("/grant", bindIgnErr(forms.GrantApplicationForm{}), user.GrantApplicationOAuth)
		// TODO manage redirection
		m.Post("/authorize", bindIgnErr(forms.AuthorizationForm{}), user.AuthorizeOAuth)
	}, ignSignInAndCsrf, reqSignIn)
	m.Post("/login/oauth/access_token", bindIgnErr(forms.AccessTokenForm{}), ignSignInAndCsrf, user.AccessTokenOAuth)

	m.Group("/user/settings", func(m *web.Route) {
		m.Get("/", userSetting.Profile)
		m.Post("/", bindIgnErr(forms.UpdateProfileForm{}), userSetting.ProfilePost)
		m.Get("/change_password", user.MustChangePassword)
		m.Post("/change_password", bindIgnErr(forms.MustChangePasswordForm{}), user.MustChangePasswordPost)
		m.Post("/avatar", bindIgnErr(forms.AvatarForm{}), userSetting.AvatarPost)
		m.Post("/avatar/delete", userSetting.DeleteAvatar)
		m.Group("/account", func(m *web.Route) {
			m.Combo("/").Get(userSetting.Account).Post(bindIgnErr(forms.ChangePasswordForm{}), userSetting.AccountPost)
			m.Post("/email", bindIgnErr(forms.AddEmailForm{}), userSetting.EmailPost)
			m.Post("/email/delete", userSetting.DeleteEmail)
			m.Post("/delete", userSetting.DeleteAccount)
			m.Post("/theme", bindIgnErr(forms.UpdateThemeForm{}), userSetting.UpdateUIThemePost)
		})
		m.Group("/security", func(m *web.Route) {
			m.Get("/", userSetting.Security)
			m.Group("/two_factor", func(m *web.Route) {
				m.Post("/regenerate_scratch", userSetting.RegenerateScratchTwoFactor)
				m.Post("/disable", userSetting.DisableTwoFactor)
				m.Get("/enroll", userSetting.EnrollTwoFactor)
				m.Post("/enroll", bindIgnErr(forms.TwoFactorAuthForm{}), userSetting.EnrollTwoFactorPost)
			})
			m.Group("/u2f", func(m *web.Route) {
				m.Post("/request_register", bindIgnErr(forms.U2FRegistrationForm{}), userSetting.U2FRegister)
				m.Post("/register", bindIgnErr(u2f.RegisterResponse{}), userSetting.U2FRegisterPost)
				m.Post("/delete", bindIgnErr(forms.U2FDeleteForm{}), userSetting.U2FDelete)
			})
			m.Group("/openid", func(m *web.Route) {
				m.Post("/", bindIgnErr(forms.AddOpenIDForm{}), userSetting.OpenIDPost)
				m.Post("/delete", userSetting.DeleteOpenID)
				m.Post("/toggle_visibility", userSetting.ToggleOpenIDVisibility)
			}, openIDSignInEnabled)
			m.Post("/account_link", userSetting.DeleteAccountLink)
		})
		m.Group("/applications/oauth2", func(m *web.Route) {
			m.Get("/:id", userSetting.OAuth2ApplicationShow)
			m.Post("/:id", bindIgnErr(forms.EditOAuth2ApplicationForm{}), userSetting.OAuthApplicationsEdit)
			m.Post("/:id/regenerate_secret", userSetting.OAuthApplicationsRegenerateSecret)
			m.Post("/", bindIgnErr(forms.EditOAuth2ApplicationForm{}), userSetting.OAuthApplicationsPost)
			m.Post("/delete", userSetting.DeleteOAuth2Application)
			m.Post("/revoke", userSetting.RevokeOAuth2Grant)
		})
		m.Combo("/applications").Get(userSetting.Applications).
			Post(bindIgnErr(forms.NewAccessTokenForm{}), userSetting.ApplicationsPost)
		m.Post("/applications/delete", userSetting.DeleteApplication)
		m.Combo("/keys").Get(userSetting.Keys).
			Post(bindIgnErr(forms.AddKeyForm{}), userSetting.KeysPost)
		m.Post("/keys/delete", userSetting.DeleteKey)
		m.Get("/organization", userSetting.Organization)
		m.Get("/repos", userSetting.Repos)
		m.Post("/repos/unadopted", userSetting.AdoptOrDeleteRepository)
	}, reqSignIn, func(ctx *context.Context) {
		ctx.Data["PageIsUserSettings"] = true
		ctx.Data["AllThemes"] = setting.UI.Themes
	})

	// ***** END: User *****

	m.Get("/avatar/:hash", user.AvatarByEmailHash)

	adminReq := context.Toggle(&context.ToggleOptions{SignInRequired: true, AdminRequired: true})

	// ***** START: Admin *****
	m.Group("/admin", func(m *web.Route) {
		m.Get("/", adminReq, admin.Dashboard)
		m.Post("/", adminReq, bindIgnErr(forms.AdminDashboardForm{}), admin.DashboardPost)
		m.Get("/config", admin.Config)
		m.Post("/config/test_mail", admin.SendTestMail)
		m.Group("/monitor", func(m *web.Route) {
			m.Get("/", admin.Monitor)
			m.Post("/cancel/:pid", admin.MonitorCancel)
			m.Group("/queue/:qid", func(m *web.Route) {
				m.Get("/", admin.Queue)
				m.Post("/set", admin.SetQueueSettings)
				m.Post("/add", admin.AddWorkers)
				m.Post("/cancel/:pid", admin.WorkerCancel)
				m.Post("/flush", admin.Flush)
			})
		})

		m.Group("/users", func(m *web.Route) {
			m.Get("/", admin.Users)
			m.Combo("/new").Get(admin.NewUser).Post(bindIgnErr(forms.AdminCreateUserForm{}), admin.NewUserPost)
			m.Combo("/:userid").Get(admin.EditUser).Post(bindIgnErr(forms.AdminEditUserForm{}), admin.EditUserPost)
			m.Post("/:userid/delete", admin.DeleteUser)
		})

		m.Group("/emails", func(m *web.Route) {
			m.Get("/", admin.Emails)
			m.Post("/activate", admin.ActivateEmail)
		})

		m.Group("/orgs", func(m *web.Route) {
			m.Get("/", admin.Organizations)
		})

		m.Group("/repos", func(m *web.Route) {
			m.Get("/", admin.Repos)
			m.Combo("/unadopted").Get(admin.UnadoptedRepos).Post(admin.AdoptOrDeleteRepository)
			m.Post("/delete", admin.DeleteRepo)
		})

		m.Group("/hooks", func() {
			m.Get("", admin.DefaultOrSystemWebhooks)
			m.Post("/delete", admin.DeleteDefaultOrSystemWebhook)
			m.Get("/:id", repo.WebHooksEdit)
			m.Post("/gitea/:id", bindIgnErr(forms.NewWebhookForm{}), repo.WebHooksEditPost)
			m.Post("/gogs/:id", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksEditPost)
			m.Post("/slack/:id", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksEditPost)
			m.Post("/discord/:id", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
			m.Post("/dingtalk/:id", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksEditPost)
			m.Post("/telegram/:id", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksEditPost)
			m.Post("/matrix/:id", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksEditPost)
			m.Post("/msteams/:id", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksEditPost)
			m.Post("/feishu/:id", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksEditPost)
		})

		m.Group("/^:configType(default-hooks|system-hooks)$", func() {
			m.Get("/:type/new", repo.WebhooksNew)
			m.Post("/gitea/new", bindIgnErr(auth.NewWebhookForm{}), repo.GiteaHooksNewPost)
			m.Post("/gogs/new", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksNewPost)
			m.Post("/slack/new", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksNewPost)
			m.Post("/discord/new", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
			m.Post("/dingtalk/new", bindIgnErr(auth.NewDingtalkHookForm{}), repo.DingtalkHooksNewPost)
			m.Post("/telegram/new", bindIgnErr(auth.NewTelegramHookForm{}), repo.TelegramHooksNewPost)
			m.Post("/matrix/new", bindIgnErr(auth.NewMatrixHookForm{}), repo.MatrixHooksNewPost)
			m.Post("/msteams/new", bindIgnErr(auth.NewMSTeamsHookForm{}), repo.MSTeamsHooksNewPost)
			m.Post("/feishu/new", bindIgnErr(auth.NewFeishuHookForm{}), repo.FeishuHooksNewPost)
		})

		m.Group("/auths", func() {
			m.Get("", admin.Authentications)
			m.Combo("/new").Get(admin.NewAuthSource).Post(bindIgnErr(forms.AuthenticationForm{}), admin.NewAuthSourcePost)
			m.Combo("/:authid").Get(admin.EditAuthSource).
				Post(bindIgnErr(forms.AuthenticationForm{}), admin.EditAuthSourcePost)
			m.Post("/:authid/delete", admin.DeleteAuthSource)
		})

		m.Group("/notices", func(m *web.Route) {
			m.Get("/", admin.Notices)
			m.Post("/delete", admin.DeleteNotices)
			m.Post("/empty", admin.EmptyNotices)
		})
	}, adminReq)
	// ***** END: Admin *****

	m.Group("/", func(m *web.Route) {
		m.Get("/:username", user.Profile)
		m.Get("/attachments/:uuid", repo.GetAttachment)
	}, ignSignIn)

	m.Group("/:username", func(m *web.Route) {
		m.Post("/action/:action", user.Action)
	}, reqSignIn)

	if !setting.IsProd() {
		m.Get("/template/*", dev.TemplatePreview)
	}

	reqRepoAdmin := context.RequireRepoAdmin()
	reqRepoCodeWriter := context.RequireRepoWriter(models.UnitTypeCode)
	reqRepoCodeReader := context.RequireRepoReader(models.UnitTypeCode)
	reqRepoReleaseWriter := context.RequireRepoWriter(models.UnitTypeReleases)
	reqRepoReleaseReader := context.RequireRepoReader(models.UnitTypeReleases)
	reqRepoWikiWriter := context.RequireRepoWriter(models.UnitTypeWiki)
	reqRepoIssueWriter := context.RequireRepoWriter(models.UnitTypeIssues)
	reqRepoIssueReader := context.RequireRepoReader(models.UnitTypeIssues)
	reqRepoPullsReader := context.RequireRepoReader(models.UnitTypePullRequests)
	reqRepoIssuesOrPullsWriter := context.RequireRepoWriterOr(models.UnitTypeIssues, models.UnitTypePullRequests)
	reqRepoIssuesOrPullsReader := context.RequireRepoReaderOr(models.UnitTypeIssues, models.UnitTypePullRequests)
	reqRepoProjectsReader := context.RequireRepoReader(models.UnitTypeProjects)
	reqRepoProjectsWriter := context.RequireRepoWriter(models.UnitTypeProjects)

	// ***** START: Organization *****
	m.Group("/org", func(m *web.Route) {
		m.Group("/", func(m *web.Route) {
			m.Get("/create", org.Create)
			m.Post("/create", bindIgnErr(forms.CreateOrgForm{}), org.CreatePost)
		})

		m.Group("/:org", func(m *web.Route) {
			m.Get("/dashboard", user.Dashboard)
			m.Get("/dashboard/:team", user.Dashboard)
			m.Get("/issues", user.Issues)
			m.Get("/issues/:team", user.Issues)
			m.Get("/pulls", user.Pulls)
			m.Get("/pulls/:team", user.Pulls)
			m.Get("/milestones", reqMilestonesDashboardPageEnabled, user.Milestones)
			m.Get("/milestones/:team", reqMilestonesDashboardPageEnabled, user.Milestones)
			m.Get("/members", org.Members)
			m.Post("/members/action/:action", org.MembersAction)
			m.Get("/teams", org.Teams)

			m.Get("/teams/:team", org.TeamMembers)
			m.Get("/teams/:team/repositories", org.TeamRepositories)
			m.Post("/teams/:team/action/:action", org.TeamsAction)
			m.Post("/teams/:team/action/repo/:action", org.TeamsRepoAction)
		}, context.OrgAssignment(true, false, true))

		m.Group("/:org", func(m *web.Route) {
			m.Get("/teams/new", org.NewTeam)
			m.Post("/teams/new", bindIgnErr(forms.CreateTeamForm{}), org.NewTeamPost)
			m.Get("/teams/:team/edit", org.EditTeam)
			m.Post("/teams/:team/edit", bindIgnErr(forms.CreateTeamForm{}), org.EditTeamPost)
			m.Post("/teams/:team/delete", org.DeleteTeam)

			m.Group("/settings", func(m *web.Route) {
				m.Combo("/").Get(org.Settings).
					Post(bindIgnErr(forms.UpdateOrgSettingForm{}), org.SettingsPost)
				m.Post("/avatar", bindIgnErr(forms.AvatarForm{}), org.SettingsAvatar)
				m.Post("/avatar/delete", org.SettingsDeleteAvatar)

				m.Group("/hooks", func(m *web.Route) {
					m.Get("/", org.Webhooks)
					m.Post("/delete", org.DeleteWebhook)
					m.Get("/:type/new", repo.WebhooksNew)
					m.Post("/gitea/new", bindIgnErr(forms.NewWebhookForm{}), repo.GiteaHooksNewPost)
					m.Post("/gogs/new", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksNewPost)
					m.Post("/slack/new", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksNewPost)
					m.Post("/discord/new", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
					m.Post("/dingtalk/new", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksNewPost)
					m.Post("/telegram/new", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksNewPost)
					m.Post("/matrix/new", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksNewPost)
					m.Post("/msteams/new", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksNewPost)
					m.Post("/feishu/new", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksNewPost)
					m.Get("/:id", repo.WebHooksEdit)
					m.Post("/gitea/:id", bindIgnErr(forms.NewWebhookForm{}), repo.WebHooksEditPost)
					m.Post("/gogs/:id", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksEditPost)
					m.Post("/slack/:id", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksEditPost)
					m.Post("/discord/:id", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
					m.Post("/dingtalk/:id", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksEditPost)
					m.Post("/telegram/:id", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksEditPost)
					m.Post("/matrix/:id", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksEditPost)
					m.Post("/msteams/:id", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksEditPost)
					m.Post("/feishu/:id", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksEditPost)
				})

				m.Group("/labels", func(m *web.Route) {
					m.Get("/", org.RetrieveLabels, org.Labels)
					m.Post("/new", bindIgnErr(forms.CreateLabelForm{}), org.NewLabel)
					m.Post("/edit", bindIgnErr(forms.CreateLabelForm{}), org.UpdateLabel)
					m.Post("/delete", org.DeleteLabel)
					m.Post("/initialize", bindIgnErr(forms.InitializeLabelsForm{}), org.InitializeLabels)
				})

				m.Route("/delete", "GET,POST", org.SettingsDelete)
			})
		}, context.OrgAssignment(true, true))
	}, reqSignIn)
	// ***** END: Organization *****

	// ***** START: Repository *****
	m.Group("/repo", func(m *web.Route) {
		m.Get("/create", repo.Create)
		m.Post("/create", bindIgnErr(forms.CreateRepoForm{}), repo.CreatePost)
		m.Get("/migrate", repo.Migrate)
		m.Post("/migrate", bindIgnErr(forms.MigrateRepoForm{}), repo.MigratePost)
		m.Group("/fork", func(m *web.Route) {
			m.Combo("/:repoid").Get(repo.Fork).
				Post(bindIgnErr(forms.CreateRepoForm{}), repo.ForkPost)
		}, context.RepoIDAssignment(), context.UnitTypes(), reqRepoCodeReader)
	}, reqSignIn)

	// ***** Release Attachment Download without Signin
	m.Get("/:username/:reponame/releases/download/:vTag/:fileName", ignSignIn, context.RepoAssignment(), repo.MustBeNotEmpty, repo.RedirectDownload)

	m.Group("/:username/:reponame", func(m *web.Route) {
		m.Group("/settings", func(m *web.Route) {
			m.Combo("/").Get(repo.Settings).
				Post(bindIgnErr(forms.RepoSettingForm{}), repo.SettingsPost)
			m.Post("/avatar", bindIgnErr(forms.AvatarForm{}), repo.SettingsAvatar)
			m.Post("/avatar/delete", repo.SettingsDeleteAvatar)

			m.Group("/collaboration", func(m *web.Route) {
				m.Combo("/").Get(repo.Collaboration).Post(repo.CollaborationPost)
				m.Post("/access_mode", repo.ChangeCollaborationAccessMode)
				m.Post("/delete", repo.DeleteCollaboration)
				m.Group("/team", func(m *web.Route) {
					m.Post("/", repo.AddTeamPost)
					m.Post("/delete", repo.DeleteTeam)
				})
			})
			m.Group("/branches", func(m *web.Route) {
				m.Combo("/").Get(repo.ProtectedBranch).Post(repo.ProtectedBranchPost)
				m.Combo("/*").Get(repo.SettingsProtectedBranch).
					Post(bindIgnErr(forms.ProtectBranchForm{}), context.RepoMustNotBeArchived(), repo.SettingsProtectedBranchPost)
			}, repo.MustBeNotEmpty)

			m.Group("/hooks", func(m *web.Route) {
				m.Get("/", repo.Webhooks)
				m.Post("/delete", repo.DeleteWebhook)
				m.Get("/:type/new", repo.WebhooksNew)
				m.Post("/gitea/new", bindIgnErr(forms.NewWebhookForm{}), repo.GiteaHooksNewPost)
				m.Post("/gogs/new", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksNewPost)
				m.Post("/slack/new", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksNewPost)
				m.Post("/discord/new", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
				m.Post("/dingtalk/new", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksNewPost)
				m.Post("/telegram/new", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksNewPost)
				m.Post("/matrix/new", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksNewPost)
				m.Post("/msteams/new", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksNewPost)
				m.Post("/feishu/new", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksNewPost)
				m.Get("/:id", repo.WebHooksEdit)
				m.Post("/:id/test", repo.TestWebhook)
				m.Post("/gitea/:id", bindIgnErr(forms.NewWebhookForm{}), repo.WebHooksEditPost)
				m.Post("/gogs/:id", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksEditPost)
				m.Post("/slack/:id", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksEditPost)
				m.Post("/discord/:id", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
				m.Post("/dingtalk/:id", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksEditPost)
				m.Post("/telegram/:id", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksEditPost)
				m.Post("/matrix/:id", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksEditPost)
				m.Post("/msteams/:id", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksEditPost)
				m.Post("/feishu/:id", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksEditPost)

				m.Group("/git", func(m *web.Route) {
					m.Get("/", repo.GitHooks)
					m.Combo("/:name").Get(repo.GitHooksEdit).
						Post(repo.GitHooksEditPost)
				}, context.GitHookService())
			})

			m.Group("/keys", func(m *web.Route) {
				m.Combo("/").Get(repo.DeployKeys).
					Post(bindIgnErr(forms.AddKeyForm{}), repo.DeployKeysPost)
				m.Post("/delete", repo.DeleteDeployKey)
			})

			m.Group("/lfs", func(m *web.Route) {
				m.Get("/", repo.LFSFiles)
				m.Get("/show/:oid", repo.LFSFileGet)
				m.Post("/delete/:oid", repo.LFSDelete)
				m.Get("/pointers", repo.LFSPointerFiles)
				m.Post("/pointers/associate", repo.LFSAutoAssociate)
				m.Get("/find", repo.LFSFileFind)
				m.Group("/locks", func(m *web.Route) {
					m.Get("/", repo.LFSLocks)
					m.Post("/", repo.LFSLockFile)
					m.Post("/:lid/unlock", repo.LFSUnlock)
				})
			})

		}, func(ctx *context.Context) {
			ctx.Data["PageIsSettings"] = true
			ctx.Data["LFSStartServer"] = setting.LFS.StartServer
		})
	}, reqSignIn, context.RepoAssignment(), context.UnitTypes(), reqRepoAdmin, context.RepoRef())

	m.Post("/:username/:reponame/action/:action", reqSignIn, context.RepoAssignment(), context.UnitTypes(), repo.Action)

	// Grouping for those endpoints not requiring authentication
	m.Group("/:username/:reponame", func(m *web.Route) {
		m.Group("/milestone", func(m *web.Route) {
			m.Get("/:id", repo.MilestoneIssuesAndPulls)
		}, reqRepoIssuesOrPullsReader, context.RepoRef())
		m.Combo("/compare/*", repo.MustBeNotEmpty, reqRepoCodeReader, repo.SetEditorconfigIfExists).
			Get(ignSignIn, repo.SetDiffViewStyle, repo.CompareDiff).
			Post(reqSignIn, context.RepoMustNotBeArchived(), reqRepoPullsReader, repo.MustAllowPulls, bindIgnErr(forms.CreateIssueForm{}), repo.CompareAndPullRequestPost)
	}, context.RepoAssignment(), context.UnitTypes())

	// Grouping for those endpoints that do require authentication
	m.Group("/:username/:reponame", func(m *web.Route) {
		m.Group("/issues", func(m *web.Route) {
			m.Group("/new", func(m *web.Route) {
				m.Combo("/").Get(context.RepoRef(), repo.NewIssue).
					Post(bindIgnErr(forms.CreateIssueForm{}), repo.NewIssuePost)
				m.Get("/choose", context.RepoRef(), repo.NewIssueChooseTemplate)
			})
		}, context.RepoMustNotBeArchived(), reqRepoIssueReader)
		// FIXME: should use different URLs but mostly same logic for comments of issue and pull reuqest.
		// So they can apply their own enable/disable logic on routers.
		m.Group("/issues", func(m *web.Route) {
			m.Group("/:index", func(m *web.Route) {
				m.Post("/title", repo.UpdateIssueTitle)
				m.Post("/content", repo.UpdateIssueContent)
				m.Post("/watch", repo.IssueWatch)
				m.Post("/ref", repo.UpdateIssueRef)
				m.Group("/dependency", func(m *web.Route) {
					m.Post("/add", repo.AddDependency)
					m.Post("/delete", repo.RemoveDependency)
				})
				m.Combo("/comments").Post(repo.MustAllowUserComment, bindIgnErr(forms.CreateCommentForm{}), repo.NewComment)
				m.Group("/times", func(m *web.Route) {
					m.Post("/add", bindIgnErr(forms.AddTimeManuallyForm{}), repo.AddTimeManually)
					m.Group("/stopwatch", func(m *web.Route) {
						m.Post("/toggle", repo.IssueStopwatch)
						m.Post("/cancel", repo.CancelStopwatch)
					})
				})
				m.Post("/reactions/:action", bindIgnErr(forms.ReactionForm{}), repo.ChangeIssueReaction)
				m.Post("/lock", reqRepoIssueWriter, bindIgnErr(forms.IssueLockForm{}), repo.LockIssue)
				m.Post("/unlock", reqRepoIssueWriter, repo.UnlockIssue)
			}, context.RepoMustNotBeArchived())
			m.Group("/:index", func(m *web.Route) {
				m.Get("/attachments", repo.GetIssueAttachments)
				m.Get("/attachments/:uuid", repo.GetAttachment)
			})

			m.Post("/labels", reqRepoIssuesOrPullsWriter, repo.UpdateIssueLabel)
			m.Post("/milestone", reqRepoIssuesOrPullsWriter, repo.UpdateIssueMilestone)
			m.Post("/projects", reqRepoIssuesOrPullsWriter, repo.UpdateIssueProject)
			m.Post("/assignee", reqRepoIssuesOrPullsWriter, repo.UpdateIssueAssignee)
			m.Post("/request_review", reqRepoIssuesOrPullsReader, repo.UpdatePullReviewRequest)
			m.Post("/status", reqRepoIssuesOrPullsWriter, repo.UpdateIssueStatus)
			m.Post("/resolve_conversation", reqRepoIssuesOrPullsReader, repo.UpdateResolveConversation)
			m.Post("/attachments", repo.UploadIssueAttachment)
			m.Post("/attachments/remove", repo.DeleteAttachment)
		}, context.RepoMustNotBeArchived())
		m.Group("/comments/:id", func(m *web.Route) {
			m.Post("", repo.UpdateCommentContent)
			m.Post("/delete", repo.DeleteComment)
			m.Post("/reactions/:action", bindIgnErr(forms.ReactionForm{}), repo.ChangeCommentReaction)
		}, context.RepoMustNotBeArchived())
		m.Group("/comments/:id", func(m *web.Route) {
			m.Get("/attachments", repo.GetCommentAttachments)
		})
		m.Group("/labels", func(m *web.Route) {
			m.Post("/new", bindIgnErr(forms.CreateLabelForm{}), repo.NewLabel)
			m.Post("/edit", bindIgnErr(forms.CreateLabelForm{}), repo.UpdateLabel)
			m.Post("/delete", repo.DeleteLabel)
			m.Post("/initialize", bindIgnErr(forms.InitializeLabelsForm{}), repo.InitializeLabels)
		}, context.RepoMustNotBeArchived(), reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Group("/milestones", func(m *web.Route) {
			m.Combo("/new").Get(repo.NewMilestone).
				Post(bindIgnErr(forms.CreateMilestoneForm{}), repo.NewMilestonePost)
			m.Get("/:id/edit", repo.EditMilestone)
			m.Post("/:id/edit", bindIgnErr(forms.CreateMilestoneForm{}), repo.EditMilestonePost)
			m.Post("/:id/:action", repo.ChangeMilestoneStatus)
			m.Post("/delete", repo.DeleteMilestone)
		}, context.RepoMustNotBeArchived(), reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Group("/pull", func(m *web.Route) {
			m.Post("/:index/target_branch", repo.UpdatePullRequestTarget)
		}, context.RepoMustNotBeArchived())

		m.Group("", func(m *web.Route) {
			m.Group("", func(m *web.Route) {
				m.Combo("/_edit/*").Get(repo.EditFile).
					Post(bindIgnErr(forms.EditRepoFileForm{}), repo.EditFilePost)
				m.Combo("/_new/*").Get(repo.NewFile).
					Post(bindIgnErr(forms.EditRepoFileForm{}), repo.NewFilePost)
				m.Post("/_preview/*", bindIgnErr(forms.EditPreviewDiffForm{}), repo.DiffPreviewPost)
				m.Combo("/_delete/*").Get(repo.DeleteFile).
					Post(bindIgnErr(forms.DeleteRepoFileForm{}), repo.DeleteFilePost)
				m.Combo("/_upload/*", repo.MustBeAbleToUpload).
					Get(repo.UploadFile).
					Post(bindIgnErr(forms.UploadRepoFileForm{}), repo.UploadFilePost)
			}, context.RepoRefByType(context.RepoRefBranch), repo.MustBeEditable)
			m.Group("", func(m *web.Route) {
				m.Post("/upload-file", repo.UploadFileToServer)
				m.Post("/upload-remove", bindIgnErr(forms.RemoveUploadFileForm{}), repo.RemoveUploadFileFromServer)
			}, context.RepoRef(), repo.MustBeEditable, repo.MustBeAbleToUpload)
		}, context.RepoMustNotBeArchived(), reqRepoCodeWriter, repo.MustBeNotEmpty)

		m.Group("/branches", func(m *web.Route) {
			m.Group("/_new/", func(m *web.Route) {
				m.Post("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.CreateBranch)
				m.Post("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.CreateBranch)
				m.Post("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.CreateBranch)
			}, bindIgnErr(forms.NewBranchForm{}))
			m.Post("/delete", repo.DeleteBranchPost)
			m.Post("/restore", repo.RestoreBranchPost)
		}, context.RepoMustNotBeArchived(), reqRepoCodeWriter, repo.MustBeNotEmpty)

	}, reqSignIn, context.RepoAssignment(), context.UnitTypes())

	// Releases
	m.Group("/:username/:reponame", func(m *web.Route) {
		m.Get("/tags", repo.TagsList, repo.MustBeNotEmpty,
			reqRepoCodeReader, context.RepoRefByType(context.RepoRefTag))
		m.Group("/releases", func(m *web.Route) {
			m.Get("/", repo.Releases)
			m.Get("/tag/*", repo.SingleRelease)
			m.Get("/latest", repo.LatestRelease)
			m.Get("/attachments/:uuid", repo.GetAttachment)
		}, repo.MustBeNotEmpty, reqRepoReleaseReader, context.RepoRefByType(context.RepoRefTag))
		m.Group("/releases", func(m *web.Route) {
			m.Get("/new", repo.NewRelease)
			m.Post("/new", bindIgnErr(forms.NewReleaseForm{}), repo.NewReleasePost)
			m.Post("/delete", repo.DeleteRelease)
			m.Post("/attachments", repo.UploadReleaseAttachment)
			m.Post("/attachments/remove", repo.DeleteAttachment)
		}, reqSignIn, repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoReleaseWriter, context.RepoRef())
		m.Post("/tags/delete", repo.DeleteTag, reqSignIn,
			repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoCodeWriter, context.RepoRef())
		m.Group("/releases", func(m *web.Route) {
			m.Get("/edit/*", repo.EditRelease)
			m.Post("/edit/*", bindIgnErr(forms.EditReleaseForm{}), repo.EditReleasePost)
		}, reqSignIn, repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoReleaseWriter, func(ctx *context.Context) {
			var err error
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
			if err != nil {
				ctx.ServerError("GetBranchCommit", err)
				return
			}
			ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
			if err != nil {
				ctx.ServerError("GetCommitsCount", err)
				return
			}
			ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
		})
	}, ignSignIn, context.RepoAssignment(), context.UnitTypes(), reqRepoReleaseReader)

	m.Group("/:username/:reponame", func(m *web.Route) {
		m.Post("/topics", repo.TopicsPost)
	}, context.RepoAssignment(), context.RepoMustNotBeArchived(), reqRepoAdmin)

	m.Group("/:username/:reponame", func(m *web.Route) {
		m.Group("", func(m *web.Route) {
			m.Get("/^:type(issues|pulls)$", repo.Issues)
			m.Get("/^:type(issues|pulls)$/:index", repo.ViewIssue)
			m.Get("/labels/", reqRepoIssuesOrPullsReader, repo.RetrieveLabels, repo.Labels)
			m.Get("/milestones", reqRepoIssuesOrPullsReader, repo.Milestones)
		}, context.RepoRef())

		m.Group("/projects", func(m *web.Route) {
			m.Get("", repo.Projects)
			m.Get("/:id", repo.ViewProject)
			m.Group("", func(m *web.Route) {
				m.Get("/new", repo.NewProject)
				m.Post("/new", bindIgnErr(forms.CreateProjectForm{}), repo.NewProjectPost)
				m.Group("/:id", func(m *web.Route) {
					m.Post("", bindIgnErr(forms.EditProjectBoardTitleForm{}), repo.AddBoardToProjectPost)
					m.Post("/delete", repo.DeleteProject)

					m.Get("/edit", repo.EditProject)
					m.Post("/edit", bindIgnErr(forms.CreateProjectForm{}), repo.EditProjectPost)
					m.Post("/^:action(open|close)$", repo.ChangeProjectStatus)

					m.Group("/:boardID", func(m *web.Route) {
						m.Put("", bindIgnErr(forms.EditProjectBoardTitleForm{}), repo.EditProjectBoardTitle)
						m.Delete("", repo.DeleteProjectBoard)
						m.Post("/default", repo.SetDefaultProjectBoard)

						m.Post("/:index", repo.MoveIssueAcrossBoards)
					})
				})
			}, reqRepoProjectsWriter, context.RepoMustNotBeArchived())
		}, reqRepoProjectsReader, repo.MustEnableProjects)

		m.Group("/wiki", func(m *web.Route) {
			m.Get("/?:page", repo.Wiki)
			m.Get("/_pages", repo.WikiPages)
			m.Get("/:page/_revision", repo.WikiRevision)
			m.Get("/commit/:sha([a-f0-9]{7,40})$", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.Diff)
			m.Get("/commit/:sha([a-f0-9]{7,40})\\.:ext(patch|diff)", repo.RawDiff)

			m.Group("", func(m *web.Route) {
				m.Combo("/_new").Get(repo.NewWiki).
					Post(bindIgnErr(forms.NewWikiForm{}), repo.NewWikiPost)
				m.Combo("/:page/_edit").Get(repo.EditWiki).
					Post(bindIgnErr(forms.NewWikiForm{}), repo.EditWikiPost)
				m.Post("/:page/delete", repo.DeleteWikiPagePost)
			}, context.RepoMustNotBeArchived(), reqSignIn, reqRepoWikiWriter)
		}, repo.MustEnableWiki, context.RepoRef(), func(ctx *context.Context) {
			ctx.Data["PageIsWiki"] = true
		})

		m.Group("/wiki", func(m *web.Route) {
			m.Get("/raw/*", repo.WikiRaw)
		}, repo.MustEnableWiki)

		m.Group("/activity", func(m *web.Route) {
			m.Get("", repo.Activity)
			m.Get("/:period", repo.Activity)
		}, context.RepoRef(), repo.MustBeNotEmpty, context.RequireRepoReaderOr(models.UnitTypePullRequests, models.UnitTypeIssues, models.UnitTypeReleases))

		m.Group("/activity_author_data", func(m *web.Route) {
			m.Get("", repo.ActivityAuthors)
			m.Get("/:period", repo.ActivityAuthors)
		}, context.RepoRef(), repo.MustBeNotEmpty, context.RequireRepoReaderOr(models.UnitTypeCode))

		m.Group("/archive", func(m *web.Route) {
			m.Get("/*", repo.Download)
			m.Post("/*", repo.InitiateDownload)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/branches", func(m *web.Route) {
			m.Get("", repo.Branches)
		}, repo.MustBeNotEmpty, context.RepoRef(), reqRepoCodeReader)

		m.Group("/blob_excerpt", func(m *web.Route) {
			m.Get("/:sha", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.ExcerptBlob)
		}, repo.MustBeNotEmpty, context.RepoRef(), reqRepoCodeReader)

		m.Group("/pulls/:index", func(m *web.Route) {
			m.Get(".diff", repo.DownloadPullDiff)
			m.Get(".patch", repo.DownloadPullPatch)
			m.Get("/commits", context.RepoRef(), repo.ViewPullCommits)
			m.Post("/merge", context.RepoMustNotBeArchived(), bindIgnErr(forms.MergePullRequestForm{}), repo.MergePullRequest)
			m.Post("/update", repo.UpdatePullRequest)
			m.Post("/cleanup", context.RepoMustNotBeArchived(), context.RepoRef(), repo.CleanUpPullRequest)
			m.Group("/files", func(m *web.Route) {
				m.Get("", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.ViewPullFiles)
				m.Group("/reviews", func(m *web.Route) {
					m.Get("/new_comment", repo.RenderNewCodeCommentForm)
					m.Post("/comments", bindIgnErr(forms.CodeCommentForm{}), repo.CreateCodeComment)
					m.Post("/submit", bindIgnErr(forms.SubmitReviewForm{}), repo.SubmitReview)
				}, context.RepoMustNotBeArchived())
			})
		}, repo.MustAllowPulls)

		m.Group("/media", func(m *web.Route) {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.SingleDownloadOrLFS)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.SingleDownloadOrLFS)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.SingleDownloadOrLFS)
			m.Get("/blob/:sha", context.RepoRefByType(context.RepoRefBlob), repo.DownloadByIDOrLFS)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.SingleDownloadOrLFS)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/raw", func(m *web.Route) {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.SingleDownload)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.SingleDownload)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.SingleDownload)
			m.Get("/blob/:sha", context.RepoRefByType(context.RepoRefBlob), repo.DownloadByID)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.SingleDownload)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/commits", func(m *web.Route) {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RefCommits)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RefCommits)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RefCommits)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.RefCommits)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/blame", func(m *web.Route) {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RefBlame)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RefBlame)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RefBlame)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("", func(m *web.Route) {
			m.Get("/graph", repo.Graph)
			m.Get("/commit/:sha([a-f0-9]{7,40})$", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.Diff)
		}, repo.MustBeNotEmpty, context.RepoRef(), reqRepoCodeReader)

		m.Group("/src", func(m *web.Route) {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.Home)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.Home)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.Home)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.Home)
		}, repo.SetEditorconfigIfExists)

		m.Group("", func(m *web.Route) {
			m.Get("/forks", repo.Forks)
		}, context.RepoRef(), reqRepoCodeReader)
		m.Get("/commit/:sha([a-f0-9]{7,40})\\.:ext(patch|diff)",
			repo.MustBeNotEmpty, reqRepoCodeReader, repo.RawDiff)
	}, ignSignIn, context.RepoAssignment(), context.UnitTypes())
	m.Group("/:username/:reponame", func(m *web.Route) {
		m.Get("/stars", repo.Stars)
		m.Get("/watchers", repo.Watchers)
		m.Get("/search", reqRepoCodeReader, repo.Search)
	}, ignSignIn, context.RepoAssignment(), context.RepoRef(), context.UnitTypes())

	m.Group("/:username", func(m *web.Route) {
		m.Group("/:reponame", func(m *web.Route) {
			m.Get("", repo.SetEditorconfigIfExists, repo.Home)
			m.Get("\\.git$", repo.SetEditorconfigIfExists, repo.Home)
		}, ignSignIn, context.RepoAssignment(), context.RepoRef(), context.UnitTypes())

		m.Group("/:reponame", func(m *web.Route) {
			m.Group("\\.git/info/lfs", func(m *web.Route) {
				m.Post("/objects/batch", lfs.BatchHandler)
				m.Get("/objects/:oid/:filename", lfs.ObjectOidHandler)
				m.Any("/objects/:oid", lfs.ObjectOidHandler)
				m.Post("/objects", lfs.PostHandler)
				m.Post("/verify", lfs.VerifyHandler)
				m.Group("/locks", func(m *web.Route) {
					m.Get("/", lfs.GetListLockHandler)
					m.Post("/", lfs.PostLockHandler)
					m.Post("/verify", lfs.VerifyLockHandler)
					m.Post("/:lid/unlock", lfs.UnLockHandler)
				})
				m.Any("/*", func(ctx *context.Context) {
					ctx.NotFound("", nil)
				})
			}, ignSignInAndCsrf)
			m.Any("/*", ignSignInAndCsrf, repo.HTTP)
			m.Head("/tasks/trigger", repo.TriggerTask)
		})
	})
	// ***** END: Repository *****

	m.Group("/notifications", func(m *web.Route) {
		m.Get("", user.Notifications)
		m.Post("/status", user.NotificationStatusPost)
		m.Post("/purge", user.NotificationPurgePost)
	}, reqSignIn)

	if setting.API.EnableSwagger {
		m.Get("/swagger.v1.json", routers.SwaggerV1Json)
	}

	var handlers []interface{}
	if setting.CORSConfig.Enabled {
		handlers = append(handlers, cors.Handler(cors.Options{
			//Scheme:           setting.CORSConfig.Scheme,
			AllowedOrigins: setting.CORSConfig.AllowDomain,
			//setting.CORSConfig.AllowSubdomain
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		}))
	}
	handlers = append(handlers, ignSignIn)
	m.Group("/api", func(m *web.Route) {
		apiv1.RegisterRoutes(m)
	}, handlers...)

	m.Group("/api/internal", func(m *web.Route) {
		// package name internal is ideal but Golang is not allowed, so we use private as package name.
		private.RegisterRoutes(m)
	})

	// Not found handler.
	m.NotFound(web.Wrap(routers.NotFound))
}
