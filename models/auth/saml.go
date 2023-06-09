package auth

import (
	"code.gitea.io/gitea/models/db"

	auth_model "code.gitea.io/gitea/models/auth"
)

// GetActiveSAMLProviderLoginSources returns all actived LoginSAML sources
func GetActiveSAMLProviderLoginSources() ([]*auth_model.Source, error) {
	sources := make([]*auth_model.Source, 0, 1)
	if err := db.GetEngine(db.DefaultContext).Where("is_active = ? and type = ?", true, auth_model.SAML).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// GetActiveSAMLLoginSourceByName returns a OAuth2 LoginSource based on the given name
func GetActiveSAMLLoginSourceByName(name string) (*auth_model.Source, error) {
	loginSource := new(auth_model.Source)
	has, err := db.GetEngine(db.DefaultContext).Where("name = ? and type = ? and is_active = ?", name, auth_model.SAML, true).Get(loginSource)
	if !has || err != nil {
		return nil, err
	}

	return loginSource, nil
}
