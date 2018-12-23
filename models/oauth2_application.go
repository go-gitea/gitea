// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"net/url"
	"time"

	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
	gouuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

type OAuth2ApplicationType string

const (
	ApplicationTypeWeb OAuth2ApplicationType = "web"
	ApplicationTypeNative OAuth2ApplicationType = "native"
)

// OAuth2Application represents an OAuth2 client (RFC 6749)
type OAuth2Application struct {
	ID   int64 `xorm:"pk autoincr"`
	UID  int64 `xorm:"INDEX"`
	User *User `xorm:"-"`

	Name string
	Type OAuth2ApplicationType

	ClientID     string `xorm:"INDEX unique"`
	ClientSecret string

	RedirectURIs []string `xorm:"redirect_uris JSON TEXT"`

	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`
}

// TableName sets the table name to `oauth2_application`
func (app *OAuth2Application) TableName() string {
	return "oauth2_application"
}

// PrimaryRedirectURI returns the first redirect uri or an empty string if empty
func (app *OAuth2Application) PrimaryRedirectURI() string {
	if len(app.RedirectURIs) == 0 {
		return ""
	}
	return app.RedirectURIs[0]
}

// LoadUser will load User by UID
func (app *OAuth2Application) LoadUser() (err error) {
	if app.User == nil {
		app.User, err = GetUserByID(app.UID)
	}
	return
}

// ContainsRedirectURI checks if redirectURI is allowed for app
func (app *OAuth2Application) ContainsRedirectURI(redirectURI string) bool {
	return com.IsSliceContainsStr(app.RedirectURIs, redirectURI)
}

// GenerateClientSecret will generate the client secret and returns the plaintext and saves the hash at the database
func (app *OAuth2Application) GenerateClientSecret() (string, error) {
	if app.Type != ApplicationTypeWeb {
		return "", fmt.Errorf("only web application use client secrets")
	}
	secret := gouuid.NewV4().String()
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	app.ClientSecret = string(hashedSecret)
	if _, err := x.ID(app.ID).Cols("client_secret").Update(app); err != nil {
		return "", err
	}
	return secret, nil
}

// ValidateClientSecret validates the given secret by the hash saved in database
func (app *OAuth2Application) ValidateClientSecret(secret []byte) bool {
	return bcrypt.CompareHashAndPassword([]byte(app.ClientSecret), secret) == nil
}

// GetGrantByUserID returns a OAuth2Grant by its user and application ID
func (app *OAuth2Application) GetGrantByUserID(userID int64) (*OAuth2Grant, error) {
	return app.getGrantByUserID(x, userID)
}

func (app *OAuth2Application) getGrantByUserID(e Engine, userID int64) (grant *OAuth2Grant, err error) {
	grant = new(OAuth2Grant)
	if has, err := e.Where("user_id = ? AND application_id = ?", userID, app.ID).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return grant, nil
}

// CreateGrant generates a grant for an user
func (app *OAuth2Application) CreateGrant(userID int64) (*OAuth2Grant, error) {
	return app.createGrant(x, userID)
}

func (app *OAuth2Application) createGrant(e Engine, userID int64) (*OAuth2Grant, error) {
	grant := &OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        userID,
	}
	_, err := e.Insert(grant)
	if err != nil {
		return nil, err
	}
	return grant, nil
}

// GetOAuth2ApplicationByClientID returns the oauth2 application with the given client_id. Returns an error if not found.
func GetOAuth2ApplicationByClientID(clientID string) (app *OAuth2Application, err error) {
	return getOAuth2ApplicationByClientID(x, clientID)
}

func getOAuth2ApplicationByClientID(e Engine, clientID string) (app *OAuth2Application, err error) {
	app = new(OAuth2Application)
	has, err := e.Where("client_id = ?", clientID).Get(app)
	if !has {
		return app, ErrOAuthClientIDInvalid{ClientID: clientID}
	}
	return
}

// GetOAuth2ApplicationsByUserID returns all oauth2 applications owned by the user
func GetOAuth2ApplicationsByUserID(userID int64) (apps []*OAuth2Application, err error) {
	return getOAuth2ApplicationsByUserID(x, userID)
}

func getOAuth2ApplicationsByUserID(e Engine, userID int64) (apps []*OAuth2Application, err error) {
	apps = make([]*OAuth2Application, 0)
	err = e.Where("uid = ?", userID).Find(&apps)
	return
}

type CreateOAuth2ApplicationOptions struct {
	Name string
	UserID int64
	Type OAuth2ApplicationType
	RedirectURIs []string
}

// CreateOAuth2Application inserts a new oauth2 application
func CreateOAuth2Application(opts CreateOAuth2ApplicationOptions) (*OAuth2Application, error) {
	return createOAuth2Application(x, opts)
}

func createOAuth2Application(e Engine, opts CreateOAuth2ApplicationOptions) (*OAuth2Application, error) {
	clientID := gouuid.NewV4().String()
	app := &OAuth2Application{
		UID:          opts.UserID,
		Name:         opts.Name,
		Type:	      opts.Type,
		ClientID:     clientID,
		RedirectURIs: opts.RedirectURIs,
	}
	if _, err := e.Insert(app); err != nil {
		return nil, err
	}
	return app, nil
}

//////////////////////////////////////////////////////

// OAuth2AuthorizationCode is a code to obtain an access token in combination with the client secret once. It has a limited lifetime.
type OAuth2AuthorizationCode struct {
	ID          int64        `xorm:"pk autoincr"`
	Grant       *OAuth2Grant `xorm:"-"`
	GrantID     int64
	Code        string `xorm:"INDEX unique"`
	RedirectURI string
	ValidUntil  util.TimeStamp `xorm:"index"`
}

// TableName sets the table name to `oauth2_authorization_code`
func (code *OAuth2AuthorizationCode) TableName() string {
	return "oauth2_authorization_code"
}

// GenerateRedirectURI generates a redirect URI for a successful authorization request. State will be used if not empty.
func (code *OAuth2AuthorizationCode) GenerateRedirectURI(state string) (redirect *url.URL, err error) {
	if redirect, err = url.Parse(code.RedirectURI); err != nil {
		return
	}
	q := redirect.Query()
	if state != "" {
		q.Set("state", state)
	}
	q.Set("code", code.Code)
	redirect.RawQuery = q.Encode()
	return
}

// Invalidate deletes the auth code from the database to invalidate this code
func (code *OAuth2AuthorizationCode) Invalidate() error {
	return code.invalidate(x)
}

func (code *OAuth2AuthorizationCode) invalidate(e Engine) error {
	_, err := e.Delete(code)
	return err
}

// GetOAuth2AuthorizationByCode returns an authorization by its code
func GetOAuth2AuthorizationByCode(code string) (*OAuth2AuthorizationCode, error) {
	return getOAuth2AuthorizationByCode(x, code)
}

func getOAuth2AuthorizationByCode(e Engine, code string) (auth *OAuth2AuthorizationCode, err error) {
	auth = new(OAuth2AuthorizationCode)
	if has, err := e.Where("code = ?", code).Get(auth); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	auth.Grant = new(OAuth2Grant)
	if has, err := e.ID(auth.GrantID).Get(auth.Grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return auth, nil
}

//////////////////////////////////////////////////////

// OAuth2Grant represents the permission of an user for a specifc application to access resources
type OAuth2Grant struct {
	ID            int64          `xorm:"pk autoincr"`
	UserID        int64          `xorm:"INDEX unique(user_application)"`
	ApplicationID int64          `xorm:"INDEX unique(user_application)"`
	CreatedUnix   util.TimeStamp `xorm:"created"`
	UpdatedUnix   util.TimeStamp `xorm:"updated"`
}

// TableName sets the table name to `oauth2_grant`
func (grant *OAuth2Grant) TableName() string {
	return "oauth2_grant"
}

// GenerateNewAuthorizationCode generates a new authorization code for a grant and saves it to the databse
func (grant *OAuth2Grant) GenerateNewAuthorizationCode(redirectURI string) (*OAuth2AuthorizationCode, error) {
	return grant.generateNewAuthorizationCode(x, redirectURI)
}

func (grant *OAuth2Grant) generateNewAuthorizationCode(e Engine, redirectURI string) (*OAuth2AuthorizationCode, error) {
	secret := gouuid.NewV4().String()
	code := &OAuth2AuthorizationCode{
		Grant:       grant,
		GrantID:     grant.ID,
		RedirectURI: redirectURI,
		Code:        secret,
	}
	if _, err := e.Insert(code); err != nil {
		return nil, err
	}
	return code, nil
}

// GetOAuth2GrantByID returns the grant with the given ID
func GetOAuth2GrantByID(id int64) (*OAuth2Grant, error) {
	return getOAuth2GrantByID(x, id)
}

func getOAuth2GrantByID(e Engine, id int64) (grant *OAuth2Grant, err error) {
	grant = new(OAuth2Grant)
	if has, err := e.ID(id).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return
}

//////////////////////////////////////////////////////////////

// OAuth2TokenType represents the type of token for an oauth application
type OAuth2TokenType int

const (
	// TypeAccessToken is a token with short lifetime to access the api
	TypeAccessToken OAuth2TokenType = 0
	// TypeRefreshToken is token with long lifetime to refresh access tokens obtained by the client
	TypeRefreshToken = iota
)

// OAuth2Token represents a JWT token used to authenticate a client
type OAuth2Token struct {
	GrantID int64           `json:"sub"`
	Type    OAuth2TokenType `json:"tt"`
	jwt.StandardClaims
}

// ParseOAuth2Token parses a singed jwt string
func ParseOAuth2Token(jwtToken string) (*OAuth2Token, error) {
	parsedToken, err := jwt.ParseWithClaims(jwtToken, &OAuth2Token{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing algo: %v", token.Header["alg"])
		}
		return setting.OAuth2.JWTSecretBytes, nil
	})
	if err != nil {
		return nil, err
	}
	var token *OAuth2Token
	var ok bool
	if token, ok = parsedToken.Claims.(*OAuth2Token); !ok || !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return token, nil
}

// SignToken signs the token with the JWT secret
func (token *OAuth2Token) SignToken() (string, error) {
	token.IssuedAt = time.Now().Unix()
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS512, token)
	return jwtToken.SignedString(setting.OAuth2.JWTSecretBytes)
}
