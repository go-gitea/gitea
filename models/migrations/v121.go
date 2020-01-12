package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/xorm"
)

func addProjectsInfo(x *xorm.Engine) error {

	sess := x.NewSession()
	defer sess.Close()

	type (
		ProjectType      uint8
		ProjectBoardType uint8
	)

	type Project struct {
		ID              int64  `xorm:"pk autoincr"`
		Title           string `xorm:"INDEX NOT NULL"`
		Description     string `xorm:"TEXT"`
		RepoID          int64  `xorm:"NOT NULL"`
		CreatorID       int64  `xorm:"NOT NULL"`
		IsClosed        bool   `xorm:"INDEX"`
		NumIssues       int
		NumClosedIssues int

		BoardType ProjectBoardType
		Type      ProjectType

		ClosedDateUnix timeutil.TimeStamp
		CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := sess.Sync2(new(Project)); err != nil {
		return err
	}

	type Comment struct {
		OldProjectID int64
		ProjectID    int64
	}

	if err := sess.Sync2(new(Comment)); err != nil {
		return err
	}

	type Repository struct {
		NumProjects       int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedProjects int `xorm:"NOT NULL DEFAULT 0"`
		NumOpenProjects   int `xorm:"-"`
	}

	if err := sess.Sync2(new(Repository)); err != nil {
		return err
	}

	type Issue struct {
		ProjectID      int64 `xorm:"INDEX"`
		ProjectBoardID int64 `xorm:"INDEX"`
	}

	if err := sess.Sync2(new(Issue)); err != nil {
		return err
	}

	type ProjectBoard struct {
		ID        int64 `xorm:"pk autoincr"`
		ProjectID int64 `xorm:"INDEX NOT NULL"`
		Title     string
		RepoID    int64 `xorm:"INDEX NOT NULL"`

		// Not really needed but helpful
		CreatorID int64 `xorm:"NOT NULL"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := sess.Sync2(new(ProjectBoard)); err != nil {
		return err
	}

	return sess.Commit()
}
