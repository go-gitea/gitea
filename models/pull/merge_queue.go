// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type MergeQueueEntryState string

const (
	MergeQueueStateQueued         MergeQueueEntryState = "queued"
	MergeQueueStateAwaitingChecks MergeQueueEntryState = "awaiting_checks"
	MergeQueueStateMergeable      MergeQueueEntryState = "mergeable"
	MergeQueueStateUnmergeable    MergeQueueEntryState = "unmergeable"
)

type MergeQueueEntry struct {
	ID                int64                `xorm:"pk autoincr"`
	RepoID            int64                `xorm:"INDEX(branch) NOT NULL"`
	BaseBranch        string               `xorm:"INDEX(branch) VARCHAR(255) NOT NULL"`
	PullID            int64                `xorm:"UNIQUE NOT NULL"`
	EnqueuerID        int64                `xorm:"NOT NULL"`
	JumpUnix          timeutil.TimeStamp   `xorm:"NOT NULL DEFAULT 0"`
	State             MergeQueueEntryState `xorm:"VARCHAR(20) NOT NULL"`
	HeadSHA           string               `xorm:"VARCHAR(64) NOT NULL"`
	BaseSHA           string               `xorm:"VARCHAR(64)"`
	GroupHeadSHA      string               `xorm:"VARCHAR(64) INDEX"`
	GroupRef          string               `xorm:"VARCHAR(512)"`
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

func MergeQueueRefPrefix() string {
	return git.BranchPrefix + setting.Repository.PullRequest.MergeQueueBranchPrefix
}

func MergeQueueGroupRef(baseBranch string, prIndex int64, baseSHA string) string {
	return fmt.Sprintf("%s%s/pr-%d-%s", MergeQueueRefPrefix(), baseBranch, prIndex, baseSHA)
}

func IsMergeQueueRef(ctx context.Context, repoID int64, ref git.RefName) (bool, error) {
	if !strings.HasPrefix(ref.String(), MergeQueueRefPrefix()) {
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
	if exist, err := db.Exist[MergeQueueEntry](ctx, builder.Eq{"pull_id": entry.PullID}); err != nil {
		return err
	} else if exist {
		return ErrAlreadyInMergeQueue
	}
	entry.State = MergeQueueStateQueued
	return db.Insert(ctx, entry)
}

func GetMergeQueueEntryByPullID(ctx context.Context, pullID int64) (*MergeQueueEntry, error) {
	entry, _, err := db.Get[MergeQueueEntry](ctx, builder.Eq{"pull_id": pullID})
	return entry, err
}

func findMergeQueueEntries(query db.Session) ([]*MergeQueueEntry, error) {
	entries := []*MergeQueueEntry{}
	err := query.Find(&entries)
	return entries, err
}

func GetMergeQueueEntriesByGroupHeadSHA(ctx context.Context, repoID int64, sha string) ([]*MergeQueueEntry, error) {
	return findMergeQueueEntries(db.GetEngine(ctx).Where("repo_id = ? AND group_head_sha = ?", repoID, sha))
}

func mergeQueueForBranch(ctx context.Context, repoID int64, baseBranch string) db.Session {
	return db.GetEngine(ctx).Table(new(MergeQueueEntry)).Where("repo_id = ? AND base_branch = ?", repoID, baseBranch)
}

func GetMergeQueuePosition(ctx context.Context, entry *MergeQueueEntry) (int, error) {
	count, err := mergeQueueForBranch(ctx, entry.RepoID, entry.BaseBranch).
		And("(jump_unix > ? OR (jump_unix = ? AND id < ?))", entry.JumpUnix, entry.JumpUnix, entry.ID).Count()
	return int(count) + 1, err
}

func GetMergeQueueEntries(ctx context.Context, repoID int64, baseBranch string) ([]*MergeQueueEntry, error) {
	return findMergeQueueEntries(mergeQueueForBranch(ctx, repoID, baseBranch).OrderBy("jump_unix DESC, id ASC"))
}

func GetMergeQueueBranches(ctx context.Context) ([]*MergeQueueEntry, error) {
	return findMergeQueueEntries(db.GetEngine(ctx).Select("repo_id, base_branch").GroupBy("repo_id, base_branch"))
}

func UpdateMergeQueueEntry(ctx context.Context, entry *MergeQueueEntry) error {
	_, err := db.GetEngine(ctx).ID(entry.ID).Cols("state", "base_sha", "group_head_sha", "group_ref", "checks_started_unix").Update(entry)
	return err
}

func NextJumpUnix(ctx context.Context, repoID int64, baseBranch string) (timeutil.TimeStamp, error) {
	var last MergeQueueEntry
	_, err := mergeQueueForBranch(ctx, repoID, baseBranch).Cols("jump_unix").Desc("jump_unix").Get(&last)
	return max(timeutil.TimeStampNow(), last.JumpUnix+1), err // Keep jumps in order within the same second.
}

func JumpMergeQueueEntry(ctx context.Context, entry *MergeQueueEntry) (err error) {
	if entry.JumpUnix, err = NextJumpUnix(ctx, entry.RepoID, entry.BaseBranch); err != nil {
		return err
	}
	_, err = db.GetEngine(ctx).ID(entry.ID).Cols("jump_unix").Update(entry)
	return err
}

func DeleteMergeQueueEntry(ctx context.Context, pullID int64) (bool, error) {
	deleted, err := db.GetEngine(ctx).Where("pull_id = ?", pullID).Delete(&MergeQueueEntry{})
	return deleted > 0, err
}

func CountMergeQueueEntries(ctx context.Context, repoID int64) (map[string]int, error) {
	var rows []struct {
		BaseBranch string
		Num        int
	}
	if err := db.GetEngine(ctx).Table(&MergeQueueEntry{}).Select("base_branch, COUNT(*) AS num").
		Where("repo_id = ?", repoID).GroupBy("base_branch").Find(&rows); err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.BaseBranch] = row.Num
	}
	return counts, nil
}

func GetQueuedPullIDs(ctx context.Context, pullIDs []int64) (container.Set[int64], error) {
	if len(pullIDs) == 0 {
		return container.Set[int64]{}, nil
	}
	ids, err := db.FindIDs(ctx, MergeQueueEntry{}.TableName(), "pull_id", builder.In("pull_id", pullIDs))
	return container.SetOf(ids...), err
}

func HasMergeQueueEntries(ctx context.Context, repoID int64, baseBranch string) (bool, error) {
	return mergeQueueForBranch(ctx, repoID, baseBranch).Exist()
}
