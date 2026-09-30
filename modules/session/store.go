// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"net/http"
	"sync"

	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
)

type Store interface {
	Set(key, value any) error
	Get(key any) any
	Delete(key any) error
	ID() string
	Release() error
	Flush() error
	Destroy(http.ResponseWriter, *http.Request) error
	Regenerate(http.ResponseWriter, *http.Request)
}

type store struct {
	backend   backend
	resp      http.ResponseWriter
	lock      sync.RWMutex
	sid       string
	cookieSID string // the session ID the client holds
	data      map[any]any
	stored    bool // the backend holds data for sid
	changed   bool
}

type contextKeyStruct struct{}

var ContextKey = contextKeyStruct{}

func (s *store) Set(key, value any) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data[key] = value
	s.changed = true
	s.sendCookie(s.resp)
	return nil
}

func (s *store) Get(key any) any {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.data[key]
}

func (s *store) Delete(key any) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		s.changed = true
	}
	return nil
}

func (s *store) ID() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.sid
}

func (s *store) Flush() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	clear(s.data)
	s.changed = true
	return nil
}

// Release saves changed data to the backend, or only refreshes the expiry of unchanged data.
func (s *store) Release() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	var err error
	switch {
	case !s.changed && s.stored:
		return s.backend.touch(s.sid)
	case !s.changed:
		return nil
	case len(s.data) > 0:
		var data []byte
		if data, err = util.PackData(s.data); err == nil {
			err = s.backend.save(s.sid, data, !s.stored)
		}
	case s.stored:
		err = s.backend.destroy(s.sid)
	}
	if err == nil {
		s.changed, s.stored = false, len(s.data) > 0
	}
	return err
}

// Destroy deletes the session and its cookie, leaving an empty new session.
func (s *store) Destroy(resp http.ResponseWriter, _ *http.Request) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	err := s.backend.destroy(s.sid)
	cookie := newCookie("")
	cookie.MaxAge = -1
	http.SetCookie(resp, cookie)
	if err == nil {
		s.sid, s.stored = newSessionID(), false
	}
	s.cookieSID, s.changed = "", err != nil // Release retries a failed destroy
	clear(s.data)
	return err
}

// Regenerate moves the session data to a new session ID, so an ID known before sign-in is never authenticated
func (s *store) Regenerate(resp http.ResponseWriter, req *http.Request) {
	for _, f := range BeforeRegenerateSession {
		f(resp, req)
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.stored {
		if err := s.backend.destroy(s.sid); err != nil {
			log.Error("Unable to destroy the regenerated session: %v", err)
		}
	}
	s.sid, s.stored, s.changed = newSessionID(), false, true
	s.sendCookie(resp)
}

// sendCookie sets the session cookie once the session holds data under an ID the client doesn't know yet
func (s *store) sendCookie(resp http.ResponseWriter) {
	if s.cookieSID != s.sid && len(s.data) > 0 {
		http.SetCookie(resp, newCookie(s.sid))
		s.cookieSID = s.sid
	}
}

func GetContextSession(req *http.Request) Store {
	sess, _ := req.Context().Value(ContextKey).(Store)
	return sess
}

// BeforeRegenerateSession is a list of functions that are called before a session is regenerated.
var BeforeRegenerateSession []func(http.ResponseWriter, *http.Request)
