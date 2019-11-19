// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sso

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/macaron"
	"gitea.com/macaron/session"

	"github.com/quasoft/websspi"
	gouuid "github.com/satori/go.uuid"
)

const (
	tplSignIn base.TplName = "user/auth/signin"
)

var (
	// sspiAuth is a global instance of the websspi authentication package,
	// which is used to avoid acquiring the server credential handle on
	// every request
	sspiAuth *websspi.Authenticator

	// Ensure the struct implements the interface.
	_ SingleSignOn = &SSPI{}
)

// SSPI implements the SingleSignOn interface and authenticates requests
// via the built-in SSPI module in Windows for SPNEGO authentication.
// On successful authentication returns a valid user object.
// Returns nil if authentication fails.
type SSPI struct {
}

// Init creates a new global websspi.Authenticator object
func (s *SSPI) Init() error {
	config := websspi.NewConfig()
	var err error
	sspiAuth, err = websspi.New(config)
	return err
}

// Free releases resources used by the global websspi.Authenticator object
func (s *SSPI) Free() error {
	return sspiAuth.Free()
}

// IsEnabled checks if there is an active SSPI authentication source
func (s *SSPI) IsEnabled() bool {
	return models.IsSSPIEnabled()
}

// VerifyAuthData uses SSPI (Windows implementation of SPNEGO) to authenticate the request.
// If authentication is successful, returs the corresponding user object.
// If negotiation should continue or authentication fails, immediately returns a 401 HTTP
// response code, as required by the SPNEGO protocol.
func (s *SSPI) VerifyAuthData(ctx *macaron.Context, sess session.Store) *models.User {
	if !s.shouldAuthenticate(ctx) {
		return nil
	}

	cfg, err := s.getConfig()
	if err != nil {
		log.Error("could not get SSPI config: %v", err)
		return nil
	}

	userInfo, outToken, err := sspiAuth.Authenticate(ctx.Req.Request, ctx.Resp)
	if err != nil {
		log.Warn("Authentication failed with error: %v\n", err)
		sspiAuth.AppendAuthenticateHeader(ctx.Resp, outToken)

		// Include the user login page in the 401 response to allow the user
		// to login with another authentication method if SSPI authentication
		// fails
		addFlashErr(ctx, ctx.Tr("auth.sspi_auth_failed"))
		ctx.Data["EnableOpenIDSignIn"] = setting.Service.EnableOpenIDSignIn
		ctx.Data["EnableSSPI"] = true
		ctx.HTML(401, string(tplSignIn))
		return nil
	}
	if outToken != "" {
		sspiAuth.AppendAuthenticateHeader(ctx.Resp, outToken)
	}

	username := sanitizeUsername(userInfo.Username, cfg)
	if len(username) == 0 {
		return nil
	}
	log.Info("Authenticated as %s\n", username)

	user, err := models.GetUserByName(username)
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			log.Error("GetUserByName: %v", err)
			return nil
		}
		if !cfg.AutoCreateUsers {
			log.Error("User '%s' not found", username)
			return nil
		}
		user = s.newUser(ctx, username, cfg)
	}

	// Make sure requests to API paths and PWA resources do not create a new session
	if !isAPIPath(ctx) && !isAttachmentDownload(ctx) {
		handleSignIn(ctx, sess, user)
	}

	return user
}

// getConfig retrieves the SSPI configuration from login sources
func (s *SSPI) getConfig() (*models.SSPIConfig, error) {
	sources, err := models.ActiveLoginSources(models.LoginSSPI)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("no active login sources of type SSPI found")
	}
	return sources[0].SSPI(), nil
}

func (s *SSPI) shouldAuthenticate(ctx *macaron.Context) bool {
	path := strings.TrimSuffix(ctx.Req.URL.Path, "/")
	if path == "/user/login" && ctx.Req.FormValue("user_name") != "" && ctx.Req.FormValue("password") != "" {
		return false
	} else if ctx.Req.FormValue("auth_with_sspi") == "1" {
		return true
	}
	return !isPublicPage(ctx) && !isPublicResource(ctx)
}

// newUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information present in request headers.
func (s *SSPI) newUser(ctx *macaron.Context, username string, cfg *models.SSPIConfig) *models.User {
	email := gouuid.NewV4().String() + "@localhost.localdomain"
	user := &models.User{
		Name:                         username,
		Email:                        email,
		KeepEmailPrivate:             true,
		Passwd:                       gouuid.NewV4().String(),
		IsActive:                     cfg.AutoActivateUsers,
		Language:                     cfg.DefaultLanguage,
		UseCustomAvatar:              true,
		Avatar:                       base.DefaultAvatarLink(),
		EmailNotificationsPreference: models.EmailNotificationsDisabled,
	}
	if err := models.CreateUser(user); err != nil {
		// FIXME: should I create a system notice?
		log.Error("CreateUser: %v", err)
		return nil
	}
	return user
}

// isPublicResource checks if the url is of a public resource file that should be served
// without authentication (eg. the Web App Manifest, the Service Worker script or the favicon)
func isPublicResource(ctx *macaron.Context) bool {
	path := strings.TrimSuffix(ctx.Req.URL.Path, "/")
	return path == "/robots.txt" ||
		path == "/favicon.ico" ||
		path == "/favicon.png" ||
		path == "/manifest.json" ||
		path == "/serviceworker.js"
}

// isPublicPage checks if the url is of a public page that should not require authentication
func isPublicPage(ctx *macaron.Context) bool {
	path := strings.TrimSuffix(ctx.Req.URL.Path, "/")
	homePage := strings.TrimSuffix(setting.AppSubURL, "/")
	currentURL := homePage + path
	return currentURL == homePage ||
		path == "/user/login" ||
		path == "/user/login/openid" ||
		path == "/user/sign_up" ||
		path == "/user/forgot_password" ||
		path == "/user/openid/connect" ||
		path == "/user/openid/register" ||
		strings.HasPrefix(path, "/user/oauth2") ||
		path == "/user/link_account" ||
		path == "/user/link_account_signin" ||
		path == "/user/link_account_signup" ||
		path == "/user/two_factor" ||
		path == "/user/two_factor/scratch" ||
		path == "/user/u2f" ||
		path == "/user/u2f/challenge" ||
		path == "/user/u2f/sign" ||
		(!setting.Service.RequireSignInView && (path == "/explore/repos" ||
			path == "/explore/users" ||
			path == "/explore/organizations" ||
			path == "/explore/code"))
}

// stripDomainNames removes NETBIOS domain name and separator from down-level logon names
// (eg. "DOMAIN\user" becomes "user"), and removes the UPN suffix (domain name) and separator
// from UPNs (eg. "user@domain.local" becomes "user")
func stripDomainNames(username string) string {
	if strings.Contains(username, "\\") {
		parts := strings.SplitN(username, "\\", 2)
		if len(parts) > 1 {
			username = parts[1]
		}
	} else if strings.Contains(username, "@") {
		parts := strings.Split(username, "@")
		if len(parts) > 1 {
			username = parts[0]
		}
	}
	return username
}

func replaceSeparators(username string, cfg *models.SSPIConfig) string {
	newSep := cfg.SeparatorReplacement
	username = strings.ReplaceAll(username, "\\", newSep)
	username = strings.ReplaceAll(username, "/", newSep)
	username = strings.ReplaceAll(username, "@", newSep)
	return username
}

func sanitizeUsername(username string, cfg *models.SSPIConfig) string {
	if len(username) == 0 {
		return ""
	}
	if cfg.StripDomainNames {
		username = stripDomainNames(username)
	}
	// Replace separators even if we have already stripped the domain name part,
	// as the username can contain several separators: eg. "MICROSOFT\useremail@live.com"
	username = replaceSeparators(username, cfg)
	return username
}

// handleSignIn clears existing session variables and stores new ones for the specified user object
func handleSignIn(ctx *macaron.Context, sess session.Store, user *models.User) {
	_ = sess.Delete("openid_verified_uri")
	_ = sess.Delete("openid_signin_remember")
	_ = sess.Delete("openid_determined_email")
	_ = sess.Delete("openid_determined_username")
	_ = sess.Delete("twofaUid")
	_ = sess.Delete("twofaRemember")
	_ = sess.Delete("u2fChallenge")
	_ = sess.Delete("linkAccount")
	err := sess.Set("uid", user.ID)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}
	err = sess.Set("uname", user.Name)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}

	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(user.Language) == 0 {
		user.Language = ctx.Locale.Language()
		if err := models.UpdateUserCols(user, "language"); err != nil {
			log.Error(fmt.Sprintf("Error updating user language [user: %d, locale: %s]", user.ID, user.Language))
			return
		}
	}

	ctx.SetCookie("lang", user.Language, nil, setting.AppSubURL, setting.SessionConfig.Domain, setting.SessionConfig.Secure, true)

	// Clear whatever CSRF has right now, force to generate a new one
	ctx.SetCookie(setting.CSRFCookieName, "", -1, setting.AppSubURL, setting.SessionConfig.Domain, setting.SessionConfig.Secure, true)
}

// addFlashErr adds an error message to the Flash object mapped to a macaron.Context
func addFlashErr(ctx *macaron.Context, err string) {
	fv := ctx.GetVal(reflect.TypeOf(&session.Flash{}))
	if !fv.IsValid() {
		return
	}
	flash, ok := fv.Interface().(*session.Flash)
	if !ok {
		return
	}
	flash.Error(err)
	ctx.Data["Flash"] = flash
}
