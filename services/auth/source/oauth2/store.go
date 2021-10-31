// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"fmt"
	"net/http"

	chiSession "gitea.com/go-chi/session"
	"github.com/gorilla/sessions"
)

// SessionsStore creates a gothic store from our session
type SessionsStore struct {
}

// Get should return a cached session.
func (st *SessionsStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	chiStore := chiSession.GetSession(r)

	rawData := chiStore.Get(name)
	if rawData == nil {
		return st.New(r, name)
	}

	oldSession, ok := rawData.(*sessions.Session)
	if !ok {
		return nil, fmt.Errorf("unexpected object in session: %v at name: %s", rawData, name)
	}

	// Copy over the old data into the session
	session := sessions.NewSession(st, name)
	session.ID = oldSession.ID
	session.IsNew = oldSession.IsNew
	session.Options = oldSession.Options
	session.Values = oldSession.Values

	return session, nil
}

// New should create and return a new session.
//
// Note that New should never return a nil session, even in the case of
// an error if using the Registry infrastructure to cache the session.
func (st *SessionsStore) New(r *http.Request, name string) (*sessions.Session, error) {
	chiStore := chiSession.GetSession(r)

	session := sessions.NewSession(st, name)
	session.ID = chiStore.ID()

	rawData := chiStore.Get(name)
	if rawData != nil {
		oldSession, ok := rawData.(*sessions.Session)
		if ok {
			session.ID = oldSession.ID
			session.IsNew = oldSession.IsNew
			session.Options = oldSession.Options
			session.Values = oldSession.Values

			return session, nil
		}
	}

	return session, chiStore.Set(name, session)
}

// Save should persist session to the underlying store implementation.
func (st *SessionsStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	chiStore := chiSession.GetSession(r)

	if err := chiStore.Set(session.Name(), session); err != nil {
		return err
	}

	return chiStore.Release()
}

var _ (sessions.Store) = &SessionsStore{}
