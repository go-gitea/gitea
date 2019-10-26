package sso

import (
	"reflect"
	"sort"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"gitea.com/macaron/session"
)

var (
	ssoMethods []SingleSignOn
)

// Methods returns the instances of all registered SSO methods
func Methods() []SingleSignOn {
	return ssoMethods
}

// MethodsByPriority returns the instances of all registered SSO methods, ordered by ascending priority
func MethodsByPriority() []SingleSignOn {
	methods := Methods()
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Priority() < methods[j].Priority()
	})
	return methods
}

// Register adds the specified instance to the list of available SSO methods
func Register(method SingleSignOn) {
	ssoMethods = append(ssoMethods, method)
}

// Init should be called exactly once when the application starts to allow SSO plugins
// to allocate necessary resources
func Init() {
	for _, method := range Methods() {
		if !method.IsEnabled() {
			continue
		}
		err := method.Init()
		if err != nil {
			log.Error("Could not initialize '%s' SSO method, error: %s", reflect.TypeOf(method).String(), err)
		}
	}
}

// Free should be called exactly once when the application is terminating to allow SSO plugins
// to release necessary resources
func Free() {
	for _, method := range Methods() {
		if !method.IsEnabled() {
			continue
		}
		err := method.Free()
		if err != nil {
			log.Error("Could not free '%s' SSO method, error: %s", reflect.TypeOf(method).String(), err)
		}
	}
}

// SessionUser returns the user object corresponding to the "uid" session variable.
func SessionUser(sess session.Store) *models.User {
	// Get user ID
	uid := sess.Get("uid")
	if uid == nil {
		return nil
	}
	id, ok := uid.(int64)
	if !ok {
		return nil
	}

	// Get user object
	user, err := models.GetUserByID(id)
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			log.Error("GetUserById: %v", err)
		}
		return nil
	}
	return user
}

// isAPIPath returns true if the specified URL is an API path
func isAPIPath(url string) bool {
	return strings.HasPrefix(url, "/api/")
}
