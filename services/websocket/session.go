// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"fmt"
	"net/url"

	user_model "code.gitea.io/gitea/models/user"

	"github.com/olahol/melody"
)

type sessionData struct {
	user  *user_model.User
	onURL string
}

func (d *sessionData) isOnURL(_u1 string) bool {
	if d.onURL == "" {
		return true
	}

	u1, _ := url.Parse(d.onURL)
	u2, _ := url.Parse(_u1)
	return u1.Path == u2.Path
}

func getSessionData(s *melody.Session) (*sessionData, error) {
	_data, ok := s.Get("data")
	if !ok {
		return nil, fmt.Errorf("no session data")
	}

	data, ok := _data.(*sessionData)
	if !ok {
		return nil, fmt.Errorf("invalid session data")
	}

	return data, nil
}
