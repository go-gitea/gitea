// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"sort"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type IssuePin struct {
	ID       int64 `xorm:"pk autoincr"`
	RepoID   int64 `xorm:"UNIQUE(s) NOT NULL"`
	IssueID  int64 `xorm:"UNIQUE(s) NOT NULL"`
	IsPull   bool  `xorm:"NOT NULL"`
	PinOrder int   `xorm:"DEFAULT 0"`
}

var ErrIssueMaxPinReached = util.NewInvalidArgumentErrorf("the max number of pinned issues has been readched")

// IsErrIssueMaxPinReached returns if the error is, that the User can't pin more Issues
func IsErrIssueMaxPinReached(err error) bool {
	return err == ErrIssueMaxPinReached
}

func init() {
	db.RegisterModel(new(IssuePin))
}

func GetIssuePin(ctx context.Context, issue *Issue) (*IssuePin, error) {
	pin := new(IssuePin)
	has, err := db.GetEngine(ctx).
		Where("repo_id = ?", issue.RepoID).
		And("issue_id = ?", issue.ID).Get(pin)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, db.ErrNotExist{
			Resource: "IssuePin",
			ID:       issue.ID,
		}
	}
	return pin, nil
}

func GetIssuePinsByIssueIDs(ctx context.Context, issueIDs []int64) ([]IssuePin, error) {
	var pins []IssuePin
	if err := db.GetEngine(ctx).In("issue_id", issueIDs).Find(&pins); err != nil {
		return nil, err
	}
	return pins, nil
}

// Pin pins a Issue
func PinIssue(ctx context.Context, issue *Issue, user *user_model.User) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		pinnedIssuesNum, err := getPinnedIssuesNum(ctx, issue.RepoID, issue.IsPull)
		if err != nil {
			return err
		}

		// Check if the maximum allowed Pins reached
		if pinnedIssuesNum >= setting.Repository.Issue.MaxPinned {
			return ErrIssueMaxPinReached
		}

		pinnedIssuesMaxPinOrder, err := getPinnedIssuesMaxPinOrder(ctx, issue.RepoID, issue.IsPull)
		if err != nil {
			return err
		}

		if _, err = db.GetEngine(ctx).Insert(&IssuePin{
			RepoID:   issue.RepoID,
			IssueID:  issue.ID,
			IsPull:   issue.IsPull,
			PinOrder: pinnedIssuesMaxPinOrder + 1,
		}); err != nil {
			return err
		}

		// Add the pin event to the history
		_, err = CreateComment(ctx, &CreateCommentOptions{
			Type:  CommentTypePin,
			Doer:  user,
			Repo:  issue.Repo,
			Issue: issue,
		})
		return err
	})
}

// UnpinIssue unpins a Issue
func UnpinIssue(ctx context.Context, issue *Issue, user *user_model.User) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		// This sets the Pin for all Issues that come after the unpined Issue to the correct value
		cnt, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Delete(new(IssuePin))
		if err != nil {
			return err
		}
		if cnt == 0 {
			return nil
		}

		// Add the unpin event to the history
		_, err = CreateComment(ctx, &CreateCommentOptions{
			Type:  CommentTypeUnpin,
			Doer:  user,
			Repo:  issue.Repo,
			Issue: issue,
		})
		return err
	})
}

func getPinnedIssuesNum(ctx context.Context, repoID int64, isPull bool) (int, error) {
	var pinnedIssuesNum int
	_, err := db.GetEngine(ctx).SQL("SELECT count(pin_order) FROM issue_pin WHERE repo_id = ? AND is_pull = ?", repoID, isPull).Get(&pinnedIssuesNum)
	return pinnedIssuesNum, err
}

func getPinnedIssuesMaxPinOrder(ctx context.Context, repoID int64, isPull bool) (int, error) {
	var maxPinnedIssuesMaxPinOrder int
	_, err := db.GetEngine(ctx).SQL("SELECT max(pin_order) FROM issue_pin WHERE repo_id = ? AND is_pull = ?", repoID, isPull).Get(&maxPinnedIssuesMaxPinOrder)
	return maxPinnedIssuesMaxPinOrder, err
}

// MovePin moves a Pinned Issue to a new Position
func MovePin(ctx context.Context, issue *Issue, newPosition int) error {
	if newPosition < 1 {
		return errors.New("The Position can't be lower than 1")
	}

	issuePin, err := GetIssuePin(ctx, issue)
	if err != nil {
		return err
	}
	if issuePin.PinOrder == newPosition {
		return nil
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		if issuePin.PinOrder > newPosition { // move the issue to a lower position
			_, err = db.GetEngine(ctx).Exec("UPDATE issue_pin SET pin_order = pin_order + 1 WHERE repo_id = ? AND is_pull = ? AND pin_order >= ? AND pin_order < ?", issue.RepoID, issue.IsPull, newPosition, issuePin.PinOrder)
		} else { // move the issue to a higher position
			// Lower the Position of all Pinned Issue that came after the current Position
			_, err = db.GetEngine(ctx).Exec("UPDATE issue_pin SET pin_order = pin_order - 1 WHERE repo_id = ? AND is_pull = ? AND pin_order > ? AND pin_order <= ?", issue.RepoID, issue.IsPull, issuePin.PinOrder, newPosition)
		}
		if err != nil {
			return err
		}

		_, err = db.GetEngine(ctx).
			Table("issue_pin").
			Where("id = ?", issuePin.ID).
			Update(map[string]any{
				"pin_order": newPosition,
			})
		return err
	})
}

func GetPinnedIssueIDs(ctx context.Context, repoID int64, isPull bool) ([]int64, error) {
	var issuePins []IssuePin
	if err := db.GetEngine(ctx).
		Table("issue_pin").
		Where("repo_id = ?", repoID).
		And("is_pull = ?", isPull).
		Find(&issuePins); err != nil {
		return nil, err
	}

	sort.Slice(issuePins, func(i, j int) bool {
		return issuePins[i].PinOrder < issuePins[j].PinOrder
	})

	var ids []int64
	for _, pin := range issuePins {
		ids = append(ids, pin.IssueID)
	}
	return ids, nil
}

func GetIssuePinsByRepoID(ctx context.Context, repoID int64, isPull bool) ([]*IssuePin, error) {
	var pins []*IssuePin
	if err := db.GetEngine(ctx).Where("repo_id = ? AND is_pull = ?", repoID, isPull).Find(&pins); err != nil {
		return nil, err
	}
	return pins, nil
}

// GetPinnedIssues returns the pinned Issues for the given Repo and type
func GetPinnedIssues(ctx context.Context, repoID int64, isPull bool) (IssueList, error) {
	issuePins, err := GetIssuePinsByRepoID(ctx, repoID, isPull)
	if err != nil {
		return nil, err
	}
	if len(issuePins) == 0 {
		return IssueList{}, nil
	}
	ids := make([]int64, 0, len(issuePins))
	for _, pin := range issuePins {
		ids = append(ids, pin.IssueID)
	}

	issues := make(IssueList, 0, len(ids))
	if err := db.GetEngine(ctx).In("id", ids).Find(&issues); err != nil {
		return nil, err
	}
	for _, issue := range issues {
		for _, pin := range issuePins {
			if pin.IssueID == issue.ID {
				issue.PinOrder = pin.PinOrder
				break
			}
		}
		if (!setting.IsProd || setting.IsInTesting) && issue.PinOrder == 0 {
			panic("It should not happen that a pinned Issue has no PinOrder")
		}
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].PinOrder < issues[j].PinOrder
	})

	if err = issues.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	return issues, nil
}

// IsNewPinAllowed returns if a new Issue or Pull request can be pinned
func IsNewPinAllowed(ctx context.Context, repoID int64, isPull bool) (bool, error) {
	var maxPin int
	_, err := db.GetEngine(ctx).SQL("SELECT COUNT(pin_order) FROM issue_pin WHERE repo_id = ? AND is_pull = ?", repoID, isPull).Get(&maxPin)
	if err != nil {
		return false, err
	}

	return maxPin < setting.Repository.Issue.MaxPinned, nil
}
