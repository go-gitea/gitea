// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/git"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// MergeQueueEntryState uses the state names of GitHub's merge queue
type MergeQueueEntryState string

const (
	MergeQueueStateQueued         MergeQueueEntryState = "queued"          // waiting to be built
	MergeQueueStateAwaitingChecks MergeQueueEntryState = "awaiting_checks" // queue commit built, checks running
	MergeQueueStateMergeable      MergeQueueEntryState = "mergeable"       // checks passed, waiting for the entries before it
	MergeQueueStateUnmergeable    MergeQueueEntryState = "unmergeable"     // checks failed, kept only for HEADGREEN
)

type MergeQueueEntry struct {
	ID                int64                `xorm:"pk autoincr"`
	RepoID            int64                `xorm:"INDEX(branch) NOT NULL"`
	BaseBranch        string               `xorm:"INDEX(branch) VARCHAR(255) NOT NULL"`
	PullID            int64                `xorm:"UNIQUE NOT NULL"`
	EnqueuerID        int64                `xorm:"NOT NULL"`
	Jump              bool                 `xorm:"NOT NULL DEFAULT false"`
	State             MergeQueueEntryState `xorm:"VARCHAR(20) NOT NULL"`
	HeadSHA           string               `xorm:"VARCHAR(64) NOT NULL"`
	BaseSHA           string               `xorm:"VARCHAR(64)"`
	GroupHeadSHA      string               `xorm:"VARCHAR(64) INDEX"`
	GroupRef          string               `xorm:"VARCHAR(255)"`
	ChecksStartedUnix timeutil.TimeStamp
	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
}

func (MergeQueueEntry) TableName() string {
	return "pull_merge_queue_entry"
}

func init() {
	db.RegisterModel(new(MergeQueueEntry))
}

func (e *MergeQueueEntry) IsBuilt() bool {
	return e.GroupHeadSHA != ""
}

const MergeQueueRefPrefix = git.BranchPrefix + "gh-readonly-queue/"

// MergeQueueGroupRef uses the SHA the group is built on so a rebuild on a new base gets a new ref.
func MergeQueueGroupRef(baseBranch string, prIndex int64, baseSHA string) string {
	return fmt.Sprintf("%s%s/pr-%d-%s", MergeQueueRefPrefix, baseBranch, prIndex, baseSHA)
}

func IsMergeQueueRef(ctx context.Context, repoID int64, ref git.RefName) (bool, error) {
	if !strings.HasPrefix(ref.String(), MergeQueueRefPrefix) {
		return false, nil
	}
	return db.Exist[MergeQueueEntry](ctx, builder.Eq{"repo_id": repoID, "group_ref": ref.String()})
}

func GetMergeQueueGroupRefs(ctx context.Context, repoID int64) ([]string, error) {
	var refs []string
	err := db.GetEngine(ctx).Table(&MergeQueueEntry{}).Where("repo_id = ? AND group_ref <> ''", repoID).Cols("group_ref").Find(&refs)
	return refs, err
}

var ErrAlreadyInMergeQueue = util.NewAlreadyExistErrorf("pull request is already in the merge queue")

func AddToMergeQueue(ctx context.Context, entry *MergeQueueEntry) error {
	exist, err := db.Exist[MergeQueueEntry](ctx, builder.Eq{"pull_id": entry.PullID})
	if err != nil {
		return err
	} else if exist {
		return ErrAlreadyInMergeQueue
	}
	entry.State = MergeQueueStateQueued
	_, err = db.GetEngine(ctx).Insert(entry)
	return err
}

// GetMergeQueueEntryByPullID returns nil when the pull request is not queued
func GetMergeQueueEntryByPullID(ctx context.Context, pullID int64) (*MergeQueueEntry, error) {
	entry, _, err := db.Get[MergeQueueEntry](ctx, builder.Eq{"pull_id": pullID})
	return entry, err
}

func GetMergeQueueEntriesByGroupHeadSHA(ctx context.Context, repoID int64, sha string) ([]*MergeQueueEntry, error) {
	entries := make([]*MergeQueueEntry, 0, 1)
	return entries, db.GetEngine(ctx).Where("repo_id = ? AND group_head_sha = ?", repoID, sha).Find(&entries)
}

// GetMergeQueuePosition returns the 1-based position of the entry in GetMergeQueueEntries order
func GetMergeQueuePosition(ctx context.Context, entry *MergeQueueEntry) (int, error) {
	before := util.Iif[builder.Cond](entry.Jump,
		builder.Eq{"jump": true}.And(builder.Lt{"id": entry.ID}),
		builder.Eq{"jump": true}.Or(builder.Lt{"id": entry.ID}))
	n, err := db.GetEngine(ctx).Where(builder.Eq{"repo_id": entry.RepoID, "base_branch": entry.BaseBranch}.And(before)).Count(&MergeQueueEntry{})
	return int(n) + 1, err
}

// GetMergeQueueEntries returns the queue of a branch in merge order, jumped entries first
func GetMergeQueueEntries(ctx context.Context, repoID int64, baseBranch string) ([]*MergeQueueEntry, error) {
	entries := make([]*MergeQueueEntry, 0, 10)
	return entries, db.GetEngine(ctx).Where("repo_id = ? AND base_branch = ?", repoID, baseBranch).
		OrderBy("jump DESC, id ASC").Find(&entries)
}

// GetMergeQueueBranches returns the branches of all repositories that have queued pull requests
func GetMergeQueueBranches(ctx context.Context) ([]*MergeQueueEntry, error) {
	entries := make([]*MergeQueueEntry, 0, 10)
	return entries, db.GetEngine(ctx).Select("repo_id, base_branch").GroupBy("repo_id, base_branch").Find(&entries)
}

func UpdateMergeQueueEntry(ctx context.Context, entry *MergeQueueEntry) error {
	_, err := db.GetEngine(ctx).ID(entry.ID).Cols("state", "base_sha", "group_head_sha", "group_ref", "checks_started_unix").Update(entry)
	return err
}

// DeleteMergeQueueEntry removes the pull request from the queue, returns whether it was queued
func DeleteMergeQueueEntry(ctx context.Context, pullID int64) (bool, error) {
	n, err := db.GetEngine(ctx).Where(builder.Eq{"pull_id": pullID}).Delete(&MergeQueueEntry{})
	return n > 0, err
}

func HasMergeQueueEntries(ctx context.Context, repoID int64, baseBranch string) (bool, error) {
	return db.Exist[MergeQueueEntry](ctx, builder.Eq{"repo_id": repoID, "base_branch": baseBranch})
}
