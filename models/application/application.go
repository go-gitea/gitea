// Copyright 2025 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package application

import (
	"context"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/xorm"
)

type Application user_model.User

// AppFromUser converts user to application
func AppFromUser(user *user_model.User) *Application {
	return (*Application)(user)
}

type JWTPubKey struct {
	RawKey    string `json:"key"`
	RawKeySHA string `json:"key_sha"`
}

type AppExternalData struct {
	ID  int64 `xorm:"pk autoincr"`
	UID int64 `xorm:"INDEX"`

	OwnerID int64            `xorm:"INDEX"`
	Owner   *user_model.User `xorm:"-"`

	SetupURL                   string `xorm:"TEXT"`
	RedirectToSetupURLOnUpdate bool

	HomePageURL string `xorm:"TEXT"`

	Readme         string        `xorm:"TEXT"`
	RenderedReadme template.HTML `xorm:"-"`

	ClientID   string      `xorm:"INDEX"`
	JWTKeyList []JWTPubKey `xorm:"JSON TEXT"`

	Permission AppPermList `xorm:"TEXT"`
}

func init() {
	db.RegisterModel(new(AppExternalData))
	db.RegisterModel(new(AppInstallation))
}

// TableName represents the real table name of Application
func (Application) TableName() string {
	return "user"
}

func (app *Application) LoadExternalData(ctx context.Context) error {
	if app.ExternalData != nil {
		return nil
	}

	extData := &AppExternalData{}

	ok, err := db.GetEngine(ctx).Where("uid = ?", app.ID).Get(extData)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAppNotExist{ID: app.ID, Name: app.Name}
	}

	app.ExternalData = extData

	return nil
}

func (app *Application) AppExternalData() *AppExternalData {
	return app.ExternalData.(*AppExternalData)
}

func (app *Application) AsUser() *user_model.User {
	return (*user_model.User)(app)
}

func ListAppsByOwnerID(ctx context.Context, ownerID int64) ([]*Application, error) {
	type joinedApp struct {
		App          *Application     `xorm:"extends"`
		ExternalData *AppExternalData `xorm:"extends"`
	}

	var results *xorm.Rows
	var err error
	if results, err = db.GetEngine(ctx).
		Table("user").
		Where("owner_id = ?", ownerID).
		Join("INNER", "app_external_data", "uid = user.id").
		Rows(new(joinedApp)); err != nil {
		return nil, err
	}
	defer results.Close()

	apps := make([]*Application, 0)
	for results.Next() {
		joinedApp := new(joinedApp)
		if err := results.Scan(joinedApp); err != nil {
			return nil, err
		}
		joinedApp.App.ExternalData = joinedApp.ExternalData
		apps = append(apps, joinedApp.App)
	}

	return apps, err
}

type CreateApplicationOptions struct {
	Owner                      *user_model.User
	Permission                 AppPermList
	Readme                     string
	SetupURL                   string
	RedirectToSetupURLOnUpdate bool
	HomePageURL                string
	Private                    bool
}

// CreateApplication creates record of a new application.
func CreateApplication(ctx context.Context, app *Application, opts *CreateApplicationOptions) (err error) {
	// if !owner.CanCreateOrganization() {
	// 	return ErrUserNotAllowedCreateOrg{}
	// }

	if opts.Private {
		app.Visibility = structs.VisibleTypePrivate
	} else {
		app.Visibility = structs.VisibleTypePublic
	}

	if err = user_model.IsUsableUsername(app.Name); err != nil {
		return err
	}

	isExist, err := user_model.IsUserExist(ctx, 0, app.Name)
	if err != nil {
		return err
	} else if isExist {
		return user_model.ErrUserAlreadyExist{Name: app.Name}
	}

	app.LowerName = strings.ToLower(app.Name)
	if app.Rands, err = user_model.GetUserSalt(); err != nil {
		return err
	}
	if app.Salt, err = user_model.GetUserSalt(); err != nil {
		return err
	}
	app.UseCustomAvatar = true
	app.MaxRepoCreation = 0
	app.Type = user_model.UserTypeBot
	app.IsActive = true

	return db.WithTx(ctx, func(ctx context.Context) error {
		if err = user_model.DeleteUserRedirect(ctx, app.Name); err != nil {
			return err
		}

		if err = db.Insert(ctx, app); err != nil {
			return fmt.Errorf("insert application: %w", err)
		}
		if err = user_model.GenerateRandomAvatar(ctx, app.AsUser()); err != nil {
			return fmt.Errorf("generate random avatar: %w", err)
		}

		oauth2App, err := auth_model.CreateOAuth2Application(ctx, auth_model.CreateOAuth2ApplicationOptions{
			GiteaAppID: app.ID,
		})
		if err != nil {
			return err
		}

		app.ExternalData = &AppExternalData{
			UID:                        app.ID,
			OwnerID:                    opts.Owner.ID,
			SetupURL:                   opts.SetupURL,
			RedirectToSetupURLOnUpdate: opts.RedirectToSetupURLOnUpdate,
			HomePageURL:                opts.HomePageURL,
			Permission:                 opts.Permission,
			Readme:                     opts.Readme,
			ClientID:                   oauth2App.ClientID,
		}

		if err = db.Insert(ctx, app.ExternalData); err != nil {
			return fmt.Errorf("insert org-user relation: %w", err)
		}

		return nil
	})
}

func (app *Application) HomeLink() string {
	return setting.AppSubURL + "/-/apps/" + url.PathEscape(app.Name)
}

func (app *Application) InstallURL() string {
	return app.HomeLink() + "/installations"
}

func (app *Application) SettingsURL() string {
	return app.HomeLink() + "/settings"
}

type ErrAppNotExist struct {
	ID       int64
	Name     string
	ClientID string
}

func (err ErrAppNotExist) Error() string {
	return fmt.Sprintf("app does not exist [id: %d, name: %s, client_id: %s]", err.ID, err.Name, err.ClientID)
}

func (err ErrAppNotExist) Unwrap() error {
	return util.ErrNotExist
}

// GetAppByName returns application by given name.
func GetAppByName(ctx context.Context, name string) (*Application, error) {
	if len(name) == 0 {
		return nil, ErrAppNotExist{Name: name}
	}
	u := &Application{
		LowerName: strings.ToLower(name),
		Type:      user_model.UserTypeBot,
	}
	has, err := db.GetEngine(ctx).Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrAppNotExist{Name: name}
	}

	return u, u.LoadExternalData(ctx)
}

func (app *Application) CanViewBy(ctx context.Context, doer *user_model.User) (bool, error) {
	if app.Visibility == structs.VisibleTypePublic {
		return true, nil
	}

	if doer == nil {
		return false, nil
	}

	if doer.IsAdmin || doer.ID == app.AppExternalData().OwnerID {
		return true, nil
	}

	if err := app.LoadOwner(ctx); err != nil {
		return false, err
	}

	if !app.AppExternalData().Owner.IsOrganization() {
		return false, nil
	}

	return org_model.IsOrganizationMember(ctx, app.AppExternalData().Owner.ID, doer.ID)
}

func (app *Application) LoadOwner(ctx context.Context) error {
	if app.AppExternalData() == nil {
		return ErrAppNotExist{ID: app.ID, Name: app.Name}
	}

	if app.AppExternalData().Owner != nil {
		return nil
	}

	u := &user_model.User{ID: app.AppExternalData().OwnerID}
	has, err := db.GetEngine(ctx).Get(u)
	if err != nil {
		return err
	} else if !has {
		return user_model.ErrUserNotExist{UID: app.AppExternalData().OwnerID}
	}

	app.AppExternalData().Owner = u

	return nil
}

func (app *Application) IsAppManager(user *user_model.User) bool {
	if user == nil {
		return false
	}

	if user.IsAdmin {
		return true
	}

	if user.ID == app.AppExternalData().OwnerID {
		return true
	}

	if app.AppExternalData().Owner != nil && app.AppExternalData().Owner.IsOrganization() {
		isOwner, err := org_model.IsOrganizationOwner(context.Background(), app.AppExternalData().Owner.ID, user.ID)
		if err != nil {
			return false
		}
		if isOwner {
			return true
		}
	}

	return false
}
