// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"reflect"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
	"xorm.io/xorm/convert"
)

// Type represents an login type.
type Type int

// Note: new type must append to the end of list to maintain compatibility.
const (
	NoType Type = iota
	Plain       // 1
	LDAP        // 2
	SMTP        // 3
	PAM         // 4
	DLDAP       // 5
	OAuth2      // 6
	SSPI        // 7
)

// String returns the string name of the LoginType
func (typ Type) String() string {
	return Names[typ]
}

// Int returns the int value of the LoginType
func (typ Type) Int() int {
	return int(typ)
}

// Names contains the name of LoginType values.
var Names = map[Type]string{
	LDAP:   "LDAP (via BindDN)",
	DLDAP:  "LDAP (simple auth)", // Via direct bind
	SMTP:   "SMTP",
	PAM:    "PAM",
	OAuth2: "OAuth2",
	SSPI:   "SPNEGO with SSPI",
}

// Config represents login config as far as the db is concerned
type Config interface {
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

var registeredConfigs = map[Type]func() Config{}

// RegisterTypeConfig register a config for a provided type
func RegisterTypeConfig(typ Type, exemplar Config) {
	if reflect.TypeOf(exemplar).Kind() == reflect.Ptr {
		// Pointer:
		registeredConfigs[typ] = func() Config {
			return reflect.New(reflect.ValueOf(exemplar).Elem().Type()).Interface().(Config)
		}
		return
	}

	// Not a Pointer
	registeredConfigs[typ] = func() Config {
		return reflect.New(reflect.TypeOf(exemplar)).Elem().Interface().(Config)
	}
}

// SourceSettable configurations can have their authSource set on them
type SourceSettable interface {
	SetAuthSource(*Source)
}

// Source represents an external way for authorizing users.
type Source struct {
	ID            int64 `xorm:"pk autoincr"`
	Type          Type
	Name          string             `xorm:"UNIQUE"`
	IsActive      bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	IsSyncEnabled bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	Cfg           convert.Conversion `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName xorm will read the table name from this method
func (Source) TableName() string {
	return "login_source"
}

func init() {
	db.RegisterModel(new(Source))
}

// BeforeSet is invoked from XORM before setting the value of a field of this object.
func (source *Source) BeforeSet(colName string, val xorm.Cell) {
	if colName == "type" {
		typ := Type(db.Cell2Int64(val))
		constructor, ok := registeredConfigs[typ]
		if !ok {
			return
		}
		source.Cfg = constructor()
		if settable, ok := source.Cfg.(SourceSettable); ok {
			settable.SetAuthSource(source)
		}
	}
}

// TypeName return name of this login source type.
func (source *Source) TypeName() string {
	return Names[source.Type]
}

// IsLDAP returns true of this source is of the LDAP type.
func (source *Source) IsLDAP() bool {
	return source.Type == LDAP
}

// IsDLDAP returns true of this source is of the DLDAP type.
func (source *Source) IsDLDAP() bool {
	return source.Type == DLDAP
}

// IsSMTP returns true of this source is of the SMTP type.
func (source *Source) IsSMTP() bool {
	return source.Type == SMTP
}

// IsPAM returns true of this source is of the PAM type.
func (source *Source) IsPAM() bool {
	return source.Type == PAM
}

// IsOAuth2 returns true of this source is of the OAuth2 type.
func (source *Source) IsOAuth2() bool {
	return source.Type == OAuth2
}

// IsSSPI returns true of this source is of the SSPI type.
func (source *Source) IsSSPI() bool {
	return source.Type == SSPI
}

// HasTLS returns true of this source supports TLS.
func (source *Source) HasTLS() bool {
	hasTLSer, ok := source.Cfg.(HasTLSer)
	return ok && hasTLSer.HasTLS()
}

// UseTLS returns true of this source is configured to use TLS.
func (source *Source) UseTLS() bool {
	useTLSer, ok := source.Cfg.(UseTLSer)
	return ok && useTLSer.UseTLS()
}

// SkipVerify returns true if this source is configured to skip SSL
// verification.
func (source *Source) SkipVerify() bool {
	skipVerifiable, ok := source.Cfg.(SkipVerifiable)
	return ok && skipVerifiable.IsSkipVerify()
}

// CreateSource inserts a AuthSource in the DB if not already
// existing with the given name.
func CreateSource(ctx context.Context, source *Source) error {
	has, err := db.GetEngine(ctx).Where("name=?", source.Name).Exist(new(Source))
	if err != nil {
		return err
	} else if has {
		return ErrSourceAlreadyExist{source.Name}
	}
	// Synchronization is only available with LDAP for now
	if !source.IsLDAP() {
		source.IsSyncEnabled = false
	}

	_, err = db.GetEngine(ctx).Insert(source)
	if err != nil {
		return err
	}

	if !source.IsActive {
		return nil
	}

	if settable, ok := source.Cfg.(SourceSettable); ok {
		settable.SetAuthSource(source)
	}

	registerableSource, ok := source.Cfg.(RegisterableSource)
	if !ok {
		return nil
	}

	err = registerableSource.RegisterSource()
	if err != nil {
		// remove the AuthSource in case of errors while registering configuration
		if _, err := db.GetEngine(ctx).ID(source.ID).Delete(new(Source)); err != nil {
			log.Error("CreateSource: Error while wrapOpenIDConnectInitializeError: %v", err)
		}
	}
	return err
}

type FindSourcesOptions struct {
	IsActive  util.OptionalBool
	LoginType Type
}

func (opts FindSourcesOptions) ToConds() builder.Cond {
	conds := builder.NewCond()
	if !opts.IsActive.IsNone() {
		conds = conds.And(builder.Eq{"is_active": opts.IsActive.IsTrue()})
	}
	if opts.LoginType != NoType {
		conds = conds.And(builder.Eq{"`type`": opts.LoginType})
	}
	return conds
}

// FindSources returns a slice of login sources found in DB according to given conditions.
func FindSources(ctx context.Context, opts FindSourcesOptions) ([]*Source, error) {
	auths := make([]*Source, 0, 6)
	return auths, db.GetEngine(ctx).Where(opts.ToConds()).Find(&auths)
}

// IsSSPIEnabled returns true if there is at least one activated login
// source of type LoginSSPI
func IsSSPIEnabled(ctx context.Context) bool {
	if !db.HasEngine {
		return false
	}
	sources, err := FindSources(ctx, FindSourcesOptions{
		IsActive:  util.OptionalBoolTrue,
		LoginType: SSPI,
	})
	if err != nil {
		log.Error("ActiveSources: %v", err)
		return false
	}
	return len(sources) > 0
}

// GetSourceByID returns login source by given ID.
func GetSourceByID(ctx context.Context, id int64) (*Source, error) {
	source := new(Source)
	if id == 0 {
		source.Cfg = registeredConfigs[NoType]()
		// Set this source to active
		// FIXME: allow disabling of db based password authentication in future
		source.IsActive = true
		return source, nil
	}

	has, err := db.GetEngine(ctx).ID(id).Get(source)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrSourceNotExist{id}
	}
	return source, nil
}

// UpdateSource updates a Source record in DB.
func UpdateSource(ctx context.Context, source *Source) error {
	var originalSource *Source
	if source.IsOAuth2() {
		// keep track of the original values so we can restore in case of errors while registering OAuth2 providers
		var err error
		if originalSource, err = GetSourceByID(ctx, source.ID); err != nil {
			return err
		}
	}

	has, err := db.GetEngine(ctx).Where("name=? AND id!=?", source.Name, source.ID).Exist(new(Source))
	if err != nil {
		return err
	} else if has {
		return ErrSourceAlreadyExist{source.Name}
	}

	_, err = db.GetEngine(ctx).ID(source.ID).AllCols().Update(source)
	if err != nil {
		return err
	}

	if !source.IsActive {
		return nil
	}

	if settable, ok := source.Cfg.(SourceSettable); ok {
		settable.SetAuthSource(source)
	}

	registerableSource, ok := source.Cfg.(RegisterableSource)
	if !ok {
		return nil
	}

	err = registerableSource.RegisterSource()
	if err != nil {
		// restore original values since we cannot update the provider it self
		if _, err := db.GetEngine(ctx).ID(source.ID).AllCols().Update(originalSource); err != nil {
			log.Error("UpdateSource: Error while wrapOpenIDConnectInitializeError: %v", err)
		}
	}
	return err
}

// CountSources returns number of login sources.
func CountSources(ctx context.Context, opts FindSourcesOptions) int64 {
	count, _ := db.GetEngine(ctx).Where(opts.ToConds()).Count(new(Source))
	return count
}

// ErrSourceNotExist represents a "SourceNotExist" kind of error.
type ErrSourceNotExist struct {
	ID int64
}

// IsErrSourceNotExist checks if an error is a ErrSourceNotExist.
func IsErrSourceNotExist(err error) bool {
	_, ok := err.(ErrSourceNotExist)
	return ok
}

func (err ErrSourceNotExist) Error() string {
	return fmt.Sprintf("login source does not exist [id: %d]", err.ID)
}

// Unwrap unwraps this as a ErrNotExist err
func (err ErrSourceNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrSourceAlreadyExist represents a "SourceAlreadyExist" kind of error.
type ErrSourceAlreadyExist struct {
	Name string
}

// IsErrSourceAlreadyExist checks if an error is a ErrSourceAlreadyExist.
func IsErrSourceAlreadyExist(err error) bool {
	_, ok := err.(ErrSourceAlreadyExist)
	return ok
}

func (err ErrSourceAlreadyExist) Error() string {
	return fmt.Sprintf("login source already exists [name: %s]", err.Name)
}

// Unwrap unwraps this as a ErrExist err
func (err ErrSourceAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrSourceInUse represents a "SourceInUse" kind of error.
type ErrSourceInUse struct {
	ID int64
}

// IsErrSourceInUse checks if an error is a ErrSourceInUse.
func IsErrSourceInUse(err error) bool {
	_, ok := err.(ErrSourceInUse)
	return ok
}

func (err ErrSourceInUse) Error() string {
	return fmt.Sprintf("login source is still used by some users [id: %d]", err.ID)
}
