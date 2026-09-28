// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"reflect"

	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

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
	SMTP        // 2
	OAuth2      // 3
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
	SMTP:   "SMTP",
	OAuth2: "OAuth2",
}

// Config represents login config as far as the db is concerned
type Config interface {
	convert.Conversion
	SetAuthSource(*Source)
}

type ConfigBase struct {
	AuthSource *Source
}

func (p *ConfigBase) SetAuthSource(s *Source) {
	p.AuthSource = s
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

// RegisterableSource configurations provide RegisterSource which needs to be run on creation
type RegisterableSource interface {
	RegisterSource() error
	UnregisterSource() error
}

var registeredConfigs = map[Type]func() Config{}

// RegisterTypeConfig register a config for a provided type, the exemplar argument only serves type inference
func RegisterTypeConfig[T interface {
	*E
	Config
}, E any](typ Type, _ T) {
	registeredConfigs[typ] = func() Config { return T(new(E)) }
}

// Source represents an external way for authorizing users.
type Source struct {
	ID              int64 `xorm:"pk autoincr"`
	Type            Type
	Name            string `xorm:"UNIQUE"` // it can be the OIDC's provider name, see services/auth/source/oauth2/source_register.go: RegisterSource
	IsActive        bool   `xorm:"INDEX NOT NULL DEFAULT false"`
	IsSyncEnabled   bool   `xorm:"INDEX NOT NULL DEFAULT false"`
	TwoFactorPolicy string `xorm:"two_factor_policy NOT NULL DEFAULT ''"`
	Cfg             Config `xorm:"TEXT"`

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
		typ, _, err := db.CellToInt(val, NoType)
		if err != nil {
			setting.PanicInDevOrTesting("Unable to convert login source (id=%d) type: %v", source.ID, err)
		}
		constructor, ok := registeredConfigs[typ]
		if !ok {
			return
		}
		source.Cfg = constructor()
		source.Cfg.SetAuthSource(source)
	}
}

// TypeName return name of this login source type.
func (source *Source) TypeName() string {
	if name, ok := Names[source.Type]; ok {
		return name
	}
	return fmt.Sprintf("Unsupported (%d)", source.Type)
}

// IsSMTP returns true of this source is of the SMTP type.
func (source *Source) IsSMTP() bool {
	return source.Type == SMTP
}

// IsOAuth2 returns true of this source is of the OAuth2 type.
func (source *Source) IsOAuth2() bool {
	return source.Type == OAuth2
}

// MustSourceCfg returns the source's config as T. The registry populates Cfg from the
// source type, so a mismatch is a programming error the caller can't recover from.
func MustSourceCfg[T Config](source *Source) T {
	cfg, ok := source.Cfg.(T)
	if !ok {
		panic(fmt.Errorf("auth source %q (id=%d, type=%s) has config %T, expected %s", source.Name, source.ID, source.Type, source.Cfg, reflect.TypeFor[T]()))
	}
	return cfg
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

func (source *Source) TwoFactorShouldSkip() bool {
	return source.TwoFactorPolicy == "skip"
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
	if !source.IsOAuth2() {
		source.IsSyncEnabled = false
	}

	_, err = db.GetEngine(ctx).Insert(source)
	if err != nil {
		return err
	}

	if !source.IsActive {
		return nil
	}

	source.Cfg.SetAuthSource(source)

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
	db.ListOptions
	IsActive      optional.Option[bool]
	IsSyncEnabled optional.Option[bool]
	LoginType     Type
}

func (opts FindSourcesOptions) ToOrders() string {
	return "name"
}

func (opts FindSourcesOptions) ToConds() builder.Cond {
	conds := builder.NewCond()
	if opts.IsActive.Has() {
		conds = conds.And(builder.Eq{"is_active": opts.IsActive.Value()})
	}
	if opts.IsSyncEnabled.Has() {
		conds = conds.And(builder.Eq{"is_sync_enabled": opts.IsSyncEnabled.Value()})
	}
	if opts.LoginType != NoType {
		conds = conds.And(builder.Eq{"`type`": opts.LoginType})
	}
	return conds
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

	source.Cfg.SetAuthSource(source)

	registerableSource, ok := source.Cfg.(RegisterableSource)
	if !ok {
		return nil
	}

	err = registerableSource.RegisterSource()
	if err != nil {
		// restore original values since we cannot update the provider itself
		if _, err := db.GetEngine(ctx).ID(source.ID).AllCols().Update(originalSource); err != nil {
			log.Error("UpdateSource: Error while wrapOpenIDConnectInitializeError: %v", err)
		}
	}
	return err
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
