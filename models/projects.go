// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"github.com/go-xorm/xorm"
)

// ProjectType is used to represent a project board type
type ProjectType uint8

// ProjectBoard is used to represent boards on a kanban project
type ProjectBoard struct {
	ID        int64 `xorm:"pk autoincr"`
	ProjectID int64 `xorm:"INDEX NOT NULL"`
	Title     string

	// Not really needed but helpful
	CreatorID int64 `xorm:"NOT NULL"`

	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`
}

const (
	// None is a project board type that has no predefined columns
	None ProjectType = iota
	// BasicKanban is a project board type that has basic predefined columns
	BasicKanban
	// BugTriage is a project type that has predefined columns suited to
	// hunting down bugs
	BugTriage
)

// ProjectsConfig is used to identify the type of board that is being created
type ProjectsConfig struct {
	Type        ProjectType
	Translation string
}

// GetProjectsConfig retrieves the types of configurations projects could have
func GetProjectsConfig() []ProjectsConfig {
	return []ProjectsConfig{
		{None, "repo.projects.type.none"},
		{BasicKanban, "repo.projects.type.basic_kanban"},
		{BugTriage, "repo.projects.type.bug_triage"},
	}
}

// IsProjectTypeValid checks if the project type is valid
func IsProjectTypeValid(p ProjectType) bool {
	switch p {
	case None, BasicKanban, BugTriage:
		return true
	default:
		return false
	}
}

// Project is a kanban board
type Project struct {
	ID              int64  `xorm:"pk autoincr"`
	Title           string `xorm:"INDEX NOT NULL"`
	Description     string `xorm:"TEXT"`
	RepoID          int64  `xorm:"NOT NULL"`
	CreatorID       int64  `xorm:"NOT NULL"`
	IsClosed        bool   `xorm:"INDEX"`
	NumIssues       int
	NumClosedIssues int
	NumOpenIssues   int `xorm:"-"`
	Type            ProjectType

	RenderedContent string `xorm:"-"`

	CreatedUnix    util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix    util.TimeStamp `xorm:"INDEX updated"`
	ClosedDateUnix util.TimeStamp
}

// AfterLoad is invoked from XORM after setting the value of a field of
// this object.
func (p *Project) AfterLoad() {
	p.NumOpenIssues = p.NumIssues - p.NumClosedIssues
}

// GetProjects returns a list of all projects that have been created in the
// repository
func GetProjects(repoID int64, page int, isClosed bool, sortType string) ([]*Project, error) {

	projects := make([]*Project, 0, setting.UI.IssuePagingNum)
	sess := x.Where("repo_id = ? AND is_closed = ?", repoID, isClosed)
	if page > 0 {
		sess = sess.Limit(setting.UI.IssuePagingNum, (page-1)*setting.UI.IssuePagingNum)
	}

	switch sortType {
	case "oldest":
		sess.Desc("created_unix")
	case "recentupdate":
		sess.Desc("updated_unix")
	case "leastupdate":
		sess.Asc("updated_unix")
	default:
		sess.Asc("created_unix")
	}

	return projects, sess.Find(&projects)
}

// NewProject creates a new Project
func NewProject(p *Project) error {
	if !IsProjectTypeValid(p.Type) {
		p.Type = None
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Insert(p); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE `repository` SET num_projects = num_projects + 1 WHERE id = ?", p.RepoID); err != nil {
		return err
	}

	if err := createBoardsForProjectsType(sess, p); err != nil {
		return err
	}

	return sess.Commit()
}

func createBoardsForProjectsType(sess *xorm.Session, project *Project) error {

	var items []string

	switch project.Type {

	case BugTriage:
		items = setting.Repository.ProjectBoardBugTriageType

	case BasicKanban:
		items = setting.Repository.ProjectBoardBasicKanbanType

	case None:
		fallthrough
	default:
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	var boards = make([]ProjectBoard, 0)

	for _, v := range items {
		boards = append(boards, ProjectBoard{
			CreatedUnix: util.TimeStampNow(),
			UpdatedUnix: util.TimeStampNow(),
			CreatorID:   project.CreatorID,
			Title:       v,
			ProjectID:   project.ID,
		})
	}

	_, err := sess.Insert(boards)
	return err
}

// GetProjectByRepoID returns the projects in a repository.
func GetProjectByRepoID(repoID, id int64) (*Project, error) {

	p := &Project{
		ID:     id,
		RepoID: repoID,
	}

	has, err := x.Get(p)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectNotExist{id, repoID}
	}

	return p, nil
}

func updateProject(e Engine, p *Project) error {
	_, err := e.ID(p.ID).AllCols().Update(p)
	return err
}

func countRepoProjects(e Engine, repoID int64) (int64, error) {
	return e.
		Where("repo_id=?", repoID).
		Count(new(Project))
}

func countRepoClosedProjects(e Engine, repoID int64) (int64, error) {
	return e.
		Where("repo_id=? AND is_closed=?", repoID, true).
		Count(new(Project))
}

// ChangeProjectStatus togggles a project between opened and closed
func ChangeProjectStatus(p *Project, isClosed bool) error {

	repo, err := GetRepositoryByID(p.RepoID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	p.IsClosed = isClosed
	if err = updateProject(sess, p); err != nil {
		return err
	}

	numProjects, err := countRepoProjects(sess, repo.ID)
	if err != nil {
		return err
	}

	numClosedProjects, err := countRepoClosedProjects(sess, repo.ID)
	if err != nil {
		return err
	}

	repo.NumProjects = int(numProjects)
	repo.NumClosedProjects = int(numClosedProjects)

	if _, err = sess.ID(repo.ID).Cols("num_projects, num_closed_projects").Update(repo); err != nil {
		return err
	}

	return sess.Commit()
}

// DeleteProjectByRepoID deletes a project from a repository.
func DeleteProjectByRepoID(repoID, id int64) error {
	p, err := GetProjectByRepoID(repoID, id)
	if err != nil {
		if IsErrProjectNotExist(err) {
			return nil
		}
		return err
	}

	repo, err := GetRepositoryByID(p.RepoID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.ID(p.ID).Delete(new(Project)); err != nil {
		return err
	}

	numProjects, err := countRepoProjects(sess, repo.ID)
	if err != nil {
		return err
	}

	numClosedProjects, err := countRepoClosedProjects(sess, repo.ID)
	if err != nil {
		return err
	}

	repo.NumProjects = int(numProjects)
	repo.NumClosedProjects = int(numClosedProjects)

	if _, err = sess.ID(repo.ID).Cols("num_projects, num_closed_projects").Update(repo); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `issue` SET project_id = 0 WHERE project_id = ?", p.ID); err != nil {
		return err
	}

	return sess.Commit()
}

// UpdateProject updates a project
func UpdateProject(p *Project) error {
	_, err := x.ID(p.ID).AllCols().Update(p)
	return err
}
