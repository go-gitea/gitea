// Copyright 2013 Beego Authors
// Copyright 2014 The Macaron Authors
// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

type backend interface {
	load(sid string) ([]byte, error)                 // returns nil for missing or expired sessions, refreshes the expiry of others
	save(sid string, data []byte, create bool) error // without create, only an existing session is updated
	destroy(sid string) error
	gc()
}

// CHI-SESSION-GOB-REGISTER: packages must gob.Register the types they store at startup, so data stored before a restart still decodes
func init() {
	gob.Register([]any{})
	gob.Register(map[int]any{})
	gob.Register(map[string]any{})
	gob.Register(map[any]any{})
	gob.Register(map[string]string{})
	gob.Register(map[int]string{})
	gob.Register(map[int]int{})
	gob.Register(map[int]int64{})
}

func newBackend(provider, config string, maxLifetime int64) (backend, error) {
	switch provider {
	case "memory":
		return newMemoryBackend(maxLifetime), nil
	case "file":
		return newFileBackend(config, maxLifetime), nil
	case "redis":
		return newRedisBackend(config, maxLifetime)
	case "db":
		return &dbBackend{maxLifetime: maxLifetime}, nil
	}
	return nil, fmt.Errorf(`unsupported [session] PROVIDER %q, supported are "memory", "file", "redis" and "db", use "db" or "redis" to replace the removed "mysql", "postgres", "couchbase" and "memcache" providers`, provider)
}

func Sessioner() (func(next http.Handler) http.Handler, error) {
	backend, err := newBackend(setting.SessionConfig.Provider, setting.SessionConfig.ProviderConfig, setting.SessionConfig.Maxlifetime)
	if err != nil {
		return nil, err
	}
	go runGC(graceful.GetManager().ShutdownContext(), backend, time.Duration(setting.SessionConfig.Gclifetime)*time.Second)
	return func(next http.Handler) http.Handler {
		return sessionHandler(backend, next)
	}, nil
}

func sessionHandler(backend backend, next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		sess, err := startSession(backend, resp, req)
		if err != nil {
			log.Error("Unable to start session: %v", err)
			resp.WriteHeader(http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(resp, req.WithContext(context.WithValue(req.Context(), ContextKey, sess)))
		if err := sess.Release(); err != nil {
			log.Error("Unable to release session: %v", err)
		}
	})
}

func startSession(backend backend, resp http.ResponseWriter, req *http.Request) (*store, error) {
	sess := &store{backend: backend, resp: resp, data: map[any]any{}}
	cookie, err := req.Cookie(setting.SessionConfig.CookieName)
	if err != nil || !isValidSessionID(cookie.Value) {
		sess.sid = newSessionID()
		return sess, nil
	}
	sess.sid, sess.cookieSID = cookie.Value, cookie.Value
	encoded, err := backend.load(sess.sid)
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 {
		return sess, nil
	}
	var data map[any]any
	if err := util.UnpackData(encoded, &data); err != nil {
		log.Error("Unable to decode session data, starting with an empty session: %v", err)
		sess.stored, sess.changed = true, true
	} else if len(data) > 0 {
		sess.data, sess.stored = data, true
	}
	return sess, nil
}

func newSessionID() string {
	return hex.EncodeToString(util.CryptoRandomBytes(8))
}

// isValidSessionID also keeps the file provider's paths inside its directory
func isValidSessionID(sid string) bool {
	return len(sid) == 16 && strings.Trim(sid, "0123456789abcdef") == ""
}

func newCookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     setting.SessionConfig.CookieName,
		Value:    value,
		Path:     util.IfZero(setting.SessionConfig.CookiePath, "/"),
		Domain:   setting.SessionConfig.Domain,
		Secure:   setting.SessionConfig.Secure,
		HttpOnly: true,
		SameSite: setting.SessionConfig.SameSite,
	}
}

func runGC(ctx context.Context, backend backend, interval time.Duration) {
	for {
		backend.gc()
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
