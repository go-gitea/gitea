// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package login

import (
	"fmt"
	"reflect"
	"strconv"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

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

// String returns the string name of the LoginType
func (typ LoginType) String() string {
	return LoginNames[typ]
}

// Int returns the int value of the LoginType
func (typ LoginType) Int() int {
	return int(typ)
}

// LoginNames contains the name of LoginType values.
var LoginNames = map[LoginType]string{
	LoginLDAP:   "LDAP (via BindDN)",
	LoginDLDAP:  "LDAP (simple auth)", // Via direct bind
	LoginSMTP:   "SMTP",
	LoginPAM:    "PAM",
	LoginOAuth2: "OAuth2",
	LoginSSPI:   "SPNEGO with SSPI",
}

// LoginConfig represents login config as far as the db is concerned
type LoginConfig interface {
	convert.Conversion
}

// SkipVerifiable configurations provide a IsSkipVerify to check if SkipVerify is set
type SkipVerifiable interface {
	IsSkipVerify() bool
}

// HasTLSer configurations provide a HasTLS to check if TLS can be enabled
type HasTLSer interface {
	HasTLS() bool
}

// UseTLSer configurations provide a HasTLS to check if TLS is enabled
type UseTLSer interface {
	UseTLS() bool
}

// SSHKeyProvider configurations provide ProvidesSSHKeys to check if they provide SSHKeys
type SSHKeyProvider interface {
	ProvidesSSHKeys() bool
}

// RegisterableSource configurations provide RegisterSource which needs to be run on creation
type RegisterableSource interface {
	RegisterSource() error
	UnregisterSource() error
}

// LoginSourceSettable configurations can have their loginSource set on them
type LoginSourceSettable interface {
	SetLoginSource(*LoginSource)
}

// RegisterLoginTypeConfig register a config for a provided type
func RegisterLoginTypeConfig(typ LoginType, exemplar LoginConfig) {
	if reflect.TypeOf(exemplar).Kind() == reflect.Ptr {
		// Pointer:
		registeredLoginConfigs[typ] = func() LoginConfig {
			return reflect.New(reflect.ValueOf(exemplar).Elem().Type()).Interface().(LoginConfig)
		}
		return
	}

	// Not a Pointer
	registeredLoginConfigs[typ] = func() LoginConfig {
		return reflect.New(reflect.TypeOf(exemplar)).Elem().Interface().(LoginConfig)
	}
}

var registeredLoginConfigs = map[LoginType]func() LoginConfig{}

// LoginSource represents an external way for authorizing users.
type LoginSource struct {
	ID            int64 `xorm:"pk autoincr"`
	Type          LoginType
	Name          string             `xorm:"UNIQUE"`
	IsActive      bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	IsSyncEnabled bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	Cfg           convert.Conversion `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(LoginSource))
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
		typ := LoginType(Cell2Int64(val))
		constructor, ok := registeredLoginConfigs[typ]
		if !ok {
			return
		}
		source.Cfg = constructor()
		if settable, ok := source.Cfg.(LoginSourceSettable); ok {
			settable.SetLoginSource(source)
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
	hasTLSer, ok := source.Cfg.(HasTLSer)
	return ok && hasTLSer.HasTLS()
}

// UseTLS returns true of this source is configured to use TLS.
func (source *LoginSource) UseTLS() bool {
	useTLSer, ok := source.Cfg.(UseTLSer)
	return ok && useTLSer.UseTLS()
}

// SkipVerify returns true if this source is configured to skip SSL
// verification.
func (source *LoginSource) SkipVerify() bool {
	skipVerifiable, ok := source.Cfg.(SkipVerifiable)
	return ok && skipVerifiable.IsSkipVerify()
}

// CreateLoginSource inserts a LoginSource in the DB if not already
// existing with the given name.
func CreateLoginSource(source *LoginSource) error {
	has, err := db.GetEngine(db.DefaultContext).Where("name=?", source.Name).Exist(new(LoginSource))
	if err != nil {
		return err
	} else if has {
		return ErrLoginSourceAlreadyExist{source.Name}
	}
	// Synchronization is only available with LDAP for now
	if !source.IsLDAP() {
		source.IsSyncEnabled = false
	}

	_, err = db.GetEngine(db.DefaultContext).Insert(source)
	if err != nil {
		return err
	}

	if !source.IsActive {
		return nil
	}

	if settable, ok := source.Cfg.(LoginSourceSettable); ok {
		settable.SetLoginSource(source)
	}

	registerableSource, ok := source.Cfg.(RegisterableSource)
	if !ok {
		return nil
	}

	err = registerableSource.RegisterSource()
	if err != nil {
		// remove the LoginSource in case of errors while registering configuration
		if _, err := db.GetEngine(db.DefaultContext).Delete(source); err != nil {
			log.Error("CreateLoginSource: Error while wrapOpenIDConnectInitializeError: %v", err)
		}
	}
	return err
}

// LoginSources returns a slice of all login sources found in DB.
func LoginSources() ([]*LoginSource, error) {
	auths := make([]*LoginSource, 0, 6)
	return auths, db.GetEngine(db.DefaultContext).Find(&auths)
}

// LoginSourcesByType returns all sources of the specified type
func LoginSourcesByType(loginType LoginType) ([]*LoginSource, error) {
	sources := make([]*LoginSource, 0, 1)
	if err := db.GetEngine(db.DefaultContext).Where("type = ?", loginType).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// AllActiveLoginSources returns all active sources
func AllActiveLoginSources() ([]*LoginSource, error) {
	sources := make([]*LoginSource, 0, 5)
	if err := db.GetEngine(db.DefaultContext).Where("is_active = ?", true).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// ActiveLoginSources returns all active sources of the specified type
func ActiveLoginSources(loginType LoginType) ([]*LoginSource, error) {
	sources := make([]*LoginSource, 0, 1)
	if err := db.GetEngine(db.DefaultContext).Where("is_active = ? and type = ?", true, loginType).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// IsSSPIEnabled returns true if there is at least one activated login
// source of type LoginSSPI
func IsSSPIEnabled() bool {
	if !db.HasEngine {
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
	if id == 0 {
		source.Cfg = registeredLoginConfigs[LoginNoType]()
		// Set this source to active
		// FIXME: allow disabling of db based password authentication in future
		source.IsActive = true
		return source, nil
	}

	has, err := db.GetEngine(db.DefaultContext).ID(id).Get(source)
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

	_, err := db.GetEngine(db.DefaultContext).ID(source.ID).AllCols().Update(source)
	if err != nil {
		return err
	}

	if !source.IsActive {
		return nil
	}

	if settable, ok := source.Cfg.(LoginSourceSettable); ok {
		settable.SetLoginSource(source)
	}

	registerableSource, ok := source.Cfg.(RegisterableSource)
	if !ok {
		return nil
	}

	err = registerableSource.RegisterSource()
	if err != nil {
		// restore original values since we cannot update the provider it self
		if _, err := db.GetEngine(db.DefaultContext).ID(source.ID).AllCols().Update(originalLoginSource); err != nil {
			log.Error("UpdateSource: Error while wrapOpenIDConnectInitializeError: %v", err)
		}
	}
	return err
}

// CountLoginSources returns number of login sources.
func CountLoginSources() int64 {
	count, _ := db.GetEngine(db.DefaultContext).Count(new(LoginSource))
	return count
}

// .____                 .__           _________
// |    |    ____   ____ |__| ____    /   _____/ ____  __ _________   ____  ____
// |    |   /  _ \ / ___\|  |/    \   \_____  \ /  _ \|  |  \_  __ \_/ ___\/ __ \
// |    |__(  <_> ) /_/  >  |   |  \  /        (  <_> )  |  /|  | \/\  \__\  ___/
// |_______ \____/\___  /|__|___|  / /_______  /\____/|____/ |__|    \___  >___  >
//         \/    /_____/         \/          \/                          \/    \/

// ErrLoginSourceNotExist represents a "LoginSourceNotExist" kind of error.
type ErrLoginSourceNotExist struct {
	ID int64
}

// IsErrLoginSourceNotExist checks if an error is a ErrLoginSourceNotExist.
func IsErrLoginSourceNotExist(err error) bool {
	_, ok := err.(ErrLoginSourceNotExist)
	return ok
}

func (err ErrLoginSourceNotExist) Error() string {
	return fmt.Sprintf("login source does not exist [id: %d]", err.ID)
}

// ErrLoginSourceAlreadyExist represents a "LoginSourceAlreadyExist" kind of error.
type ErrLoginSourceAlreadyExist struct {
	Name string
}

// IsErrLoginSourceAlreadyExist checks if an error is a ErrLoginSourceAlreadyExist.
func IsErrLoginSourceAlreadyExist(err error) bool {
	_, ok := err.(ErrLoginSourceAlreadyExist)
	return ok
}

func (err ErrLoginSourceAlreadyExist) Error() string {
	return fmt.Sprintf("login source already exists [name: %s]", err.Name)
}

// ErrLoginSourceInUse represents a "LoginSourceInUse" kind of error.
type ErrLoginSourceInUse struct {
	ID int64
}

// IsErrLoginSourceInUse checks if an error is a ErrLoginSourceInUse.
func IsErrLoginSourceInUse(err error) bool {
	_, ok := err.(ErrLoginSourceInUse)
	return ok
}

func (err ErrLoginSourceInUse) Error() string {
	return fmt.Sprintf("login source is still used by some users [id: %d]", err.ID)
}
