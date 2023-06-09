// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"code.gitea.io/gitea/models/db"
)

// GetActiveSAMLProviderLoginSources returns all actived LoginSAML sources
func GetActiveSAMLProviderLoginSources() ([]*Source, error) {
	sources := make([]*Source, 0, 1)
	if err := db.GetEngine(db.DefaultContext).Where("is_active = ? and type = ?", true, SAML).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// GetActiveSAMLLoginSourceByName returns a OAuth2 LoginSource based on the given name
func GetActiveSAMLLoginSourceByName(name string) (*Source, error) {
	loginSource := new(Source)
	has, err := db.GetEngine(db.DefaultContext).Where("name = ? and type = ? and is_active = ?", name, SAML, true).Get(loginSource)
	if !has || err != nil {
		return nil, err
	}

	return loginSource, nil
}
