// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "github.com/go-xorm/xorm"

func refactorIndexes(x *xorm.Engine) error {

	// Action see models/action.go
	type Action struct {
		ID        int64 `xorm:"pk autoincr"`
		UserID    int64 `xorm:"INDEX"` // Receiver user id.
		OpType    int   // ActionType
		ActUserID int64 `xorm:"INDEX"` // Action user id.
		//	ActUser     *User       `xorm:"-"`
		RepoID int64 `xorm:"INDEX"`
		//	Repo        *Repository `xorm:"-"`
		CommentID int64 `xorm:"INDEX"`
		//	Comment     *Comment    `xorm:"-"`
		IsDeleted   bool `xorm:"NOT NULL DEFAULT false"`
		RefName     string
		IsPrivate   bool   `xorm:"NOT NULL DEFAULT false"`
		Content     string `xorm:"TEXT"`
		CreatedUnix int64  `xorm:"INDEX created"` // timeutil.TimeStamp
	}

	// Collaboration see models/repo_collaboration.go
	type Collaboration struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"UNIQUE(s) NOT NULL"`
		UserID int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Mode   int   `xorm:"DEFAULT 2 NOT NULL"` // AccessMode
	}

	// CommitStatus see models/commit_status.go
	type CommitStatus struct {
		ID     int64  `xorm:"pk autoincr"`
		SHA    string `xorm:"VARCHAR(64) NOT NULL UNIQUE(repo_sha_index)"`
		Index  int64  `xorm:"UNIQUE(repo_sha_index)"`
		RepoID int64  `xorm:"UNIQUE(repo_sha_index)"`
		//	Repo        *Repository  `xorm:"-"`
		State       string `xorm:"VARCHAR(7) NOT NULL"` // CommitStatusState
		TargetURL   string `xorm:"TEXT"`
		Description string `xorm:"TEXT"`
		ContextHash string `xorm:"char(40)"`
		Context     string `xorm:"TEXT"`
		//	Creator     *User        `xorm:"-"`
		CreatorID int64

		CreatedUnix int64 `xorm:"INDEX created"` // timeutil.TimeStamp
		UpdatedUnix int64 `xorm:"INDEX updated"` // timeutil.TimeStamp
	}

	// DeployKey see models/ssh_key.go
	type DeployKey struct {
		ID          int64 `xorm:"pk autoincr"`
		KeyID       int64 `xorm:"UNIQUE(s)"`
		RepoID      int64 `xorm:"UNIQUE(s) INDEX"`
		Name        string
		Fingerprint string
		//	Content     string `xorm:"-"`

		Mode int `xorm:"NOT NULL DEFAULT 1"` // AccessMode

		CreatedUnix int64 `xorm:"created"` // timeutil.TimeStamp
		UpdatedUnix int64 `xorm:"updated"` // timeutil.TimeStamp
		//	HasRecentActivity bool               `xorm:"-"`
		//	HasUsed           bool               `xorm:"-"`
	}

	// Issue see models/issue.go
	type Issue struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"UNIQUE(repo_index)"`
		//	Repo             *Repository `xorm:"-"`
		Index    int64 `xorm:"UNIQUE(repo_index)"` // Index in one repository.
		PosterID int64 `xorm:"INDEX"`
		//	Poster           *User       `xorm:"-"`
		OriginalAuthor   string
		OriginalAuthorID int64
		Title            string `xorm:"name"`
		Content          string `xorm:"TEXT"`
		//	RenderedContent  string     `xorm:"-"`
		//	Labels           []*Label   `xorm:"-"`
		MilestoneID int64 `xorm:"INDEX"`
		//	Milestone        *Milestone `xorm:"-"`
		Priority int
		//	AssigneeID       int64 `xorm:"-"`
		//	Assignee         *User `xorm:"-"`
		IsClosed bool
		//	IsRead           bool         `xorm:"-"`
		IsPull bool // Indicates whether is a pull request or not.
		//	PullRequest      *PullRequest `xorm:"-"`
		NumComments int
		Ref         string

		DeadlineUnix int64 `xorm:"INDEX"` // timeutil.TimeStamp

		CreatedUnix int64 `xorm:"INDEX created"` // timeutil.TimeStamp
		UpdatedUnix int64 `xorm:"INDEX updated"` // timeutil.TimeStamp
		ClosedUnix  int64 `xorm:"INDEX"`         // timeutil.TimeStamp

		//	Attachments      []*Attachment `xorm:"-"`
		//	Comments         []*Comment    `xorm:"-"`
		//	Reactions        ReactionList  `xorm:"-"`
		//	TotalTrackedTime int64         `xorm:"-"`
		//	Assignees        []*User       `xorm:"-"`

		// IsLocked limits commenting abilities to users on an issue
		// with write access
		IsLocked bool `xorm:"NOT NULL DEFAULT false"`
	}

	// LFSMetaObject see models/lfs.go
	type LFSMetaObject struct {
		ID           int64  `xorm:"pk autoincr"`
		Oid          string `xorm:"UNIQUE(s) NOT NULL"`
		Size         int64  `xorm:"NOT NULL"`
		RepositoryID int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
		//	Existing     bool               `xorm:"-"`
		CreatedUnix int64 `xorm:"created"` // timeutil.TimeStamp
	}

	// Notification see models/notification.go
	type Notification struct {
		ID     int64 `xorm:"pk autoincr"`
		UserID int64 `xorm:"INDEX NOT NULL"`
		RepoID int64 `xorm:"INDEX NOT NULL"`

		Status uint8 `xorm:"SMALLINT NOT NULL"` // NotificationStatus
		Source uint8 `xorm:"SMALLINT NOT NULL"` // NotificationSource

		IssueID  int64 `xorm:"INDEX NOT NULL"`
		CommitID string

		UpdatedBy int64 `xorm:"INDEX NOT NULL"`

		//	Issue      *Issue      `xorm:"-"`
		//	Repository *Repository `xorm:"-"`

		CreatedUnix int64 `xorm:"created INDEX NOT NULL"` // timeutil.TimeStamp
		UpdatedUnix int64 `xorm:"updated INDEX NOT NULL"` // timeutil.TimeStamp
	}

	// OrgUser see models/org.go
	type OrgUser struct {
		ID       int64 `xorm:"pk autoincr"`
		UID      int64 `xorm:"UNIQUE(s)"`
		OrgID    int64 `xorm:"INDEX UNIQUE(s)"`
		IsPublic bool
	}

	// ProtectedBranch see models/pull.go
	type ProtectedBranch struct {
		ID                        int64  `xorm:"pk autoincr"`
		RepoID                    int64  `xorm:"UNIQUE(s)"`
		BranchName                string `xorm:"UNIQUE(s)"`
		CanPush                   bool   `xorm:"NOT NULL DEFAULT false"`
		EnableWhitelist           bool
		WhitelistUserIDs          []int64 `xorm:"JSON TEXT"`
		WhitelistTeamIDs          []int64 `xorm:"JSON TEXT"`
		EnableMergeWhitelist      bool    `xorm:"NOT NULL DEFAULT false"`
		MergeWhitelistUserIDs     []int64 `xorm:"JSON TEXT"`
		MergeWhitelistTeamIDs     []int64 `xorm:"JSON TEXT"`
		ApprovalsWhitelistUserIDs []int64 `xorm:"JSON TEXT"`
		ApprovalsWhitelistTeamIDs []int64 `xorm:"JSON TEXT"`
		RequiredApprovals         int64   `xorm:"NOT NULL DEFAULT 0"`
		CreatedUnix               int64   `xorm:"created"` // timeutil.TimeStamp
		UpdatedUnix               int64   `xorm:"updated"` // timeutil.TimeStamp
	}

	// Reaction see models/issue_reaction.go
	type Reaction struct {
		ID        int64  `xorm:"pk autoincr"`
		IssueID   int64  `xorm:"UNIQUE(s) NOT NULL"`
		CommentID int64  `xorm:"INDEX UNIQUE(s)"`
		Type      string `xorm:"UNIQUE(s) NOT NULL"`
		UserID    int64  `xorm:"INDEX UNIQUE(s) NOT NULL"`
		//	User        *User              `xorm:"-"`
		CreatedUnix int64 `xorm:"INDEX created"` // timeutil.TimeStamp
	}

	// Release see models/release.go
	type Release struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"UNIQUE(n)"`
		//	Repo             *Repository `xorm:"-"`
		PublisherID int64 `xorm:"INDEX"`
		//	Publisher        *User       `xorm:"-"`
		TagName      string `xorm:"UNIQUE(n)"`
		LowerTagName string
		Target       string
		Title        string
		Sha1         string `xorm:"VARCHAR(40)"`
		NumCommits   int64
		//	NumCommitsBehind int64              `xorm:"-"`
		Note         string `xorm:"TEXT"`
		IsDraft      bool   `xorm:"NOT NULL DEFAULT false"`
		IsPrerelease bool   `xorm:"NOT NULL DEFAULT false"`
		IsTag        bool   `xorm:"NOT NULL DEFAULT false"`
		//	Attachments      []*Attachment      `xorm:"-"`
		CreatedUnix int64 `xorm:"INDEX"`
	}

	// RepoRedirect see models/repo_redirect.go
	type RepoRedirect struct {
		ID             int64  `xorm:"pk autoincr"`
		OwnerID        int64  `xorm:"UNIQUE(s)"`
		LowerName      string `xorm:"UNIQUE(s) NOT NULL"`
		RedirectRepoID int64  // repoID to redirect to
	}

	// Repository see models/repo.go
	type Repository struct {
		ID      int64 `xorm:"pk autoincr"`
		OwnerID int64 `xorm:"UNIQUE(s)"`
		//	OwnerName     string `xorm:"-"`
		//	Owner         *User  `xorm:"-"`
		LowerName     string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name          string `xorm:"INDEX NOT NULL"`
		Description   string `xorm:"TEXT"`
		Website       string `xorm:"VARCHAR(2048)"`
		OriginalURL   string `xorm:"VARCHAR(2048)"`
		DefaultBranch string

		NumWatches      int
		NumStars        int
		NumForks        int
		NumIssues       int
		NumClosedIssues int
		//	NumOpenIssues       int `xorm:"-"`
		NumPulls       int
		NumClosedPulls int
		//	NumOpenPulls        int `xorm:"-"`
		NumMilestones       int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedMilestones int `xorm:"NOT NULL DEFAULT 0"`
		//	NumOpenMilestones   int `xorm:"-"`
		//	NumReleases         int `xorm:"-"`

		IsPrivate  bool
		IsEmpty    bool
		IsArchived bool

		IsMirror bool
		//	*Mirror  `xorm:"-"`

		//	ExternalMetas map[string]string `xorm:"-"`
		//	Units         []*RepoUnit       `xorm:"-"`

		IsFork bool  `xorm:"NOT NULL DEFAULT false"`
		ForkID int64 `xorm:"INDEX"`
		//	BaseRepo                        *Repository        `xorm:"-"`
		Size int64 `xorm:"NOT NULL DEFAULT 0"`
		//	IndexerStatus                   *RepoIndexerStatus `xorm:"-"`
		IsFsckEnabled                   bool     `xorm:"NOT NULL DEFAULT true"`
		CloseIssuesViaCommitInAnyBranch bool     `xorm:"NOT NULL DEFAULT false"`
		Topics                          []string `xorm:"TEXT JSON"`

		// Avatar: ID(10-20)-md5(32) - must fit into 64 symbols
		Avatar string `xorm:"VARCHAR(64)"`

		CreatedUnix int64 `xorm:"INDEX created"` // timeutil.TimeStamp
		UpdatedUnix int64 `xorm:"INDEX updated"` // timeutil.TimeStamp
	}

	if err := x.Sync2(new(Action)); err != nil {
		return err
	}
	if err := x.Sync2(new(Collaboration)); err != nil {
		return err
	}
	if err := dropTableIndex(x, "commit_status", "UQE_commit_status_repo_sha_index"); err != nil {
		return err
	}
	if err := x.Sync2(new(CommitStatus)); err != nil {
		return err
	}
	if err := x.Sync2(new(DeployKey)); err != nil {
		return err
	}
	if err := x.Sync2(new(Issue)); err != nil {
		return err
	}
	if err := x.Sync2(new(LFSMetaObject)); err != nil {
		return err
	}
	if err := x.Sync2(new(Notification)); err != nil {
		return err
	}
	if err := x.Sync2(new(OrgUser)); err != nil {
		return err
	}
	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return err
	}
	if err := dropTableIndex(x, "reaction", "UQE_reaction_s"); err != nil {
		return err
	}
	if err := x.Sync2(new(Reaction)); err != nil {
		return err
	}
	if err := x.Sync2(new(Release)); err != nil {
		return err
	}
	if err := x.Sync2(new(RepoRedirect)); err != nil {
		return err
	}
	return x.Sync2(new(Repository))
}
