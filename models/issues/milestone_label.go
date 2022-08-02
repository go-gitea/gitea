// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"context"
	"fmt"
	"sort"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
)

// MilestoneLabel represents an milestone-label relation.
type MilestoneLabel struct {
	ID          int64 `xorm:"pk autoincr"`
	MilestoneID int64 `xorm:"UNIQUE(milestoneid_labelid)"`
	LabelID     int64 `xorm:"UNIQUE(milestoneid_labelid)"`
}

func init() {
	db.RegisterModel(new(MilestoneLabel))
}

func hasMilestoneLabel(e db.Engine, milestoneID, labelID int64) bool {
	has, _ := e.Where("milestone_id = ? AND label_id = ?", milestoneID, labelID).Get(new(MilestoneLabel))
	return has
}

// HasMilestoneLabel returns true if milestone has been labeled.
func HasMilestoneLabel(milestoneID, labelID int64) bool {
	return hasMilestoneLabel(db.GetEngine(db.DefaultContext), milestoneID, labelID)
}

func getLabelsByMilestoneID(e db.Engine, milestoneID int64) ([]*Label, error) {
	var labels []*Label
	return labels, e.Where("milestone_label.milestone_id = ?", milestoneID).
		Join("LEFT", "milestone_label", "milestone_label.label_id = label.id").
		Asc("label.name").
		Find(&labels)
}

// GetLabelsByMilestoneID returns all labels that belong to given milestone by ID.
func GetLabelsByMilestoneID(milestoneID int64) ([]*Label, error) {
	return getLabelsByMilestoneID(db.GetEngine(db.DefaultContext), milestoneID)
}

// newMilestoneLabel this function creates a new label it does not check if the label is valid for the milestone
// YOU MUST CHECK THIS BEFORE THIS FUNCTION
func newMilestoneLabel(ctx context.Context, milestone *Milestone, label *Label, doer *user_model.User) (err error) {
	if err = db.Insert(ctx, &MilestoneLabel{
		MilestoneID: milestone.ID,
		LabelID:     label.ID,
	}); err != nil {
		return err
	}

	return updateLabelCols(ctx, label, "num_issues", "num_closed_issues", "num_milestones")
}

// NewMilestoneLabel creates a new milestone-label relation.
func NewMilestoneLabel(milestone *Milestone, label *Label, doer *user_model.User) (err error) {
	if HasMilestoneLabel(milestone.ID, label.ID) {
		return nil
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	// Do NOT add invalid labels
	if milestone.RepoID != label.RepoID && milestone.Repo.OwnerID != label.OrgID {
		return nil
	}

	if err = newMilestoneLabel(ctx, milestone, label, doer); err != nil {
		return err
	}

	milestone.Labels = nil
	if err = milestone.loadLabels(sess); err != nil {
		return err
	}

	return committer.Commit()
}

// newMilestoneLabels add labels to an milestone. It will check if the labels are valid for the milestone
func newMilestoneLabels(ctx context.Context, e db.Engine, milestone *Milestone, labels []*Label, doer *user_model.User) (err error) {
	for _, label := range labels {
		// Don't add already present labels and invalid labels
		if hasMilestoneLabel(e, milestone.ID, label.ID) ||
			(label.RepoID != milestone.RepoID && label.OrgID != milestone.Repo.OwnerID) {
			continue
		}

		if err = newMilestoneLabel(ctx, milestone, label, doer); err != nil {
			return fmt.Errorf("newMilestoneLabel: %v", err)
		}
	}

	return nil
}

// NewMilestoneLabels creates a list of milestone-label relations.
func NewMilestoneLabels(milestone *Milestone, labels []*Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = newMilestoneLabels(ctx, db.GetEngine(ctx), milestone, labels, doer); err != nil {
		return err
	}

	milestone.Labels = nil
	if err = milestone.loadLabels(db.GetEngine(ctx)); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteMilestoneLabel(ctx context.Context, milestone *Milestone, label *Label, doer *user_model.User) (err error) {
	// if count, err := db.DeleteByBean(ctx, &MilestoneLabel{
	if count, err := db.DeleteByBean(ctx, &MilestoneLabel{
		MilestoneID: milestone.ID,
		LabelID:     label.ID,
	}); err != nil {
		return err
	} else if count == 0 {
		return nil
	}
	return updateLabelCols(ctx, label, "num_issues", "num_closed_issues", "num_milestones")
}

// DeleteMilestoneLabel deletes milestone-label relation.
func DeleteMilestoneLabel(milestone *Milestone, label *Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = deleteMilestoneLabel(ctx, milestone, label, doer); err != nil {
		return err
	}

	return committer.Commit()
}

// LoadLabels loads labels
func (milestone *Milestone) LoadLabels() error {
	return milestone.loadLabels(db.GetEngine(db.DefaultContext))
}

func (milestone *Milestone) loadLabels(e db.Engine) (err error) {
	if milestone.Labels == nil {
		milestone.Labels, err = getLabelsByMilestoneID(e, milestone.ID)
		if err != nil {
			return fmt.Errorf("getLabelsByMilestoneID [%d]: %v", milestone.ID, err)
		}
	}
	return nil
}

func (milestone *Milestone) hasLabel(e db.Engine, labelID int64) bool {
	return hasMilestoneLabel(e, milestone.ID, labelID)
}

// HasLabel returns true if milestone has been labeled by given ID.
func (milestone *Milestone) HasLabel(labelID int64) bool {
	return milestone.hasLabel(db.GetEngine(db.DefaultContext), labelID)
}

func (milestone *Milestone) addLabel(ctx context.Context, label *Label, doer *user_model.User) error {
	return newMilestoneLabel(ctx, milestone, label, doer)
}

// AddLabels adds a list of new labels to the milestone.
func (milestone *Milestone) AddLabels(doer *user_model.User, labels []*Label) error {
	return NewMilestoneLabels(milestone, labels, doer)
}

func (milestone *Milestone) addLabels(ctx context.Context, e db.Engine, labels []*Label, doer *user_model.User) error {
	return newMilestoneLabels(ctx, e, milestone, labels, doer)
}

func (milestone *Milestone) removeLabel(ctx context.Context, doer *user_model.User, label *Label) error {
	return deleteMilestoneLabel(ctx, milestone, label, doer)
}

func (milestone *Milestone) clearLabels(ctx context.Context, e db.Engine, doer *user_model.User) (err error) {
	if err = milestone.loadLabels(e); err != nil {
		return fmt.Errorf("getLabels: %v", err)
	}

	for i := range milestone.Labels {
		if err = milestone.removeLabel(ctx, doer, milestone.Labels[i]); err != nil {
			return fmt.Errorf("removeLabel: %v", err)
		}
	}

	return nil
}

// ClearLabels removes all milestone labels as the given user.
// Triggers appropriate WebHooks, if any.
func (milestone *Milestone) ClearLabels(doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	perm, err := access_model.GetUserRepoPermission(ctx, milestone.Repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWriteIssuesOrPulls(true) {
		return ErrRepoLabelNotExist{}
	}

	if err = milestone.clearLabels(ctx, db.GetEngine(ctx), doer); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	return nil
}

// ReplaceLabels removes all current labels and adds the given labels to the milestone.
// Triggers appropriate WebHooks, if any.
func (milestone *Milestone) ReplaceLabels(labels []*Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = milestone.loadLabels(db.GetEngine(ctx)); err != nil {
		return err
	}

	sort.Sort(labelSorter(labels))
	sort.Sort(labelSorter(milestone.Labels))

	var toAdd, toRemove []*Label

	addIndex, removeIndex := 0, 0
	for addIndex < len(labels) && removeIndex < len(milestone.Labels) {
		addLabel := labels[addIndex]
		removeLabel := milestone.Labels[removeIndex]
		if addLabel.ID == removeLabel.ID {
			// Silently drop invalid labels
			if removeLabel.RepoID != milestone.RepoID && removeLabel.OrgID != milestone.Repo.OwnerID {
				toRemove = append(toRemove, removeLabel)
			}

			addIndex++
			removeIndex++
		} else if addLabel.ID < removeLabel.ID {
			// Only add if the label is valid
			if addLabel.RepoID == milestone.RepoID || addLabel.OrgID == milestone.Repo.OwnerID {
				toAdd = append(toAdd, addLabel)
			}
			addIndex++
		} else {
			toRemove = append(toRemove, removeLabel)
			removeIndex++
		}
	}
	toAdd = append(toAdd, labels[addIndex:]...)
	toRemove = append(toRemove, milestone.Labels[removeIndex:]...)

	if len(toAdd) > 0 {
		if err = milestone.addLabels(ctx, db.GetEngine(ctx), toAdd, doer); err != nil {
			return fmt.Errorf("addLabels: %v", err)
		}
	}

	for _, l := range toRemove {
		if err = milestone.removeLabel(ctx, doer, l); err != nil {
			return fmt.Errorf("removeLabel: %v", err)
		}
	}

	milestone.Labels = nil
	if err = milestone.loadLabels(db.GetEngine(ctx)); err != nil {
		return err
	}

	return committer.Commit()
}
