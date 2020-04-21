// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
)

type (
	// ProjectsConfig is used to identify the type of board that is being created
	ProjectsConfig struct {
		BoardType   ProjectBoardType
		Translation string
	}

	// ProjectType is used to identify the type of project in question and
	// ownership
	ProjectType uint8
)

const (
	// IndividualType is a type of project board that is owned by an
	// individual.
	IndividualType ProjectType = iota + 1

	// RepositoryType is a project that is tied to a repository.
	RepositoryType

	// OrganizationType is a project that is tied to an organisation.
	OrganizationType
)

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
	BoardType       ProjectBoardType
	Type            ProjectType

	RenderedContent string `xorm:"-"`

	CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	ClosedDateUnix timeutil.TimeStamp
}

// GetProjectsConfig retrieves the types of configurations projects could have
func GetProjectsConfig() []ProjectsConfig {
	return []ProjectsConfig{
		{None, "repo.projects.type.none"},
		{BasicKanban, "repo.projects.type.basic_kanban"},
		{BugTriage, "repo.projects.type.bug_triage"},
	}
}

// IsProjectTypeValid checks if a project typeis valid
func IsProjectTypeValid(p ProjectType) bool {
	switch p {
	case IndividualType, RepositoryType, OrganizationType:
		return true
	default:
		return false
	}
}

// AfterLoad is invoked from XORM after setting the value of a field of this object.
func (p *Project) AfterLoad() {
	p.NumOpenIssues = p.NumIssues - p.NumClosedIssues
}

// ProjectSearchOptions are options for GetProjects
type ProjectSearchOptions struct {
	RepoID   int64
	Page     int
	IsClosed util.OptionalBool
	SortType string
	Type     ProjectType
}

// GetProjects returns a list of all projects that have been created in the repository
func GetProjects(opts ProjectSearchOptions) ([]*Project, error) {

	projects := make([]*Project, 0, setting.UI.IssuePagingNum)

	sess := x.Where("repo_id = ?", opts.RepoID)
	switch opts.IsClosed {
	case util.OptionalBoolTrue:
		sess = sess.Where("is_closed = ?", true)
	case util.OptionalBoolFalse:
		sess = sess.Where("is_closed = ?", false)
	}

	if opts.Type > 0 {
		sess = sess.Where("type = ?", opts.Type)
	}

	if opts.Page > 0 {
		sess = sess.Limit(setting.UI.IssuePagingNum, (opts.Page-1)*setting.UI.IssuePagingNum)
	}

	switch opts.SortType {
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
	if !IsProjectBoardTypeValid(p.BoardType) {
		p.BoardType = None
	}

	if !IsProjectTypeValid(p.Type) {
		return errors.New("project type is not valid")
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

// GetProjectByRepoID returns the projects in a repository.
func GetProjectByRepoID(repoID, id int64) (*Project, error) {
	return getProjectByRepoID(x, repoID, id)
}

func getProjectByRepoID(e Engine, repoID, id int64) (*Project, error) {
	p := &Project{
		ID:     id,
		RepoID: repoID,
	}

	has, err := e.Get(p)
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

// ChangeProjectStatus toggle a project between opened and closed
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
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := deleteProjectByRepoID(sess, repoID, id); err != nil {
		return err
	}

	return sess.Commit()
}

func deleteProjectByRepoID(e Engine, repoID, id int64) error {
	p, err := getProjectByRepoID(e, repoID, id)
	if err != nil {
		if IsErrProjectNotExist(err) {
			return nil
		}
		return err
	}

	repo, err := getRepositoryByID(e, p.RepoID)
	if err != nil {
		return err
	}

	if _, err = e.ID(p.ID).Delete(new(Project)); err != nil {
		return err
	}

	numProjects, err := countRepoProjects(e, repo.ID)
	if err != nil {
		return err
	}

	numClosedProjects, err := countRepoClosedProjects(e, repo.ID)
	if err != nil {
		return err
	}

	repo.NumProjects = int(numProjects)
	repo.NumClosedProjects = int(numClosedProjects)

	if _, err = e.ID(repo.ID).Cols("num_projects, num_closed_projects").Update(repo); err != nil {
		return err
	}

	if _, err = e.Exec("UPDATE `issue` SET project_id = 0 WHERE project_id = ?", p.ID); err != nil {
		return err
	}

	return nil
}

// UpdateProject updates a project
func UpdateProject(p *Project) error {
	_, err := x.ID(p.ID).AllCols().Update(p)
	return err
}

// ChangeProjectAssign changes the project associated with an issue
func ChangeProjectAssign(issue *Issue, doer *User, oldProjectID int64) error {

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := changeProjectAssign(sess, doer, issue, oldProjectID); err != nil {
		return err
	}

	return sess.Commit()
}

func changeProjectAssign(sess *xorm.Session, doer *User, issue *Issue, oldProjectID int64) error {

	if oldProjectID > 0 {
		p, err := getProjectByRepoID(sess, issue.RepoID, oldProjectID)
		if err != nil {
			return err
		}

		p.NumIssues--
		if issue.IsClosed {
			p.NumClosedIssues--
		}

		if err := updateProject(sess, p); err != nil {
			return err
		}
	}

	if issue.ProjectID > 0 {
		p, err := getProjectByRepoID(sess, issue.RepoID, issue.ProjectID)
		if err != nil {
			return err
		}

		p.NumIssues++
		if issue.IsClosed {
			p.NumClosedIssues++
		}

		if err := updateProject(sess, p); err != nil {
			return err
		}
	}

	if err := issue.loadRepo(sess); err != nil {
		return err
	}

	if oldProjectID > 0 || issue.ProjectID > 0 {
		if _, err := createComment(sess, &CreateCommentOptions{
			Type:         CommentTypeProject,
			Doer:         doer,
			Repo:         issue.Repo,
			Issue:        issue,
			OldProjectID: oldProjectID,
			ProjectID:    issue.ProjectID,
		}); err != nil {
			return err
		}
	}

	return updateIssueCols(sess, issue, "project_id")
}
