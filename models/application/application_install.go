// Copyright 2025 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package application

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
)

type AppInstallationType int64

const (
	AppInstallationAdmin AppInstallationType = iota
	AppInstallationOrganization
	AppInstallationUser
)

type AppInstallationStatus int64

const (
	AppInstallationStatusActive AppInstallationStatus = iota
	AppInstallationStatusSuspended
	AppInstallationStatusNotInstalled
)

type AppInstallation struct {
	ID      int64 `xorm:"pk autoincr"`
	Type    AppInstallationType
	OwnerID int64            `xorm:"INDEX"`
	Owner   *user_model.User `xorm:"-"`

	InstalledBy int64
	SuspendedBy int64

	AppID      int64       `xorm:"INDEX"`
	Permission AppPermList `xorm:"TEXT"`

	SelectAllRepositories bool    `xorm:"INDEX"`
	SelectedRepoIDs       []int64 `xorm:"JSON TEXT"`

	Status AppInstallationStatus

	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	SuspendedUnix timeutil.TimeStamp
}

func (i *AppInstallation) IsInstalled() bool {
	return i.Status == AppInstallationStatusActive
}

type AppRepoInstallation struct {
	ID                int64       `xorm:"pk autoincr"`
	AppInstallationID int64       `xorm:"INDEX"`
	RepoID            int64       `xorm:"INDEX"`
	AppID             int64       `xorm:"INDEX"`
	Permission        AppPermList `xorm:"TEXT"`
}

func (a *Application) IsInstallableByOwernerID(ownerID int64) bool {
	if ownerID == user_model.SystemAdminUserID {
		return true
	}

	if ownerID == a.AppExternalData().OwnerID {
		return true
	}

	if a.Visibility == structs.VisibleTypePublic {
		return true
	}

	// TODO: will have app grant logic in future

	return false
}

func (a *Application) GetInstallationByOwnerID(ctx context.Context, ownerID int64) (*AppInstallation, error) {
	installation := &AppInstallation{}
	has, err := db.GetEngine(ctx).Where("app_id = ? AND owner_id = ?", a.ID, ownerID).Get(installation)
	if err != nil {
		return nil, err
	}
	if !has {
		if !a.IsInstallableByOwernerID(ownerID) {
			return nil, err
		}

		return &AppInstallation{
			AppID:   a.ID,
			OwnerID: ownerID,
			Status:  AppInstallationStatusNotInstalled,
		}, nil
	}
	return installation, nil
}

func (a *Application) GetInstallationByOwnerIDs(ctx context.Context, actorIDs []int64) (map[int64]*AppInstallation, error) {
	installations := make([]*AppInstallation, 0, len(actorIDs))

	err := db.GetEngine(ctx).
		In("owner_id", actorIDs).
		Find(&installations)
	if err != nil {
		return nil, err
	}

	installationMap := make(map[int64]*AppInstallation, len(installations))
	for _, installation := range installations {
		installationMap[installation.OwnerID] = installation
	}

	for _, actorID := range actorIDs {
		if _, ok := installationMap[actorID]; !ok {
			installationMap[actorID] = &AppInstallation{
				AppID:   a.ID,
				OwnerID: actorID,
				Status:  AppInstallationStatusNotInstalled,
			}
		}
	}

	return installationMap, nil
}
