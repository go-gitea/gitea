// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	gocontext "context"
	"net/http"
	"os"
	"path"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/metrics"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/routing"
	"code.gitea.io/gitea/routers/web/admin"
	"code.gitea.io/gitea/routers/web/auth"
	"code.gitea.io/gitea/routers/web/dev"
	"code.gitea.io/gitea/routers/web/events"
	"code.gitea.io/gitea/routers/web/explore"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/routers/web/healthcheck"
	"code.gitea.io/gitea/routers/web/misc"
	"code.gitea.io/gitea/routers/web/org"
	"code.gitea.io/gitea/routers/web/repo"
	"code.gitea.io/gitea/routers/web/user"
	user_setting "code.gitea.io/gitea/routers/web/user/setting"
	"code.gitea.io/gitea/routers/web/user/setting/security"
	auth_service "code.gitea.io/gitea/services/auth"
	context_service "code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/lfs"
	"code.gitea.io/gitea/services/mailer"

	_ "code.gitea.io/gitea/modules/session" // to registers all internal adapters

	"gitea.com/go-chi/captcha"
	"gitea.com/go-chi/session"
	"github.com/NYTimes/gziphandler"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// GzipMinSize represents min size to compress for the body size of response
	GzipMinSize = 1400
)

// CorsHandler return a http handler who set CORS options if enabled by config
func CorsHandler() func(next http.Handler) http.Handler {
	if setting.CORSConfig.Enabled {
		return cors.Handler(cors.Options{
			// Scheme:           setting.CORSConfig.Scheme, // FIXME: the cors middleware needs scheme option
			AllowedOrigins: setting.CORSConfig.AllowDomain,
			// setting.CORSConfig.AllowSubdomain // FIXME: the cors middleware needs allowSubdomain option
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		})
	}

	return func(next http.Handler) http.Handler {
		return next
	}
}

// The OAuth2 plugin is expected to be executed first, as it must ignore the user id stored
// in the session (if there is a user id stored in session other plugins might return the user
// object for that id).
//
// The Session plugin is expected to be executed second, in order to skip authentication
// for users that have already signed in.
func buildAuthGroup() *auth_service.Group {
	group := auth_service.NewGroup(
		&auth_service.OAuth2{}, // FIXME: this should be removed and only applied in download and oauth related routers
		&auth_service.Basic{},  // FIXME: this should be removed and only applied in download and git/lfs routers
		&auth_service.Session{},
	)
	if setting.Service.EnableReverseProxyAuth {
		group.Add(&auth_service.ReverseProxy{})
	}
	specialAdd(group)

	return group
}

// Routes returns all web routes
func Routes() *web.Route {
	routes := web.NewRoute()

	routes.Use(web.WrapWithPrefix(public.AssetsURLPathPrefix, public.AssetsHandlerFunc(&public.Options{
		Directory:   path.Join(setting.StaticRootPath, "public"),
		Prefix:      public.AssetsURLPathPrefix,
		CorsHandler: CorsHandler(),
	}), "AssetsHandler"))

	sessioner := session.Sessioner(session.Options{
		Provider:       setting.SessionConfig.Provider,
		ProviderConfig: setting.SessionConfig.ProviderConfig,
		CookieName:     setting.SessionConfig.CookieName,
		CookiePath:     setting.SessionConfig.CookiePath,
		Gclifetime:     setting.SessionConfig.Gclifetime,
		Maxlifetime:    setting.SessionConfig.Maxlifetime,
		Secure:         setting.SessionConfig.Secure,
		SameSite:       setting.SessionConfig.SameSite,
		Domain:         setting.SessionConfig.Domain,
	})
	routes.Use(sessioner)

	routes.Use(Recovery())

	// We use r.Route here over r.Use because this prevents requests that are not for avatars having to go through this additional handler
	routes.Route("/avatars/*", "GET, HEAD", storageHandler(setting.Avatar.Storage, "avatars", storage.Avatars))
	routes.Route("/repo-avatars/*", "GET, HEAD", storageHandler(setting.RepoAvatar.Storage, "repo-avatars", storage.RepoAvatars))

	// for health check - doesn't need to be passed through gzip handler
	routes.Head("/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// this png is very likely to always be below the limit for gzip so it doesn't need to pass through gzip
	routes.Get("/apple-touch-icon.png", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, path.Join(setting.StaticURLPrefix, "/assets/img/apple-touch-icon.png"), http.StatusPermanentRedirect)
	})

	// redirect default favicon to the path of the custom favicon with a default as a fallback
	routes.Get("/favicon.ico", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, path.Join(setting.StaticURLPrefix, "/assets/img/favicon.png"), http.StatusMovedPermanently)
	})

	common := []interface{}{}

	if setting.EnableGzip {
		h, err := gziphandler.GzipHandlerWithOpts(gziphandler.MinSize(GzipMinSize))
		if err != nil {
			log.Fatal("GzipHandlerWithOpts failed: %v", err)
		}
		common = append(common, h)
	}

	mailer.InitMailRender(templates.Mailer())

	if setting.Service.EnableCaptcha {
		// The captcha http.Handler should only fire on /captcha/* so we can just mount this on that url
		routes.Route("/captcha/*", "GET,HEAD", append(common, captcha.Captchaer(context.GetImageCaptcha()))...)
	}

	if setting.HasRobotsTxt {
		routes.Get("/robots.txt", append(common, func(w http.ResponseWriter, req *http.Request) {
			filePath := path.Join(setting.CustomPath, "robots.txt")
			fi, err := os.Stat(filePath)
			if err == nil && httpcache.HandleTimeCache(req, w, fi) {
				return
			}
			http.ServeFile(w, req, filePath)
		})...)
	}

	// prometheus metrics endpoint - do not need to go through contexter
	if setting.Metrics.Enabled {
		c := metrics.NewCollector()
		prometheus.MustRegister(c)

		routes.Get("/metrics", append(common, Metrics)...)
	}

	routes.Get("/ssh_info", func(rw http.ResponseWriter, req *http.Request) {
		if !git.SupportProcReceive {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.Header().Set("content-type", "text/json;charset=UTF-8")
		_, err := rw.Write([]byte(`{"type":"gitea","version":1}`))
		if err != nil {
			log.Error("fail to write result: err: %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		rw.WriteHeader(http.StatusOK)
	})

	routes.Get("/api/healthz", healthcheck.Check)

	// Removed: toolbox.Toolboxer middleware will provide debug information which seems unnecessary
	common = append(common, context.Contexter())

	group := buildAuthGroup()
	if err := group.Init(); err != nil {
		log.Error("Could not initialize '%s' auth method, error: %s", group.Name(), err)
	}

	// Get user from session if logged in.
	common = append(common, context.Auth(group))

	// GetHead allows a HEAD request redirect to GET if HEAD method is not defined for that route
	common = append(common, middleware.GetHead)

	if setting.API.EnableSwagger {
		// Note: The route moved from apiroutes because it's in fact want to render a web page
		routes.Get("/api/swagger", append(common, misc.Swagger)...) // Render V1 by default
	}

	// TODO: These really seem like things that could be folded into Contexter or as helper functions
	common = append(common, user.GetNotificationCount)
	common = append(common, repo.GetActiveStopwatch)
	common = append(common, goGet)

	others := web.NewRoute()
	for _, middle := range common {
		others.Use(middle)
	}

	RegisterRoutes(others)
	routes.Mount("", others)
	return routes
}

// RegisterRoutes register routes
func RegisterRoutes(m *web.Route) {
	reqSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: true})
	ignSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: setting.Service.RequireSignInView})
	ignExploreSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: setting.Service.RequireSignInView || setting.Service.Explore.RequireSigninView})
	ignSignInAndCsrf := context.Toggle(&context.ToggleOptions{DisableCSRF: true})
	reqSignOut := context.Toggle(&context.ToggleOptions{SignOutRequired: true})

	bindIgnErr := web.Bind
	validation.AddBindingRules()

	linkAccountEnabled := func(ctx *context.Context) {
		if !setting.Service.EnableOpenIDSignIn && !setting.Service.EnableOpenIDSignUp && !setting.OAuth2.Enable {
			ctx.Error(http.StatusForbidden)
			return
		}
	}

	openIDSignInEnabled := func(ctx *context.Context) {
		if !setting.Service.EnableOpenIDSignIn {
			ctx.Error(http.StatusForbidden)
			return
		}
	}

	openIDSignUpEnabled := func(ctx *context.Context) {
		if !setting.Service.EnableOpenIDSignUp {
			ctx.Error(http.StatusForbidden)
			return
		}
	}

	reqMilestonesDashboardPageEnabled := func(ctx *context.Context) {
		if !setting.Service.ShowMilestonesDashboardPage {
			ctx.Error(http.StatusForbidden)
			return
		}
	}

	// webhooksEnabled requires webhooks to be enabled by admin.
	webhooksEnabled := func(ctx *context.Context) {
		if setting.DisableWebhooks {
			ctx.Error(http.StatusForbidden)
			return
		}
	}

	lfsServerEnabled := func(ctx *context.Context) {
		if !setting.LFS.StartServer {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	federationEnabled := func(ctx *context.Context) {
		if !setting.Federation.Enabled {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	// FIXME: not all routes need go through same middleware.
	// Especially some AJAX requests, we can reduce middleware number to improve performance.
	// Routers.
	// for health check
	m.Get("/", Home)
	m.Get("/sitemap.xml", ignExploreSignIn, HomeSitemap)
	m.Group("/.well-known", func() {
		m.Get("/openid-configuration", auth.OIDCWellKnown)
		m.Group("", func() {
			m.Get("/nodeinfo", NodeInfoLinks)
			m.Get("/webfinger", WebfingerQuery)
		}, federationEnabled)
		m.Get("/change-password", func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "/user/settings/account", http.StatusTemporaryRedirect)
		})
	})

	m.Group("/explore", func() {
		m.Get("", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL + "/explore/repos")
		})
		m.Get("/repos", explore.Repos)
		m.Get("/repos/sitemap-{idx}.xml", explore.Repos)
		m.Get("/users", explore.Users)
		m.Get("/users/sitemap-{idx}.xml", explore.Users)
		m.Get("/organizations", explore.Organizations)
		m.Get("/code", explore.Code)
		m.Get("/topics/search", explore.TopicSearch)
	}, ignExploreSignIn)
	m.Group("/issues", func() {
		m.Get("", user.Issues)
		m.Get("/search", repo.SearchIssues)
	}, reqSignIn)

	m.Get("/pulls", reqSignIn, user.Pulls)
	m.Get("/milestones", reqSignIn, reqMilestonesDashboardPageEnabled, user.Milestones)

	// ***** START: User *****
	m.Group("/user", func() {
		m.Get("/login", auth.SignIn)
		m.Post("/login", bindIgnErr(forms.SignInForm{}), auth.SignInPost)
		m.Group("", func() {
			m.Combo("/login/openid").
				Get(auth.SignInOpenID).
				Post(bindIgnErr(forms.SignInOpenIDForm{}), auth.SignInOpenIDPost)
		}, openIDSignInEnabled)
		m.Group("/openid", func() {
			m.Combo("/connect").
				Get(auth.ConnectOpenID).
				Post(bindIgnErr(forms.ConnectOpenIDForm{}), auth.ConnectOpenIDPost)
			m.Group("/register", func() {
				m.Combo("").
					Get(auth.RegisterOpenID, openIDSignUpEnabled).
					Post(bindIgnErr(forms.SignUpOpenIDForm{}), auth.RegisterOpenIDPost)
			}, openIDSignUpEnabled)
		}, openIDSignInEnabled)
		m.Get("/sign_up", auth.SignUp)
		m.Post("/sign_up", bindIgnErr(forms.RegisterForm{}), auth.SignUpPost)
		m.Get("/link_account", linkAccountEnabled, auth.LinkAccount)
		m.Post("/link_account_signin", linkAccountEnabled, bindIgnErr(forms.SignInForm{}), auth.LinkAccountPostSignIn)
		m.Post("/link_account_signup", linkAccountEnabled, bindIgnErr(forms.RegisterForm{}), auth.LinkAccountPostRegister)
		m.Group("/two_factor", func() {
			m.Get("", auth.TwoFactor)
			m.Post("", bindIgnErr(forms.TwoFactorAuthForm{}), auth.TwoFactorPost)
			m.Get("/scratch", auth.TwoFactorScratch)
			m.Post("/scratch", bindIgnErr(forms.TwoFactorScratchAuthForm{}), auth.TwoFactorScratchPost)
		})
		m.Group("/webauthn", func() {
			m.Get("", auth.WebAuthn)
			m.Get("/assertion", auth.WebAuthnLoginAssertion)
			m.Post("/assertion", auth.WebAuthnLoginAssertionPost)
		})
	}, reqSignOut)

	m.Any("/user/events", routing.MarkLongPolling, events.Events)

	m.Group("/login/oauth", func() {
		m.Get("/authorize", bindIgnErr(forms.AuthorizationForm{}), auth.AuthorizeOAuth)
		m.Post("/grant", bindIgnErr(forms.GrantApplicationForm{}), auth.GrantApplicationOAuth)
		// TODO manage redirection
		m.Post("/authorize", bindIgnErr(forms.AuthorizationForm{}), auth.AuthorizeOAuth)
	}, ignSignInAndCsrf, reqSignIn)
	m.Get("/login/oauth/userinfo", ignSignInAndCsrf, auth.InfoOAuth)
	m.Post("/login/oauth/access_token", CorsHandler(), bindIgnErr(forms.AccessTokenForm{}), ignSignInAndCsrf, auth.AccessTokenOAuth)
	m.Get("/login/oauth/keys", ignSignInAndCsrf, auth.OIDCKeys)
	m.Post("/login/oauth/introspect", CorsHandler(), bindIgnErr(forms.IntrospectTokenForm{}), ignSignInAndCsrf, auth.IntrospectOAuth)

	m.Group("/user/settings", func() {
		m.Get("", user_setting.Profile)
		m.Post("", bindIgnErr(forms.UpdateProfileForm{}), user_setting.ProfilePost)
		m.Get("/change_password", auth.MustChangePassword)
		m.Post("/change_password", bindIgnErr(forms.MustChangePasswordForm{}), auth.MustChangePasswordPost)
		m.Post("/avatar", bindIgnErr(forms.AvatarForm{}), user_setting.AvatarPost)
		m.Post("/avatar/delete", user_setting.DeleteAvatar)
		m.Group("/account", func() {
			m.Combo("").Get(user_setting.Account).Post(bindIgnErr(forms.ChangePasswordForm{}), user_setting.AccountPost)
			m.Post("/email", bindIgnErr(forms.AddEmailForm{}), user_setting.EmailPost)
			m.Post("/email/delete", user_setting.DeleteEmail)
			m.Post("/delete", user_setting.DeleteAccount)
		})
		m.Group("/appearance", func() {
			m.Get("", user_setting.Appearance)
			m.Post("/language", bindIgnErr(forms.UpdateLanguageForm{}), user_setting.UpdateUserLang)
			m.Post("/hidden_comments", user_setting.UpdateUserHiddenComments)
			m.Post("/theme", bindIgnErr(forms.UpdateThemeForm{}), user_setting.UpdateUIThemePost)
		})
		m.Group("/security", func() {
			m.Get("", security.Security)
			m.Group("/two_factor", func() {
				m.Post("/regenerate_scratch", security.RegenerateScratchTwoFactor)
				m.Post("/disable", security.DisableTwoFactor)
				m.Get("/enroll", security.EnrollTwoFactor)
				m.Post("/enroll", bindIgnErr(forms.TwoFactorAuthForm{}), security.EnrollTwoFactorPost)
			})
			m.Group("/webauthn", func() {
				m.Post("/request_register", bindIgnErr(forms.WebauthnRegistrationForm{}), security.WebAuthnRegister)
				m.Post("/register", security.WebauthnRegisterPost)
				m.Post("/delete", bindIgnErr(forms.WebauthnDeleteForm{}), security.WebauthnDelete)
			})
			m.Group("/openid", func() {
				m.Post("", bindIgnErr(forms.AddOpenIDForm{}), security.OpenIDPost)
				m.Post("/delete", security.DeleteOpenID)
				m.Post("/toggle_visibility", security.ToggleOpenIDVisibility)
			}, openIDSignInEnabled)
			m.Post("/account_link", linkAccountEnabled, security.DeleteAccountLink)
		})
		m.Group("/applications/oauth2", func() {
			m.Get("/{id}", user_setting.OAuth2ApplicationShow)
			m.Post("/{id}", bindIgnErr(forms.EditOAuth2ApplicationForm{}), user_setting.OAuthApplicationsEdit)
			m.Post("/{id}/regenerate_secret", user_setting.OAuthApplicationsRegenerateSecret)
			m.Post("", bindIgnErr(forms.EditOAuth2ApplicationForm{}), user_setting.OAuthApplicationsPost)
			m.Post("/delete", user_setting.DeleteOAuth2Application)
			m.Post("/revoke", user_setting.RevokeOAuth2Grant)
		})
		m.Combo("/applications").Get(user_setting.Applications).
			Post(bindIgnErr(forms.NewAccessTokenForm{}), user_setting.ApplicationsPost)
		m.Post("/applications/delete", user_setting.DeleteApplication)
		m.Combo("/keys").Get(user_setting.Keys).
			Post(bindIgnErr(forms.AddKeyForm{}), user_setting.KeysPost)
		m.Post("/keys/delete", user_setting.DeleteKey)
		m.Get("/organization", user_setting.Organization)
		m.Get("/repos", user_setting.Repos)
		m.Post("/repos/unadopted", user_setting.AdoptOrDeleteRepository)
	}, reqSignIn, func(ctx *context.Context) {
		ctx.Data["PageIsUserSettings"] = true
		ctx.Data["AllThemes"] = setting.UI.Themes
	})

	m.Group("/user", func() {
		// r.Get("/feeds", binding.Bind(auth.FeedsForm{}), user.Feeds)
		m.Get("/activate", auth.Activate)
		m.Post("/activate", auth.ActivatePost)
		m.Any("/activate_email", auth.ActivateEmail)
		m.Get("/avatar/{username}/{size}", user.AvatarByUserName)
		m.Get("/recover_account", auth.ResetPasswd)
		m.Post("/recover_account", auth.ResetPasswdPost)
		m.Get("/forgot_password", auth.ForgotPasswd)
		m.Post("/forgot_password", auth.ForgotPasswdPost)
		m.Post("/logout", auth.SignOut)
		m.Get("/task/{task}", reqSignIn, user.TaskStatus)
		m.Get("/stopwatches", reqSignIn, user.GetStopwatches)
		m.Get("/search", ignExploreSignIn, user.Search)
		m.Group("/oauth2", func() {
			m.Get("/{provider}", auth.SignInOAuth)
			m.Get("/{provider}/callback", auth.SignInOAuthCallback)
		})
	})
	// ***** END: User *****

	m.Get("/avatar/{hash}", user.AvatarByEmailHash)

	adminReq := context.Toggle(&context.ToggleOptions{SignInRequired: true, AdminRequired: true})

	// ***** START: Admin *****
	m.Group("/admin", func() {
		m.Get("", adminReq, admin.Dashboard)
		m.Post("", adminReq, bindIgnErr(forms.AdminDashboardForm{}), admin.DashboardPost)
		m.Get("/config", admin.Config)
		m.Post("/config/test_mail", admin.SendTestMail)
		m.Group("/monitor", func() {
			m.Get("", admin.Monitor)
			m.Get("/stacktrace", admin.GoroutineStacktrace)
			m.Post("/cancel/{pid}", admin.MonitorCancel)
			m.Group("/queue/{qid}", func() {
				m.Get("", admin.Queue)
				m.Post("/set", admin.SetQueueSettings)
				m.Post("/add", admin.AddWorkers)
				m.Post("/cancel/{pid}", admin.WorkerCancel)
				m.Post("/flush", admin.Flush)
				m.Post("/pause", admin.Pause)
				m.Post("/resume", admin.Resume)
			})
		})

		m.Group("/users", func() {
			m.Get("", admin.Users)
			m.Combo("/new").Get(admin.NewUser).Post(bindIgnErr(forms.AdminCreateUserForm{}), admin.NewUserPost)
			m.Combo("/{userid}").Get(admin.EditUser).Post(bindIgnErr(forms.AdminEditUserForm{}), admin.EditUserPost)
			m.Post("/{userid}/delete", admin.DeleteUser)
			m.Post("/{userid}/avatar", bindIgnErr(forms.AvatarForm{}), admin.AvatarPost)
			m.Post("/{userid}/avatar/delete", admin.DeleteAvatar)
		})

		m.Group("/emails", func() {
			m.Get("", admin.Emails)
			m.Post("/activate", admin.ActivateEmail)
		})

		m.Group("/orgs", func() {
			m.Get("", admin.Organizations)
		})

		m.Group("/repos", func() {
			m.Get("", admin.Repos)
			m.Combo("/unadopted").Get(admin.UnadoptedRepos).Post(admin.AdoptOrDeleteRepository)
			m.Post("/delete", admin.DeleteRepo)
		})

		if setting.Packages.Enabled {
			m.Group("/packages", func() {
				m.Get("", admin.Packages)
				m.Post("/delete", admin.DeletePackageVersion)
			})
		}

		m.Group("/hooks", func() {
			m.Get("", admin.DefaultOrSystemWebhooks)
			m.Post("/delete", admin.DeleteDefaultOrSystemWebhook)
			m.Group("/{id}", func() {
				m.Get("", repo.WebHooksEdit)
				m.Post("/replay/{uuid}", repo.ReplayWebhook)
			})
			m.Post("/gitea/{id}", bindIgnErr(forms.NewWebhookForm{}), repo.WebHooksEditPost)
			m.Post("/gogs/{id}", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksEditPost)
			m.Post("/slack/{id}", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksEditPost)
			m.Post("/discord/{id}", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
			m.Post("/dingtalk/{id}", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksEditPost)
			m.Post("/telegram/{id}", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksEditPost)
			m.Post("/matrix/{id}", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksEditPost)
			m.Post("/msteams/{id}", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksEditPost)
			m.Post("/feishu/{id}", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksEditPost)
			m.Post("/wechatwork/{id}", bindIgnErr(forms.NewWechatWorkHookForm{}), repo.WechatworkHooksEditPost)
			m.Post("/packagist/{id}", bindIgnErr(forms.NewPackagistHookForm{}), repo.PackagistHooksEditPost)
		}, webhooksEnabled)

		m.Group("/{configType:default-hooks|system-hooks}", func() {
			m.Get("/{type}/new", repo.WebhooksNew)
			m.Post("/gitea/new", bindIgnErr(forms.NewWebhookForm{}), repo.GiteaHooksNewPost)
			m.Post("/gogs/new", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksNewPost)
			m.Post("/slack/new", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksNewPost)
			m.Post("/discord/new", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
			m.Post("/dingtalk/new", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksNewPost)
			m.Post("/telegram/new", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksNewPost)
			m.Post("/matrix/new", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksNewPost)
			m.Post("/msteams/new", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksNewPost)
			m.Post("/feishu/new", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksNewPost)
			m.Post("/wechatwork/new", bindIgnErr(forms.NewWechatWorkHookForm{}), repo.WechatworkHooksNewPost)
			m.Post("/packagist/new", bindIgnErr(forms.NewPackagistHookForm{}), repo.PackagistHooksNewPost)
		})

		m.Group("/auths", func() {
			m.Get("", admin.Authentications)
			m.Combo("/new").Get(admin.NewAuthSource).Post(bindIgnErr(forms.AuthenticationForm{}), admin.NewAuthSourcePost)
			m.Combo("/{authid}").Get(admin.EditAuthSource).
				Post(bindIgnErr(forms.AuthenticationForm{}), admin.EditAuthSourcePost)
			m.Post("/{authid}/delete", admin.DeleteAuthSource)
		})

		m.Group("/notices", func() {
			m.Get("", admin.Notices)
			m.Post("/delete", admin.DeleteNotices)
			m.Post("/empty", admin.EmptyNotices)
		})
	}, adminReq)
	// ***** END: Admin *****

	m.Group("", func() {
		m.Get("/favicon.ico", func(ctx *context.Context) {
			ctx.ServeFile(path.Join(setting.StaticRootPath, "public/img/favicon.png"))
		})
		m.Group("/{username}", func() {
			m.Get(".png", func(ctx *context.Context) { ctx.Error(http.StatusNotFound) })
			m.Get(".keys", user.ShowSSHKeys)
			m.Get(".gpg", user.ShowGPGKeys)
			m.Get(".rss", feed.ShowUserFeedRSS)
			m.Get(".atom", feed.ShowUserFeedAtom)
			m.Get("", user.Profile)
		}, context_service.UserAssignmentWeb())
		m.Get("/attachments/{uuid}", repo.GetAttachment)
	}, ignSignIn)

	m.Post("/{username}", reqSignIn, context_service.UserAssignmentWeb(), user.Action)

	if !setting.IsProd {
		m.Get("/template/*", dev.TemplatePreview)
	}

	reqRepoAdmin := context.RequireRepoAdmin()
	reqRepoCodeWriter := context.RequireRepoWriter(unit.TypeCode)
	canEnableEditor := context.CanEnableEditor()
	reqRepoCodeReader := context.RequireRepoReader(unit.TypeCode)
	reqRepoReleaseWriter := context.RequireRepoWriter(unit.TypeReleases)
	reqRepoReleaseReader := context.RequireRepoReader(unit.TypeReleases)
	reqRepoWikiWriter := context.RequireRepoWriter(unit.TypeWiki)
	reqRepoIssueWriter := context.RequireRepoWriter(unit.TypeIssues)
	reqRepoIssueReader := context.RequireRepoReader(unit.TypeIssues)
	reqRepoPullsReader := context.RequireRepoReader(unit.TypePullRequests)
	reqRepoIssuesOrPullsWriter := context.RequireRepoWriterOr(unit.TypeIssues, unit.TypePullRequests)
	reqRepoIssuesOrPullsReader := context.RequireRepoReaderOr(unit.TypeIssues, unit.TypePullRequests)
	reqRepoProjectsReader := context.RequireRepoReader(unit.TypeProjects)
	reqRepoProjectsWriter := context.RequireRepoWriter(unit.TypeProjects)

	reqPackageAccess := func(accessMode perm.AccessMode) func(ctx *context.Context) {
		return func(ctx *context.Context) {
			if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
				ctx.NotFound("", nil)
			}
		}
	}

	// ***** START: Organization *****
	m.Group("/org", func() {
		m.Group("/{org}", func() {
			m.Get("/members", org.Members)
		}, context.OrgAssignment())
	}, ignSignIn)

	m.Group("/org", func() {
		m.Group("", func() {
			m.Get("/create", org.Create)
			m.Post("/create", bindIgnErr(forms.CreateOrgForm{}), org.CreatePost)
		})

		m.Group("/{org}", func() {
			m.Get("/dashboard", user.Dashboard)
			m.Get("/dashboard/{team}", user.Dashboard)
			m.Get("/issues", user.Issues)
			m.Get("/issues/{team}", user.Issues)
			m.Get("/pulls", user.Pulls)
			m.Get("/pulls/{team}", user.Pulls)
			m.Get("/milestones", reqMilestonesDashboardPageEnabled, user.Milestones)
			m.Get("/milestones/{team}", reqMilestonesDashboardPageEnabled, user.Milestones)
			m.Post("/members/action/{action}", org.MembersAction)
			m.Get("/teams", org.Teams)
		}, context.OrgAssignment(true, false, true))

		m.Group("/{org}", func() {
			m.Get("/teams/{team}", org.TeamMembers)
			m.Get("/teams/{team}/repositories", org.TeamRepositories)
			m.Post("/teams/{team}/action/{action}", org.TeamsAction)
			m.Post("/teams/{team}/action/repo/{action}", org.TeamsRepoAction)
		}, context.OrgAssignment(true, false, true))

		m.Group("/{org}", func() {
			m.Get("/teams/new", org.NewTeam)
			m.Post("/teams/new", bindIgnErr(forms.CreateTeamForm{}), org.NewTeamPost)
			m.Get("/teams/-/search", org.SearchTeam)
			m.Get("/teams/{team}/edit", org.EditTeam)
			m.Post("/teams/{team}/edit", bindIgnErr(forms.CreateTeamForm{}), org.EditTeamPost)
			m.Post("/teams/{team}/delete", org.DeleteTeam)

			m.Group("/settings", func() {
				m.Combo("").Get(org.Settings).
					Post(bindIgnErr(forms.UpdateOrgSettingForm{}), org.SettingsPost)
				m.Post("/avatar", bindIgnErr(forms.AvatarForm{}), org.SettingsAvatar)
				m.Post("/avatar/delete", org.SettingsDeleteAvatar)

				m.Group("/hooks", func() {
					m.Get("", org.Webhooks)
					m.Post("/delete", org.DeleteWebhook)
					m.Get("/{type}/new", repo.WebhooksNew)
					m.Post("/gitea/new", bindIgnErr(forms.NewWebhookForm{}), repo.GiteaHooksNewPost)
					m.Post("/gogs/new", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksNewPost)
					m.Post("/slack/new", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksNewPost)
					m.Post("/discord/new", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
					m.Post("/dingtalk/new", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksNewPost)
					m.Post("/telegram/new", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksNewPost)
					m.Post("/matrix/new", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksNewPost)
					m.Post("/msteams/new", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksNewPost)
					m.Post("/feishu/new", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksNewPost)
					m.Post("/wechatwork/new", bindIgnErr(forms.NewWechatWorkHookForm{}), repo.WechatworkHooksNewPost)
					m.Group("/{id}", func() {
						m.Get("", repo.WebHooksEdit)
						m.Post("/replay/{uuid}", repo.ReplayWebhook)
					})
					m.Post("/gitea/{id}", bindIgnErr(forms.NewWebhookForm{}), repo.WebHooksEditPost)
					m.Post("/gogs/{id}", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksEditPost)
					m.Post("/slack/{id}", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksEditPost)
					m.Post("/discord/{id}", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
					m.Post("/dingtalk/{id}", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksEditPost)
					m.Post("/telegram/{id}", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksEditPost)
					m.Post("/matrix/{id}", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksEditPost)
					m.Post("/msteams/{id}", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksEditPost)
					m.Post("/feishu/{id}", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksEditPost)
					m.Post("/wechatwork/{id}", bindIgnErr(forms.NewWechatWorkHookForm{}), repo.WechatworkHooksEditPost)
				}, webhooksEnabled)

				m.Group("/labels", func() {
					m.Get("", org.RetrieveLabels, org.Labels)
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
	m.Group("/repo", func() {
		m.Get("/create", repo.Create)
		m.Post("/create", bindIgnErr(forms.CreateRepoForm{}), repo.CreatePost)
		m.Get("/migrate", repo.Migrate)
		m.Post("/migrate", bindIgnErr(forms.MigrateRepoForm{}), repo.MigratePost)
		m.Group("/fork", func() {
			m.Combo("/{repoid}").Get(repo.Fork).
				Post(bindIgnErr(forms.CreateRepoForm{}), repo.ForkPost)
		}, context.RepoIDAssignment(), context.UnitTypes(), reqRepoCodeReader)
		m.Get("/search", repo.SearchRepo)
	}, reqSignIn)

	m.Group("/{username}/-", func() {
		if setting.Packages.Enabled {
			m.Group("/packages", func() {
				m.Get("", user.ListPackages)
				m.Group("/{type}/{name}", func() {
					m.Get("", user.RedirectToLastVersion)
					m.Get("/versions", user.ListPackageVersions)
					m.Group("/{version}", func() {
						m.Get("", user.ViewPackageVersion)
						m.Get("/files/{fileid}", user.DownloadPackageFile)
						m.Group("/settings", func() {
							m.Get("", user.PackageSettings)
							m.Post("", bindIgnErr(forms.PackageSettingForm{}), user.PackageSettingsPost)
						}, reqPackageAccess(perm.AccessModeWrite))
					})
				})
			}, ignSignIn, context.PackageAssignment(), reqPackageAccess(perm.AccessModeRead))
		}
	}, context_service.UserAssignmentWeb())

	// ***** Release Attachment Download without Signin
	m.Get("/{username}/{reponame}/releases/download/{vTag}/{fileName}", ignSignIn, context.RepoAssignment, repo.MustBeNotEmpty, repo.RedirectDownload)

	m.Group("/{username}/{reponame}", func() {
		m.Group("/settings", func() {
			m.Group("", func() {
				m.Combo("").Get(repo.Settings).
					Post(bindIgnErr(forms.RepoSettingForm{}), repo.SettingsPost)
			}, repo.SettingsCtxData)
			m.Post("/avatar", bindIgnErr(forms.AvatarForm{}), repo.SettingsAvatar)
			m.Post("/avatar/delete", repo.SettingsDeleteAvatar)

			m.Group("/collaboration", func() {
				m.Combo("").Get(repo.Collaboration).Post(repo.CollaborationPost)
				m.Post("/access_mode", repo.ChangeCollaborationAccessMode)
				m.Post("/delete", repo.DeleteCollaboration)
				m.Group("/team", func() {
					m.Post("", repo.AddTeamPost)
					m.Post("/delete", repo.DeleteTeam)
				})
			})

			m.Group("/branches", func() {
				m.Combo("").Get(repo.ProtectedBranch).Post(repo.ProtectedBranchPost)
				m.Combo("/*").Get(repo.SettingsProtectedBranch).
					Post(bindIgnErr(forms.ProtectBranchForm{}), context.RepoMustNotBeArchived(), repo.SettingsProtectedBranchPost)
			}, repo.MustBeNotEmpty)
			m.Post("/rename_branch", bindIgnErr(forms.RenameBranchForm{}), context.RepoMustNotBeArchived(), repo.RenameBranchPost)

			m.Group("/tags", func() {
				m.Get("", repo.Tags)
				m.Post("", bindIgnErr(forms.ProtectTagForm{}), context.RepoMustNotBeArchived(), repo.NewProtectedTagPost)
				m.Post("/delete", context.RepoMustNotBeArchived(), repo.DeleteProtectedTagPost)
				m.Get("/{id}", repo.EditProtectedTag)
				m.Post("/{id}", bindIgnErr(forms.ProtectTagForm{}), context.RepoMustNotBeArchived(), repo.EditProtectedTagPost)
			})

			m.Group("/hooks/git", func() {
				m.Get("", repo.GitHooks)
				m.Combo("/{name}").Get(repo.GitHooksEdit).
					Post(repo.GitHooksEditPost)
			}, context.GitHookService())

			m.Group("/hooks", func() {
				m.Get("", repo.Webhooks)
				m.Post("/delete", repo.DeleteWebhook)
				m.Get("/{type}/new", repo.WebhooksNew)
				m.Post("/gitea/new", bindIgnErr(forms.NewWebhookForm{}), repo.GiteaHooksNewPost)
				m.Post("/gogs/new", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksNewPost)
				m.Post("/slack/new", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksNewPost)
				m.Post("/discord/new", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
				m.Post("/dingtalk/new", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksNewPost)
				m.Post("/telegram/new", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksNewPost)
				m.Post("/matrix/new", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksNewPost)
				m.Post("/msteams/new", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksNewPost)
				m.Post("/feishu/new", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksNewPost)
				m.Post("/wechatwork/new", bindIgnErr(forms.NewWechatWorkHookForm{}), repo.WechatworkHooksNewPost)
				m.Post("/packagist/new", bindIgnErr(forms.NewPackagistHookForm{}), repo.PackagistHooksNewPost)
				m.Group("/{id}", func() {
					m.Get("", repo.WebHooksEdit)
					m.Post("/test", repo.TestWebhook)
					m.Post("/replay/{uuid}", repo.ReplayWebhook)
				})
				m.Post("/gitea/{id}", bindIgnErr(forms.NewWebhookForm{}), repo.WebHooksEditPost)
				m.Post("/gogs/{id}", bindIgnErr(forms.NewGogshookForm{}), repo.GogsHooksEditPost)
				m.Post("/slack/{id}", bindIgnErr(forms.NewSlackHookForm{}), repo.SlackHooksEditPost)
				m.Post("/discord/{id}", bindIgnErr(forms.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
				m.Post("/dingtalk/{id}", bindIgnErr(forms.NewDingtalkHookForm{}), repo.DingtalkHooksEditPost)
				m.Post("/telegram/{id}", bindIgnErr(forms.NewTelegramHookForm{}), repo.TelegramHooksEditPost)
				m.Post("/matrix/{id}", bindIgnErr(forms.NewMatrixHookForm{}), repo.MatrixHooksEditPost)
				m.Post("/msteams/{id}", bindIgnErr(forms.NewMSTeamsHookForm{}), repo.MSTeamsHooksEditPost)
				m.Post("/feishu/{id}", bindIgnErr(forms.NewFeishuHookForm{}), repo.FeishuHooksEditPost)
				m.Post("/wechatwork/{id}", bindIgnErr(forms.NewWechatWorkHookForm{}), repo.WechatworkHooksEditPost)
				m.Post("/packagist/{id}", bindIgnErr(forms.NewPackagistHookForm{}), repo.PackagistHooksEditPost)
			}, webhooksEnabled)

			m.Group("/keys", func() {
				m.Combo("").Get(repo.DeployKeys).
					Post(bindIgnErr(forms.AddKeyForm{}), repo.DeployKeysPost)
				m.Post("/delete", repo.DeleteDeployKey)
			})

			m.Group("/lfs", func() {
				m.Get("/", repo.LFSFiles)
				m.Get("/show/{oid}", repo.LFSFileGet)
				m.Post("/delete/{oid}", repo.LFSDelete)
				m.Get("/pointers", repo.LFSPointerFiles)
				m.Post("/pointers/associate", repo.LFSAutoAssociate)
				m.Get("/find", repo.LFSFileFind)
				m.Group("/locks", func() {
					m.Get("/", repo.LFSLocks)
					m.Post("/", repo.LFSLockFile)
					m.Post("/{lid}/unlock", repo.LFSUnlock)
				})
			})
		}, func(ctx *context.Context) {
			ctx.Data["PageIsSettings"] = true
			ctx.Data["LFSStartServer"] = setting.LFS.StartServer
		})
	}, reqSignIn, context.RepoAssignment, context.UnitTypes(), reqRepoAdmin, context.RepoRef())

	m.Post("/{username}/{reponame}/action/{action}", reqSignIn, context.RepoAssignment, context.UnitTypes(), repo.Action)

	// Grouping for those endpoints not requiring authentication
	m.Group("/{username}/{reponame}", func() {
		m.Group("/milestone", func() {
			m.Get("/{id}", repo.MilestoneIssuesAndPulls)
		}, reqRepoIssuesOrPullsReader, context.RepoRef())
		m.Get("/find/*", repo.FindFiles)
		m.Group("/tree-list", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.TreeList)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.TreeList)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.TreeList)
		})
		m.Get("/compare", repo.MustBeNotEmpty, reqRepoCodeReader, repo.SetEditorconfigIfExists, ignSignIn, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.CompareDiff)
		m.Combo("/compare/*", repo.MustBeNotEmpty, reqRepoCodeReader, repo.SetEditorconfigIfExists).
			Get(ignSignIn, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.CompareDiff).
			Post(reqSignIn, context.RepoMustNotBeArchived(), reqRepoPullsReader, repo.MustAllowPulls, bindIgnErr(forms.CreateIssueForm{}), repo.SetWhitespaceBehavior, repo.CompareAndPullRequestPost)
		m.Group("/{type:issues|pulls}", func() {
			m.Group("/{index}", func() {
				m.Get("/info", repo.GetIssueInfo)
			})
		})
	}, context.RepoAssignment, context.UnitTypes())

	// Grouping for those endpoints that do require authentication
	m.Group("/{username}/{reponame}", func() {
		m.Group("/issues", func() {
			m.Group("/new", func() {
				m.Combo("").Get(context.RepoRef(), repo.NewIssue).
					Post(bindIgnErr(forms.CreateIssueForm{}), repo.NewIssuePost)
				m.Get("/choose", context.RepoRef(), repo.NewIssueChooseTemplate)
			})
			m.Get("/search", repo.ListIssues)
		}, context.RepoMustNotBeArchived(), reqRepoIssueReader)
		// FIXME: should use different URLs but mostly same logic for comments of issue and pull request.
		// So they can apply their own enable/disable logic on routers.
		m.Group("/{type:issues|pulls}", func() {
			m.Group("/{index}", func() {
				m.Post("/title", repo.UpdateIssueTitle)
				m.Post("/content", repo.UpdateIssueContent)
				m.Post("/deadline", bindIgnErr(structs.EditDeadlineOption{}), repo.UpdateIssueDeadline)
				m.Post("/watch", repo.IssueWatch)
				m.Post("/ref", repo.UpdateIssueRef)
				m.Post("/viewed-files", repo.UpdateViewedFiles)
				m.Group("/dependency", func() {
					m.Post("/add", repo.AddDependency)
					m.Post("/delete", repo.RemoveDependency)
				})
				m.Combo("/comments").Post(repo.MustAllowUserComment, bindIgnErr(forms.CreateCommentForm{}), repo.NewComment)
				m.Group("/times", func() {
					m.Post("/add", bindIgnErr(forms.AddTimeManuallyForm{}), repo.AddTimeManually)
					m.Post("/{timeid}/delete", repo.DeleteTime)
					m.Group("/stopwatch", func() {
						m.Post("/toggle", repo.IssueStopwatch)
						m.Post("/cancel", repo.CancelStopwatch)
					})
				})
				m.Post("/reactions/{action}", bindIgnErr(forms.ReactionForm{}), repo.ChangeIssueReaction)
				m.Post("/lock", reqRepoIssueWriter, bindIgnErr(forms.IssueLockForm{}), repo.LockIssue)
				m.Post("/unlock", reqRepoIssueWriter, repo.UnlockIssue)
				m.Post("/delete", reqRepoAdmin, repo.DeleteIssue)
			}, context.RepoMustNotBeArchived())
			m.Group("/{index}", func() {
				m.Get("/attachments", repo.GetIssueAttachments)
				m.Get("/attachments/{uuid}", repo.GetAttachment)
			})
			m.Group("/{index}", func() {
				m.Post("/content-history/soft-delete", repo.SoftDeleteContentHistory)
			})

			m.Post("/labels", reqRepoIssuesOrPullsWriter, repo.UpdateIssueLabel)
			m.Post("/milestone", reqRepoIssuesOrPullsWriter, repo.UpdateIssueMilestone)
			m.Post("/projects", reqRepoIssuesOrPullsWriter, reqRepoProjectsReader, repo.UpdateIssueProject)
			m.Post("/assignee", reqRepoIssuesOrPullsWriter, repo.UpdateIssueAssignee)
			m.Post("/request_review", reqRepoIssuesOrPullsReader, repo.UpdatePullReviewRequest)
			m.Post("/dismiss_review", reqRepoAdmin, bindIgnErr(forms.DismissReviewForm{}), repo.DismissReview)
			m.Post("/status", reqRepoIssuesOrPullsWriter, repo.UpdateIssueStatus)
			m.Post("/resolve_conversation", reqRepoIssuesOrPullsReader, repo.UpdateResolveConversation)
			m.Post("/attachments", repo.UploadIssueAttachment)
			m.Post("/attachments/remove", repo.DeleteAttachment)
		}, context.RepoMustNotBeArchived())
		m.Group("/comments/{id}", func() {
			m.Post("", repo.UpdateCommentContent)
			m.Post("/delete", repo.DeleteComment)
			m.Post("/reactions/{action}", bindIgnErr(forms.ReactionForm{}), repo.ChangeCommentReaction)
		}, context.RepoMustNotBeArchived())
		m.Group("/comments/{id}", func() {
			m.Get("/attachments", repo.GetCommentAttachments)
		})
		m.Post("/markdown", bindIgnErr(structs.MarkdownOption{}), misc.Markdown)
		m.Group("/labels", func() {
			m.Post("/new", bindIgnErr(forms.CreateLabelForm{}), repo.NewLabel)
			m.Post("/edit", bindIgnErr(forms.CreateLabelForm{}), repo.UpdateLabel)
			m.Post("/delete", repo.DeleteLabel)
			m.Post("/initialize", bindIgnErr(forms.InitializeLabelsForm{}), repo.InitializeLabels)
		}, context.RepoMustNotBeArchived(), reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Group("/milestones", func() {
			m.Combo("/new").Get(repo.NewMilestone).
				Post(bindIgnErr(forms.CreateMilestoneForm{}), repo.NewMilestonePost)
			m.Get("/{id}/edit", repo.EditMilestone)
			m.Post("/{id}/edit", bindIgnErr(forms.CreateMilestoneForm{}), repo.EditMilestonePost)
			m.Post("/{id}/{action}", repo.ChangeMilestoneStatus)
			m.Post("/delete", repo.DeleteMilestone)
		}, context.RepoMustNotBeArchived(), reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Group("/pull", func() {
			m.Post("/{index}/target_branch", repo.UpdatePullRequestTarget)
		}, context.RepoMustNotBeArchived())

		m.Group("", func() {
			m.Group("", func() {
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
				m.Combo("/_diffpatch/*").Get(repo.NewDiffPatch).
					Post(bindIgnErr(forms.EditRepoFileForm{}), repo.NewDiffPatchPost)
				m.Combo("/_cherrypick/{sha:([a-f0-9]{7,40})}/*").Get(repo.CherryPick).
					Post(bindIgnErr(forms.CherryPickForm{}), repo.CherryPickPost)
			}, repo.MustBeEditable)
			m.Group("", func() {
				m.Post("/upload-file", repo.UploadFileToServer)
				m.Post("/upload-remove", bindIgnErr(forms.RemoveUploadFileForm{}), repo.RemoveUploadFileFromServer)
			}, repo.MustBeEditable, repo.MustBeAbleToUpload)
		}, context.RepoRef(), canEnableEditor, context.RepoMustNotBeArchived(), repo.MustBeNotEmpty)

		m.Group("/branches", func() {
			m.Group("/_new", func() {
				m.Post("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.CreateBranch)
				m.Post("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.CreateBranch)
				m.Post("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.CreateBranch)
			}, bindIgnErr(forms.NewBranchForm{}))
			m.Post("/delete", repo.DeleteBranchPost)
			m.Post("/restore", repo.RestoreBranchPost)
		}, context.RepoMustNotBeArchived(), reqRepoCodeWriter, repo.MustBeNotEmpty)
	}, reqSignIn, context.RepoAssignment, context.UnitTypes())

	// Releases
	m.Group("/{username}/{reponame}", func() {
		m.Get("/tags", repo.TagsList, repo.MustBeNotEmpty,
			reqRepoCodeReader, context.RepoRefByType(context.RepoRefTag))
		m.Group("/releases", func() {
			m.Get("/", repo.Releases)
			m.Get("/tag/*", repo.SingleRelease)
			m.Get("/latest", repo.LatestRelease)
		}, repo.MustBeNotEmpty, reqRepoReleaseReader, context.RepoRefByType(context.RepoRefTag, true))
		m.Get("/releases/attachments/{uuid}", repo.GetAttachment, repo.MustBeNotEmpty, reqRepoReleaseReader)
		m.Group("/releases", func() {
			m.Get("/new", repo.NewRelease)
			m.Post("/new", bindIgnErr(forms.NewReleaseForm{}), repo.NewReleasePost)
			m.Post("/delete", repo.DeleteRelease)
			m.Post("/attachments", repo.UploadReleaseAttachment)
			m.Post("/attachments/remove", repo.DeleteAttachment)
		}, reqSignIn, repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoReleaseWriter, context.RepoRef())
		m.Post("/tags/delete", repo.DeleteTag, reqSignIn,
			repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoCodeWriter, context.RepoRef())
		m.Group("/releases", func() {
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
			ctx.Repo.GitRepo.LastCommitCache = git.NewLastCommitCache(ctx.Repo.CommitsCount, ctx.Repo.Repository.FullName(), ctx.Repo.GitRepo, cache.GetCache())
		})
	}, ignSignIn, context.RepoAssignment, context.UnitTypes(), reqRepoReleaseReader)

	// to maintain compatibility with old attachments
	m.Group("/{username}/{reponame}", func() {
		m.Get("/attachments/{uuid}", repo.GetAttachment)
	}, ignSignIn, context.RepoAssignment, context.UnitTypes())

	m.Group("/{username}/{reponame}", func() {
		m.Post("/topics", repo.TopicsPost)
	}, context.RepoAssignment, context.RepoMustNotBeArchived(), reqRepoAdmin)

	m.Group("/{username}/{reponame}", func() {
		m.Group("", func() {
			m.Get("/{type:issues|pulls}", repo.Issues)
			m.Get("/{type:issues|pulls}/{index}", repo.ViewIssue)
			m.Group("/{type:issues|pulls}/{index}/content-history", func() {
				m.Get("/overview", repo.GetContentHistoryOverview)
				m.Get("/list", repo.GetContentHistoryList)
				m.Get("/detail", repo.GetContentHistoryDetail)
			})
			m.Get("/labels", reqRepoIssuesOrPullsReader, repo.RetrieveLabels, repo.Labels)
			m.Get("/milestones", reqRepoIssuesOrPullsReader, repo.Milestones)
		}, context.RepoRef())

		if setting.Packages.Enabled {
			m.Get("/packages", repo.Packages)
		}

		m.Group("/projects", func() {
			m.Get("", repo.Projects)
			m.Get("/{id}", repo.ViewProject)
			m.Group("", func() {
				m.Get("/new", repo.NewProject)
				m.Post("/new", bindIgnErr(forms.CreateProjectForm{}), repo.NewProjectPost)
				m.Group("/{id}", func() {
					m.Post("", bindIgnErr(forms.EditProjectBoardForm{}), repo.AddBoardToProjectPost)
					m.Post("/delete", repo.DeleteProject)

					m.Get("/edit", repo.EditProject)
					m.Post("/edit", bindIgnErr(forms.CreateProjectForm{}), repo.EditProjectPost)
					m.Post("/{action:open|close}", repo.ChangeProjectStatus)

					m.Group("/{boardID}", func() {
						m.Put("", bindIgnErr(forms.EditProjectBoardForm{}), repo.EditProjectBoard)
						m.Delete("", repo.DeleteProjectBoard)
						m.Post("/default", repo.SetDefaultProjectBoard)

						m.Post("/move", repo.MoveIssues)
					})
				})
			}, reqRepoProjectsWriter, context.RepoMustNotBeArchived())
		}, reqRepoProjectsReader, repo.MustEnableProjects)

		m.Group("/wiki", func() {
			m.Combo("/").
				Get(repo.Wiki).
				Post(context.RepoMustNotBeArchived(),
					reqSignIn,
					reqRepoWikiWriter,
					bindIgnErr(forms.NewWikiForm{}),
					repo.WikiPost)
			m.Combo("/*").
				Get(repo.Wiki).
				Post(context.RepoMustNotBeArchived(),
					reqSignIn,
					reqRepoWikiWriter,
					bindIgnErr(forms.NewWikiForm{}),
					repo.WikiPost)
			m.Get("/commit/{sha:[a-f0-9]{7,40}}", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.Diff)
			m.Get("/commit/{sha:[a-f0-9]{7,40}}.{ext:patch|diff}", repo.RawDiff)
		}, repo.MustEnableWiki, func(ctx *context.Context) {
			ctx.Data["PageIsWiki"] = true
			ctx.Data["CloneButtonOriginLink"] = ctx.Repo.Repository.WikiCloneLink()
		})

		m.Group("/wiki", func() {
			m.Get("/raw/*", repo.WikiRaw)
		}, repo.MustEnableWiki)

		m.Group("/activity", func() {
			m.Get("", repo.Activity)
			m.Get("/{period}", repo.Activity)
		}, context.RepoRef(), repo.MustBeNotEmpty, context.RequireRepoReaderOr(unit.TypePullRequests, unit.TypeIssues, unit.TypeReleases))

		m.Group("/activity_author_data", func() {
			m.Get("", repo.ActivityAuthors)
			m.Get("/{period}", repo.ActivityAuthors)
		}, context.RepoRef(), repo.MustBeNotEmpty, context.RequireRepoReaderOr(unit.TypeCode))

		m.Group("/archive", func() {
			m.Get("/*", repo.Download)
			m.Post("/*", repo.InitiateDownload)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/branches", func() {
			m.Get("", repo.Branches)
		}, repo.MustBeNotEmpty, context.RepoRef(), reqRepoCodeReader)

		m.Group("/blob_excerpt", func() {
			m.Get("/{sha}", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.ExcerptBlob)
		}, func(ctx *context.Context) (cancel gocontext.CancelFunc) {
			if ctx.FormBool("wiki") {
				ctx.Data["PageIsWiki"] = true
				repo.MustEnableWiki(ctx)
				return
			}

			reqRepoCodeReader(ctx)
			if ctx.Written() {
				return
			}
			cancel = context.RepoRef()(ctx)
			if ctx.Written() {
				return
			}

			repo.MustBeNotEmpty(ctx)
			return cancel
		})

		m.Group("/pulls/{index}", func() {
			m.Get(".diff", repo.DownloadPullDiff)
			m.Get(".patch", repo.DownloadPullPatch)
			m.Get("/commits", context.RepoRef(), repo.ViewPullCommits)
			m.Post("/merge", context.RepoMustNotBeArchived(), bindIgnErr(forms.MergePullRequestForm{}), repo.MergePullRequest)
			m.Post("/cancel_auto_merge", context.RepoMustNotBeArchived(), repo.CancelAutoMergePullRequest)
			m.Post("/update", repo.UpdatePullRequest)
			m.Post("/set_allow_maintainer_edit", bindIgnErr(forms.UpdateAllowEditsForm{}), repo.SetAllowEdits)
			m.Post("/cleanup", context.RepoMustNotBeArchived(), context.RepoRef(), repo.CleanUpPullRequest)
			m.Group("/files", func() {
				m.Get("", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.ViewPullFiles)
				m.Group("/reviews", func() {
					m.Get("/new_comment", repo.RenderNewCodeCommentForm)
					m.Post("/comments", bindIgnErr(forms.CodeCommentForm{}), repo.CreateCodeComment)
					m.Post("/submit", bindIgnErr(forms.SubmitReviewForm{}), repo.SubmitReview)
				}, context.RepoMustNotBeArchived())
			})
		}, repo.MustAllowPulls)

		m.Group("/media", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.SingleDownloadOrLFS)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.SingleDownloadOrLFS)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.SingleDownloadOrLFS)
			m.Get("/blob/{sha}", context.RepoRefByType(context.RepoRefBlob), repo.DownloadByIDOrLFS)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.SingleDownloadOrLFS)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/raw", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.SingleDownload)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.SingleDownload)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.SingleDownload)
			m.Get("/blob/{sha}", context.RepoRefByType(context.RepoRefBlob), repo.DownloadByID)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.SingleDownload)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/render", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RenderFile)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RenderFile)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RenderFile)
			m.Get("/blob/{sha}", context.RepoRefByType(context.RepoRefBlob), repo.RenderFile)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/commits", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RefCommits)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RefCommits)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RefCommits)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.RefCommits)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/blame", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RefBlame)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RefBlame)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RefBlame)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("", func() {
			m.Get("/graph", repo.Graph)
			m.Get("/commit/{sha:([a-f0-9]{7,40})$}", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.Diff)
			m.Get("/cherry-pick/{sha:([a-f0-9]{7,40})$}", repo.SetEditorconfigIfExists, repo.CherryPick)
		}, repo.MustBeNotEmpty, context.RepoRef(), reqRepoCodeReader)

		m.Group("/src", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.Home)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.Home)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.Home)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.Home)
		}, repo.SetEditorconfigIfExists)

		m.Group("", func() {
			m.Get("/forks", repo.Forks)
		}, context.RepoRef(), reqRepoCodeReader)
		m.Get("/commit/{sha:([a-f0-9]{7,40})}.{ext:patch|diff}",
			repo.MustBeNotEmpty, reqRepoCodeReader, repo.RawDiff)
	}, ignSignIn, context.RepoAssignment, context.UnitTypes())

	m.Post("/{username}/{reponame}/lastcommit/*", ignSignInAndCsrf, context.RepoAssignment, context.UnitTypes(), context.RepoRefByType(context.RepoRefCommit), reqRepoCodeReader, repo.LastCommit)

	m.Group("/{username}/{reponame}", func() {
		m.Get("/stars", repo.Stars)
		m.Get("/watchers", repo.Watchers)
		m.Get("/search", reqRepoCodeReader, repo.Search)
	}, ignSignIn, context.RepoAssignment, context.RepoRef(), context.UnitTypes())

	m.Group("/{username}", func() {
		m.Group("/{reponame}", func() {
			m.Get("", repo.SetEditorconfigIfExists, repo.Home)
		}, ignSignIn, context.RepoAssignment, context.RepoRef(), context.UnitTypes())

		m.Group("/{reponame}", func() {
			m.Group("/info/lfs", func() {
				m.Post("/objects/batch", lfs.CheckAcceptMediaType, lfs.BatchHandler)
				m.Put("/objects/{oid}/{size}", lfs.UploadHandler)
				m.Get("/objects/{oid}/{filename}", lfs.DownloadHandler)
				m.Get("/objects/{oid}", lfs.DownloadHandler)
				m.Post("/verify", lfs.CheckAcceptMediaType, lfs.VerifyHandler)
				m.Group("/locks", func() {
					m.Get("/", lfs.GetListLockHandler)
					m.Post("/", lfs.PostLockHandler)
					m.Post("/verify", lfs.VerifyLockHandler)
					m.Post("/{lid}/unlock", lfs.UnLockHandler)
				}, lfs.CheckAcceptMediaType)
				m.Any("/*", func(ctx *context.Context) {
					ctx.NotFound("", nil)
				})
			}, ignSignInAndCsrf, lfsServerEnabled)

			m.Group("", func() {
				m.PostOptions("/git-upload-pack", repo.ServiceUploadPack)
				m.PostOptions("/git-receive-pack", repo.ServiceReceivePack)
				m.GetOptions("/info/refs", repo.GetInfoRefs)
				m.GetOptions("/HEAD", repo.GetTextFile("HEAD"))
				m.GetOptions("/objects/info/alternates", repo.GetTextFile("objects/info/alternates"))
				m.GetOptions("/objects/info/http-alternates", repo.GetTextFile("objects/info/http-alternates"))
				m.GetOptions("/objects/info/packs", repo.GetInfoPacks)
				m.GetOptions("/objects/info/{file:[^/]*}", repo.GetTextFile(""))
				m.GetOptions("/objects/{head:[0-9a-f]{2}}/{hash:[0-9a-f]{38}}", repo.GetLooseObject)
				m.GetOptions("/objects/pack/pack-{file:[0-9a-f]{40}}.pack", repo.GetPackFile)
				m.GetOptions("/objects/pack/pack-{file:[0-9a-f]{40}}.idx", repo.GetIdxFile)
			}, ignSignInAndCsrf, context_service.UserAssignmentWeb())
		})
	})
	// ***** END: Repository *****

	m.Group("/notifications", func() {
		m.Get("", user.Notifications)
		m.Post("/status", user.NotificationStatusPost)
		m.Post("/purge", user.NotificationPurgePost)
		m.Get("/new", user.NewAvailable)
	}, reqSignIn)

	if setting.API.EnableSwagger {
		m.Get("/swagger.v1.json", SwaggerV1Json)
	}
	m.NotFound(func(w http.ResponseWriter, req *http.Request) {
		ctx := context.GetContext(req)
		ctx.NotFound("", nil)
	})
}
