// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"sort"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
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

// HasMilestoneLabel returns true if milestone has been labeled.
func HasMilestoneLabel(ctx context.Context, milestoneID, labelID int64) (bool, error) {
	has, err := db.GetEngine(ctx).Where("milestone_id = ? AND label_id = ?", milestoneID, labelID).Exist(new(MilestoneLabel))
	if err != nil {
		return false, err
	}
	return has, nil
}

// GetLabelsByMilestoneID returns all labels that belong to given milestone by ID.
func GetLabelsByMilestoneID(ctx context.Context, milestoneID int64) ([]*Label, error) {
	var labels []*Label
	return labels, db.GetEngine(ctx).Where("milestone_label.milestone_id = ?", milestoneID).
		Join("LEFT", "milestone_label", "milestone_label.label_id = label.id").
		Asc("label.name").
		Find(&labels)
}

// newMilestoneLabel creates a new label, but it does NOT check if the label is valid for the specified milestone
// YOU MUST CHECK THIS BEFORE EXECUTING THIS FUNCTION
func newMilestoneLabel(ctx context.Context, m *Milestone, label *Label, doer *user_model.User) (err error) {
	if err = db.Insert(ctx, &MilestoneLabel{
		MilestoneID: m.ID,
		LabelID:     label.ID,
	}); err != nil {
		return err
	}

	return updateLabelCols(ctx, label, "num_issues", "num_closed_issues", "num_milestones")
}

// NewMilestoneLabel creates a new milestone-label relation.
func NewMilestoneLabel(m *Milestone, label *Label, doer *user_model.User) (err error) {
	hasLabel, err := HasMilestoneLabel(db.DefaultContext, m.ID, label.ID)
	if hasLabel {
		return nil
	}
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Do NOT add invalid labels
	if m.RepoID != label.RepoID && m.Repo.OwnerID != label.OrgID {
		return nil
	}

	if err = newMilestoneLabel(ctx, m, label, doer); err != nil {
		return err
	}

	m.Labels = nil
	if err = m.LoadLabels(ctx); err != nil {
		return err
	}

	return committer.Commit()
}

// newMilestoneLabels add labels to an milestone. It will check if the labels are valid for the milestone
func newMilestoneLabels(ctx context.Context, m *Milestone, labels []*Label, doer *user_model.User) (err error) {
	for _, label := range labels {
		hasLabel, err := HasMilestoneLabel(ctx, m.ID, label.ID)
		if err != nil {
			return err
		}
		// Don't add already present labels and invalid labels
		if hasLabel ||
			(label.RepoID != m.RepoID && label.OrgID != m.Repo.OwnerID) {
			continue
		}

		if err = newMilestoneLabel(ctx, m, label, doer); err != nil {
			return fmt.Errorf("newMilestoneLabel: %v", err)
		}
	}

	return nil
}

// NewMilestoneLabels creates a list of milestone-label relations.
func NewMilestoneLabels(m *Milestone, labels []*Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = newMilestoneLabels(ctx, m, labels, doer); err != nil {
		return err
	}

	m.Labels = nil
	if err = m.LoadLabels(ctx); err != nil {
		return err
	}

	return committer.Commit()
}

func deleteMilestoneLabel(ctx context.Context, m *Milestone, label *Label, doer *user_model.User) (err error) {
	if count, err := db.DeleteByBean(ctx, &MilestoneLabel{
		MilestoneID: m.ID,
		LabelID:     label.ID,
	}); err != nil {
		return err
	} else if count == 0 {
		return nil
	}
	return updateLabelCols(ctx, label, "num_issues", "num_closed_issues", "num_milestones")
}

// DeleteMilestoneLabel deletes milestone-label relation.
func DeleteMilestoneLabel(m *Milestone, label *Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = deleteMilestoneLabel(ctx, m, label, doer); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteMilestoneLabelsByRepoID deletes Milestone Labels
func DeleteMilestoneLabelsByRepoID(ctx context.Context, repoID int64) error {
	deleteCond := builder.Select("id").From("label").Where(builder.Eq{"label.repo_id": repoID})

	if _, err := db.GetEngine(ctx).In("label_id", deleteCond).
		Delete(&MilestoneLabel{}); err != nil {
		return err
	}
	return nil
}

// LoadLabels loads labels
func (m *Milestone) LoadLabels(ctx context.Context) (err error) {
	if m.Labels == nil {
		m.Labels, err = GetLabelsByMilestoneID(ctx, m.ID)
		if err != nil {
			return fmt.Errorf("GetLabelsByMilestoneID [%d]: %v", m.ID, err)
		}
	}
	return nil
}

func (m *Milestone) hasLabel(ctx context.Context, labelID int64) (bool, error) {
	hasLabel, err := HasMilestoneLabel(ctx, m.ID, labelID)
	if err != nil {
		return false, err
	}
	return hasLabel, nil
}

// HasLabel returns true if milestone has been labeled by given ID.
func (m *Milestone) HasLabel(labelID int64) (bool, error) {
	hasLabel, err := m.hasLabel(db.DefaultContext, labelID)
	return hasLabel, err
}

func (m *Milestone) addLabel(ctx context.Context, label *Label, doer *user_model.User) error {
	return newMilestoneLabel(ctx, m, label, doer)
}

// AddLabels adds a list of new labels to the milestone.
func (m *Milestone) AddLabels(doer *user_model.User, labels []*Label) error {
	return NewMilestoneLabels(m, labels, doer)
}

func (m *Milestone) addLabels(ctx context.Context, labels []*Label, doer *user_model.User) error {
	return newMilestoneLabels(ctx, m, labels, doer)
}

func (m *Milestone) removeLabel(ctx context.Context, doer *user_model.User, label *Label) error {
	return deleteMilestoneLabel(ctx, m, label, doer)
}

func (m *Milestone) clearLabels(ctx context.Context, e db.Engine, doer *user_model.User) (err error) {
	if err = m.LoadLabels(ctx); err != nil {
		return fmt.Errorf("getLabels: %v", err)
	}

	for i := range m.Labels {
		if err = m.removeLabel(ctx, doer, m.Labels[i]); err != nil {
			return fmt.Errorf("removeLabel: %v", err)
		}
	}

	return nil
}

// ClearLabels removes all milestone labels as the given user.
// Triggers appropriate WebHooks, if any.
func (m *Milestone) ClearLabels(doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	perm, err := access_model.GetUserRepoPermission(ctx, m.Repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWriteIssuesOrPulls(true) {
		return ErrRepoLabelNotExist{}
	}

	if err = m.clearLabels(ctx, db.GetEngine(ctx), doer); err != nil {
		return err
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	return nil
}

// ReplaceLabels removes all current labels and adds the given labels to the milestone.
// Triggers appropriate WebHooks, if any.
func (m *Milestone) ReplaceLabels(labels []*Label, doer *user_model.User) (err error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = m.LoadLabels(ctx); err != nil {
		return err
	}

	sort.Sort(labelSorter(labels))
	sort.Sort(labelSorter(m.Labels))

	var toAdd, toRemove []*Label

	addIndex, removeIndex := 0, 0
	for addIndex < len(labels) && removeIndex < len(m.Labels) {
		addLabel := labels[addIndex]
		removeLabel := m.Labels[removeIndex]
		if addLabel.ID == removeLabel.ID {
			// Silently drop invalid labels
			if removeLabel.RepoID != m.RepoID && removeLabel.OrgID != m.Repo.OwnerID {
				toRemove = append(toRemove, removeLabel)
			}

			addIndex++
			removeIndex++
		} else if addLabel.ID < removeLabel.ID {
			// Only add if the label is valid
			if addLabel.RepoID == m.RepoID || addLabel.OrgID == m.Repo.OwnerID {
				toAdd = append(toAdd, addLabel)
			}
			addIndex++
		} else {
			toRemove = append(toRemove, removeLabel)
			removeIndex++
		}
	}
	toAdd = append(toAdd, labels[addIndex:]...)
	toRemove = append(toRemove, m.Labels[removeIndex:]...)

	if len(toAdd) > 0 {
		if err = m.addLabels(ctx, toAdd, doer); err != nil {
			return fmt.Errorf("addLabels: %v", err)
		}
	}

	for _, l := range toRemove {
		if err = m.removeLabel(ctx, doer, l); err != nil {
			return fmt.Errorf("removeLabel: %v", err)
		}
	}

	m.Labels = nil
	if err = m.LoadLabels(ctx); err != nil {
		return err
	}

	return committer.Commit()
}
