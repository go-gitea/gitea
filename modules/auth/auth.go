// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"reflect"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"github.com/go-macaron/binding"
	"github.com/go-macaron/session"
	gouuid "github.com/satori/go.uuid"
	"gopkg.in/macaron.v1"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"
)

// IsAPIPath if URL is an api path
func IsAPIPath(url string) bool {
	return strings.HasPrefix(url, "/api/")
}

// SignedInID returns the id of signed in user.
func SignedInID(ctx *macaron.Context, sess session.Store) int64 {
	if !models.HasEngine {
		return 0
	}

	// Check access token.
	if IsAPIPath(ctx.Req.URL.Path) {
		tokenSHA := ctx.Query("token")
		if len(tokenSHA) == 0 {
			tokenSHA = ctx.Query("access_token")
		}
		if len(tokenSHA) == 0 {
			// Well, check with header again.
			auHead := ctx.Req.Header.Get("Authorization")
			if len(auHead) > 0 {
				auths := strings.Fields(auHead)
				if len(auths) == 2 && (auths[0] == "token" || strings.ToLower(auths[0]) == "bearer") {
					tokenSHA = auths[1]
				}
			}
		}

		// Let's see if token is valid.
		if len(tokenSHA) > 0 {
			if strings.Contains(tokenSHA, ".") {
				uid := CheckOAuthAccessToken(tokenSHA)
				if uid != 0 {
					ctx.Data["IsApiToken"] = true
				}
				return uid
			}
			t, err := models.GetAccessTokenBySHA(tokenSHA)
			if err != nil {
				if models.IsErrAccessTokenNotExist(err) || models.IsErrAccessTokenEmpty(err) {
					log.Error(4, "GetAccessTokenBySHA: %v", err)
				}
				return 0
			}
			t.UpdatedUnix = util.TimeStampNow()
			if err = models.UpdateAccessToken(t); err != nil {
				log.Error(4, "UpdateAccessToken: %v", err)
			}
			ctx.Data["IsApiToken"] = true
			return t.UID
		}
	}

	uid := sess.Get("uid")
	if uid == nil {
		return 0
	} else if id, ok := uid.(int64); ok {
		return id
	}
	return 0
}

// CheckOAuthAccessToken returns uid of user from oauth token token
func CheckOAuthAccessToken(accessToken string) int64 {
	// JWT tokens require a "."
	if !strings.Contains(accessToken, ".") {
		return 0
	}
	token, err := models.ParseOAuth2Token(accessToken)
	if err != nil {
		log.Trace("ParseOAuth2Token", err)
		return 0
	}
	var grant *models.OAuth2Grant
	if grant, err = models.GetOAuth2GrantByID(token.GrantID); err != nil || grant == nil {
		return 0
	}
	if token.Type != models.TypeAccessToken {
		return 0
	}
	if token.ExpiresAt < time.Now().Unix() || token.IssuedAt > time.Now().Unix() {
		return 0
	}
	return grant.UserID
}

// SignedInUser returns the user object of signed user.
// It returns a bool value to indicate whether user uses basic auth or not.
func SignedInUser(ctx *macaron.Context, sess session.Store) (*models.User, bool) {
	if !models.HasEngine {
		return nil, false
	}

	if uid := SignedInID(ctx, sess); uid > 0 {
		user, err := models.GetUserByID(uid)
		if err == nil {
			return user, false
		} else if !models.IsErrUserNotExist(err) {
			log.Error(4, "GetUserById: %v", err)
		}
	}

	if setting.Service.EnableReverseProxyAuth {
		webAuthUser := ctx.Req.Header.Get(setting.ReverseProxyAuthUser)
		if len(webAuthUser) > 0 {
			u, err := models.GetUserByName(webAuthUser)
			if err != nil {
				if !models.IsErrUserNotExist(err) {
					log.Error(4, "GetUserByName: %v", err)
					return nil, false
				}

				// Check if enabled auto-registration.
				if setting.Service.EnableReverseProxyAutoRegister {
					email := gouuid.NewV4().String() + "@localhost"
					if setting.Service.EnableReverseProxyEmail {
						webAuthEmail := ctx.Req.Header.Get(setting.ReverseProxyAuthEmail)
						if len(webAuthEmail) > 0 {
							email = webAuthEmail
						}
					}
					u := &models.User{
						Name:     webAuthUser,
						Email:    email,
						Passwd:   webAuthUser,
						IsActive: true,
					}
					if err = models.CreateUser(u); err != nil {
						// FIXME: should I create a system notice?
						log.Error(4, "CreateUser: %v", err)
						return nil, false
					}
					return u, false
				}
			}
			return u, false
		}
	}

	// Check with basic auth.
	baHead := ctx.Req.Header.Get("Authorization")
	if len(baHead) > 0 {
		auths := strings.Fields(baHead)
		if len(auths) == 2 && auths[0] == "Basic" {
			var u *models.User

			uname, passwd, _ := base.BasicAuthDecode(auths[1])

			// Check if username or password is a token
			isUsernameToken := len(passwd) == 0 || passwd == "x-oauth-basic"
			// Assume username is token
			authToken := uname
			if !isUsernameToken {
				// Assume password is token
				authToken = passwd
			}

			uid := CheckOAuthAccessToken(authToken)
			if uid != 0 {
				var err error
				ctx.Data["IsApiToken"] = true

				u, err = models.GetUserByID(uid)
				if err != nil {
					log.Error(4, "GetUserByID:  %v", err)
					return nil, false
				}
			}
			token, err := models.GetAccessTokenBySHA(authToken)
			if err == nil {
				if isUsernameToken {
					u, err = models.GetUserByID(token.UID)
					if err != nil {
						log.Error(4, "GetUserByID:  %v", err)
						return nil, false
					}
				} else {
					u, err = models.GetUserByName(uname)
					if err != nil {
						log.Error(4, "GetUserByID:  %v", err)
						return nil, false
					}
					if u.ID != token.UID {
						return nil, false
					}
				}
				token.UpdatedUnix = util.TimeStampNow()
				if err = models.UpdateAccessToken(token); err != nil {
					log.Error(4, "UpdateAccessToken:  %v", err)
				}
			} else {
				if !models.IsErrAccessTokenNotExist(err) && !models.IsErrAccessTokenEmpty(err) {
					log.Error(4, "GetAccessTokenBySha: %v", err)
				}
			}

			if u == nil {
				u, err = models.UserSignIn(uname, passwd)
				if err != nil {
					if !models.IsErrUserNotExist(err) {
						log.Error(4, "UserSignIn: %v", err)
					}
					return nil, false
				}
			} else {
				ctx.Data["IsApiToken"] = true
			}

			return u, true
		}
	}
	return nil, false
}

// Form form binding interface
type Form interface {
	binding.Validator
}

func init() {
	binding.SetNameMapper(com.ToSnakeCase)
}

// AssignForm assign form values back to the template data.
func AssignForm(form interface{}, data map[string]interface{}) {
	typ := reflect.TypeOf(form)
	val := reflect.ValueOf(form)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		fieldName := field.Tag.Get("form")
		// Allow ignored fields in the struct
		if fieldName == "-" {
			continue
		} else if len(fieldName) == 0 {
			fieldName = com.ToSnakeCase(field.Name)
		}

		data[fieldName] = val.Field(i).Interface()
	}
}

func getRuleBody(field reflect.StructField, prefix string) string {
	for _, rule := range strings.Split(field.Tag.Get("binding"), ";") {
		if strings.HasPrefix(rule, prefix) {
			return rule[len(prefix) : len(rule)-1]
		}
	}
	return ""
}

// GetSize get size int form tag
func GetSize(field reflect.StructField) string {
	return getRuleBody(field, "Size(")
}

// GetMinSize get minimal size in form tag
func GetMinSize(field reflect.StructField) string {
	return getRuleBody(field, "MinSize(")
}

// GetMaxSize get max size in form tag
func GetMaxSize(field reflect.StructField) string {
	return getRuleBody(field, "MaxSize(")
}

// GetInclude get include in form tag
func GetInclude(field reflect.StructField) string {
	return getRuleBody(field, "Include(")
}

// FIXME: struct contains a struct
func validateStruct(obj interface{}) binding.Errors {

	return nil
}

func validate(errs binding.Errors, data map[string]interface{}, f Form, l macaron.Locale) binding.Errors {
	if errs.Len() == 0 {
		return errs
	}

	data["HasError"] = true
	AssignForm(f, data)

	typ := reflect.TypeOf(f)
	val := reflect.ValueOf(f)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		fieldName := field.Tag.Get("form")
		// Allow ignored fields in the struct
		if fieldName == "-" {
			continue
		}

		if errs[0].FieldNames[0] == field.Name {
			data["Err_"+field.Name] = true

			trName := field.Tag.Get("locale")
			if len(trName) == 0 {
				trName = l.Tr("form." + field.Name)
			} else {
				trName = l.Tr(trName)
			}

			switch errs[0].Classification {
			case binding.ERR_REQUIRED:
				data["ErrorMsg"] = trName + l.Tr("form.require_error")
			case binding.ERR_ALPHA_DASH:
				data["ErrorMsg"] = trName + l.Tr("form.alpha_dash_error")
			case binding.ERR_ALPHA_DASH_DOT:
				data["ErrorMsg"] = trName + l.Tr("form.alpha_dash_dot_error")
			case validation.ErrGitRefName:
				data["ErrorMsg"] = trName + l.Tr("form.git_ref_name_error")
			case binding.ERR_SIZE:
				data["ErrorMsg"] = trName + l.Tr("form.size_error", GetSize(field))
			case binding.ERR_MIN_SIZE:
				data["ErrorMsg"] = trName + l.Tr("form.min_size_error", GetMinSize(field))
			case binding.ERR_MAX_SIZE:
				data["ErrorMsg"] = trName + l.Tr("form.max_size_error", GetMaxSize(field))
			case binding.ERR_EMAIL:
				data["ErrorMsg"] = trName + l.Tr("form.email_error")
			case binding.ERR_URL:
				data["ErrorMsg"] = trName + l.Tr("form.url_error")
			case binding.ERR_INCLUDE:
				data["ErrorMsg"] = trName + l.Tr("form.include_error", GetInclude(field))
			default:
				data["ErrorMsg"] = l.Tr("form.unknown_error") + " " + errs[0].Classification
			}
			return errs
		}
	}
	return errs
}
