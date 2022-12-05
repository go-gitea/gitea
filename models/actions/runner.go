// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	runnerv1 "code.gitea.io/bots-proto-go/runner/v1"
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

// BotRunner represents runner machines
type BotRunner struct {
	ID          int64
	UUID        string                 `xorm:"CHAR(36) UNIQUE"`
	Name        string                 `xorm:"VARCHAR(32)"`
	OwnerID     int64                  `xorm:"index"` // org level runner, 0 means system
	Owner       *user_model.User       `xorm:"-"`
	RepoID      int64                  `xorm:"index"` // repo level runner, if orgid also is zero, then it's a global
	Repo        *repo_model.Repository `xorm:"-"`
	Description string                 `xorm:"TEXT"`
	Base        int                    // 0 native 1 docker 2 virtual machine
	RepoRange   string                 // glob match which repositories could use this runner

	Token     string `xorm:"-"`
	TokenHash string `xorm:"UNIQUE"` // sha256 of token
	TokenSalt string
	// TokenLastEight string `xorm:"token_last_eight"` // it's unnecessary because we don't find runners by token

	LastOnline timeutil.TimeStamp `xorm:"index"`
	LastActive timeutil.TimeStamp `xorm:"index"`

	// Store OS and Artch.
	AgentLabels []string
	// Store custom labes use defined.
	CustomLabels []string

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
	Deleted timeutil.TimeStamp `xorm:"deleted"`
}

func (r *BotRunner) OwnType() string {
	if r.OwnerID == 0 {
		return "Global"
	}
	if r.RepoID == 0 {
		return r.Owner.Name
	}

	return r.Repo.FullName()
}

func (r *BotRunner) Status() runnerv1.RunnerStatus {
	if time.Since(r.LastOnline.AsTime()) > time.Minute {
		return runnerv1.RunnerStatus_RUNNER_STATUS_OFFLINE
	}
	if time.Since(r.LastActive.AsTime()) > 10*time.Second {
		return runnerv1.RunnerStatus_RUNNER_STATUS_IDLE
	}
	return runnerv1.RunnerStatus_RUNNER_STATUS_ACTIVE
}

func (r *BotRunner) StatusName() string {
	return strings.ToLower(strings.TrimPrefix(r.Status().String(), "RUNNER_STATUS_"))
}

func (r *BotRunner) IsOnline() bool {
	status := r.Status()
	if status == runnerv1.RunnerStatus_RUNNER_STATUS_IDLE || status == runnerv1.RunnerStatus_RUNNER_STATUS_ACTIVE {
		return true
	}
	return false
}

// AllLabels returns agent and custom labels
func (r *BotRunner) AllLabels() []string {
	return append(r.AgentLabels, r.CustomLabels...)
}

// Editable checks if the runner is editable by the user
func (r *BotRunner) Editable(ownerID, repoID int64) bool {
	if ownerID == 0 && repoID == 0 {
		return true
	}
	if ownerID > 0 && r.OwnerID == ownerID {
		return true
	}
	return repoID > 0 && r.RepoID == repoID
}

// LoadAttributes loads the attributes of the runner
func (r *BotRunner) LoadAttributes(ctx context.Context) error {
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

func (r *BotRunner) GenerateToken() (err error) {
	r.Token, r.TokenSalt, r.TokenHash, _, err = generateSaltedToken()
	return err
}

func init() {
	db.RegisterModel(&BotRunner{})
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
		Table(BotRunner{}).
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
func GetUsableRunner(opts FindRunnerOptions) (*BotRunner, error) {
	var runner BotRunner
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
func GetRunnerByUUID(uuid string) (*BotRunner, error) {
	var runner BotRunner
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
func GetRunnerByID(id int64) (*BotRunner, error) {
	var runner BotRunner
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

// UpdateRunner updates runner's information.
func UpdateRunner(ctx context.Context, r *BotRunner, cols ...string) error {
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
func DeleteRunner(ctx context.Context, r *BotRunner) error {
	e := db.GetEngine(ctx)
	_, err := e.Delete(r)
	return err
}

// FindRunnersByRepoID returns all workers for the repository
func FindRunnersByRepoID(repoID int64) ([]*BotRunner, error) {
	var runners []*BotRunner
	err := db.GetEngine(db.DefaultContext).Where("repo_id=? OR repo_id=0", repoID).
		Find(&runners)
	if err != nil {
		return nil, err
	}
	err = db.GetEngine(db.DefaultContext).Join("INNER", "repository", "repository.owner_id = bot_runner.owner_id").Find(&runners)
	return runners, err
}

// NewRunner creates new runner.
func NewRunner(ctx context.Context, t *BotRunner) error {
	_, err := db.GetEngine(ctx).Insert(t)
	return err
}
