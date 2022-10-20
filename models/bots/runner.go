// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"xorm.io/builder"
)

// ErrRunnerNotExist represents an error for bot runner not exist
type ErrRunnerNotExist struct {
	ID    int64
	UUID  string
	Token string
}

func (err ErrRunnerNotExist) Error() string {
	if err.UUID != "" {
		return fmt.Sprintf("Bot runner ID [%s] is not exist", err.UUID)
	}

	return fmt.Sprintf("Bot runner token [%s] is not exist", err.Token)
}

// Runner represents runner machines
type Runner struct {
	ID          int64
	UUID        string                 `xorm:"CHAR(36) UNIQUE"`
	Name        string                 `xorm:"VARCHAR(32) UNIQUE"`
	OwnerID     int64                  `xorm:"index"` // org level runner, 0 means system
	Owner       *user_model.User       `xorm:"-"`
	RepoID      int64                  `xorm:"index"` // repo level runner, if orgid also is zero, then it's a global
	Repo        *repo_model.Repository `xorm:"-"`
	Description string                 `xorm:"TEXT"`
	Base        int                    // 0 native 1 docker 2 virtual machine
	RepoRange   string                 // glob match which repositories could use this runner
	Token       string                 `xorm:"CHAR(36) UNIQUE"`

	// instance status (idle, active, offline)
	Status runnerv1.RunnerStatus
	// Store OS and Artch.
	AgentLabels []string
	// Store custom labes use defined.
	CustomLabels []string

	LastOnline timeutil.TimeStamp `xorm:"index"`
	Created    timeutil.TimeStamp `xorm:"created"`
	Updated    timeutil.TimeStamp `xorm:"updated"`
	Deleted    timeutil.TimeStamp `xorm:"deleted"`
}

func (Runner) TableName() string {
	return "bots_runner"
}

func (r *Runner) OwnType() string {
	if r.OwnerID == 0 {
		return "Global"
	}
	if r.RepoID == 0 {
		return r.Owner.Name
	}

	return r.Repo.FullName()
}

func (r *Runner) StatusType() string {
	switch r.Status {
	case runnerv1.RunnerStatus_RUNNER_STATUS_OFFLINE:
		return "offline"
	case runnerv1.RunnerStatus_RUNNER_STATUS_IDLE:
		return "online"
	case runnerv1.RunnerStatus_RUNNER_STATUS_ACTIVE:
		return "online"
	}
	return "unknown"
}

// AllLabels returns agent and custom labels
func (r *Runner) AllLabels() []string {
	return append(r.AgentLabels, r.CustomLabels...)
}

// Editable checks if the runner is editable by the user
func (r *Runner) Editable(ownerID, repoID int64) bool {
	if ownerID == 0 && repoID == 0 {
		return true
	}
	if ownerID > 0 && r.OwnerID == ownerID {
		return true
	}
	return repoID > 0 && r.RepoID == repoID
}

// LoadAttributes loads the attributes of the runner
func (r *Runner) LoadAttributes(ctx context.Context) error {
	if r.OwnerID > 0 {
		var user user_model.User
		has, err := db.GetEngine(ctx).ID(r.OwnerID).Get(&user)
		if err != nil {
			return err
		}
		if has {
			r.Owner = &user
		}
	}
	if r.RepoID > 0 {
		var repo repo_model.Repository
		has, err := db.GetEngine(ctx).ID(r.RepoID).Get(&repo)
		if err != nil {
			return err
		}
		if has {
			r.Repo = &repo
		}
	}
	return nil
}

func init() {
	db.RegisterModel(&Runner{})
}

type FindRunnerOptions struct {
	db.ListOptions
	RepoID      int64
	OwnerID     int64
	Sort        string
	Filter      string
	WithDeleted bool
}

func (opts FindRunnerOptions) toCond() builder.Cond {
	cond := builder.NewCond()

	withGlobal := false
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
		withGlobal = true
	}
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
		withGlobal = true
	}
	if withGlobal {
		cond = cond.Or(builder.Eq{"repo_id": 0, "owner_id": 0})
	}

	if opts.Filter != "" {
		cond = cond.And(builder.Like{"name", opts.Filter})
	}
	if !opts.WithDeleted {
		cond = cond.And(builder.IsNull{"deleted"})
	}
	return cond
}

func (opts FindRunnerOptions) toOrder() string {
	switch opts.Sort {
	case "online":
		return "last_online DESC"
	case "offline":
		return "last_online ASC"
	case "alphabetically":
		return "name ASC"
	}
	return "last_online DESC"
}

func CountRunners(opts FindRunnerOptions) (int64, error) {
	return db.GetEngine(db.DefaultContext).
		Table("bots_runner").
		Where(opts.toCond()).
		OrderBy(opts.toOrder()).
		Count()
}

func FindRunners(opts FindRunnerOptions) (runners RunnerList, err error) {
	sess := db.GetEngine(db.DefaultContext).
		Where(opts.toCond()).
		OrderBy(opts.toOrder())
	if opts.Page > 0 {
		sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	return runners, sess.Find(&runners)
}

// GetUsableRunner returns the usable runner
func GetUsableRunner(opts FindRunnerOptions) (*Runner, error) {
	var runner Runner
	has, err := db.GetEngine(db.DefaultContext).
		Where(opts.toCond()).
		Asc("last_online").
		Get(&runner)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrRunnerNotExist{}
	}

	return &runner, nil
}

// GetRunnerByUUID returns a bot runner via uuid
func GetRunnerByUUID(uuid string) (*Runner, error) {
	var runner Runner
	has, err := db.GetEngine(db.DefaultContext).Where("uuid=?", uuid).Get(&runner)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunnerNotExist{
			UUID: uuid,
		}
	}
	return &runner, nil
}

// GetRunnerByID returns a bot runner via id
func GetRunnerByID(id int64) (*Runner, error) {
	var runner Runner
	has, err := db.GetEngine(db.DefaultContext).Where("id=?", id).Get(&runner)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunnerNotExist{
			ID: id,
		}
	}
	return &runner, nil
}

// GetRunnerByToken returns a bot runner via token
func GetRunnerByToken(token string) (*Runner, error) {
	var runner Runner
	has, err := db.GetEngine(db.DefaultContext).Where("token=?", token).Get(&runner)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRunnerNotExist{
			Token: token,
		}
	}
	return &runner, nil
}

// UpdateRunner updates runner's information.
func UpdateRunner(ctx context.Context, r *Runner, cols ...string) error {
	e := db.GetEngine(ctx)
	var err error
	if len(cols) == 0 {
		_, err = e.ID(r.ID).AllCols().Update(r)
	} else {
		_, err = e.ID(r.ID).Cols(cols...).Update(r)
	}
	return err
}

// DeleteRunner deletes a runner by given ID.
func DeleteRunner(ctx context.Context, r *Runner) error {
	e := db.GetEngine(ctx)
	_, err := e.Delete(r)
	return err
}

// FindRunnersByRepoID returns all workers for the repository
func FindRunnersByRepoID(repoID int64) ([]*Runner, error) {
	var runners []*Runner
	err := db.GetEngine(db.DefaultContext).Where("repo_id=? OR repo_id=0", repoID).
		Find(&runners)
	if err != nil {
		return nil, err
	}
	err = db.GetEngine(db.DefaultContext).Join("INNER", "repository", "repository.owner_id = bot_runner.owner_id").Find(&runners)
	return runners, err
}

// NewRunner creates new runner.
func NewRunner(ctx context.Context, t *Runner) error {
	_, err := db.GetEngine(ctx).Insert(t)
	return err
}
