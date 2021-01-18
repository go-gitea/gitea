// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/builder"
	"xorm.io/xorm"
)

func recreateUserTableToFixDefaultValues(x *xorm.Engine) error {
	type User struct {
		ID        int64  `xorm:"pk autoincr"`
		LowerName string `xorm:"UNIQUE NOT NULL"`
		Name      string `xorm:"UNIQUE NOT NULL"`
		FullName  string
		// Email is the primary email address (to be used for communication)
		Email                        string `xorm:"NOT NULL"`
		KeepEmailPrivate             bool
		EmailNotificationsPreference string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'enabled'"`
		Passwd                       string `xorm:"NOT NULL"`
		PasswdHashAlgo               string `xorm:"NOT NULL DEFAULT 'argon2'"`

		// MustChangePassword is an attribute that determines if a user
		// is to change his/her password after registration.
		MustChangePassword bool `xorm:"NOT NULL DEFAULT false"`

		LoginType   int
		LoginSource int64 `xorm:"NOT NULL DEFAULT 0"`
		LoginName   string
		Type        int
		Location    string
		Website     string
		Rands       string `xorm:"VARCHAR(10)"`
		Salt        string `xorm:"VARCHAR(10)"`
		Language    string `xorm:"VARCHAR(5)"`
		Description string

		CreatedUnix   int64 `xorm:"INDEX created"`
		UpdatedUnix   int64 `xorm:"INDEX updated"`
		LastLoginUnix int64 `xorm:"INDEX"`

		// Remember visibility choice for convenience, true for private
		LastRepoVisibility bool
		// Maximum repository creation limit, -1 means use global default
		MaxRepoCreation int `xorm:"NOT NULL DEFAULT -1"`

		// Permissions
		IsActive                bool `xorm:"INDEX"` // Activate primary email
		IsAdmin                 bool
		IsRestricted            bool `xorm:"NOT NULL DEFAULT false"`
		AllowGitHook            bool
		AllowImportLocal        bool // Allow migrate repository by local path
		AllowCreateOrganization bool `xorm:"DEFAULT true"`
		ProhibitLogin           bool `xorm:"NOT NULL DEFAULT false"`

		// Avatar
		Avatar          string `xorm:"VARCHAR(2048) NOT NULL"`
		AvatarEmail     string `xorm:"NOT NULL"`
		UseCustomAvatar bool

		// Counters
		NumFollowers int
		NumFollowing int `xorm:"NOT NULL DEFAULT 0"`
		NumStars     int
		NumRepos     int

		// For organization
		NumTeams                  int
		NumMembers                int
		Visibility                int  `xorm:"NOT NULL DEFAULT 0"`
		RepoAdminChangeTeamAccess bool `xorm:"NOT NULL DEFAULT false"`

		// Preferences
		DiffViewStyle       string `xorm:"NOT NULL DEFAULT ''"`
		Theme               string `xorm:"NOT NULL DEFAULT ''"`
		KeepActivityPrivate bool   `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(User)); err != nil {
		return err
	}

	if _, err := x.Where(builder.Eq{"keep_activity_private": nil}).
		Cols("keep_activity_private").
		Update(&User{KeepActivityPrivate: false}); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := recreateTable(sess, new(User)); err != nil {
		return err
	}

	return sess.Commit()
}
