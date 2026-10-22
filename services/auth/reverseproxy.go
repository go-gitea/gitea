// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"net/http"
	"strings"
	"uuid"

	audit_model "gitea.dev/models/audit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"
	"gitea.dev/services/audit"
)

// Ensure the struct implements the interface.
var (
	_ Method = &ReverseProxy{}
)

// ReverseProxyMethodName is the constant name of the ReverseProxy authentication method
const ReverseProxyMethodName = "reverse_proxy"

// ReverseProxy implements the Auth interface, but actually relies on
// a reverse proxy for authentication of users.
// On successful authentication the proxy is expected to populate the username in the
// "setting.ReverseProxyAuthUser" header. Optionally it can also populate the email of the
// user in the "setting.ReverseProxyAuthEmail" header.
type ReverseProxy struct {
	CreateSession bool
}

// getUserName extracts the username from the "setting.ReverseProxyAuthUser" header
func (r *ReverseProxy) getUserName(req *http.Request) string {
	return strings.TrimSpace(req.Header.Get(setting.ReverseProxyAuthUser))
}

// Name represents the name of auth method
func (r *ReverseProxy) Name() string {
	return ReverseProxyMethodName
}

// getUserFromAuthUser extracts the username from the "setting.ReverseProxyAuthUser" header
// of the request and returns the corresponding user object for that name.
// Verification of header data is not performed as it should have already been done by
// the reverse proxy.
// If a username is available in the "setting.ReverseProxyAuthUser" header an existing
// user object is returned (populated with username or email found in header).
// Returns nil if header is empty.
func (r *ReverseProxy) getUserFromAuthUser(req *http.Request) (*user_model.User, error) {
	return getReverseProxyUserByName(req.Context(), r.getIdentity(req))
}

// reverseProxyIdentity is the user identity asserted by a reverse proxy, either via plain headers or a verified JWT.
// Email and FullName are only populated when the corresponding ENABLE_REVERSE_PROXY_* setting is enabled.
type reverseProxyIdentity struct {
	Username string
	Email    string
	FullName string
}

func (r *ReverseProxy) getIdentity(req *http.Request) reverseProxyIdentity {
	id := reverseProxyIdentity{Username: r.getUserName(req)}
	if setting.Service.EnableReverseProxyEmail {
		id.Email = r.getEmail(req)
	}
	if setting.Service.EnableReverseProxyFullName {
		id.FullName = req.Header.Get(setting.ReverseProxyAuthFullName)
	}
	return id
}

// getReverseProxyUser looks up the user by name, falling back to email, see getReverseProxyUserByName and getReverseProxyUserByEmail.
// Returns nil if no user is found.
func getReverseProxyUser(ctx context.Context, id reverseProxyIdentity) (*user_model.User, error) {
	user, err := getReverseProxyUserByName(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		user = getReverseProxyUserByEmail(ctx, id.Email)
	}
	return user, nil
}

// getReverseProxyUserByName returns the user with the identity's username, auto-registering it if allowed.
// Returns nil if the username is empty.
func getReverseProxyUserByName(ctx context.Context, id reverseProxyIdentity) (*user_model.User, error) {
	if len(id.Username) == 0 {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}
	log.Trace("ReverseProxy Authorization: Found username: %s", id.Username)

	user, err := user_model.GetUserByName(ctx, id.Username)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) || !setting.Service.EnableReverseProxyAutoRegister {
			log.Error("GetUserByName: %v", err)
			return nil, err
		}
		user = newReverseProxyUser(ctx, id)
	}
	return user, nil
}

// getEmail extracts the email from the "setting.ReverseProxyAuthEmail" header
func (r *ReverseProxy) getEmail(req *http.Request) string {
	return strings.TrimSpace(req.Header.Get(setting.ReverseProxyAuthEmail))
}

// getUserFromAuthEmail extracts the username from the "setting.ReverseProxyAuthEmail" header
// of the request and returns the corresponding user object for that email.
// Verification of header data is not performed as it should have already been done by
// the reverse proxy.
// If an email is available in the "setting.ReverseProxyAuthEmail" header an existing
// user object is returned (populated with the email found in header).
// Returns nil if header is empty or if "setting.EnableReverseProxyEmail" is disabled.
func (r *ReverseProxy) getUserFromAuthEmail(req *http.Request) *user_model.User {
	return getReverseProxyUserByEmail(req.Context(), r.getIdentity(req).Email)
}

// getReverseProxyUserByEmail returns the user with the given email, or nil if the email is empty or no user is found.
func getReverseProxyUserByEmail(ctx context.Context, email string) *user_model.User {
	if len(email) == 0 {
		return nil
	}
	log.Trace("ReverseProxy Authorization: Found email: %s", email)

	user, err := user_model.GetUserByEmail(ctx, email)
	if err != nil {
		// Do not allow auto-registration, we don't have a username here
		if !user_model.IsErrUserNotExist(err) {
			log.Error("GetUserByEmail: %v", err)
		}
		return nil
	}
	return user
}

// Verify attempts to load a user object based on headers sent by the reverse proxy.
// First it will attempt to load it based on the username (see docs for getUserFromAuthUser),
// and failing that it will attempt to load it based on the email (see docs for getUserFromAuthEmail).
// Returns nil if the headers are empty or the user is not found.
func (r *ReverseProxy) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	user, err := r.getUserFromAuthUser(req)
	if err != nil {
		return nil, err
	}
	if user == nil {
		user = r.getUserFromAuthEmail(req)
	}
	if user == nil || !user.IsIndividual() {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	if r.CreateSession && sess != nil {
		sessionUID, ok := sess.Get(session.KeyUID).(int64)
		if !ok || sessionUID != user.ID {
			handleSignInNonInteractive(w, req, sess, user)
		}
	}
	store.GetData()["IsReverseProxy"] = true

	log.Trace("ReverseProxy Authorization: Logged in user %-v", user)
	return user, nil
}

// newReverseProxyUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information asserted by the reverse proxy.
func newReverseProxyUser(ctx context.Context, id reverseProxyIdentity) *user_model.User {
	if len(id.Username) == 0 {
		return nil
	}

	email := id.Email
	if len(email) == 0 {
		email = uuid.New().String() + "@localhost"
	}

	user := &user_model.User{
		Name:     id.Username,
		Email:    email,
		FullName: id.FullName,
	}

	overwriteDefault := user_model.CreateUserOverwriteOptions{
		IsActive: optional.Some(true),
	}

	if err := user_model.CreateUser(ctx, user, &user_model.Meta{}, &overwriteDefault); err != nil {
		// FIXME: should I create a system notice?
		log.Error("CreateUser: %v", err)
		return nil
	}

	audit.RecordAs(ctx, user_model.NewAuthSourceUser(), audit_model.UserCreate, user)

	return user
}
