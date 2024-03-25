// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"code.gitea.io/gitea/modules/setting"
)

func GetSessionProvider() (*VirtualSessionProvider, error) {
	sessionProvider := &VirtualSessionProvider{}
	if err := sessionProvider.Init(setting.SessionConfig.Gclifetime, setting.SessionConfig.ProviderConfig); err != nil {
		return nil, err
	}
	return sessionProvider, nil
}
