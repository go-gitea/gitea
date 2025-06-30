// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	uuid "github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"xorm.io/builder"
	"xorm.io/xorm"
)

// OAuth2Application represents an OAuth2 client (RFC 6749)
type OAuth2Application struct {
	ID           int64 `xorm:"pk autoincr"`
	UID          int64 `xorm:"INDEX"`
	Name         string
	ClientID     string `xorm:"unique"`
	ClientSecret string
	// OAuth defines both Confidential and Public client types
	// https://datatracker.ietf.org/doc/html/rfc6749#section-2.1
	// "Authorization servers MUST record the client type in the client registration details"
	// https://datatracker.ietf.org/doc/html/rfc8252#section-8.4
	ConfidentialClient         bool               `xorm:"NOT NULL DEFAULT TRUE"`
	SkipSecondaryAuthorization bool               `xorm:"NOT NULL DEFAULT FALSE"`
	RedirectURIs               []string           `xorm:"redirect_uris JSON TEXT"`
	CreatedUnix                timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix                timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(OAuth2Application))
	db.RegisterModel(new(OAuth2AuthorizationCode))
	db.RegisterModel(new(OAuth2Grant))
}

type BuiltinOAuth2Application struct {
	ConfigName   string
	DisplayName  string
	RedirectURIs []string
}

func BuiltinApplications() map[string]*BuiltinOAuth2Application {
	m := make(map[string]*BuiltinOAuth2Application)
	m["a4792ccc-144e-407e-86c9-5e7d8d9c3269"] = &BuiltinOAuth2Application{
		ConfigName:   "git-credential-oauth",
		DisplayName:  "git-credential-oauth",
		RedirectURIs: []string{"http://127.0.0.1", "https://127.0.0.1"},
	}
	m["e90ee53c-94e2-48ac-9358-a874fb9e0662"] = &BuiltinOAuth2Application{
		ConfigName:   "git-credential-manager",
		DisplayName:  "Git Credential Manager",
		RedirectURIs: []string{"http://127.0.0.1", "https://127.0.0.1"},
	}
	m["d57cb8c4-630c-4168-8324-ec79935e18d4"] = &BuiltinOAuth2Application{
		ConfigName:   "tea",
		DisplayName:  "tea",
		RedirectURIs: []string{"http://127.0.0.1", "https://127.0.0.1"},
	}
	return m
}

func Init(ctx context.Context) error {
	builtinApps := BuiltinApplications()
	var builtinAllClientIDs []string
	for clientID := range builtinApps {
		builtinAllClientIDs = append(builtinAllClientIDs, clientID)
	}

	var registeredApps []*OAuth2Application
	if err := db.GetEngine(ctx).In("client_id", builtinAllClientIDs).Find(&registeredApps); err != nil {
		return err
	}

	clientIDsToAdd := container.Set[string]{}
	for _, configName := range setting.OAuth2.DefaultApplications {
		found := false
		for clientID, builtinApp := range builtinApps {
			if builtinApp.ConfigName == configName {
				clientIDsToAdd.Add(clientID) // add all user-configured apps to the "add" list
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unknown oauth2 application: %q", configName)
		}
	}
	clientIDsToDelete := container.Set[string]{}
	for _, app := range registeredApps {
		if !clientIDsToAdd.Contains(app.ClientID) {
			clientIDsToDelete.Add(app.ClientID) // if a registered app is not in the "add" list, it should be deleted
		}
	}
	for _, app := range registeredApps {
		clientIDsToAdd.Remove(app.ClientID) // no need to re-add existing (registered) apps, so remove them from the set
	}

	for _, app := range registeredApps {
		if clientIDsToDelete.Contains(app.ClientID) {
			if err := deleteOAuth2Application(ctx, app.ID, 0); err != nil {
				return err
			}
		}
	}
	for clientID := range clientIDsToAdd {
		builtinApp := builtinApps[clientID]
		if err := db.Insert(ctx, &OAuth2Application{
			Name:         builtinApp.DisplayName,
			ClientID:     clientID,
			RedirectURIs: builtinApp.RedirectURIs,
		}); err != nil {
			return err
		}
	}

	return nil
}

// TableName sets the table name to `oauth2_application`
func (app *OAuth2Application) TableName() string {
	return "oauth2_application"
}

// ContainsRedirectURI checks if redirectURI is allowed for app
func (app *OAuth2Application) ContainsRedirectURI(redirectURI string) bool {
	// OAuth2 requires the redirect URI to be an exact match, no dynamic parts are allowed.
	// https://stackoverflow.com/questions/55524480/should-dynamic-query-parameters-be-present-in-the-redirection-uri-for-an-oauth2
	// https://www.rfc-editor.org/rfc/rfc6819#section-5.2.3.3
	// https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest
	// https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics-12#section-3.1
	contains := func(s string) bool {
		s = strings.TrimSuffix(strings.ToLower(s), "/")
		for _, u := range app.RedirectURIs {
			if strings.TrimSuffix(strings.ToLower(u), "/") == s {
				return true
			}
		}
		return false
	}
	if !app.ConfidentialClient {
		uri, err := url.Parse(redirectURI)
		// ignore port for http loopback uris following https://datatracker.ietf.org/doc/html/rfc8252#section-7.3
		if err == nil && uri.Scheme == "http" && uri.Port() != "" {
			ip := net.ParseIP(uri.Hostname())
			if ip != nil && ip.IsLoopback() {
				// strip port
				uri.Host = uri.Hostname()
				if contains(uri.String()) {
					return true
				}
			}
		}
	}
	return contains(redirectURI)
}

// Base32 characters, but lowercased.
const lowerBase32Chars = "abcdefghijklmnopqrstuvwxyz234567"

// base32 encoder that uses lowered characters without padding.
var base32Lower = base32.NewEncoding(lowerBase32Chars).WithPadding(base32.NoPadding)

// GenerateClientSecret will generate the client secret and returns the plaintext and saves the hash at the database
func (app *OAuth2Application) GenerateClientSecret(ctx context.Context) (string, error) {
	rBytes, err := util.CryptoRandomBytes(32)
	if err != nil {
		return "", err
	}
	// Add a prefix to the base32, this is in order to make it easier
	// for code scanners to grab sensitive tokens.
	clientSecret := "gto_" + base32Lower.EncodeToString(rBytes)

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	app.ClientSecret = string(hashedSecret)
	if _, err := db.GetEngine(ctx).ID(app.ID).Cols("client_secret").Update(app); err != nil {
		return "", err
	}
	return clientSecret, nil
}

// ValidateClientSecret validates the given secret by the hash saved in database
func (app *OAuth2Application) ValidateClientSecret(secret []byte) bool {
	return bcrypt.CompareHashAndPassword([]byte(app.ClientSecret), secret) == nil
}

// GetGrantByUserID returns a OAuth2Grant by its user and application ID
func (app *OAuth2Application) GetGrantByUserID(ctx context.Context, userID int64) (grant *OAuth2Grant, err error) {
	grant = new(OAuth2Grant)
	if has, err := db.GetEngine(ctx).Where("user_id = ? AND application_id = ?", userID, app.ID).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return grant, nil
}

// CreateGrant generates a grant for an user
func (app *OAuth2Application) CreateGrant(ctx context.Context, userID int64, scope string) (*OAuth2Grant, error) {
	grant := &OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        userID,
		Scope:         scope,
	}
	err := db.Insert(ctx, grant)
	if err != nil {
		return nil, err
	}
	return grant, nil
}

// GetOAuth2ApplicationByClientID returns the oauth2 application with the given client_id. Returns an error if not found.
func GetOAuth2ApplicationByClientID(ctx context.Context, clientID string) (app *OAuth2Application, err error) {
	app = new(OAuth2Application)
	has, err := db.GetEngine(ctx).Where("client_id = ?", clientID).Get(app)
	if !has {
		return nil, ErrOAuthClientIDInvalid{ClientID: clientID}
	}
	return app, err
}

// GetOAuth2ApplicationByID returns the oauth2 application with the given id. Returns an error if not found.
func GetOAuth2ApplicationByID(ctx context.Context, id int64) (app *OAuth2Application, err error) {
	app = new(OAuth2Application)
	has, err := db.GetEngine(ctx).ID(id).Get(app)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrOAuthApplicationNotFound{ID: id}
	}
	return app, nil
}

// CreateOAuth2ApplicationOptions holds options to create an oauth2 application
type CreateOAuth2ApplicationOptions struct {
	Name                       string
	UserID                     int64
	ConfidentialClient         bool
	SkipSecondaryAuthorization bool
	RedirectURIs               []string
}

// CreateOAuth2Application inserts a new oauth2 application
func CreateOAuth2Application(ctx context.Context, opts CreateOAuth2ApplicationOptions) (*OAuth2Application, error) {
	clientID := uuid.New().String()
	app := &OAuth2Application{
		UID:                        opts.UserID,
		Name:                       opts.Name,
		ClientID:                   clientID,
		RedirectURIs:               opts.RedirectURIs,
		ConfidentialClient:         opts.ConfidentialClient,
		SkipSecondaryAuthorization: opts.SkipSecondaryAuthorization,
	}
	if err := db.Insert(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

// UpdateOAuth2ApplicationOptions holds options to update an oauth2 application
type UpdateOAuth2ApplicationOptions struct {
	ID                         int64
	Name                       string
	UserID                     int64
	ConfidentialClient         bool
	SkipSecondaryAuthorization bool
	RedirectURIs               []string
}

// UpdateOAuth2Application updates an oauth2 application
func UpdateOAuth2Application(ctx context.Context, opts UpdateOAuth2ApplicationOptions) (*OAuth2Application, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	app, err := GetOAuth2ApplicationByID(ctx, opts.ID)
	if err != nil {
		return nil, err
	}
	if app.UID != opts.UserID {
		return nil, errors.New("UID mismatch")
	}
	builtinApps := BuiltinApplications()
	if _, builtin := builtinApps[app.ClientID]; builtin {
		return nil, fmt.Errorf("failed to edit OAuth2 application: application is locked: %s", app.ClientID)
	}

	app.Name = opts.Name
	app.RedirectURIs = opts.RedirectURIs
	app.ConfidentialClient = opts.ConfidentialClient
	app.SkipSecondaryAuthorization = opts.SkipSecondaryAuthorization

	if err = updateOAuth2Application(ctx, app); err != nil {
		return nil, err
	}
	app.ClientSecret = ""

	return app, committer.Commit()
}

func updateOAuth2Application(ctx context.Context, app *OAuth2Application) error {
	if _, err := db.GetEngine(ctx).ID(app.ID).UseBool("confidential_client", "skip_secondary_authorization").Update(app); err != nil {
		return err
	}
	return nil
}

func deleteOAuth2Application(ctx context.Context, id, userid int64) error {
	sess := db.GetEngine(ctx)
	// the userid could be 0 if the app is instance-wide
	if deleted, err := sess.Where(builder.Eq{"id": id, "uid": userid}).Delete(&OAuth2Application{}); err != nil {
		return err
	} else if deleted == 0 {
		return ErrOAuthApplicationNotFound{ID: id}
	}
	codes := make([]*OAuth2AuthorizationCode, 0)
	// delete correlating auth codes
	if err := sess.Join("INNER", "oauth2_grant",
		"oauth2_authorization_code.grant_id = oauth2_grant.id AND oauth2_grant.application_id = ?", id).Find(&codes); err != nil {
		return err
	}
	codeIDs := make([]int64, 0, len(codes))
	for _, grant := range codes {
		codeIDs = append(codeIDs, grant.ID)
	}

	if _, err := sess.In("id", codeIDs).Delete(new(OAuth2AuthorizationCode)); err != nil {
		return err
	}

	if _, err := sess.Where("application_id = ?", id).Delete(new(OAuth2Grant)); err != nil {
		return err
	}
	return nil
}

// DeleteOAuth2Application deletes the application with the given id and the grants and auth codes related to it. It checks if the userid was the creator of the app.
func DeleteOAuth2Application(ctx context.Context, id, userid int64) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	app, err := GetOAuth2ApplicationByID(ctx, id)
	if err != nil {
		return err
	}
	builtinApps := BuiltinApplications()
	if _, builtin := builtinApps[app.ClientID]; builtin {
		return fmt.Errorf("failed to delete OAuth2 application: application is locked: %s", app.ClientID)
	}
	if err := deleteOAuth2Application(ctx, id, userid); err != nil {
		return err
	}
	return committer.Commit()
}

//////////////////////////////////////////////////////

// OAuth2AuthorizationCode is a code to obtain an access token in combination with the client secret once. It has a limited lifetime.
type OAuth2AuthorizationCode struct {
	ID                  int64        `xorm:"pk autoincr"`
	Grant               *OAuth2Grant `xorm:"-"`
	GrantID             int64
	Code                string `xorm:"INDEX unique"`
	CodeChallenge       string
	CodeChallengeMethod string
	RedirectURI         string
	ValidUntil          timeutil.TimeStamp `xorm:"index"`
}

// TableName sets the table name to `oauth2_authorization_code`
func (code *OAuth2AuthorizationCode) TableName() string {
	return "oauth2_authorization_code"
}

// GenerateRedirectURI generates a redirect URI for a successful authorization request. State will be used if not empty.
func (code *OAuth2AuthorizationCode) GenerateRedirectURI(state string) (*url.URL, error) {
	redirect, err := url.Parse(code.RedirectURI)
	if err != nil {
		return nil, err
	}
	q := redirect.Query()
	if state != "" {
		q.Set("state", state)
	}
	q.Set("code", code.Code)
	redirect.RawQuery = q.Encode()
	return redirect, err
}

// Invalidate deletes the auth code from the database to invalidate this code
func (code *OAuth2AuthorizationCode) Invalidate(ctx context.Context) error {
	_, err := db.GetEngine(ctx).ID(code.ID).NoAutoCondition().Delete(code)
	return err
}

// ValidateCodeChallenge validates the given verifier against the saved code challenge. This is part of the PKCE implementation.
func (code *OAuth2AuthorizationCode) ValidateCodeChallenge(verifier string) bool {
	switch code.CodeChallengeMethod {
	case "S256":
		// base64url(SHA256(verifier)) see https://tools.ietf.org/html/rfc7636#section-4.6
		h := sha256.Sum256([]byte(verifier))
		hashedVerifier := base64.RawURLEncoding.EncodeToString(h[:])
		return hashedVerifier == code.CodeChallenge
	case "plain":
		return verifier == code.CodeChallenge
	case "":
		return true
	default:
		// unsupported method -> return false
		return false
	}
}

// GetOAuth2AuthorizationByCode returns an authorization by its code
func GetOAuth2AuthorizationByCode(ctx context.Context, code string) (auth *OAuth2AuthorizationCode, err error) {
	auth = new(OAuth2AuthorizationCode)
	if has, err := db.GetEngine(ctx).Where("code = ?", code).Get(auth); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	auth.Grant = new(OAuth2Grant)
	if has, err := db.GetEngine(ctx).ID(auth.GrantID).Get(auth.Grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return auth, nil
}

//////////////////////////////////////////////////////

// OAuth2Grant represents the permission of an user for a specific application to access resources
type OAuth2Grant struct {
	ID            int64              `xorm:"pk autoincr"`
	UserID        int64              `xorm:"INDEX unique(user_application)"`
	Application   *OAuth2Application `xorm:"-"`
	ApplicationID int64              `xorm:"INDEX unique(user_application)"`
	Counter       int64              `xorm:"NOT NULL DEFAULT 1"`
	Scope         string             `xorm:"TEXT"`
	Nonce         string             `xorm:"TEXT"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets the table name to `oauth2_grant`
func (grant *OAuth2Grant) TableName() string {
	return "oauth2_grant"
}

// GenerateNewAuthorizationCode generates a new authorization code for a grant and saves it to the database
func (grant *OAuth2Grant) GenerateNewAuthorizationCode(ctx context.Context, redirectURI, codeChallenge, codeChallengeMethod string) (code *OAuth2AuthorizationCode, err error) {
	rBytes, err := util.CryptoRandomBytes(32)
	if err != nil {
		return &OAuth2AuthorizationCode{}, err
	}
	// Add a prefix to the base32, this is in order to make it easier
	// for code scanners to grab sensitive tokens.
	codeSecret := "gta_" + base32Lower.EncodeToString(rBytes)

	code = &OAuth2AuthorizationCode{
		Grant:               grant,
		GrantID:             grant.ID,
		RedirectURI:         redirectURI,
		Code:                codeSecret,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	}
	if err := db.Insert(ctx, code); err != nil {
		return nil, err
	}
	return code, nil
}

// IncreaseCounter increases the counter and updates the grant
func (grant *OAuth2Grant) IncreaseCounter(ctx context.Context) error {
	_, err := db.GetEngine(ctx).ID(grant.ID).Incr("counter").Update(new(OAuth2Grant))
	if err != nil {
		return err
	}
	updatedGrant, err := GetOAuth2GrantByID(ctx, grant.ID)
	if err != nil {
		return err
	}
	grant.Counter = updatedGrant.Counter
	return nil
}

// ScopeContains returns true if the grant scope contains the specified scope
func (grant *OAuth2Grant) ScopeContains(scope string) bool {
	return slices.Contains(strings.Split(grant.Scope, " "), scope)
}

// SetNonce updates the current nonce value of a grant
func (grant *OAuth2Grant) SetNonce(ctx context.Context, nonce string) error {
	grant.Nonce = nonce
	_, err := db.GetEngine(ctx).ID(grant.ID).Cols("nonce").Update(grant)
	if err != nil {
		return err
	}
	return nil
}

// GetOAuth2GrantByID returns the grant with the given ID
func GetOAuth2GrantByID(ctx context.Context, id int64) (grant *OAuth2Grant, err error) {
	grant = new(OAuth2Grant)
	if has, err := db.GetEngine(ctx).ID(id).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return grant, err
}

// GetOAuth2GrantsByUserID lists all grants of a certain user
func GetOAuth2GrantsByUserID(ctx context.Context, uid int64) ([]*OAuth2Grant, error) {
	type joinedOAuth2Grant struct {
		Grant       *OAuth2Grant       `xorm:"extends"`
		Application *OAuth2Application `xorm:"extends"`
	}
	var results *xorm.Rows
	var err error
	if results, err = db.GetEngine(ctx).
		Table("oauth2_grant").
		Where("user_id = ?", uid).
		Join("INNER", "oauth2_application", "application_id = oauth2_application.id").
		Rows(new(joinedOAuth2Grant)); err != nil {
		return nil, err
	}
	defer results.Close()
	grants := make([]*OAuth2Grant, 0)
	for results.Next() {
		joinedGrant := new(joinedOAuth2Grant)
		if err := results.Scan(joinedGrant); err != nil {
			return nil, err
		}
		joinedGrant.Grant.Application = joinedGrant.Application
		grants = append(grants, joinedGrant.Grant)
	}
	return grants, nil
}

// RevokeOAuth2Grant deletes the grant with grantID and userID
func RevokeOAuth2Grant(ctx context.Context, grantID, userID int64) error {
	_, err := db.GetEngine(ctx).Where(builder.Eq{"id": grantID, "user_id": userID}).Delete(&OAuth2Grant{})
	return err
}

// ErrOAuthClientIDInvalid will be thrown if client id cannot be found
type ErrOAuthClientIDInvalid struct {
	ClientID string
}

// IsErrOauthClientIDInvalid checks if an error is a ErrOAuthClientIDInvalid.
func IsErrOauthClientIDInvalid(err error) bool {
	_, ok := err.(ErrOAuthClientIDInvalid)
	return ok
}

// Error returns the error message
func (err ErrOAuthClientIDInvalid) Error() string {
	return fmt.Sprintf("Client ID invalid [Client ID: %s]", err.ClientID)
}

// Unwrap unwraps this as a ErrNotExist err
func (err ErrOAuthClientIDInvalid) Unwrap() error {
	return util.ErrNotExist
}

// ErrOAuthApplicationNotFound will be thrown if id cannot be found
type ErrOAuthApplicationNotFound struct {
	ID int64
}

// IsErrOAuthApplicationNotFound checks if an error is a ErrReviewNotExist.
func IsErrOAuthApplicationNotFound(err error) bool {
	_, ok := err.(ErrOAuthApplicationNotFound)
	return ok
}

// Error returns the error message
func (err ErrOAuthApplicationNotFound) Error() string {
	return fmt.Sprintf("OAuth application not found [ID: %d]", err.ID)
}

// Unwrap unwraps this as a ErrNotExist err
func (err ErrOAuthApplicationNotFound) Unwrap() error {
	return util.ErrNotExist
}

// GetActiveOAuth2SourceByName returns a OAuth2 AuthSource based on the given name
func GetActiveOAuth2SourceByName(ctx context.Context, name string) (*Source, error) {
	authSource := new(Source)
	has, err := db.GetEngine(ctx).Where("name = ? and type = ? and is_active = ?", name, OAuth2, true).Get(authSource)
	if err != nil {
		return nil, err
	}

	if !has {
		return nil, fmt.Errorf("oauth2 source not found, name: %q", name)
	}

	return authSource, nil
}

func DeleteOAuth2RelictsByUserID(ctx context.Context, userID int64) error {
	deleteCond := builder.Select("id").From("oauth2_grant").Where(builder.Eq{"oauth2_grant.user_id": userID})

	if _, err := db.GetEngine(ctx).In("grant_id", deleteCond).
		Delete(&OAuth2AuthorizationCode{}); err != nil {
		return err
	}

	if err := db.DeleteBeans(ctx,
		&OAuth2Application{UID: userID},
		&OAuth2Grant{UserID: userID},
	); err != nil {
		return fmt.Errorf("DeleteBeans: %w", err)
	}

	return nil
}
