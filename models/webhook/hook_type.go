// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"errors"
	"fmt"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// HookTypeDef is a customizable webhook type definition (built-in or admin-defined).
type HookTypeDef struct {
	ID                     int64              `xorm:"pk autoincr"`
	Name                   string             `xorm:"UNIQUE VARCHAR(64) NOT NULL"`
	DisplayName            string             `xorm:"VARCHAR(255) NOT NULL"`
	DocsURL                string             `xorm:"TEXT"`
	IconMIME               string             `xorm:"VARCHAR(64)"`
	IconData               []byte             `xorm:"LONGBLOB"`
	IconAsset              string             `xorm:"VARCHAR(255)"`
	FormSchema             string             `xorm:"LONGTEXT"`
	PayloadJsonnet         string             `xorm:"LONGTEXT"`
	RequestJsonnet         string             `xorm:"LONGTEXT"`
	RequiresPayloadURL     bool               `xorm:"NOT NULL DEFAULT true"`
	UseAuthorizationHeader string             `xorm:"VARCHAR(16)"` // "", "optional", "required"
	UseRequestSecret       string             `xorm:"VARCHAR(16)"`
	IsBuiltin              bool               `xorm:"NOT NULL DEFAULT false"`
	IsActive               bool               `xorm:"NOT NULL DEFAULT true"`
	CreatedUnix            timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix            timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(HookTypeDef))
}

func (*HookTypeDef) TableName() string { return "hook_type" }

// ErrHookTypeNotExist represents a missing hook type.
type ErrHookTypeNotExist struct {
	Name string
	ID   int64
}

func (err ErrHookTypeNotExist) Error() string {
	return fmt.Sprintf("hook type does not exist [name: %s, id: %d]", err.Name, err.ID)
}

func (err ErrHookTypeNotExist) Unwrap() error { return util.ErrNotExist }

// GetHookTypeByName returns a hook type by name.
func GetHookTypeByName(ctx context.Context, name string) (*HookTypeDef, error) {
	ht := &HookTypeDef{Name: name}
	has, err := db.GetEngine(ctx).Get(ht)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrHookTypeNotExist{Name: name}
	}
	return ht, nil
}

// GetHookTypeByID returns a hook type by id.
func GetHookTypeByID(ctx context.Context, id int64) (*HookTypeDef, error) {
	ht := &HookTypeDef{}
	has, err := db.GetEngine(ctx).ID(id).Get(ht)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrHookTypeNotExist{ID: id}
	}
	return ht, nil
}

// ListHookTypesOptions options for listing hook types.
type ListHookTypesOptions struct {
	db.ListOptions
	IsActive  *bool
	IsBuiltin *bool
}

func (opts ListHookTypesOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.IsActive != nil {
		cond = cond.And(builder.Eq{"is_active": *opts.IsActive})
	}
	if opts.IsBuiltin != nil {
		cond = cond.And(builder.Eq{"is_builtin": *opts.IsBuiltin})
	}
	return cond
}

func (opts ListHookTypesOptions) ToOrders() string {
	return "is_builtin DESC, name ASC"
}

// CreateHookType inserts a new hook type definition.
func CreateHookType(ctx context.Context, ht *HookTypeDef) error {
	_, err := db.GetEngine(ctx).Insert(ht)
	return err
}

// UpdateHookType updates a hook type definition.
func UpdateHookType(ctx context.Context, ht *HookTypeDef) error {
	_, err := db.GetEngine(ctx).ID(ht.ID).AllCols().Update(ht)
	return err
}

// DeleteHookType deletes a hook type by id.
func DeleteHookType(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&HookTypeDef{})
	return err
}

// InsertHookTypeIfNotExists inserts a hook type when no row with the same name exists.
func InsertHookTypeIfNotExists(ctx context.Context, ht *HookTypeDef) error {
	_, err := GetHookTypeByName(ctx, ht.Name)
	if err == nil {
		return nil
	}
	if !errors.Is(err, util.ErrNotExist) {
		return err
	}
	return CreateHookType(ctx, ht)
}

// UpsertHookTypeByName inserts or overwrites a hook type keyed by name (reset-to-shipped).
func UpsertHookTypeByName(ctx context.Context, ht *HookTypeDef) error {
	existing, err := GetHookTypeByName(ctx, ht.Name)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return CreateHookType(ctx, ht)
		}
		return err
	}
	ht.ID = existing.ID
	ht.CreatedUnix = existing.CreatedUnix
	return UpdateHookType(ctx, ht)
}

// DeactivateWebhooksOfType sets IsActive=false for all webhooks of the given type.
func DeactivateWebhooksOfType(ctx context.Context, hookType string) error {
	_, err := db.GetEngine(ctx).Where("type = ?", hookType).Cols("is_active").Update(&Webhook{IsActive: false})
	return err
}

// ListActiveHookTypes returns active hook type definitions.
func ListActiveHookTypes(ctx context.Context) ([]*HookTypeDef, error) {
	active := true
	return db.Find[HookTypeDef](ctx, ListHookTypesOptions{
		ListOptions: db.ListOptionsAll,
		IsActive:    &active,
	})
}

// ListAllHookTypes returns all hook type definitions.
func ListAllHookTypes(ctx context.Context) ([]*HookTypeDef, error) {
	return db.Find[HookTypeDef](ctx, ListHookTypesOptions{ListOptions: db.ListOptionsAll})
}
