// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"sort"

	"code.gitea.io/gitea/models/db"
)

// MilestoneLabel represents an milestone-label relation.
type MilestoneLabel struct {
	ID          int64 `xorm:"pk autoincr"`
	MilestoneID int64 `xorm:"UNIQUE(s)"`
	LabelID     int64 `xorm:"UNIQUE(s)"`
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
func newMilestoneLabel(e db.Engine, milestone *Milestone, label *Label, doer *User) (err error) {
	if _, err = e.Insert(&MilestoneLabel{
		MilestoneID: milestone.ID,
		LabelID:     label.ID,
	}); err != nil {
		return err
	}

	return updateLabelCols(e, label, "num_issues", "num_closed_issue", "num_milestone")
}

// NewMilestoneLabel creates a new milestone-label relation.
func NewMilestoneLabel(milestone *Milestone, label *Label, doer *User) (err error) {
	if HasMilestoneLabel(milestone.ID, label.ID) {
		return nil
	}

	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	// Do NOT add invalid labels
	if milestone.RepoID != label.RepoID && milestone.Repo.OwnerID != label.OrgID {
		return nil
	}

	if err = newMilestoneLabel(sess, milestone, label, doer); err != nil {
		return err
	}

	milestone.Labels = nil
	if err = milestone.loadLabels(sess); err != nil {
		return err
	}

	return sess.Commit()
}

// newMilestoneLabels add labels to an milestone. It will check if the labels are valid for the milestone
func newMilestoneLabels(e db.Engine, milestone *Milestone, labels []*Label, doer *User) (err error) {
	for _, label := range labels {
		// Don't add already present labels and invalid labels
		if hasMilestoneLabel(e, milestone.ID, label.ID) ||
			(label.RepoID != milestone.RepoID && label.OrgID != milestone.Repo.OwnerID) {
			continue
		}

		if err = newMilestoneLabel(e, milestone, label, doer); err != nil {
			return fmt.Errorf("newMilestoneLabel: %v", err)
		}
	}

	return nil
}

// NewMilestoneLabels creates a list of milestone-label relations.
func NewMilestoneLabels(milestone *Milestone, labels []*Label, doer *User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = newMilestoneLabels(db.GetEngine(ctx), milestone, labels, doer); err != nil {
		return err
	}

	milestone.Labels = nil
	if err = milestone.loadLabels(db.GetEngine(ctx)); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteMilestoneLabel(e db.Engine, milestone *Milestone, label *Label, doer *User) (err error) {
	if count, err := e.Delete(&MilestoneLabel{
		MilestoneID: milestone.ID,
		LabelID:     label.ID,
	}); err != nil {
		return err
	} else if count == 0 {
		return nil
	}
	return updateLabelCols(e, label, "num_issues", "num_closed_issue", "num_milestones")
}

// DeleteMilestoneLabel deletes milestone-label relation.
func DeleteMilestoneLabel(milestone *Milestone, label *Label, doer *User) (err error) {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deleteMilestoneLabel(sess, milestone, label, doer); err != nil {
		return err
	}

	milestone.Labels = nil
	if err = milestone.loadLabels(sess); err != nil {
		return err
	}

	return sess.Commit()
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

func (milestone *Milestone) addLabel(e db.Engine, label *Label, doer *User) error {
	return newMilestoneLabel(e, milestone, label, doer)
}

// AddLabels adds a list of new labels to the milestone.
func (milestone *Milestone) AddLabels(doer *User, labels []*Label) error {
	return NewMilestoneLabels(milestone, labels, doer)
}

func (milestone *Milestone) addLabels(e db.Engine, labels []*Label, doer *User) error {
	return newMilestoneLabels(e, milestone, labels, doer)
}

func (milestone *Milestone) getLabels(e db.Engine) (err error) {
	if len(milestone.Labels) > 0 {
		return nil
	}

	milestone.Labels, err = getLabelsByMilestoneID(e, milestone.ID)
	if err != nil {
		return fmt.Errorf("getLabelsByMilestoneID: %v", err)
	}
	return nil
}

func (milestone *Milestone) removeLabel(e db.Engine, doer *User, label *Label) error {
	return deleteMilestoneLabel(e, milestone, label, doer)
}

func (milestone *Milestone) clearLabels(e db.Engine, doer *User) (err error) {
	if err = milestone.getLabels(e); err != nil {
		return fmt.Errorf("getLabels: %v", err)
	}

	for i := range milestone.Labels {
		if err = milestone.removeLabel(e, doer, milestone.Labels[i]); err != nil {
			return fmt.Errorf("removeLabel: %v", err)
		}
	}

	return nil
}

// ClearLabels removes all milestone labels as the given user.
// Triggers appropriate WebHooks, if any.
func (milestone *Milestone) ClearLabels(doer *User) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	perm, err := getUserRepoPermission(db.GetEngine(ctx), milestone.Repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWriteIssuesOrPulls(true) {
		return ErrRepoLabelNotExist{}
	}

	if err = milestone.clearLabels(db.GetEngine(ctx), doer); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	return nil
}

// ReplaceLabels removes all current labels and add new labels to the milestone.
// Triggers appropriate WebHooks, if any.
func (milestone *Milestone) ReplaceLabels(labels []*Label, doer *User) (err error) {
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
		if err = milestone.addLabels(db.GetEngine(ctx), toAdd, doer); err != nil {
			return fmt.Errorf("addLabels: %v", err)
		}
	}

	for _, l := range toRemove {
		if err = milestone.removeLabel(db.GetEngine(ctx), doer, l); err != nil {
			return fmt.Errorf("removeLabel: %v", err)
		}
	}

	milestone.Labels = nil
	if err = milestone.loadLabels(db.GetEngine(ctx)); err != nil {
		return err
	}

	return committer.Commit()
}
