// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"sort"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
)

type WorkflowBranch struct {
	Branch string
	Labels []string
	ErrMsg string
}

type ActionWorkflow struct {
	ID      int64 `xorm:"pk autoincr"`
	RepoID  int64
	Name    string
	Branchs []*WorkflowBranch `xorm:"JSON TEXT"`
	ErrMsg  string            `json:"-"` // for ui logic
}

func (w *ActionWorkflow) LoadLabels() []string {
	labelsSet := make(container.Set[string])

	for _, b := range w.Branchs {
		for _, l := range b.Labels {
			labelsSet.AddMultiple(l)
		}
	}

	labels := labelsSet.Values()
	sort.Slice(labels, func(i, j int) bool {
		return labels[i] < labels[j]
	})

	return labels
}

func (w *ActionWorkflow) UpdateBranchLabels(branch string, labels []string) {
	if w.Branchs == nil {
		w.Branchs = make([]*WorkflowBranch, 0, 5)
	}

	for _, b := range w.Branchs {
		if b.Branch == branch {
			b.Labels = labels
			return
		}
	}

	w.Branchs = append(w.Branchs, &WorkflowBranch{Branch: branch, Labels: labels})
}

func (w *ActionWorkflow) UpdateBranchErrMsg(branch string, errMsg error) {
	if w.Branchs == nil {
		w.Branchs = make([]*WorkflowBranch, 0, 5)
	}

	for _, b := range w.Branchs {
		if b.Branch == branch {
			if errMsg == nil {
				b.ErrMsg = ""
			} else {
				b.ErrMsg = errMsg.Error()
			}
			return
		}
	}

	w.Branchs = append(w.Branchs, &WorkflowBranch{Branch: branch, ErrMsg: errMsg.Error()})
}

func (w *ActionWorkflow) DeleteBranch(branch string) {
	for index, b := range w.Branchs {
		if b.Branch == branch {
			w.Branchs = append(w.Branchs[:index], w.Branchs[index+1:]...)
			break
		}
	}
}

func (w *ActionWorkflow) BranchNum() int {
	return len(w.Branchs)
}

func UpdateWorkFlowErrMsg(ctx context.Context, repoID int64, workflow, branch string, err error) error {
	return updateWorkFlowLabelsOrErrmsg(ctx, repoID, workflow, branch, nil, err)
}

func UpdateWorkFlowLabels(ctx context.Context, repoID int64, workflow, branch string, labels []string) error {
	return updateWorkFlowLabelsOrErrmsg(ctx, repoID, workflow, branch, labels, nil)
}

func updateWorkFlowLabelsOrErrmsg(ctx context.Context, repoID int64, workflow, branch string, labels []string, errMsg error) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	w := &ActionWorkflow{
		RepoID: repoID,
		Name:   workflow,
	}
	has, err := db.GetEngine(ctx).Get(w)
	if err != nil {
		return err
	}

	if len(labels) > 0 {
		w.UpdateBranchLabels(branch, labels)
	}

	w.UpdateBranchErrMsg(branch, errMsg)

	if has {
		_, err = db.GetEngine(ctx).Cols("branchs").ID(w.ID).Update(w)
	} else {
		_, err = db.GetEngine(ctx).Insert(w)
	}
	if err != nil {
		return err
	}

	return committer.Commit()
}

func DeleteWorkFlowBranch(ctx context.Context, repoID int64, workflow, branch string) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	w := &ActionWorkflow{
		RepoID: repoID,
		Name:   workflow,
	}
	has, err := db.GetEngine(ctx).Get(w)
	if err != nil {
		return err
	}
	if !has {
		return nil
	}

	w.DeleteBranch(branch)

	if w.BranchNum() == 0 {
		_, err = db.GetEngine(ctx).ID(w.ID).Delete(w)
	} else {
		_, err = db.GetEngine(ctx).Cols("branchs").ID(w.ID).Update(w)
	}
	if err != nil {
		return err
	}

	return committer.Commit()
}

func ListWorkflowBranchs(ctx context.Context, repoID int64) ([]*ActionWorkflow, error) {
	result := make([]*ActionWorkflow, 0, 10)

	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Find(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
