// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"encoding/gob"
	"net/http"
	"sync"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth/gothic"
)

var gothRWMutex = sync.RWMutex{}

// UsersStoreKey is the key for the store
const UsersStoreKey = "gitea-oauth2-sessions"

// ProviderHeaderKey is the HTTP header key
const ProviderHeaderKey = "gitea-oauth2-provider"

// Init initializes the oauth source
func Init() error {
	if err := InitSigningKey(); err != nil {
		return err
	}

	// Lock our mutex
	gothRWMutex.Lock()

	gob.Register(&sessions.Session{})

	gothic.Store = &SessionsStore{
		maxLength: int64(setting.OAuth2.MaxTokenLength),
	}

	gothic.SetState = func(req *http.Request) string {
		return uuid.New().String()
	}

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return req.Header.Get(ProviderHeaderKey), nil
	}

	// Unlock our mutex
	gothRWMutex.Unlock()

	return initOAuth2Sources()
}

// ResetOAuth2 clears existing OAuth2 providers and loads them from DB
func ResetOAuth2() error {
	ClearProviders()
	return initOAuth2Sources()
}

// initOAuth2Sources is used to load and register all active OAuth2 providers
func initOAuth2Sources() error {
	authSources, _ := auth.GetActiveOAuth2ProviderSources()
	for _, source := range authSources {
		oauth2Source, ok := source.Cfg.(*Source)
		if !ok {
			continue
		}
		err := oauth2Source.RegisterSource()
		if err != nil {
			log.Critical("Unable to register source: %s due to Error: %v.", source.Name, err)
		}
	}
	return nil
}
