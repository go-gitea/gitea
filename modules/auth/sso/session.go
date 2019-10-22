package sso

import (
	"code.gitea.io/gitea/models"

	"gitea.com/macaron/macaron"
	"gitea.com/macaron/session"
)

// Session checks if there is a user uid stored in the session and returns the user
// object for that uid.
type Session struct {
}

// Init does nothing as the Session implementation does not need to allocate any resources
func (s *Session) Init() error {
	return nil
}

// Free does nothing as the Session implementation does not have to release any resources
func (s *Session) Free() error {
	return nil
}

// IsEnabled returns true as this plugin is enabled by default and its not possible to disable
// it from settings.
func (s *Session) IsEnabled() bool {
	return true
}

// Priority determines the order in which authentication methods are executed.
// The lower the priority, the sooner the plugin is executed.
func (s *Session) Priority() int {
	return 20000
}

// VerifyAuthData checks if there is a user uid stored in the session and returns the user
// object for that uid.
// Returns nil if there is no user uid stored in the session.
func (s *Session) VerifyAuthData(ctx *macaron.Context, sess session.Store) *models.User {
	user := SessionUser(sess)
	if user != nil {
		return user
	}
	return nil
}

// init registers the plugin to the list of available SSO methods
func init() {
	Register(&Session{})
}
