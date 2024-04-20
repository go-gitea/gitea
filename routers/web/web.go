// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	gocontext "context"
	"net/http"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/metrics"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/modules/web/routing"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/routers/web/admin"
	"code.gitea.io/gitea/routers/web/auth"
	"code.gitea.io/gitea/routers/web/devtest"
	"code.gitea.io/gitea/routers/web/events"
	"code.gitea.io/gitea/routers/web/explore"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/routers/web/healthcheck"
	"code.gitea.io/gitea/routers/web/misc"
	"code.gitea.io/gitea/routers/web/org"
	org_setting "code.gitea.io/gitea/routers/web/org/setting"
	"code.gitea.io/gitea/routers/web/repo"
	"code.gitea.io/gitea/routers/web/repo/actions"
	repo_setting "code.gitea.io/gitea/routers/web/repo/setting"
	"code.gitea.io/gitea/routers/web/user"
	user_setting "code.gitea.io/gitea/routers/web/user/setting"
	"code.gitea.io/gitea/routers/web/user/setting/security"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/lfs"

	_ "code.gitea.io/gitea/modules/session" // to registers all internal adapters

	"gitea.com/go-chi/captcha"
	chi_middleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/klauspost/compress/gzhttp"
	"github.com/prometheus/client_golang/prometheus"
)

const GzipMinSize = 1400 // min size to compress for the body size of response

// optionsCorsHandler return a http handler which sets CORS options if enabled by config, it blocks non-CORS OPTIONS requests.
func optionsCorsHandler() func(next http.Handler) http.Handler {
	var corsHandler func(next http.Handler) http.Handler
	if setting.CORSConfig.Enabled {
		corsHandler = cors.Handler(cors.Options{
			AllowedOrigins:   setting.CORSConfig.AllowDomain,
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			AllowedHeaders:   setting.CORSConfig.Headers,
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		})
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				if corsHandler != nil && r.Header.Get("Access-Control-Request-Method") != "" {
					corsHandler(next).ServeHTTP(w, r)
				} else {
					// it should explicitly deny OPTIONS requests if CORS handler is not executed, to avoid the next GET/POST handler being incorrectly called by the OPTIONS request
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
				return
			}
			// for non-OPTIONS requests, call the CORS handler to add some related headers like "Vary"
			if corsHandler != nil {
				corsHandler(next).ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
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

	if setting.IsWindows && auth_model.IsSSPIEnabled(db.DefaultContext) {
		group.Add(&auth_service.SSPI{}) // it MUST be the last, see the comment of SSPI
	}

	return group
}

func webAuth(authMethod auth_service.Method) func(*context.Context) {
	return func(ctx *context.Context) {
		ar, err := common.AuthShared(ctx.Base, ctx.Session, authMethod)
		if err != nil {
			log.Error("Failed to verify user: %v", err)
			ctx.Error(http.StatusUnauthorized, "Verify")
			return
		}
		ctx.Doer = ar.Doer
		ctx.IsSigned = ar.Doer != nil
		ctx.IsBasicAuth = ar.IsBasicAuth
		if ctx.Doer == nil {
			// ensure the session uid is deleted
			_ = ctx.Session.Delete("uid")
		}
	}
}

// verifyAuthWithOptions checks authentication according to options
func verifyAuthWithOptions(options *common.VerifyOptions) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		// Check prohibit login users.
		if ctx.IsSigned {
			if !ctx.Doer.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, "user/auth/activate")
				return
			}
			if !ctx.Doer.IsActive || ctx.Doer.ProhibitLogin {
				log.Info("Failed authentication attempt for %s from %s", ctx.Doer.Name, ctx.RemoteAddr())
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
				return
			}

			if ctx.Doer.MustChangePassword {
				if ctx.Req.URL.Path != "/user/settings/change_password" {
					if strings.HasPrefix(ctx.Req.UserAgent(), "git") {
						ctx.Error(http.StatusUnauthorized, ctx.Locale.TrString("auth.must_change_password"))
						return
					}
					ctx.Data["Title"] = ctx.Tr("auth.must_change_password")
					ctx.Data["ChangePasscodeLink"] = setting.AppSubURL + "/user/change_password"
					if ctx.Req.URL.Path != "/user/events" {
						middleware.SetRedirectToCookie(ctx.Resp, setting.AppSubURL+ctx.Req.URL.RequestURI())
					}
					ctx.Redirect(setting.AppSubURL + "/user/settings/change_password")
					return
				}
			} else if ctx.Req.URL.Path == "/user/settings/change_password" {
				// make sure that the form cannot be accessed by users who don't need this
				ctx.Redirect(setting.AppSubURL + "/")
				return
			}
		}

		// Redirect to dashboard (or alternate location) if user tries to visit any non-login page.
		if options.SignOutRequired && ctx.IsSigned && ctx.Req.URL.RequestURI() != "/" {
			ctx.RedirectToCurrentSite(ctx.FormString("redirect_to"))
			return
		}

		if !options.SignOutRequired && !options.DisableCSRF && ctx.Req.Method == "POST" {
			ctx.Csrf.Validate(ctx)
			if ctx.Written() {
				return
			}
		}

		if options.SignInRequired {
			if !ctx.IsSigned {
				if ctx.Req.URL.Path != "/user/events" {
					middleware.SetRedirectToCookie(ctx.Resp, setting.AppSubURL+ctx.Req.URL.RequestURI())
				}
				ctx.Redirect(setting.AppSubURL + "/user/login")
				return
			} else if !ctx.Doer.IsActive && setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, "user/auth/activate")
				return
			}
		}

		// Redirect to log in page if auto-signin info is provided and has not signed in.
		if !options.SignOutRequired && !ctx.IsSigned &&
			ctx.GetSiteCookie(setting.CookieRememberName) != "" {
			if ctx.Req.URL.Path != "/user/events" {
				middleware.SetRedirectToCookie(ctx.Resp, setting.AppSubURL+ctx.Req.URL.RequestURI())
			}
			ctx.Redirect(setting.AppSubURL + "/user/login")
			return
		}

		if options.AdminRequired {
			if !ctx.Doer.IsAdmin {
				ctx.Error(http.StatusForbidden)
				return
			}
			ctx.Data["PageIsAdmin"] = true
		}
	}
}

func ctxDataSet(args ...any) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		for i := 0; i < len(args); i += 2 {
			ctx.Data[args[i].(string)] = args[i+1]
		}
	}
}

// Routes returns all web routes
func Routes() *web.Route {
	routes := web.NewRoute()

	routes.Head("/", misc.DummyOK) // for health check - doesn't need to be passed through gzip handler
	routes.Methods("GET, HEAD, OPTIONS", "/assets/*", optionsCorsHandler(), public.FileHandlerFunc())
	routes.Methods("GET, HEAD", "/avatars/*", storageHandler(setting.Avatar.Storage, "avatars", storage.Avatars))
	routes.Methods("GET, HEAD", "/repo-avatars/*", storageHandler(setting.RepoAvatar.Storage, "repo-avatars", storage.RepoAvatars))
	routes.Methods("GET, HEAD", "/apple-touch-icon.png", misc.StaticRedirect("/assets/img/apple-touch-icon.png"))
	routes.Methods("GET, HEAD", "/apple-touch-icon-precomposed.png", misc.StaticRedirect("/assets/img/apple-touch-icon.png"))
	routes.Methods("GET, HEAD", "/favicon.ico", misc.StaticRedirect("/assets/img/favicon.png"))

	_ = templates.HTMLRenderer()

	var mid []any

	if setting.EnableGzip {
		// random jitter is recommended by: https://pkg.go.dev/github.com/klauspost/compress/gzhttp#readme-breach-mitigation
		// compression level 6 is the gzip default and a good general tradeoff between speed, CPU usage, and compression
		wrapper, err := gzhttp.NewWrapper(gzhttp.RandomJitter(32, 0, false), gzhttp.MinSize(GzipMinSize), gzhttp.CompressionLevel(6))
		if err != nil {
			log.Fatal("gzhttp.NewWrapper failed: %v", err)
		}
		mid = append(mid, wrapper)
	}

	if setting.Service.EnableCaptcha {
		// The captcha http.Handler should only fire on /captcha/* so we can just mount this on that url
		routes.Methods("GET,HEAD", "/captcha/*", append(mid, captcha.Captchaer(context.GetImageCaptcha()))...)
	}

	if setting.Metrics.Enabled {
		prometheus.MustRegister(metrics.NewCollector())
		routes.Get("/metrics", append(mid, Metrics)...)
	}

	routes.Methods("GET,HEAD", "/robots.txt", append(mid, misc.RobotsTxt)...)
	routes.Get("/ssh_info", misc.SSHInfo)
	routes.Get("/api/healthz", healthcheck.Check)

	mid = append(mid, common.Sessioner(), context.Contexter())

	// Get user from session if logged in.
	mid = append(mid, webAuth(buildAuthGroup()))

	// GetHead allows a HEAD request redirect to GET if HEAD method is not defined for that route
	mid = append(mid, chi_middleware.GetHead)

	if setting.API.EnableSwagger {
		// Note: The route is here but no in API routes because it renders a web page
		routes.Get("/api/swagger", append(mid, misc.Swagger)...) // Render V1 by default
	}

	// TODO: These really seem like things that could be folded into Contexter or as helper functions
	mid = append(mid, user.GetNotificationCount)
	mid = append(mid, repo.GetActiveStopwatch)
	mid = append(mid, goGet)

	others := web.NewRoute()
	others.Use(mid...)
	registerRoutes(others)
	routes.Mount("", others)
	return routes
}

var ignSignInAndCsrf = verifyAuthWithOptions(&common.VerifyOptions{DisableCSRF: true})

// registerRoutes register routes
func registerRoutes(m *web.Route) {
	reqSignIn := verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: true})
	reqSignOut := verifyAuthWithOptions(&common.VerifyOptions{SignOutRequired: true})
	// TODO: rename them to "optSignIn", which means that the "sign-in" could be optional, depends on the VerifyOptions (RequireSignInView)
	ignSignIn := verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: setting.Service.RequireSignInView})
	ignExploreSignIn := verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: setting.Service.RequireSignInView || setting.Service.Explore.RequireSigninView})

	validation.AddBindingRules()

	linkAccountEnabled := func(ctx *context.Context) {
		if !setting.Service.EnableOpenIDSignIn && !setting.Service.EnableOpenIDSignUp && !setting.OAuth2.Enabled {
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

	dlSourceEnabled := func(ctx *context.Context) {
		if setting.Repository.DisableDownloadSourceArchives {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	sitemapEnabled := func(ctx *context.Context) {
		if !setting.Other.EnableSitemap {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	packagesEnabled := func(ctx *context.Context) {
		if !setting.Packages.Enabled {
			ctx.Error(http.StatusForbidden)
			return
		}
	}

	feedEnabled := func(ctx *context.Context) {
		if !setting.Other.EnableFeed {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	reqUnitAccess := func(unitType unit.Type, accessMode perm.AccessMode, ignoreGlobal bool) func(ctx *context.Context) {
		return func(ctx *context.Context) {
			// only check global disabled units when ignoreGlobal is false
			if !ignoreGlobal && unitType.UnitGlobalDisabled() {
				ctx.NotFound(unitType.String(), nil)
				return
			}

			if ctx.ContextUser == nil {
				ctx.NotFound(unitType.String(), nil)
				return
			}

			if ctx.ContextUser.IsOrganization() {
				if ctx.Org.Organization.UnitPermission(ctx, ctx.Doer, unitType) < accessMode {
					ctx.NotFound(unitType.String(), nil)
					return
				}
			}
		}
	}

	addWebhookAddRoutes := func() {
		m.Get("/{type}/new", repo_setting.WebhooksNew)
		m.Post("/gitea/new", web.Bind(forms.NewWebhookForm{}), repo_setting.GiteaHooksNewPost)
		m.Post("/gogs/new", web.Bind(forms.NewGogshookForm{}), repo_setting.GogsHooksNewPost)
		m.Post("/slack/new", web.Bind(forms.NewSlackHookForm{}), repo_setting.SlackHooksNewPost)
		m.Post("/discord/new", web.Bind(forms.NewDiscordHookForm{}), repo_setting.DiscordHooksNewPost)
		m.Post("/dingtalk/new", web.Bind(forms.NewDingtalkHookForm{}), repo_setting.DingtalkHooksNewPost)
		m.Post("/telegram/new", web.Bind(forms.NewTelegramHookForm{}), repo_setting.TelegramHooksNewPost)
		m.Post("/matrix/new", web.Bind(forms.NewMatrixHookForm{}), repo_setting.MatrixHooksNewPost)
		m.Post("/msteams/new", web.Bind(forms.NewMSTeamsHookForm{}), repo_setting.MSTeamsHooksNewPost)
		m.Post("/feishu/new", web.Bind(forms.NewFeishuHookForm{}), repo_setting.FeishuHooksNewPost)
		m.Post("/wechatwork/new", web.Bind(forms.NewWechatWorkHookForm{}), repo_setting.WechatworkHooksNewPost)
		m.Post("/packagist/new", web.Bind(forms.NewPackagistHookForm{}), repo_setting.PackagistHooksNewPost)
	}

	addWebhookEditRoutes := func() {
		m.Post("/gitea/{id}", web.Bind(forms.NewWebhookForm{}), repo_setting.GiteaHooksEditPost)
		m.Post("/gogs/{id}", web.Bind(forms.NewGogshookForm{}), repo_setting.GogsHooksEditPost)
		m.Post("/slack/{id}", web.Bind(forms.NewSlackHookForm{}), repo_setting.SlackHooksEditPost)
		m.Post("/discord/{id}", web.Bind(forms.NewDiscordHookForm{}), repo_setting.DiscordHooksEditPost)
		m.Post("/dingtalk/{id}", web.Bind(forms.NewDingtalkHookForm{}), repo_setting.DingtalkHooksEditPost)
		m.Post("/telegram/{id}", web.Bind(forms.NewTelegramHookForm{}), repo_setting.TelegramHooksEditPost)
		m.Post("/matrix/{id}", web.Bind(forms.NewMatrixHookForm{}), repo_setting.MatrixHooksEditPost)
		m.Post("/msteams/{id}", web.Bind(forms.NewMSTeamsHookForm{}), repo_setting.MSTeamsHooksEditPost)
		m.Post("/feishu/{id}", web.Bind(forms.NewFeishuHookForm{}), repo_setting.FeishuHooksEditPost)
		m.Post("/wechatwork/{id}", web.Bind(forms.NewWechatWorkHookForm{}), repo_setting.WechatworkHooksEditPost)
		m.Post("/packagist/{id}", web.Bind(forms.NewPackagistHookForm{}), repo_setting.PackagistHooksEditPost)
	}

	addSettingsVariablesRoutes := func() {
		m.Group("/variables", func() {
			m.Get("", repo_setting.Variables)
			m.Post("/new", web.Bind(forms.EditVariableForm{}), repo_setting.VariableCreate)
			m.Post("/{variable_id}/edit", web.Bind(forms.EditVariableForm{}), repo_setting.VariableUpdate)
			m.Post("/{variable_id}/delete", repo_setting.VariableDelete)
		})
	}

	addSettingsSecretsRoutes := func() {
		m.Group("/secrets", func() {
			m.Get("", repo_setting.Secrets)
			m.Post("", web.Bind(forms.AddSecretForm{}), repo_setting.SecretsPost)
			m.Post("/delete", repo_setting.SecretsDelete)
		})
	}

	addSettingsRunnersRoutes := func() {
		m.Group("/runners", func() {
			m.Get("", repo_setting.Runners)
			m.Combo("/{runnerid}").Get(repo_setting.RunnersEdit).
				Post(web.Bind(forms.EditRunnerForm{}), repo_setting.RunnersEditPost)
			m.Post("/{runnerid}/delete", repo_setting.RunnerDeletePost)
			m.Get("/reset_registration_token", repo_setting.ResetRunnerRegistrationToken)
		})
	}

	// FIXME: not all routes need go through same middleware.
	// Especially some AJAX requests, we can reduce middleware number to improve performance.

	m.Get("/", Home)
	m.Get("/sitemap.xml", sitemapEnabled, ignExploreSignIn, HomeSitemap)
	m.Group("/.well-known", func() {
		m.Get("/openid-configuration", auth.OIDCWellKnown)
		m.Group("", func() {
			m.Get("/nodeinfo", NodeInfoLinks)
			m.Get("/webfinger", WebfingerQuery)
		}, federationEnabled)
		m.Get("/change-password", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		})
		m.Get("/passkey-endpoints", passkeyEndpoints)
		m.Methods("GET, HEAD", "/*", public.FileHandlerFunc())
	}, optionsCorsHandler())

	m.Group("/explore", func() {
		m.Get("", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL + "/explore/repos")
		})
		m.Get("/repos", explore.Repos)
		m.Get("/repos/sitemap-{idx}.xml", sitemapEnabled, explore.Repos)
		m.Get("/users", explore.Users)
		m.Get("/users/sitemap-{idx}.xml", sitemapEnabled, explore.Users)
		m.Get("/organizations", explore.Organizations)
		m.Get("/code", func(ctx *context.Context) {
			if unit.TypeCode.UnitGlobalDisabled() {
				ctx.NotFound(unit.TypeCode.String(), nil)
				return
			}
		}, explore.Code)
		m.Get("/topics/search", explore.TopicSearch)
	}, ignExploreSignIn)

	m.Group("/issues", func() {
		m.Get("", user.Issues)
		m.Get("/search", repo.SearchIssues)
	}, reqSignIn)

	m.Get("/pulls", reqSignIn, user.Pulls)
	m.Get("/milestones", reqSignIn, reqMilestonesDashboardPageEnabled, user.Milestones)

	// ***** START: User *****
	// "user/login" doesn't need signOut, then logged-in users can still access this route for redirection purposes by "/user/login?redirec_to=..."
	m.Get("/user/login", auth.SignIn)
	m.Group("/user", func() {
		m.Post("/login", web.Bind(forms.SignInForm{}), auth.SignInPost)
		m.Group("", func() {
			m.Combo("/login/openid").
				Get(auth.SignInOpenID).
				Post(web.Bind(forms.SignInOpenIDForm{}), auth.SignInOpenIDPost)
		}, openIDSignInEnabled)
		m.Group("/openid", func() {
			m.Combo("/connect").
				Get(auth.ConnectOpenID).
				Post(web.Bind(forms.ConnectOpenIDForm{}), auth.ConnectOpenIDPost)
			m.Group("/register", func() {
				m.Combo("").
					Get(auth.RegisterOpenID, openIDSignUpEnabled).
					Post(web.Bind(forms.SignUpOpenIDForm{}), auth.RegisterOpenIDPost)
			}, openIDSignUpEnabled)
		}, openIDSignInEnabled)
		m.Get("/sign_up", auth.SignUp)
		m.Post("/sign_up", web.Bind(forms.RegisterForm{}), auth.SignUpPost)
		m.Get("/link_account", linkAccountEnabled, auth.LinkAccount)
		m.Post("/link_account_signin", linkAccountEnabled, web.Bind(forms.SignInForm{}), auth.LinkAccountPostSignIn)
		m.Post("/link_account_signup", linkAccountEnabled, web.Bind(forms.RegisterForm{}), auth.LinkAccountPostRegister)
		m.Group("/two_factor", func() {
			m.Get("", auth.TwoFactor)
			m.Post("", web.Bind(forms.TwoFactorAuthForm{}), auth.TwoFactorPost)
			m.Get("/scratch", auth.TwoFactorScratch)
			m.Post("/scratch", web.Bind(forms.TwoFactorScratchAuthForm{}), auth.TwoFactorScratchPost)
		})
		m.Group("/webauthn", func() {
			m.Get("", auth.WebAuthn)
			m.Get("/assertion", auth.WebAuthnLoginAssertion)
			m.Post("/assertion", auth.WebAuthnLoginAssertionPost)
		})
	}, reqSignOut)

	m.Any("/user/events", routing.MarkLongPolling, events.Events)

	m.Group("/login/oauth", func() {
		m.Get("/authorize", web.Bind(forms.AuthorizationForm{}), auth.AuthorizeOAuth)
		m.Post("/grant", web.Bind(forms.GrantApplicationForm{}), auth.GrantApplicationOAuth)
		// TODO manage redirection
		m.Post("/authorize", web.Bind(forms.AuthorizationForm{}), auth.AuthorizeOAuth)
	}, ignSignInAndCsrf, reqSignIn)

	m.Methods("GET, OPTIONS", "/login/oauth/userinfo", optionsCorsHandler(), ignSignInAndCsrf, auth.InfoOAuth)
	m.Methods("POST, OPTIONS", "/login/oauth/access_token", optionsCorsHandler(), web.Bind(forms.AccessTokenForm{}), ignSignInAndCsrf, auth.AccessTokenOAuth)
	m.Methods("GET, OPTIONS", "/login/oauth/keys", optionsCorsHandler(), ignSignInAndCsrf, auth.OIDCKeys)
	m.Methods("POST, OPTIONS", "/login/oauth/introspect", optionsCorsHandler(), web.Bind(forms.IntrospectTokenForm{}), ignSignInAndCsrf, auth.IntrospectOAuth)

	m.Group("/user/settings", func() {
		m.Get("", user_setting.Profile)
		m.Post("", web.Bind(forms.UpdateProfileForm{}), user_setting.ProfilePost)
		m.Get("/change_password", auth.MustChangePassword)
		m.Post("/change_password", web.Bind(forms.MustChangePasswordForm{}), auth.MustChangePasswordPost)
		m.Post("/avatar", web.Bind(forms.AvatarForm{}), user_setting.AvatarPost)
		m.Post("/avatar/delete", user_setting.DeleteAvatar)
		m.Group("/account", func() {
			m.Combo("").Get(user_setting.Account).Post(web.Bind(forms.ChangePasswordForm{}), user_setting.AccountPost)
			m.Post("/email", web.Bind(forms.AddEmailForm{}), user_setting.EmailPost)
			m.Post("/email/delete", user_setting.DeleteEmail)
			m.Post("/delete", user_setting.DeleteAccount)
		})
		m.Group("/appearance", func() {
			m.Get("", user_setting.Appearance)
			m.Post("/language", web.Bind(forms.UpdateLanguageForm{}), user_setting.UpdateUserLang)
			m.Post("/hidden_comments", user_setting.UpdateUserHiddenComments)
			m.Post("/theme", web.Bind(forms.UpdateThemeForm{}), user_setting.UpdateUIThemePost)
		})
		m.Group("/security", func() {
			m.Get("", security.Security)
			m.Group("/two_factor", func() {
				m.Post("/regenerate_scratch", security.RegenerateScratchTwoFactor)
				m.Post("/disable", security.DisableTwoFactor)
				m.Get("/enroll", security.EnrollTwoFactor)
				m.Post("/enroll", web.Bind(forms.TwoFactorAuthForm{}), security.EnrollTwoFactorPost)
			})
			m.Group("/webauthn", func() {
				m.Post("/request_register", web.Bind(forms.WebauthnRegistrationForm{}), security.WebAuthnRegister)
				m.Post("/register", security.WebauthnRegisterPost)
				m.Post("/delete", web.Bind(forms.WebauthnDeleteForm{}), security.WebauthnDelete)
			})
			m.Group("/openid", func() {
				m.Post("", web.Bind(forms.AddOpenIDForm{}), security.OpenIDPost)
				m.Post("/delete", security.DeleteOpenID)
				m.Post("/toggle_visibility", security.ToggleOpenIDVisibility)
			}, openIDSignInEnabled)
			m.Post("/account_link", linkAccountEnabled, security.DeleteAccountLink)
		})
		m.Group("/applications/oauth2", func() {
			m.Get("/{id}", user_setting.OAuth2ApplicationShow)
			m.Post("/{id}", web.Bind(forms.EditOAuth2ApplicationForm{}), user_setting.OAuthApplicationsEdit)
			m.Post("/{id}/regenerate_secret", user_setting.OAuthApplicationsRegenerateSecret)
			m.Post("", web.Bind(forms.EditOAuth2ApplicationForm{}), user_setting.OAuthApplicationsPost)
			m.Post("/{id}/delete", user_setting.DeleteOAuth2Application)
			m.Post("/{id}/revoke/{grantId}", user_setting.RevokeOAuth2Grant)
		})
		m.Combo("/applications").Get(user_setting.Applications).
			Post(web.Bind(forms.NewAccessTokenForm{}), user_setting.ApplicationsPost)
		m.Post("/applications/delete", user_setting.DeleteApplication)
		m.Combo("/keys").Get(user_setting.Keys).
			Post(web.Bind(forms.AddKeyForm{}), user_setting.KeysPost)
		m.Post("/keys/delete", user_setting.DeleteKey)
		m.Group("/packages", func() {
			m.Get("", user_setting.Packages)
			m.Group("/rules", func() {
				m.Group("/add", func() {
					m.Get("", user_setting.PackagesRuleAdd)
					m.Post("", web.Bind(forms.PackageCleanupRuleForm{}), user_setting.PackagesRuleAddPost)
				})
				m.Group("/{id}", func() {
					m.Get("", user_setting.PackagesRuleEdit)
					m.Post("", web.Bind(forms.PackageCleanupRuleForm{}), user_setting.PackagesRuleEditPost)
					m.Get("/preview", user_setting.PackagesRulePreview)
				})
			})
			m.Group("/cargo", func() {
				m.Post("/initialize", user_setting.InitializeCargoIndex)
				m.Post("/rebuild", user_setting.RebuildCargoIndex)
			})
			m.Post("/chef/regenerate_keypair", user_setting.RegenerateChefKeyPair)
		}, packagesEnabled)

		m.Group("/actions", func() {
			m.Get("", user_setting.RedirectToDefaultSetting)
			addSettingsRunnersRoutes()
			addSettingsSecretsRoutes()
			addSettingsVariablesRoutes()
		}, actions.MustEnableActions)

		m.Get("/organization", user_setting.Organization)
		m.Get("/repos", user_setting.Repos)
		m.Post("/repos/unadopted", user_setting.AdoptOrDeleteRepository)

		m.Group("/hooks", func() {
			m.Get("", user_setting.Webhooks)
			m.Post("/delete", user_setting.DeleteWebhook)
			addWebhookAddRoutes()
			m.Group("/{id}", func() {
				m.Get("", repo_setting.WebHooksEdit)
				m.Post("/replay/{uuid}", repo_setting.ReplayWebhook)
			})
			addWebhookEditRoutes()
		}, webhooksEnabled)

		m.Group("/blocked_users", func() {
			m.Get("", user_setting.BlockedUsers)
			m.Post("", web.Bind(forms.BlockUserForm{}), user_setting.BlockedUsersPost)
		})
	}, reqSignIn, ctxDataSet("PageIsUserSettings", true, "AllThemes", setting.UI.Themes, "EnablePackages", setting.Packages.Enabled))

	m.Group("/user", func() {
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

	adminReq := verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: true, AdminRequired: true})

	// ***** START: Admin *****
	m.Group("/admin", func() {
		m.Get("", admin.Dashboard)
		m.Get("/system_status", admin.SystemStatus)
		m.Post("", web.Bind(forms.AdminDashboardForm{}), admin.DashboardPost)

		m.Get("/self_check", admin.SelfCheck)

		m.Group("/config", func() {
			m.Get("", admin.Config)
			m.Post("", admin.ChangeConfig)
			m.Post("/test_mail", admin.SendTestMail)
			m.Get("/settings", admin.ConfigSettings)
		})

		m.Group("/monitor", func() {
			m.Get("/stats", admin.MonitorStats)
			m.Get("/cron", admin.CronTasks)
			m.Get("/stacktrace", admin.Stacktrace)
			m.Post("/stacktrace/cancel/{pid}", admin.StacktraceCancel)
			m.Get("/queue", admin.Queues)
			m.Group("/queue/{qid}", func() {
				m.Get("", admin.QueueManage)
				m.Post("/set", admin.QueueSet)
				m.Post("/remove-all-items", admin.QueueRemoveAllItems)
			})
			m.Get("/diagnosis", admin.MonitorDiagnosis)
		})

		m.Group("/users", func() {
			m.Get("", admin.Users)
			m.Combo("/new").Get(admin.NewUser).Post(web.Bind(forms.AdminCreateUserForm{}), admin.NewUserPost)
			m.Get("/{userid}", admin.ViewUser)
			m.Combo("/{userid}/edit").Get(admin.EditUser).Post(web.Bind(forms.AdminEditUserForm{}), admin.EditUserPost)
			m.Post("/{userid}/delete", admin.DeleteUser)
			m.Post("/{userid}/avatar", web.Bind(forms.AvatarForm{}), admin.AvatarPost)
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

		m.Group("/packages", func() {
			m.Get("", admin.Packages)
			m.Post("/delete", admin.DeletePackageVersion)
			m.Post("/cleanup", admin.CleanupExpiredData)
		}, packagesEnabled)

		m.Group("/hooks", func() {
			m.Get("", admin.DefaultOrSystemWebhooks)
			m.Post("/delete", admin.DeleteDefaultOrSystemWebhook)
			m.Group("/{id}", func() {
				m.Get("", repo_setting.WebHooksEdit)
				m.Post("/replay/{uuid}", repo_setting.ReplayWebhook)
			})
			addWebhookEditRoutes()
		}, webhooksEnabled)

		m.Group("/{configType:default-hooks|system-hooks}", func() {
			addWebhookAddRoutes()
		})

		m.Group("/auths", func() {
			m.Get("", admin.Authentications)
			m.Combo("/new").Get(admin.NewAuthSource).Post(web.Bind(forms.AuthenticationForm{}), admin.NewAuthSourcePost)
			m.Combo("/{authid}").Get(admin.EditAuthSource).
				Post(web.Bind(forms.AuthenticationForm{}), admin.EditAuthSourcePost)
			m.Post("/{authid}/delete", admin.DeleteAuthSource)
		})

		m.Group("/notices", func() {
			m.Get("", admin.Notices)
			m.Post("/delete", admin.DeleteNotices)
			m.Post("/empty", admin.EmptyNotices)
		})

		m.Group("/applications", func() {
			m.Get("", admin.Applications)
			m.Post("/oauth2", web.Bind(forms.EditOAuth2ApplicationForm{}), admin.ApplicationsPost)
			m.Group("/oauth2/{id}", func() {
				m.Combo("").Get(admin.EditApplication).Post(web.Bind(forms.EditOAuth2ApplicationForm{}), admin.EditApplicationPost)
				m.Post("/regenerate_secret", admin.ApplicationsRegenerateSecret)
				m.Post("/delete", admin.DeleteApplication)
			})
		}, func(ctx *context.Context) {
			if !setting.OAuth2.Enabled {
				ctx.Error(http.StatusForbidden)
				return
			}
		})

		m.Group("/actions", func() {
			m.Get("", admin.RedirectToDefaultSetting)
			addSettingsRunnersRoutes()
			addSettingsVariablesRoutes()
		})
	}, adminReq, ctxDataSet("EnableOAuth2", setting.OAuth2.Enabled, "EnablePackages", setting.Packages.Enabled))
	// ***** END: Admin *****

	m.Group("", func() {
		m.Get("/{username}", user.UsernameSubRoute)
		m.Methods("GET, OPTIONS", "/attachments/{uuid}", optionsCorsHandler(), repo.GetAttachment)
	}, ignSignIn)

	m.Post("/{username}", reqSignIn, context.UserAssignmentWeb(), user.Action)

	reqRepoAdmin := context.RequireRepoAdmin()
	reqRepoCodeWriter := context.RequireRepoWriter(unit.TypeCode)
	canEnableEditor := context.CanEnableEditor()
	reqRepoCodeReader := context.RequireRepoReader(unit.TypeCode)
	reqRepoReleaseWriter := context.RequireRepoWriter(unit.TypeReleases)
	reqRepoReleaseReader := context.RequireRepoReader(unit.TypeReleases)
	reqRepoWikiReader := context.RequireRepoReader(unit.TypeWiki)
	reqRepoWikiWriter := context.RequireRepoWriter(unit.TypeWiki)
	reqRepoIssueReader := context.RequireRepoReader(unit.TypeIssues)
	reqRepoPullsReader := context.RequireRepoReader(unit.TypePullRequests)
	reqRepoIssuesOrPullsWriter := context.RequireRepoWriterOr(unit.TypeIssues, unit.TypePullRequests)
	reqRepoIssuesOrPullsReader := context.RequireRepoReaderOr(unit.TypeIssues, unit.TypePullRequests)
	reqRepoProjectsReader := context.RequireRepoReader(unit.TypeProjects)
	reqRepoProjectsWriter := context.RequireRepoWriter(unit.TypeProjects)
	reqRepoActionsReader := context.RequireRepoReader(unit.TypeActions)
	reqRepoActionsWriter := context.RequireRepoWriter(unit.TypeActions)

	reqPackageAccess := func(accessMode perm.AccessMode) func(ctx *context.Context) {
		return func(ctx *context.Context) {
			if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
				ctx.NotFound("", nil)
			}
		}
	}

	individualPermsChecker := func(ctx *context.Context) {
		// org permissions have been checked in context.OrgAssignment(), but individual permissions haven't been checked.
		if ctx.ContextUser.IsIndividual() {
			switch {
			case ctx.ContextUser.Visibility == structs.VisibleTypePrivate:
				if ctx.Doer == nil || (ctx.ContextUser.ID != ctx.Doer.ID && !ctx.Doer.IsAdmin) {
					ctx.NotFound("Visit Project", nil)
					return
				}
			case ctx.ContextUser.Visibility == structs.VisibleTypeLimited:
				if ctx.Doer == nil {
					ctx.NotFound("Visit Project", nil)
					return
				}
			}
		}
	}

	m.Group("/org", func() {
		m.Group("/{org}", func() {
			m.Get("/members", org.Members)
		}, context.OrgAssignment())
	}, ignSignIn)
	// end "/org": members

	m.Group("/org", func() {
		m.Group("", func() {
			m.Get("/create", org.Create)
			m.Post("/create", web.Bind(forms.CreateOrgForm{}), org.CreatePost)
		})

		m.Group("/invite/{token}", func() {
			m.Get("", org.TeamInvite)
			m.Post("", org.TeamInvitePost)
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
			m.Post("/teams/new", web.Bind(forms.CreateTeamForm{}), org.NewTeamPost)
			m.Get("/teams/-/search", org.SearchTeam)
			m.Get("/teams/{team}/edit", org.EditTeam)
			m.Post("/teams/{team}/edit", web.Bind(forms.CreateTeamForm{}), org.EditTeamPost)
			m.Post("/teams/{team}/delete", org.DeleteTeam)

			m.Group("/settings", func() {
				m.Combo("").Get(org.Settings).
					Post(web.Bind(forms.UpdateOrgSettingForm{}), org.SettingsPost)
				m.Post("/avatar", web.Bind(forms.AvatarForm{}), org.SettingsAvatar)
				m.Post("/avatar/delete", org.SettingsDeleteAvatar)
				m.Group("/applications", func() {
					m.Get("", org.Applications)
					m.Post("/oauth2", web.Bind(forms.EditOAuth2ApplicationForm{}), org.OAuthApplicationsPost)
					m.Group("/oauth2/{id}", func() {
						m.Combo("").Get(org.OAuth2ApplicationShow).Post(web.Bind(forms.EditOAuth2ApplicationForm{}), org.OAuth2ApplicationEdit)
						m.Post("/regenerate_secret", org.OAuthApplicationsRegenerateSecret)
						m.Post("/delete", org.DeleteOAuth2Application)
					})
				}, func(ctx *context.Context) {
					if !setting.OAuth2.Enabled {
						ctx.Error(http.StatusForbidden)
						return
					}
				})

				m.Group("/hooks", func() {
					m.Get("", org.Webhooks)
					m.Post("/delete", org.DeleteWebhook)
					addWebhookAddRoutes()
					m.Group("/{id}", func() {
						m.Get("", repo_setting.WebHooksEdit)
						m.Post("/replay/{uuid}", repo_setting.ReplayWebhook)
					})
					addWebhookEditRoutes()
				}, webhooksEnabled)

				m.Group("/labels", func() {
					m.Get("", org.RetrieveLabels, org.Labels)
					m.Post("/new", web.Bind(forms.CreateLabelForm{}), org.NewLabel)
					m.Post("/edit", web.Bind(forms.CreateLabelForm{}), org.UpdateLabel)
					m.Post("/delete", org.DeleteLabel)
					m.Post("/initialize", web.Bind(forms.InitializeLabelsForm{}), org.InitializeLabels)
				})

				m.Group("/actions", func() {
					m.Get("", org_setting.RedirectToDefaultSetting)
					addSettingsRunnersRoutes()
					addSettingsSecretsRoutes()
					addSettingsVariablesRoutes()
				}, actions.MustEnableActions)

				m.Methods("GET,POST", "/delete", org.SettingsDelete)

				m.Group("/packages", func() {
					m.Get("", org.Packages)
					m.Group("/rules", func() {
						m.Group("/add", func() {
							m.Get("", org.PackagesRuleAdd)
							m.Post("", web.Bind(forms.PackageCleanupRuleForm{}), org.PackagesRuleAddPost)
						})
						m.Group("/{id}", func() {
							m.Get("", org.PackagesRuleEdit)
							m.Post("", web.Bind(forms.PackageCleanupRuleForm{}), org.PackagesRuleEditPost)
							m.Get("/preview", org.PackagesRulePreview)
						})
					})
					m.Group("/cargo", func() {
						m.Post("/initialize", org.InitializeCargoIndex)
						m.Post("/rebuild", org.RebuildCargoIndex)
					})
				}, packagesEnabled)

				m.Group("/blocked_users", func() {
					m.Get("", org.BlockedUsers)
					m.Post("", web.Bind(forms.BlockUserForm{}), org.BlockedUsersPost)
				})
			}, ctxDataSet("EnableOAuth2", setting.OAuth2.Enabled, "EnablePackages", setting.Packages.Enabled, "PageIsOrgSettings", true))
		}, context.OrgAssignment(true, true))
	}, reqSignIn)
	// end "/org": most org routes

	m.Group("/repo", func() {
		m.Get("/create", repo.Create)
		m.Post("/create", web.Bind(forms.CreateRepoForm{}), repo.CreatePost)
		m.Get("/migrate", repo.Migrate)
		m.Post("/migrate", web.Bind(forms.MigrateRepoForm{}), repo.MigratePost)
		m.Get("/search", repo.SearchRepo)
	}, reqSignIn)
	// end "/repo": create, migrate, search

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
							m.Post("", web.Bind(forms.PackageSettingForm{}), user.PackageSettingsPost)
						}, reqPackageAccess(perm.AccessModeWrite))
					})
				})
			}, context.PackageAssignment(), reqPackageAccess(perm.AccessModeRead))
		}

		m.Group("/projects", func() {
			m.Group("", func() {
				m.Get("", org.Projects)
				m.Get("/{id}", org.ViewProject)
			}, reqUnitAccess(unit.TypeProjects, perm.AccessModeRead, true))
			m.Group("", func() { //nolint:dupl
				m.Get("/new", org.RenderNewProject)
				m.Post("/new", web.Bind(forms.CreateProjectForm{}), org.NewProjectPost)
				m.Group("/{id}", func() {
					m.Post("", web.Bind(forms.EditProjectBoardForm{}), org.AddBoardToProjectPost)
					m.Post("/delete", org.DeleteProject)

					m.Get("/edit", org.RenderEditProject)
					m.Post("/edit", web.Bind(forms.CreateProjectForm{}), org.EditProjectPost)
					m.Post("/{action:open|close}", org.ChangeProjectStatus)

					m.Group("/{boardID}", func() {
						m.Put("", web.Bind(forms.EditProjectBoardForm{}), org.EditProjectBoard)
						m.Delete("", org.DeleteProjectBoard)
						m.Post("/default", org.SetDefaultProjectBoard)
						m.Post("/move", org.MoveIssues)
					})
				})
			}, reqSignIn, reqUnitAccess(unit.TypeProjects, perm.AccessModeWrite, true), func(ctx *context.Context) {
				if ctx.ContextUser.IsIndividual() && ctx.ContextUser.ID != ctx.Doer.ID {
					ctx.NotFound("NewProject", nil)
					return
				}
			})
		}, reqUnitAccess(unit.TypeProjects, perm.AccessModeRead, true), individualPermsChecker)

		m.Group("", func() {
			m.Get("/code", user.CodeSearch)
		}, reqUnitAccess(unit.TypeCode, perm.AccessModeRead, false), individualPermsChecker)
	}, ignSignIn, context.UserAssignmentWeb(), context.OrgAssignment())
	// end "/{username}/-": packages, projects, code

	m.Group("/{username}/{reponame}/settings", func() {
		m.Group("", func() {
			m.Combo("").Get(repo_setting.Settings).
				Post(web.Bind(forms.RepoSettingForm{}), repo_setting.SettingsPost)
		}, repo_setting.SettingsCtxData)
		m.Post("/avatar", web.Bind(forms.AvatarForm{}), repo_setting.SettingsAvatar)
		m.Post("/avatar/delete", repo_setting.SettingsDeleteAvatar)

		m.Group("/collaboration", func() {
			m.Combo("").Get(repo_setting.Collaboration).Post(repo_setting.CollaborationPost)
			m.Post("/access_mode", repo_setting.ChangeCollaborationAccessMode)
			m.Post("/delete", repo_setting.DeleteCollaboration)
			m.Group("/team", func() {
				m.Post("", repo_setting.AddTeamPost)
				m.Post("/delete", repo_setting.DeleteTeam)
			})
		})

		m.Group("/branches", func() {
			m.Post("/", repo_setting.SetDefaultBranchPost)
		}, repo.MustBeNotEmpty)

		m.Group("/branches", func() {
			m.Get("/", repo_setting.ProtectedBranchRules)
			m.Combo("/edit").Get(repo_setting.SettingsProtectedBranch).
				Post(web.Bind(forms.ProtectBranchForm{}), context.RepoMustNotBeArchived(), repo_setting.SettingsProtectedBranchPost)
			m.Post("/{id}/delete", repo_setting.DeleteProtectedBranchRulePost)
		}, repo.MustBeNotEmpty)

		m.Post("/rename_branch", web.Bind(forms.RenameBranchForm{}), context.RepoMustNotBeArchived(), repo_setting.RenameBranchPost)

		m.Group("/tags", func() {
			m.Get("", repo_setting.ProtectedTags)
			m.Post("", web.Bind(forms.ProtectTagForm{}), context.RepoMustNotBeArchived(), repo_setting.NewProtectedTagPost)
			m.Post("/delete", context.RepoMustNotBeArchived(), repo_setting.DeleteProtectedTagPost)
			m.Get("/{id}", repo_setting.EditProtectedTag)
			m.Post("/{id}", web.Bind(forms.ProtectTagForm{}), context.RepoMustNotBeArchived(), repo_setting.EditProtectedTagPost)
		})

		m.Group("/hooks/git", func() {
			m.Get("", repo_setting.GitHooks)
			m.Combo("/{name}").Get(repo_setting.GitHooksEdit).
				Post(repo_setting.GitHooksEditPost)
		}, context.GitHookService())

		m.Group("/hooks", func() {
			m.Get("", repo_setting.Webhooks)
			m.Post("/delete", repo_setting.DeleteWebhook)
			addWebhookAddRoutes()
			m.Group("/{id}", func() {
				m.Get("", repo_setting.WebHooksEdit)
				m.Post("/test", repo_setting.TestWebhook)
				m.Post("/replay/{uuid}", repo_setting.ReplayWebhook)
			})
			addWebhookEditRoutes()
		}, webhooksEnabled)

		m.Group("/keys", func() {
			m.Combo("").Get(repo_setting.DeployKeys).
				Post(web.Bind(forms.AddKeyForm{}), repo_setting.DeployKeysPost)
			m.Post("/delete", repo_setting.DeleteDeployKey)
		})

		m.Group("/lfs", func() {
			m.Get("/", repo_setting.LFSFiles)
			m.Get("/show/{oid}", repo_setting.LFSFileGet)
			m.Post("/delete/{oid}", repo_setting.LFSDelete)
			m.Get("/pointers", repo_setting.LFSPointerFiles)
			m.Post("/pointers/associate", repo_setting.LFSAutoAssociate)
			m.Get("/find", repo_setting.LFSFileFind)
			m.Group("/locks", func() {
				m.Get("/", repo_setting.LFSLocks)
				m.Post("/", repo_setting.LFSLockFile)
				m.Post("/{lid}/unlock", repo_setting.LFSUnlock)
			})
		})
		m.Group("/actions", func() {
			m.Get("", repo_setting.RedirectToDefaultSetting)
			addSettingsRunnersRoutes()
			addSettingsSecretsRoutes()
			addSettingsVariablesRoutes()
		}, actions.MustEnableActions)
		// the follow handler must be under "settings", otherwise this incomplete repo can't be accessed
		m.Group("/migrate", func() {
			m.Post("/retry", repo.MigrateRetryPost)
			m.Post("/cancel", repo.MigrateCancelPost)
		})
	},
		reqSignIn, context.RepoAssignment, reqRepoAdmin, context.RepoRef(),
		ctxDataSet("PageIsRepoSettings", true, "LFSStartServer", setting.LFS.StartServer),
	)
	// end "/{username}/{reponame}/settings"

	// user/org home, including rss feeds
	m.Get("/{username}/{reponame}", ignSignIn, context.RepoAssignment, context.RepoRef(), repo.SetEditorconfigIfExists, repo.Home)

	m.Group("/{username}/{reponame}", func() {
		m.Get("/find/*", repo.FindFiles)
		m.Group("/tree-list", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.TreeList)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.TreeList)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.TreeList)
		})
		m.Get("/compare", repo.MustBeNotEmpty, repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.CompareDiff)
		m.Combo("/compare/*", repo.MustBeNotEmpty, repo.SetEditorconfigIfExists).
			Get(repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.CompareDiff).
			Post(reqSignIn, context.RepoMustNotBeArchived(), reqRepoPullsReader, repo.MustAllowPulls, web.Bind(forms.CreateIssueForm{}), repo.SetWhitespaceBehavior, repo.CompareAndPullRequestPost)
	}, ignSignIn, context.RepoAssignment, reqRepoCodeReader)
	// end "/{username}/{reponame}": find, compare, list (code related)

	m.Group("/{username}/{reponame}", func() {
		m.Get("/issues/posters", repo.IssuePosters) // it can't use {type:issues|pulls} because it would conflict with other routes like "/pulls/{index}"
		m.Get("/pulls/posters", repo.PullPosters)
		m.Get("/comments/{id}/attachments", repo.GetCommentAttachments)
		m.Get("/labels", repo.RetrieveLabels, repo.Labels)
		m.Get("/milestones", repo.Milestones)
		m.Get("/milestone/{id}", context.RepoRef(), repo.MilestoneIssuesAndPulls)
		m.Group("/{type:issues|pulls}", func() {
			m.Group("/{index}", func() {
				m.Get("/info", repo.GetIssueInfo)
				m.Get("/attachments", repo.GetIssueAttachments)
				m.Get("/attachments/{uuid}", repo.GetAttachment)
				m.Group("/content-history", func() {
					m.Get("/overview", repo.GetContentHistoryOverview)
					m.Get("/list", repo.GetContentHistoryList)
					m.Get("/detail", repo.GetContentHistoryDetail)
				})
			})
		}, context.RepoRef())
	}, ignSignIn, context.RepoAssignment, reqRepoIssuesOrPullsReader)
	// end "/{username}/{reponame}": view milestone, label, issue, pull, etc

	m.Group("/{username}/{reponame}", func() {
		m.Group("/{type:issues|pulls}", func() {
			m.Get("", repo.Issues)
			m.Group("/{index}", func() {
				m.Get("", repo.ViewIssue)
			})
		})
	}, ignSignIn, context.RepoAssignment, context.RequireRepoReaderOr(unit.TypeIssues, unit.TypePullRequests, unit.TypeExternalTracker))
	// end "/{username}/{reponame}": issue/pull list, issue/pull view, external tracker

	m.Group("/{username}/{reponame}", func() { // edit issues, pulls, labels, milestones, etc
		m.Group("/issues", func() {
			m.Group("/new", func() {
				m.Combo("").Get(context.RepoRef(), repo.NewIssue).
					Post(web.Bind(forms.CreateIssueForm{}), repo.NewIssuePost)
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
				m.Post("/deadline", web.Bind(structs.EditDeadlineOption{}), repo.UpdateIssueDeadline)
				m.Post("/watch", repo.IssueWatch)
				m.Post("/ref", repo.UpdateIssueRef)
				m.Post("/pin", reqRepoAdmin, repo.IssuePinOrUnpin)
				m.Post("/viewed-files", repo.UpdateViewedFiles)
				m.Group("/dependency", func() {
					m.Post("/add", repo.AddDependency)
					m.Post("/delete", repo.RemoveDependency)
				})
				m.Combo("/comments").Post(repo.MustAllowUserComment, web.Bind(forms.CreateCommentForm{}), repo.NewComment)
				m.Group("/times", func() {
					m.Post("/add", web.Bind(forms.AddTimeManuallyForm{}), repo.AddTimeManually)
					m.Post("/{timeid}/delete", repo.DeleteTime)
					m.Group("/stopwatch", func() {
						m.Post("/toggle", repo.IssueStopwatch)
						m.Post("/cancel", repo.CancelStopwatch)
					})
				})
				m.Post("/reactions/{action}", web.Bind(forms.ReactionForm{}), repo.ChangeIssueReaction)
				m.Post("/lock", reqRepoIssuesOrPullsWriter, web.Bind(forms.IssueLockForm{}), repo.LockIssue)
				m.Post("/unlock", reqRepoIssuesOrPullsWriter, repo.UnlockIssue)
				m.Post("/delete", reqRepoAdmin, repo.DeleteIssue)
			}, context.RepoMustNotBeArchived())

			m.Group("/{index}", func() {
				m.Post("/content-history/soft-delete", repo.SoftDeleteContentHistory)
			})

			m.Post("/labels", reqRepoIssuesOrPullsWriter, repo.UpdateIssueLabel)
			m.Post("/milestone", reqRepoIssuesOrPullsWriter, repo.UpdateIssueMilestone)
			m.Post("/projects", reqRepoIssuesOrPullsWriter, reqRepoProjectsReader, repo.UpdateIssueProject)
			m.Post("/assignee", reqRepoIssuesOrPullsWriter, repo.UpdateIssueAssignee)
			m.Post("/request_review", repo.UpdatePullReviewRequest)
			m.Post("/dismiss_review", reqRepoAdmin, web.Bind(forms.DismissReviewForm{}), repo.DismissReview)
			m.Post("/status", reqRepoIssuesOrPullsWriter, repo.UpdateIssueStatus)
			m.Post("/delete", reqRepoAdmin, repo.BatchDeleteIssues)
			m.Post("/resolve_conversation", repo.SetShowOutdatedComments, repo.UpdateResolveConversation)
			m.Post("/attachments", repo.UploadIssueAttachment)
			m.Post("/attachments/remove", repo.DeleteAttachment)
			m.Delete("/unpin/{index}", reqRepoAdmin, repo.IssueUnpin)
			m.Post("/move_pin", reqRepoAdmin, repo.IssuePinMove)
		}, context.RepoMustNotBeArchived())

		m.Group("/comments/{id}", func() {
			m.Post("", repo.UpdateCommentContent)
			m.Post("/delete", repo.DeleteComment)
			m.Post("/reactions/{action}", web.Bind(forms.ReactionForm{}), repo.ChangeCommentReaction)
		}, context.RepoMustNotBeArchived())

		m.Post("/markup", web.Bind(structs.MarkupOption{}), misc.Markup)

		m.Group("/labels", func() {
			m.Post("/new", web.Bind(forms.CreateLabelForm{}), repo.NewLabel)
			m.Post("/edit", web.Bind(forms.CreateLabelForm{}), repo.UpdateLabel)
			m.Post("/delete", repo.DeleteLabel)
			m.Post("/initialize", web.Bind(forms.InitializeLabelsForm{}), repo.InitializeLabels)
		}, context.RepoMustNotBeArchived(), reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Group("/milestones", func() {
			m.Combo("/new").Get(repo.NewMilestone).
				Post(web.Bind(forms.CreateMilestoneForm{}), repo.NewMilestonePost)
			m.Get("/{id}/edit", repo.EditMilestone)
			m.Post("/{id}/edit", web.Bind(forms.CreateMilestoneForm{}), repo.EditMilestonePost)
			m.Post("/{id}/{action}", repo.ChangeMilestoneStatus)
			m.Post("/delete", repo.DeleteMilestone)
		}, context.RepoMustNotBeArchived(), reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Group("/pull", func() {
			m.Post("/{index}/target_branch", repo.UpdatePullRequestTarget)
		}, context.RepoMustNotBeArchived())
	}, reqSignIn, context.RepoAssignment, reqRepoIssuesOrPullsReader)
	// end "/{username}/{reponame}": create or edit issues, pulls, labels, milestones

	m.Group("/{username}/{reponame}", func() { // repo code
		m.Group("", func() {
			m.Group("", func() {
				m.Combo("/_edit/*").Get(repo.EditFile).
					Post(web.Bind(forms.EditRepoFileForm{}), repo.EditFilePost)
				m.Combo("/_new/*").Get(repo.NewFile).
					Post(web.Bind(forms.EditRepoFileForm{}), repo.NewFilePost)
				m.Post("/_preview/*", web.Bind(forms.EditPreviewDiffForm{}), repo.DiffPreviewPost)
				m.Combo("/_delete/*").Get(repo.DeleteFile).
					Post(web.Bind(forms.DeleteRepoFileForm{}), repo.DeleteFilePost)
				m.Combo("/_upload/*", repo.MustBeAbleToUpload).
					Get(repo.UploadFile).
					Post(web.Bind(forms.UploadRepoFileForm{}), repo.UploadFilePost)
				m.Combo("/_diffpatch/*").Get(repo.NewDiffPatch).
					Post(web.Bind(forms.EditRepoFileForm{}), repo.NewDiffPatchPost)
				m.Combo("/_cherrypick/{sha:([a-f0-9]{7,64})}/*").Get(repo.CherryPick).
					Post(web.Bind(forms.CherryPickForm{}), repo.CherryPickPost)
			}, repo.MustBeEditable)
			m.Group("", func() {
				m.Post("/upload-file", repo.UploadFileToServer)
				m.Post("/upload-remove", web.Bind(forms.RemoveUploadFileForm{}), repo.RemoveUploadFileFromServer)
			}, repo.MustBeEditable, repo.MustBeAbleToUpload)
		}, context.RepoRef(), canEnableEditor, context.RepoMustNotBeArchived())

		m.Group("/branches", func() {
			m.Group("/_new", func() {
				m.Post("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.CreateBranch)
				m.Post("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.CreateBranch)
				m.Post("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.CreateBranch)
			}, web.Bind(forms.NewBranchForm{}))
			m.Post("/delete", repo.DeleteBranchPost)
			m.Post("/restore", repo.RestoreBranchPost)
		}, context.RepoMustNotBeArchived(), reqRepoCodeWriter, repo.MustBeNotEmpty)

		m.Combo("/fork").Get(repo.Fork).Post(web.Bind(forms.CreateRepoForm{}), repo.ForkPost)
	}, reqSignIn, context.RepoAssignment, reqRepoCodeReader)
	// end "/{username}/{reponame}": repo code

	m.Group("/{username}/{reponame}", func() { // repo tags
		m.Group("/tags", func() {
			m.Get("", repo.TagsList)
			m.Get("/list", repo.GetTagList)
			m.Get(".rss", feedEnabled, repo.TagsListFeedRSS)
			m.Get(".atom", feedEnabled, repo.TagsListFeedAtom)
		}, ctxDataSet("EnableFeed", setting.Other.EnableFeed),
			repo.MustBeNotEmpty, context.RepoRefByType(context.RepoRefTag, true))
		m.Post("/tags/delete", repo.DeleteTag, reqSignIn,
			repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoCodeWriter, context.RepoRef())
	}, ignSignIn, context.RepoAssignment, reqRepoCodeReader)
	// end "/{username}/{reponame}": repo tags

	m.Group("/{username}/{reponame}", func() { // repo releases
		m.Group("/releases", func() {
			m.Get("", repo.Releases)
			m.Get("/tag/*", repo.SingleRelease)
			m.Get("/latest", repo.LatestRelease)
			m.Get(".rss", feedEnabled, repo.ReleasesFeedRSS)
			m.Get(".atom", feedEnabled, repo.ReleasesFeedAtom)
		}, ctxDataSet("EnableFeed", setting.Other.EnableFeed),
			repo.MustBeNotEmpty, context.RepoRefByType(context.RepoRefTag, true))
		m.Get("/releases/attachments/{uuid}", repo.MustBeNotEmpty, repo.GetAttachment)
		m.Get("/releases/download/{vTag}/{fileName}", repo.MustBeNotEmpty, repo.RedirectDownload)
		m.Group("/releases", func() {
			m.Get("/new", repo.NewRelease)
			m.Post("/new", web.Bind(forms.NewReleaseForm{}), repo.NewReleasePost)
			m.Post("/delete", repo.DeleteRelease)
			m.Post("/attachments", repo.UploadReleaseAttachment)
			m.Post("/attachments/remove", repo.DeleteAttachment)
		}, reqSignIn, repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoReleaseWriter, context.RepoRef())
		m.Group("/releases", func() {
			m.Get("/edit/*", repo.EditRelease)
			m.Post("/edit/*", web.Bind(forms.EditReleaseForm{}), repo.EditReleasePost)
		}, reqSignIn, repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoReleaseWriter, repo.CommitInfoCache)
	}, ignSignIn, context.RepoAssignment, reqRepoReleaseReader)
	// end "/{username}/{reponame}": repo releases

	m.Group("/{username}/{reponame}", func() { // to maintain compatibility with old attachments
		m.Get("/attachments/{uuid}", repo.GetAttachment)
	}, ignSignIn, context.RepoAssignment)
	// end "/{username}/{reponame}": compatibility with old attachments

	m.Group("/{username}/{reponame}", func() {
		m.Post("/topics", repo.TopicsPost)
	}, context.RepoAssignment, reqRepoAdmin, context.RepoMustNotBeArchived())

	m.Group("/{username}/{reponame}", func() {
		if setting.Packages.Enabled {
			m.Get("/packages", repo.Packages)
		}
	}, ignSignIn, context.RepoAssignment)

	m.Group("/{username}/{reponame}/projects", func() {
		m.Get("", repo.Projects)
		m.Get("/{id}", repo.ViewProject)
		m.Group("", func() { //nolint:dupl
			m.Get("/new", repo.RenderNewProject)
			m.Post("/new", web.Bind(forms.CreateProjectForm{}), repo.NewProjectPost)
			m.Group("/{id}", func() {
				m.Post("", web.Bind(forms.EditProjectBoardForm{}), repo.AddBoardToProjectPost)
				m.Post("/delete", repo.DeleteProject)

				m.Get("/edit", repo.RenderEditProject)
				m.Post("/edit", web.Bind(forms.CreateProjectForm{}), repo.EditProjectPost)
				m.Post("/{action:open|close}", repo.ChangeProjectStatus)

				m.Group("/{boardID}", func() {
					m.Put("", web.Bind(forms.EditProjectBoardForm{}), repo.EditProjectBoard)
					m.Delete("", repo.DeleteProjectBoard)
					m.Post("/default", repo.SetDefaultProjectBoard)
					m.Post("/move", repo.MoveIssues)
				})
			})
		}, reqRepoProjectsWriter, context.RepoMustNotBeArchived())
	}, ignSignIn, context.RepoAssignment, reqRepoProjectsReader, repo.MustEnableRepoProjects)
	// end "/{username}/{reponame}/projects"

	m.Group("/{username}/{reponame}/actions", func() {
		m.Get("", actions.List)
		m.Post("/disable", reqRepoAdmin, actions.DisableWorkflowFile)
		m.Post("/enable", reqRepoAdmin, actions.EnableWorkflowFile)

		m.Group("/runs/{run}", func() {
			m.Combo("").
				Get(actions.View).
				Post(web.Bind(actions.ViewRequest{}), actions.ViewPost)
			m.Group("/jobs/{job}", func() {
				m.Combo("").
					Get(actions.View).
					Post(web.Bind(actions.ViewRequest{}), actions.ViewPost)
				m.Post("/rerun", reqRepoActionsWriter, actions.Rerun)
				m.Get("/logs", actions.Logs)
			})
			m.Post("/cancel", reqRepoActionsWriter, actions.Cancel)
			m.Post("/approve", reqRepoActionsWriter, actions.Approve)
			m.Get("/artifacts", actions.ArtifactsView)
			m.Get("/artifacts/{artifact_name}", actions.ArtifactsDownloadView)
			m.Delete("/artifacts/{artifact_name}", actions.ArtifactsDeleteView)
			m.Post("/rerun", reqRepoActionsWriter, actions.Rerun)
		})
		m.Group("/workflows/{workflow_name}", func() {
			m.Get("/badge.svg", actions.GetWorkflowBadge)
		})
	}, ignSignIn, context.RepoAssignment, reqRepoActionsReader, actions.MustEnableActions)
	// end "/{username}/{reponame}/actions"

	m.Group("/{username}/{reponame}/wiki", func() {
		m.Combo("").
			Get(repo.Wiki).
			Post(context.RepoMustNotBeArchived(), reqSignIn, reqRepoWikiWriter, web.Bind(forms.NewWikiForm{}), repo.WikiPost)
		m.Combo("/*").
			Get(repo.Wiki).
			Post(context.RepoMustNotBeArchived(), reqSignIn, reqRepoWikiWriter, web.Bind(forms.NewWikiForm{}), repo.WikiPost)
		m.Get("/commit/{sha:[a-f0-9]{7,64}}", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.Diff)
		m.Get("/commit/{sha:[a-f0-9]{7,64}}.{ext:patch|diff}", repo.RawDiff)
		m.Get("/raw/*", repo.WikiRaw)
	}, ignSignIn, context.RepoAssignment, repo.MustEnableWiki, reqRepoWikiReader, func(ctx *context.Context) {
		ctx.Data["PageIsWiki"] = true
		ctx.Data["CloneButtonOriginLink"] = ctx.Repo.Repository.WikiCloneLink()
	})
	// end "/{username}/{reponame}/wiki"

	m.Group("/{username}/{reponame}/activity", func() {
		m.Get("", repo.Activity)
		m.Get("/{period}", repo.Activity)
		m.Group("/contributors", func() {
			m.Get("", repo.Contributors)
			m.Get("/data", repo.ContributorsData)
		})
		m.Group("/code-frequency", func() {
			m.Get("", repo.CodeFrequency)
			m.Get("/data", repo.CodeFrequencyData)
		})
		m.Group("/recent-commits", func() {
			m.Get("", repo.RecentCommits)
			m.Get("/data", repo.RecentCommitsData)
		})
	},
		ignSignIn, context.RepoAssignment, context.RequireRepoReaderOr(unit.TypePullRequests, unit.TypeIssues, unit.TypeReleases),
		context.RepoRef(), repo.MustBeNotEmpty,
	)
	// end "/{username}/{reponame}/activity"

	m.Group("/{username}/{reponame}", func() {
		m.Group("/activity_author_data", func() {
			m.Get("", repo.ActivityAuthors)
			m.Get("/{period}", repo.ActivityAuthors)
		}, context.RepoRef(), repo.MustBeNotEmpty)

		m.Group("/archive", func() {
			m.Get("/*", repo.Download)
			m.Post("/*", repo.InitiateDownload)
		}, repo.MustBeNotEmpty, dlSourceEnabled)

		m.Group("/branches", func() {
			m.Get("/list", repo.GetBranchesList)
			m.Get("", repo.Branches)
		}, repo.MustBeNotEmpty, context.RepoRef())

		m.Group("/blob_excerpt", func() {
			m.Get("/{sha}", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.ExcerptBlob)
		}, func(ctx *context.Context) gocontext.CancelFunc {
			// FIXME: refactor this function, use separate routes for wiki/code
			if ctx.FormBool("wiki") {
				ctx.Data["PageIsWiki"] = true
				repo.MustEnableWiki(ctx)
				return nil
			}

			if ctx.Written() {
				return nil
			}
			cancel := context.RepoRef()(ctx)
			if ctx.Written() {
				return cancel
			}

			repo.MustBeNotEmpty(ctx)
			return cancel
		})

		m.Group("/pulls/{index}", func() {
			m.Get("", repo.SetWhitespaceBehavior, repo.GetPullDiffStats, repo.ViewIssue)
			m.Get(".diff", repo.DownloadPullDiff)
			m.Get(".patch", repo.DownloadPullPatch)
			m.Group("/commits", func() {
				m.Get("", context.RepoRef(), repo.SetWhitespaceBehavior, repo.GetPullDiffStats, repo.ViewPullCommits)
				m.Get("/list", context.RepoRef(), repo.GetPullCommits)
				m.Get("/{sha:[a-f0-9]{7,40}}", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.SetShowOutdatedComments, repo.ViewPullFilesForSingleCommit)
			})
			m.Post("/merge", context.RepoMustNotBeArchived(), web.Bind(forms.MergePullRequestForm{}), repo.MergePullRequest)
			m.Post("/cancel_auto_merge", context.RepoMustNotBeArchived(), repo.CancelAutoMergePullRequest)
			m.Post("/update", repo.UpdatePullRequest)
			m.Post("/set_allow_maintainer_edit", web.Bind(forms.UpdateAllowEditsForm{}), repo.SetAllowEdits)
			m.Post("/cleanup", context.RepoMustNotBeArchived(), context.RepoRef(), repo.CleanUpPullRequest)
			m.Group("/files", func() {
				m.Get("", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.SetShowOutdatedComments, repo.ViewPullFilesForAllCommitsOfPr)
				m.Get("/{sha:[a-f0-9]{7,40}}", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.SetShowOutdatedComments, repo.ViewPullFilesStartingFromCommit)
				m.Get("/{shaFrom:[a-f0-9]{7,40}}..{shaTo:[a-f0-9]{7,40}}", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.SetShowOutdatedComments, repo.ViewPullFilesForRange)
				m.Group("/reviews", func() {
					m.Get("/new_comment", repo.RenderNewCodeCommentForm)
					m.Post("/comments", web.Bind(forms.CodeCommentForm{}), repo.SetShowOutdatedComments, repo.CreateCodeComment)
					m.Post("/submit", web.Bind(forms.SubmitReviewForm{}), repo.SubmitReview)
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
		}, repo.MustBeNotEmpty)

		m.Group("/raw", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.SingleDownload)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.SingleDownload)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.SingleDownload)
			m.Get("/blob/{sha}", context.RepoRefByType(context.RepoRefBlob), repo.DownloadByID)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.SingleDownload)
		}, repo.MustBeNotEmpty)

		m.Group("/render", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RenderFile)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RenderFile)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RenderFile)
			m.Get("/blob/{sha}", context.RepoRefByType(context.RepoRefBlob), repo.RenderFile)
		}, repo.MustBeNotEmpty)

		m.Group("/commits", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RefCommits)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RefCommits)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RefCommits)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.RefCommits)
		}, repo.MustBeNotEmpty)

		m.Group("/blame", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RefBlame)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RefBlame)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RefBlame)
		}, repo.MustBeNotEmpty)

		m.Group("", func() {
			m.Get("/graph", repo.Graph)
			m.Get("/commit/{sha:([a-f0-9]{7,64})$}", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.Diff)
			m.Get("/commit/{sha:([a-f0-9]{7,64})$}/load-branches-and-tags", repo.LoadBranchesAndTags)
			m.Get("/cherry-pick/{sha:([a-f0-9]{7,64})$}", repo.SetEditorconfigIfExists, repo.CherryPick)
		}, repo.MustBeNotEmpty, context.RepoRef())

		m.Get("/rss/branch/*", context.RepoRefByType(context.RepoRefBranch), feedEnabled, feed.RenderBranchFeed)
		m.Get("/atom/branch/*", context.RepoRefByType(context.RepoRefBranch), feedEnabled, feed.RenderBranchFeed)

		m.Group("/src", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.Home)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.Home)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.Home)
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.Home) // "/*" route is deprecated, and kept for backward compatibility
		}, repo.SetEditorconfigIfExists)

		m.Get("/forks", context.RepoRef(), repo.Forks)
		m.Get("/commit/{sha:([a-f0-9]{7,64})}.{ext:patch|diff}", repo.MustBeNotEmpty, repo.RawDiff)
		m.Post("/lastcommit/*", context.RepoRefByType(context.RepoRefCommit), repo.LastCommit)
	}, ignSignIn, context.RepoAssignment, reqRepoCodeReader)
	// end "/{username}/{reponame}": repo code

	m.Group("/{username}/{reponame}", func() {
		m.Get("/stars", repo.Stars)
		m.Get("/watchers", repo.Watchers)
		m.Get("/search", reqRepoCodeReader, repo.Search)
		m.Post("/action/{action}", reqSignIn, repo.Action)
	}, ignSignIn, context.RepoAssignment, context.RepoRef())

	m.Group("/{username}/{reponame}", func() {
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
		gitHTTPRouters(m)
	})
	// end "/{username}/{reponame}.git": git support

	m.Group("/notifications", func() {
		m.Get("", user.Notifications)
		m.Get("/subscriptions", user.NotificationSubscriptions)
		m.Get("/watching", user.NotificationWatching)
		m.Post("/status", user.NotificationStatusPost)
		m.Post("/purge", user.NotificationPurgePost)
		m.Get("/new", user.NewAvailable)
	}, reqSignIn)

	if setting.API.EnableSwagger {
		m.Get("/swagger.v1.json", SwaggerV1Json)
	}

	if !setting.IsProd {
		m.Any("/devtest", devtest.List)
		m.Any("/devtest/fetch-action-test", devtest.FetchActionTest)
		m.Any("/devtest/{sub}", devtest.Tmpl)
	}

	m.NotFound(func(w http.ResponseWriter, req *http.Request) {
		ctx := context.GetWebContext(req)
		routing.UpdateFuncInfo(ctx, routing.GetFuncInfo(ctx.NotFound, "GlobalNotFound"))
		ctx.NotFound("", nil)
	})
}
