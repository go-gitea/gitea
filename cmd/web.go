// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/admin"
	apiv1 "code.gitea.io/gitea/routers/api/v1"
	"code.gitea.io/gitea/routers/dev"
	"code.gitea.io/gitea/routers/org"
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
	"github.com/urfave/cli"
	macaron "gopkg.in/macaron.v1"
)

// CmdWeb represents the available web sub-command.
var CmdWeb = cli.Command{
	Name:  "web",
	Usage: "Start Gitea web server",
	Description: `Gitea web server is the only thing you need to run,
and it takes care of all the other things for you`,
	Action: runWeb,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "port, p",
			Value: "3000",
			Usage: "Temporary port number to prevent conflict",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "custom/conf/app.ini",
			Usage: "Custom configuration file path",
		},
	},
}

// VerChecker is a listing of required dependency versions.
type VerChecker struct {
	ImportPath string
	Version    func() string
	Expected   string
}

// newMacaron initializes Macaron instance.
func newMacaron() *macaron.Macaron {
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
		},
	))

	m.Use(templates.Renderer())
	models.InitMailRender(templates.Mailer())

	localeNames, err := options.Dir("locale")

	if err != nil {
		log.Fatal(4, "Fail to list locale files: %v", err)
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

func runWeb(ctx *cli.Context) error {
	if ctx.IsSet("config") {
		setting.CustomConf = ctx.String("config")
	}
	routers.GlobalInit()

	m := newMacaron()

	reqSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: true})
	ignSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: setting.Service.RequireSignInView})
	ignSignInAndCsrf := context.Toggle(&context.ToggleOptions{DisableCSRF: true})
	reqSignOut := context.Toggle(&context.ToggleOptions{SignOutRequired: true})

	bindIgnErr := binding.BindIgnErr

	// FIXME: not all routes need go through same middlewares.
	// Especially some AJAX requests, we can reduce middleware number to improve performance.
	// Routers.
	m.Get("/", ignSignIn, routers.Home)
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
		m.Get("/sign_up", user.SignUp)
		m.Post("/sign_up", bindIgnErr(auth.RegisterForm{}), user.SignUpPost)
		m.Get("/reset_password", user.ResetPasswd)
		m.Post("/reset_password", user.ResetPasswdPost)
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
		m.Combo("/ssh").Get(user.SettingsSSHKeys).
			Post(bindIgnErr(auth.AddSSHKeyForm{}), user.SettingsSSHKeysPost)
		m.Post("/ssh/delete", user.DeleteSSHKey)
		m.Combo("/applications").Get(user.SettingsApplications).
			Post(bindIgnErr(auth.NewAccessTokenForm{}), user.SettingsApplicationsPost)
		m.Post("/applications/delete", user.SettingsDeleteApplication)
		m.Route("/delete", "GET,POST", user.SettingsDelete)
	}, reqSignIn, func(ctx *context.Context) {
		ctx.Data["PageIsUserSettings"] = true
	})

	m.Group("/user", func() {
		// r.Get("/feeds", binding.Bind(auth.FeedsForm{}), user.Feeds)
		m.Any("/activate", user.Activate)
		m.Any("/activate_email", user.ActivateEmail)
		m.Get("/email2user", user.Email2User)
		m.Get("/forget_password", user.ForgotPasswd)
		m.Post("/forget_password", user.ForgotPasswdPost)
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
			m.Get("/stars", user.Stars)
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

			if err = repo.ServeData(ctx, attach.Name, fr); err != nil {
				ctx.Handle(500, "ServeData", err)
				return
			}
		})
		m.Post("/issues/attachments", repo.UploadIssueAttachment)
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
		m.Get("/create", org.Create)
		m.Post("/create", bindIgnErr(auth.CreateOrgForm{}), org.CreatePost)

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
					m.Post("/gogs/new", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksNewPost)
					m.Post("/slack/new", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksNewPost)
					m.Get("/:id", repo.WebHooksEdit)
					m.Post("/gogs/:id", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksEditPost)
					m.Post("/slack/:id", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksEditPost)
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

			m.Group("/hooks", func() {
				m.Get("", repo.Webhooks)
				m.Post("/delete", repo.DeleteWebhook)
				m.Get("/:type/new", repo.WebhooksNew)
				m.Post("/gogs/new", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksNewPost)
				m.Post("/slack/new", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksNewPost)
				m.Get("/:id", repo.WebHooksEdit)
				m.Post("/:id/test", repo.TestWebhook)
				m.Post("/gogs/:id", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksEditPost)
				m.Post("/slack/:id", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksEditPost)

				m.Group("/git", func() {
					m.Get("", repo.GitHooks)
					m.Combo("/:name").Get(repo.GitHooksEdit).
						Post(repo.GitHooksEditPost)
				}, context.GitHookService())
			})

			m.Group("/keys", func() {
				m.Combo("").Get(repo.DeployKeys).
					Post(bindIgnErr(auth.AddSSHKeyForm{}), repo.DeployKeysPost)
				m.Post("/delete", repo.DeleteDeployKey)
			})

		}, func(ctx *context.Context) {
			ctx.Data["PageIsSettings"] = true
		})
	}, reqSignIn, context.RepoAssignment(), reqRepoAdmin, context.RepoRef())

	m.Get("/:username/:reponame/action/:action", reqSignIn, context.RepoAssignment(), repo.Action)
	m.Group("/:username/:reponame", func() {
		// FIXME: should use different URLs but mostly same logic for comments of issue and pull reuqest.
		// So they can apply their own enable/disable logic on routers.
		m.Group("/issues", func() {
			m.Combo("/new", repo.MustEnableIssues).Get(context.RepoRef(), repo.NewIssue).
				Post(bindIgnErr(auth.CreateIssueForm{}), repo.NewIssuePost)

			m.Group("/:index", func() {
				m.Post("/label", repo.UpdateIssueLabel)
				m.Post("/milestone", repo.UpdateIssueMilestone)
				m.Post("/assignee", repo.UpdateIssueAssignee)
			}, reqRepoWriter)

			m.Group("/:index", func() {
				m.Post("/title", repo.UpdateIssueTitle)
				m.Post("/content", repo.UpdateIssueContent)
				m.Combo("/comments").Post(bindIgnErr(auth.CreateCommentForm{}), repo.NewComment)
			})
		})
		m.Group("/comments/:id", func() {
			m.Post("", repo.UpdateCommentContent)
			m.Post("/delete", repo.DeleteComment)
		})
		m.Group("/labels", func() {
			m.Post("/new", bindIgnErr(auth.CreateLabelForm{}), repo.NewLabel)
			m.Post("/edit", bindIgnErr(auth.CreateLabelForm{}), repo.UpdateLabel)
			m.Post("/delete", repo.DeleteLabel)
			m.Post("/initialize", bindIgnErr(auth.InitializeLabelsForm{}), repo.InitializeLabels)
		}, reqRepoWriter, context.RepoRef())
		m.Group("/milestones", func() {
			m.Combo("/new").Get(repo.NewMilestone).
				Post(bindIgnErr(auth.CreateMilestoneForm{}), repo.NewMilestonePost)
			m.Get("/:id/edit", repo.EditMilestone)
			m.Post("/:id/edit", bindIgnErr(auth.CreateMilestoneForm{}), repo.EditMilestonePost)
			m.Get("/:id/:action", repo.ChangeMilestonStatus)
			m.Post("/delete", repo.DeleteMilestone)
		}, reqRepoWriter, context.RepoRef())

		m.Group("/releases", func() {
			m.Get("/new", repo.NewRelease)
			m.Post("/new", bindIgnErr(auth.NewReleaseForm{}), repo.NewReleasePost)
			m.Post("/delete", repo.DeleteRelease)
		}, reqRepoWriter, context.RepoRef())

		m.Group("/releases", func() {
			m.Get("/edit/*", repo.EditRelease)
			m.Post("/edit/*", bindIgnErr(auth.EditReleaseForm{}), repo.EditReleasePost)
		}, reqRepoWriter, func(ctx *context.Context) {
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
		}, reqRepoWriter, context.RepoRef(), func(ctx *context.Context) {
			if !ctx.Repo.Repository.CanEnableEditor() || ctx.Repo.IsViewCommit {
				ctx.Handle(404, "", nil)
				return
			}
		})
	}, reqSignIn, context.RepoAssignment(), repo.MustBeNotBare)

	m.Group("/:username/:reponame", func() {
		m.Group("", func() {
			m.Get("/releases", repo.Releases)
			m.Get("/^:type(issues|pulls)$", repo.RetrieveLabels, repo.Issues)
			m.Get("/^:type(issues|pulls)$/:index", repo.ViewIssue)
			m.Get("/labels/", repo.RetrieveLabels, repo.Labels)
			m.Get("/milestones", repo.Milestones)
		}, context.RepoRef())

		// m.Get("/branches", repo.Branches)
		m.Post("/branches/:name/delete", reqSignIn, reqRepoWriter, repo.DeleteBranchPost)

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
		}, repo.MustEnableWiki, context.RepoRef())

		m.Get("/archive/*", repo.Download)

		m.Group("/pulls/:index", func() {
			m.Get("/commits", context.RepoRef(), repo.ViewPullCommits)
			m.Get("/files", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.ViewPullFiles)
			m.Post("/merge", reqRepoWriter, repo.MergePullRequest)
		}, repo.MustAllowPulls)

		m.Group("", func() {
			m.Get("/src/*", repo.SetEditorconfigIfExists, repo.Home)
			m.Get("/raw/*", repo.SingleDownload)
			m.Get("/commits/*", repo.RefCommits)
			m.Get("/graph", repo.Graph)
			m.Get("/commit/:sha([a-f0-9]{7,40})$", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.Diff)
			m.Get("/forks", repo.Forks)
		}, context.RepoRef())
		m.Get("/commit/:sha([a-f0-9]{7,40})\\.:ext(patch|diff)", repo.RawDiff)

		m.Get("/compare/:before([a-z0-9]{40})\\.\\.\\.:after([a-z0-9]{40})", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.CompareDiff)
	}, ignSignIn, context.RepoAssignment(), repo.MustBeNotBare)
	m.Group("/:username/:reponame", func() {
		m.Get("/stars", repo.Stars)
		m.Get("/watchers", repo.Watchers)
	}, ignSignIn, context.RepoAssignment(), context.RepoRef())

	m.Group("/:username", func() {
		m.Group("/:reponame", func() {
			m.Get("", repo.SetEditorconfigIfExists, repo.Home)
			m.Get("\\.git$", repo.SetEditorconfigIfExists, repo.Home)
		}, ignSignIn, context.RepoAssignment(true), context.RepoRef())

		m.Group("/:reponame", func() {
			m.Group("/info/lfs", func() {
				m.Post("/objects/batch", lfs.BatchHandler)
				m.Get("/objects/:oid/:filename", lfs.ObjectOidHandler)
				m.Any("/objects/:oid", lfs.ObjectOidHandler)
				m.Post("/objects", lfs.PostHandler)
			}, ignSignInAndCsrf)
			m.Any("/*", ignSignInAndCsrf, repo.HTTP)
			m.Head("/tasks/trigger", repo.TriggerTask)
		})
	})
	// ***** END: Repository *****

	m.Group("/api", func() {
		apiv1.RegisterRoutes(m)
	}, ignSignIn)

	// robots.txt
	m.Get("/robots.txt", func(ctx *context.Context) {
		if setting.HasRobotsTxt {
			ctx.ServeFileContent(path.Join(setting.CustomPath, "robots.txt"))
		} else {
			ctx.Error(404)
		}
	})

	// Not found handler.
	m.NotFound(routers.NotFound)

	// Flag for port number in case first time run conflict.
	if ctx.IsSet("port") {
		setting.AppURL = strings.Replace(setting.AppURL, setting.HTTPPort, ctx.String("port"), 1)
		setting.HTTPPort = ctx.String("port")
	}

	var listenAddr string
	if setting.Protocol == setting.UnixSocket {
		listenAddr = fmt.Sprintf("%s", setting.HTTPAddr)
	} else {
		listenAddr = fmt.Sprintf("%s:%s", setting.HTTPAddr, setting.HTTPPort)
	}
	log.Info("Listen: %v://%s%s", setting.Protocol, listenAddr, setting.AppSubURL)

	if setting.LFS.StartServer {
		log.Info("LFS server enabled")
	}

	var err error
	switch setting.Protocol {
	case setting.HTTP:
		err = http.ListenAndServe(listenAddr, m)
	case setting.HTTPS:
		server := &http.Server{Addr: listenAddr, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS10}, Handler: m}
		err = server.ListenAndServeTLS(setting.CertFile, setting.KeyFile)
	case setting.FCGI:
		err = fcgi.Serve(nil, m)
	case setting.UnixSocket:
		if err := os.Remove(listenAddr); err != nil {
			log.Fatal(4, "Fail to remove unix socket directory %s: %v", listenAddr, err)
		}
		var listener *net.UnixListener
		listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: listenAddr, Net: "unix"})
		if err != nil {
			break // Handle error after switch
		}

		// FIXME: add proper implementation of signal capture on all protocols
		// execute this on SIGTERM or SIGINT: listener.Close()
		if err = os.Chmod(listenAddr, os.FileMode(setting.UnixSocketPermission)); err != nil {
			log.Fatal(4, "Failed to set permission of unix socket: %v", err)
		}
		err = http.Serve(listener, m)
	default:
		log.Fatal(4, "Invalid protocol: %s", setting.Protocol)
	}

	if err != nil {
		log.Fatal(4, "Fail to start server: %v", err)
	}

	return nil
}
