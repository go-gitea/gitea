// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"encoding/gob"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/log"
	chiSession "gitea.com/go-chi/session"
	"github.com/gorilla/sessions"
)

// SessionsStore creates a gothic store from our session
type SessionsStore struct {
	maxLength int64
}

// Get should return a cached session.
func (st *SessionsStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return st.getOrNew(r, name, false)
}

// New should create and return a new session.
//
// Note that New should never return a nil session, even in the case of
// an error if using the Registry infrastructure to cache the session.
func (st *SessionsStore) New(r *http.Request, name string) (*sessions.Session, error) {
	return st.getOrNew(r, name, true)
}

// getOrNew gets the session from the chi-session if it exists. Override permits the overriding of an unexpected object.
func (st *SessionsStore) getOrNew(r *http.Request, name string, override bool) (*sessions.Session, error) {
	chiStore := chiSession.GetSession(r)

	session := sessions.NewSession(st, name)

	rawData := chiStore.Get(name)
	if rawData != nil {
		oldSession, ok := rawData.(*sessions.Session)
		if ok {
			session.ID = oldSession.ID
			session.IsNew = oldSession.IsNew
			session.Options = oldSession.Options
			session.Values = oldSession.Values

			return session, nil
		} else if !override {
			log.Error("Unexpected object in session at name: %s: %v", name, rawData)
			return nil, fmt.Errorf("unexpected object in session at name: %s", name)
		}
	}

	session.ID = chiStore.ID() // Simply copy the session id from the chi store

	return session, chiStore.Set(name, session)
}

// Save should persist session to the underlying store implementation.
func (st *SessionsStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	chiStore := chiSession.GetSession(r)

	if err := chiStore.Set(session.Name(), session); err != nil {
		return err
	}

	if st.maxLength > 0 {
		sizeWriter := &sizeWriter{}

		_ = gob.NewEncoder(sizeWriter).Encode(session)
		if sizeWriter.size > st.maxLength {
			return fmt.Errorf("encode session: Data too long: %d > %d", sizeWriter.size, st.maxLength)
		}
	}

	return chiStore.Release()
}

type sizeWriter struct {
	size int64
}

func (s *sizeWriter) Write(data []byte) (int, error) {
	s.size += int64(len(data))
	return len(data), nil
}

var _ (sessions.Store) = &SessionsStore{}
