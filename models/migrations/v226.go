// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func convertFromNullToDefault(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	type Label struct {
		ID              int64 `xorm:"pk autoincr"`
		RepoID          int64 `xorm:"INDEX"`
		OrgID           int64 `xorm:"INDEX"`
		Name            string
		Description     string
		Color           string             `xorm:"VARCHAR(7)"`
		NumIssues       int                `xorm:"NOT NULL DEFAULT 0"`
		NumClosedIssues int                `xorm:"NOT NULL DEFAULT 0"`
		CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	recreateTable(sess, &Label{})

	type Milestone struct {
		ID              int64 `xorm:"pk autoincr"`
		RepoID          int64 `xorm:"INDEX"`
		Name            string
		Content         string `xorm:"TEXT"`
		IsClosed        bool   `xorm:"NOT NULL DEFAULT false"`
		NumIssues       int    `xorm:"NOT NULL DEFAULT 0"`
		NumClosedIssues int    `xorm:"NOT NULL DEFAULT 0"`
		Completeness    int    `xorm:"NOT NULL DEFAULT 0"` // Percentage(1-100).

		CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
		DeadlineUnix   timeutil.TimeStamp
		ClosedDateUnix timeutil.TimeStamp
	}
	recreateTable(sess, &Milestone{})

	type Issue struct {
		ID               int64 `xorm:"pk autoincr"`
		RepoID           int64 `xorm:"INDEX UNIQUE(repo_index)"`
		Index            int64 `xorm:"UNIQUE(repo_index)"` // Index in one repository.
		PosterID         int64 `xorm:"INDEX NOT NULL DEFAULT 0"`
		OriginalAuthor   string
		OriginalAuthorID int64  `xorm:"INDEX NOT NULL DEFAULT 0"`
		Title            string `xorm:"name"`
		Content          string `xorm:"LONGTEXT"`
		MilestoneID      int64  `xorm:"INDEX NOT NULL DEFAULT 0"`
		Priority         int
		IsClosed         bool `xorm:"INDEX NOT NULL DEFAULT false"`
		IsPull           bool `xorm:"INDEX NOT NULL DEFAULT false"` // Indicates whether is a pull request or not.
		NumComments      int  `xorm:"NOT NULL DEFAULT 0"`
		Ref              string

		DeadlineUnix timeutil.TimeStamp `xorm:"INDEX"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
		ClosedUnix  timeutil.TimeStamp `xorm:"INDEX"`

		// IsLocked limits commenting abilities to users on an issue
		// with write access
		IsLocked bool `xorm:"NOT NULL DEFAULT false"`
	}
	recreateTable(sess, &Issue{})

	type Team struct {
		ID                      int64 `xorm:"pk autoincr"`
		OrgID                   int64 `xorm:"INDEX NOT NULL DEFAULT 0"`
		LowerName               string
		Name                    string
		Description             string
		AccessMode              perm.AccessMode `xorm:"'authorize'"`
		NumRepos                int             `xorm:"NOT NULL DEFAULT 0"`
		NumMembers              int             `xorm:"NOT NULL DEFAULT 0"`
		IncludesAllRepositories bool            `xorm:"NOT NULL DEFAULT false"`
		CanCreateOrgRepo        bool            `xorm:"NOT NULL DEFAULT false"`
	}
	recreateTable(sess, &Team{})

	type Attachment struct {
		ID            int64  `xorm:"pk autoincr"`
		UUID          string `xorm:"uuid UNIQUE"`
		RepoID        int64  `xorm:"INDEX NOT NULL DEFAULT 0"` // this should not be zero
		IssueID       int64  `xorm:"INDEX NOT NULL DEFAULT 0"` // maybe zero when creating
		ReleaseID     int64  `xorm:"INDEX NOT NULL DEFAULT 0"` // maybe zero when creating
		UploaderID    int64  `xorm:"INDEX NOT NULL DEFAULT 0"` // Notice: will be zero before this column added
		CommentID     int64
		Name          string
		DownloadCount int64              `xorm:"NOT NULL DEFAULT 0"`
		Size          int64              `xorm:"NOT NULL DEFAULT 0"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	}
	recreateTable(sess, &Attachment{})

	type Repository struct {
		ID                  int64 `xorm:"pk autoincr"`
		OwnerID             int64 `xorm:"UNIQUE(s) index"`
		OwnerName           string
		LowerName           string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name                string             `xorm:"INDEX NOT NULL"`
		Description         string             `xorm:"TEXT"`
		Website             string             `xorm:"VARCHAR(2048)"`
		OriginalServiceType api.GitServiceType `xorm:"index"`
		OriginalURL         string             `xorm:"VARCHAR(2048)"`
		DefaultBranch       string

		NumWatches          int `xorm:"NOT NULL DEFAULT 0"`
		NumStars            int `xorm:"NOT NULL DEFAULT 0"`
		NumForks            int `xorm:"NOT NULL DEFAULT 0"`
		NumIssues           int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedIssues     int `xorm:"NOT NULL DEFAULT 0"`
		NumPulls            int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedPulls      int `xorm:"NOT NULL DEFAULT 0"`
		NumMilestones       int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedMilestones int `xorm:"NOT NULL DEFAULT 0"`
		NumProjects         int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedProjects   int `xorm:"NOT NULL DEFAULT 0"`

		IsPrivate  bool                  `xorm:"INDEX NOT NULL DEFAULT false"`
		IsEmpty    bool                  `xorm:"INDEX NOT NULL DEFAULT true"`
		IsArchived bool                  `xorm:"INDEX NOT NULL DEFAULT false"`
		IsMirror   bool                  `xorm:"INDEX NOT NULL DEFAULT false"`
		Status     repo.RepositoryStatus `xorm:"NOT NULL DEFAULT 0"`

		IsFork                          bool     `xorm:"INDEX NOT NULL DEFAULT false"`
		ForkID                          int64    `xorm:"INDEX NOT NULL DEFAULT 0"`
		IsTemplate                      bool     `xorm:"INDEX NOT NULL DEFAULT false"`
		TemplateID                      int64    `xorm:"INDEX NOT NULL DEFAULT 0"`
		Size                            int64    `xorm:"NOT NULL DEFAULT 0"`
		IsFsckEnabled                   bool     `xorm:"NOT NULL DEFAULT true"`
		CloseIssuesViaCommitInAnyBranch bool     `xorm:"NOT NULL DEFAULT false"`
		Topics                          []string `xorm:"TEXT JSON"`

		TrustModel repo.TrustModelType

		// Avatar: ID(10-20)-md5(32) - must fit into 64 symbols
		Avatar string `xorm:"VARCHAR(64)"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	recreateTable(sess, &Repository{})

	type Topic struct {
		ID          int64              `xorm:"pk autoincr"`
		Name        string             `xorm:"UNIQUE VARCHAR(50)"`
		RepoCount   int                `xorm:"NOT NULL DEFAULT 0"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	recreateTable(sess, &Topic{})

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

		LoginType   auth.Type
		LoginSource int64 `xorm:"NOT NULL DEFAULT 0"`
		LoginName   string
		Type        user.UserType
		Location    string
		Website     string
		Rands       string `xorm:"VARCHAR(32)"`
		Salt        string `xorm:"VARCHAR(32)"`
		Language    string `xorm:"VARCHAR(5)"`
		Description string

		CreatedUnix   timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"INDEX updated"`
		LastLoginUnix timeutil.TimeStamp `xorm:"INDEX"`

		// Remember visibility choice for convenience, true for private
		LastRepoVisibility bool
		// Maximum repository creation limit, -1 means use global default
		MaxRepoCreation int `xorm:"NOT NULL DEFAULT -1"`

		// IsActive true: primary email is activated, user can access Web UI and Git SSH.
		// false: an inactive user can only log in Web UI for account operations (ex: activate the account by email), no other access.
		IsActive bool `xorm:"INDEX NOT NULL DEFAULT false"`
		// the user is a Gitea admin, who can access all repositories and the admin pages.
		IsAdmin bool `xorm:"NOT NULL DEFAULT false"`
		// true: the user is only allowed to see organizations/repositories that they has explicit rights to.
		// (ex: in private Gitea instances user won't be allowed to see even organizations/repositories that are set as public)
		IsRestricted bool `xorm:"NOT NULL DEFAULT false"`

		AllowGitHook            bool `xorm:"NOT NULL DEFAULT false"`
		AllowImportLocal        bool `xorm:"NOT NULL DEFAULT false"` // Allow migrate repository by local path
		AllowCreateOrganization bool `xorm:"NOT NULL DEFAULT true"`

		// true: the user is not allowed to log in Web UI. Git/SSH access could still be allowed (please refer to Git/SSH access related code/documents)
		ProhibitLogin bool `xorm:"NOT NULL DEFAULT false"`

		// Avatar
		Avatar          string `xorm:"VARCHAR(2048) NOT NULL"`
		AvatarEmail     string `xorm:"NOT NULL"`
		UseCustomAvatar bool   `xorm:"NOT NULL DEFAULT false"`

		// Counters
		NumFollowers int `xorm:"NOT NULL DEFAULT 0"`
		NumFollowing int `xorm:"NOT NULL DEFAULT 0"`
		NumStars     int `xorm:"NOT NULL DEFAULT 0"`
		NumRepos     int `xorm:"NOT NULL DEFAULT 0"`

		// For organization
		NumTeams                  int             `xorm:"NOT NULL DEFAULT 0"`
		NumMembers                int             `xorm:"NOT NULL DEFAULT 0"`
		Visibility                api.VisibleType `xorm:"NOT NULL DEFAULT 0"`
		RepoAdminChangeTeamAccess bool            `xorm:"NOT NULL DEFAULT false"`

		// Preferences
		DiffViewStyle       string `xorm:"NOT NULL DEFAULT ''"`
		Theme               string `xorm:"NOT NULL DEFAULT ''"`
		KeepActivityPrivate bool   `xorm:"NOT NULL DEFAULT false"`
	}
	recreateTable(sess, &User{})

	return sess.Commit()
}
