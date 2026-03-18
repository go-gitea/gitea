// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"net/http"

	"code.gitea.io/gitea/modules/setting"

	"gitea.com/go-chi/session"
)

type RawStore = session.RawStore

type Store interface {
	RawStore
	Destroy(http.ResponseWriter, *http.Request) error
}

type mockStoreContextKeyStruct struct{}

var MockStoreContextKey = mockStoreContextKeyStruct{}

// globalProvider holds a reference to the active VirtualSessionProvider
// so we can destroy sessions by ID without needing the http request/response.
var globalProvider *VirtualSessionProvider

// SetGlobalProvider stores the active session provider for use by DestroySessionByID.
func SetGlobalProvider(p *VirtualSessionProvider) {
	globalProvider = p
}

// DestroySessionByID destroys a session by its ID through the underlying provider.
// This works regardless of which session backend is configured (file, db, redis, etc).
func DestroySessionByID(sid string) error {
	if globalProvider == nil {
		return nil
	}
	return globalProvider.Destroy(sid)
}

// RegenerateSession regenerates the underlying session and returns the new store
func RegenerateSession(resp http.ResponseWriter, req *http.Request) (Store, error) {
	for _, f := range BeforeRegenerateSession {
		f(resp, req)
	}
	if setting.IsInTesting {
		if store := req.Context().Value(MockStoreContextKey); store != nil {
			return store.(Store), nil
		}
	}
	return session.RegenerateSession(resp, req)
}

func GetContextSession(req *http.Request) Store {
	if setting.IsInTesting {
		if store := req.Context().Value(MockStoreContextKey); store != nil {
			return store.(Store)
		}
	}
	return session.GetSession(req)
}

// BeforeRegenerateSession is a list of functions that are called before a session is regenerated.
var BeforeRegenerateSession []func(http.ResponseWriter, *http.Request)
