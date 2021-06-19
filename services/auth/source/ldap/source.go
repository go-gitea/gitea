// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ldap

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth/ldap"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	jsoniter "github.com/json-iterator/go"
)

// .____     ________      _____ __________
// |    |    \______ \    /  _  \\______   \
// |    |     |    |  \  /  /_\  \|     ___/
// |    |___  |    `   \/    |    \    |
// |_______ \/_______  /\____|__  /____|
//         \/        \/         \/

// Source holds configuration for LDAP login source.
type Source struct {
	*ldap.Source
}

// FromDB fills up a LDAPConfig from serialized format.
func (source *Source) FromDB(bs []byte) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(bs, &source)
	if err != nil {
		return err
	}
	if source.BindPasswordEncrypt != "" {
		source.BindPassword, err = secret.DecryptSecret(setting.SecretKey, source.BindPasswordEncrypt)
		source.BindPasswordEncrypt = ""
	}
	return err
}

// ToDB exports a LDAPConfig to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	var err error
	source.BindPasswordEncrypt, err = secret.EncryptSecret(setting.SecretKey, source.BindPassword)
	if err != nil {
		return nil, err
	}
	source.BindPassword = ""
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(source)
}

// SecurityProtocolName returns the name of configured security
// protocol.
func (source *Source) SecurityProtocolName() string {
	return ldap.SecurityProtocolNames[source.SecurityProtocol]
}

// IsSkipVerify returns if SkipVerify is set
func (source *Source) IsSkipVerify() bool {
	return source.SkipVerify
}

// HasTLS returns if HasTLS
func (source *Source) HasTLS() bool {
	return source.SecurityProtocol > ldap.SecurityProtocolUnencrypted
}

// UseTLS returns if UseTLS
func (source *Source) UseTLS() bool {
	return source.SecurityProtocol != ldap.SecurityProtocolUnencrypted
}

// ProvidesSSHKeys returns if this source provides SSH Keys
func (source *Source) ProvidesSSHKeys() bool {
	return len(strings.TrimSpace(source.AttributeSSHPublicKey)) > 0
}

func init() {
	models.RegisterLoginTypeConfig(models.LoginLDAP, &Source{})
	models.RegisterLoginTypeConfig(models.LoginDLDAP, &Source{})
}
