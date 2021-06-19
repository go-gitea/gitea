// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/auth/ldap"
	"code.gitea.io/gitea/modules/auth/oauth2"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	jsoniter "github.com/json-iterator/go"

	"xorm.io/xorm"
	"xorm.io/xorm/convert"
)

// LoginType represents an login type.
type LoginType int

// Note: new type must append to the end of list to maintain compatibility.
const (
	LoginNoType LoginType = iota
	LoginPlain            // 1
	LoginLDAP             // 2
	LoginSMTP             // 3
	LoginPAM              // 4
	LoginDLDAP            // 5
	LoginOAuth2           // 6
	LoginSSPI             // 7
)

// LoginNames contains the name of LoginType values.
var LoginNames = map[LoginType]string{
	LoginLDAP:   "LDAP (via BindDN)",
	LoginDLDAP:  "LDAP (simple auth)", // Via direct bind
	LoginSMTP:   "SMTP",
	LoginPAM:    "PAM",
	LoginOAuth2: "OAuth2",
	LoginSSPI:   "SPNEGO with SSPI",
}

// SecurityProtocolNames contains the name of SecurityProtocol values.
var SecurityProtocolNames = map[ldap.SecurityProtocol]string{
	ldap.SecurityProtocolUnencrypted: "Unencrypted",
	ldap.SecurityProtocolLDAPS:       "LDAPS",
	ldap.SecurityProtocolStartTLS:    "StartTLS",
}

// Ensure structs implemented interface.
var (
	_ convert.Conversion = &LDAPConfig{}
	_ convert.Conversion = &SMTPConfig{}
	_ convert.Conversion = &PAMConfig{}
	_ convert.Conversion = &OAuth2Config{}
	_ convert.Conversion = &SSPIConfig{}
)

// LDAPConfig holds configuration for LDAP login source.
type LDAPConfig struct {
	*ldap.Source
}

// FromDB fills up a LDAPConfig from serialized format.
func (cfg *LDAPConfig) FromDB(bs []byte) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(bs, &cfg)
	if err != nil {
		return err
	}
	if cfg.BindPasswordEncrypt != "" {
		cfg.BindPassword, err = secret.DecryptSecret(setting.SecretKey, cfg.BindPasswordEncrypt)
		cfg.BindPasswordEncrypt = ""
	}
	return err
}

// ToDB exports a LDAPConfig to a serialized format.
func (cfg *LDAPConfig) ToDB() ([]byte, error) {
	var err error
	cfg.BindPasswordEncrypt, err = secret.EncryptSecret(setting.SecretKey, cfg.BindPassword)
	if err != nil {
		return nil, err
	}
	cfg.BindPassword = ""
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(cfg)
}

// SecurityProtocolName returns the name of configured security
// protocol.
func (cfg *LDAPConfig) SecurityProtocolName() string {
	return SecurityProtocolNames[cfg.SecurityProtocol]
}

// SMTPConfig holds configuration for the SMTP login source.
type SMTPConfig struct {
	Auth           string
	Host           string
	Port           int
	AllowedDomains string `xorm:"TEXT"`
	TLS            bool
	SkipVerify     bool
}

// FromDB fills up an SMTPConfig from serialized format.
func (cfg *SMTPConfig) FromDB(bs []byte) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Unmarshal(bs, cfg)
}

// ToDB exports an SMTPConfig to a serialized format.
func (cfg *SMTPConfig) ToDB() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(cfg)
}

// PAMConfig holds configuration for the PAM login source.
type PAMConfig struct {
	ServiceName string // pam service (e.g. system-auth)
	EmailDomain string
}

// FromDB fills up a PAMConfig from serialized format.
func (cfg *PAMConfig) FromDB(bs []byte) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Unmarshal(bs, &cfg)
}

// ToDB exports a PAMConfig to a serialized format.
func (cfg *PAMConfig) ToDB() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(cfg)
}

// OAuth2Config holds configuration for the OAuth2 login source.
type OAuth2Config struct {
	Provider                      string
	ClientID                      string
	ClientSecret                  string
	OpenIDConnectAutoDiscoveryURL string
	CustomURLMapping              *oauth2.CustomURLMapping
	IconURL                       string
}

// FromDB fills up an OAuth2Config from serialized format.
func (cfg *OAuth2Config) FromDB(bs []byte) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Unmarshal(bs, cfg)
}

// ToDB exports an SMTPConfig to a serialized format.
func (cfg *OAuth2Config) ToDB() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(cfg)
}

// SSPIConfig holds configuration for SSPI single sign-on.
type SSPIConfig struct {
	AutoCreateUsers      bool
	AutoActivateUsers    bool
	StripDomainNames     bool
	SeparatorReplacement string
	DefaultLanguage      string
}

// FromDB fills up an SSPIConfig from serialized format.
func (cfg *SSPIConfig) FromDB(bs []byte) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Unmarshal(bs, cfg)
}

// ToDB exports an SSPIConfig to a serialized format.
func (cfg *SSPIConfig) ToDB() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(cfg)
}

// LoginSource represents an external way for authorizing users.
type LoginSource struct {
	ID            int64 `xorm:"pk autoincr"`
	Type          LoginType
	Name          string             `xorm:"UNIQUE"`
	IsActived     bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	IsSyncEnabled bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	Cfg           convert.Conversion `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// Cell2Int64 converts a xorm.Cell type to int64,
// and handles possible irregular cases.
func Cell2Int64(val xorm.Cell) int64 {
	switch (*val).(type) {
	case []uint8:
		log.Trace("Cell2Int64 ([]uint8): %v", *val)

		v, _ := strconv.ParseInt(string((*val).([]uint8)), 10, 64)
		return v
	}
	return (*val).(int64)
}

// BeforeSet is invoked from XORM before setting the value of a field of this object.
func (source *LoginSource) BeforeSet(colName string, val xorm.Cell) {
	if colName == "type" {
		switch LoginType(Cell2Int64(val)) {
		case LoginLDAP, LoginDLDAP:
			source.Cfg = new(LDAPConfig)
		case LoginSMTP:
			source.Cfg = new(SMTPConfig)
		case LoginPAM:
			source.Cfg = new(PAMConfig)
		case LoginOAuth2:
			source.Cfg = new(OAuth2Config)
		case LoginSSPI:
			source.Cfg = new(SSPIConfig)
		default:
			panic(fmt.Sprintf("unrecognized login source type: %v", *val))
		}
	}
}

// TypeName return name of this login source type.
func (source *LoginSource) TypeName() string {
	return LoginNames[source.Type]
}

// IsLDAP returns true of this source is of the LDAP type.
func (source *LoginSource) IsLDAP() bool {
	return source.Type == LoginLDAP
}

// IsDLDAP returns true of this source is of the DLDAP type.
func (source *LoginSource) IsDLDAP() bool {
	return source.Type == LoginDLDAP
}

// IsSMTP returns true of this source is of the SMTP type.
func (source *LoginSource) IsSMTP() bool {
	return source.Type == LoginSMTP
}

// IsPAM returns true of this source is of the PAM type.
func (source *LoginSource) IsPAM() bool {
	return source.Type == LoginPAM
}

// IsOAuth2 returns true of this source is of the OAuth2 type.
func (source *LoginSource) IsOAuth2() bool {
	return source.Type == LoginOAuth2
}

// IsSSPI returns true of this source is of the SSPI type.
func (source *LoginSource) IsSSPI() bool {
	return source.Type == LoginSSPI
}

// HasTLS returns true of this source supports TLS.
func (source *LoginSource) HasTLS() bool {
	return ((source.IsLDAP() || source.IsDLDAP()) &&
		source.LDAP().SecurityProtocol > ldap.SecurityProtocolUnencrypted) ||
		source.IsSMTP()
}

// UseTLS returns true of this source is configured to use TLS.
func (source *LoginSource) UseTLS() bool {
	switch source.Type {
	case LoginLDAP, LoginDLDAP:
		return source.LDAP().SecurityProtocol != ldap.SecurityProtocolUnencrypted
	case LoginSMTP:
		return source.SMTP().TLS
	}

	return false
}

// SkipVerify returns true if this source is configured to skip SSL
// verification.
func (source *LoginSource) SkipVerify() bool {
	switch source.Type {
	case LoginLDAP, LoginDLDAP:
		return source.LDAP().SkipVerify
	case LoginSMTP:
		return source.SMTP().SkipVerify
	}

	return false
}

// LDAP returns LDAPConfig for this source, if of LDAP type.
func (source *LoginSource) LDAP() *LDAPConfig {
	return source.Cfg.(*LDAPConfig)
}

// SMTP returns SMTPConfig for this source, if of SMTP type.
func (source *LoginSource) SMTP() *SMTPConfig {
	return source.Cfg.(*SMTPConfig)
}

// PAM returns PAMConfig for this source, if of PAM type.
func (source *LoginSource) PAM() *PAMConfig {
	return source.Cfg.(*PAMConfig)
}

// OAuth2 returns OAuth2Config for this source, if of OAuth2 type.
func (source *LoginSource) OAuth2() *OAuth2Config {
	return source.Cfg.(*OAuth2Config)
}

// SSPI returns SSPIConfig for this source, if of SSPI type.
func (source *LoginSource) SSPI() *SSPIConfig {
	return source.Cfg.(*SSPIConfig)
}

// CreateLoginSource inserts a LoginSource in the DB if not already
// existing with the given name.
func CreateLoginSource(source *LoginSource) error {
	has, err := x.Where("name=?", source.Name).Exist(new(LoginSource))
	if err != nil {
		return err
	} else if has {
		return ErrLoginSourceAlreadyExist{source.Name}
	}
	// Synchronization is only aviable with LDAP for now
	if !source.IsLDAP() {
		source.IsSyncEnabled = false
	}

	_, err = x.Insert(source)
	if err == nil && source.IsOAuth2() && source.IsActived {
		oAuth2Config := source.OAuth2()
		err = oauth2.RegisterProvider(source.Name, oAuth2Config.Provider, oAuth2Config.ClientID, oAuth2Config.ClientSecret, oAuth2Config.OpenIDConnectAutoDiscoveryURL, oAuth2Config.CustomURLMapping)
		err = wrapOpenIDConnectInitializeError(err, source.Name, oAuth2Config)
		if err != nil {
			// remove the LoginSource in case of errors while registering OAuth2 providers
			if _, err := x.Delete(source); err != nil {
				log.Error("CreateLoginSource: Error while wrapOpenIDConnectInitializeError: %v", err)
			}
			return err
		}
	}
	return err
}

// LoginSources returns a slice of all login sources found in DB.
func LoginSources() ([]*LoginSource, error) {
	auths := make([]*LoginSource, 0, 6)
	return auths, x.Find(&auths)
}

// LoginSourcesByType returns all sources of the specified type
func LoginSourcesByType(loginType LoginType) ([]*LoginSource, error) {
	sources := make([]*LoginSource, 0, 1)
	if err := x.Where("type = ?", loginType).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// ActiveLoginSources returns all active sources of the specified type
func ActiveLoginSources(loginType LoginType) ([]*LoginSource, error) {
	sources := make([]*LoginSource, 0, 1)
	if loginType < 0 {
		if err := x.Where("is_actived = ?", true).Find(&sources); err != nil {
			return nil, err
		}
		return sources, nil
	}

	if err := x.Where("is_actived = ? and type = ?", true, loginType).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// IsSSPIEnabled returns true if there is at least one activated login
// source of type LoginSSPI
func IsSSPIEnabled() bool {
	if !HasEngine {
		return false
	}
	sources, err := ActiveLoginSources(LoginSSPI)
	if err != nil {
		log.Error("ActiveLoginSources: %v", err)
		return false
	}
	return len(sources) > 0
}

// GetLoginSourceByID returns login source by given ID.
func GetLoginSourceByID(id int64) (*LoginSource, error) {
	source := new(LoginSource)
	has, err := x.ID(id).Get(source)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLoginSourceNotExist{id}
	}
	return source, nil
}

// UpdateSource updates a LoginSource record in DB.
func UpdateSource(source *LoginSource) error {
	var originalLoginSource *LoginSource
	if source.IsOAuth2() {
		// keep track of the original values so we can restore in case of errors while registering OAuth2 providers
		var err error
		if originalLoginSource, err = GetLoginSourceByID(source.ID); err != nil {
			return err
		}
	}

	_, err := x.ID(source.ID).AllCols().Update(source)
	if err == nil && source.IsOAuth2() && source.IsActived {
		oAuth2Config := source.OAuth2()
		err = oauth2.RegisterProvider(source.Name, oAuth2Config.Provider, oAuth2Config.ClientID, oAuth2Config.ClientSecret, oAuth2Config.OpenIDConnectAutoDiscoveryURL, oAuth2Config.CustomURLMapping)
		err = wrapOpenIDConnectInitializeError(err, source.Name, oAuth2Config)
		if err != nil {
			// restore original values since we cannot update the provider it self
			if _, err := x.ID(source.ID).AllCols().Update(originalLoginSource); err != nil {
				log.Error("UpdateSource: Error while wrapOpenIDConnectInitializeError: %v", err)
			}
			return err
		}
	}
	return err
}

// DeleteSource deletes a LoginSource record in DB.
func DeleteSource(source *LoginSource) error {
	count, err := x.Count(&User{LoginSource: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return ErrLoginSourceInUse{source.ID}
	}

	count, err = x.Count(&ExternalLoginUser{LoginSourceID: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return ErrLoginSourceInUse{source.ID}
	}

	if source.IsOAuth2() {
		oauth2.RemoveProvider(source.Name)
	}

	_, err = x.ID(source.ID).Delete(new(LoginSource))
	return err
}

// CountLoginSources returns number of login sources.
func CountLoginSources() int64 {
	count, _ := x.Count(new(LoginSource))
	return count
}
