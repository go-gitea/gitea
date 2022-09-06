// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type Milestone struct {
	issues_model.Milestone
}

func MilestoneConverter(f *issues_model.Milestone) *Milestone {
	return &Milestone{
		Milestone: *f,
	}
}

func (o Milestone) GetID() int64 {
	return o.ID
}

func (o Milestone) GetName() string {
	return o.Name
}

func (o *Milestone) SetID(id int64) {
	o.ID = id
}

func (o *Milestone) IsNil() bool {
	return o.ID == 0
}

func (o *Milestone) Equals(other *Milestone) bool {
	return o.Name == other.Name
}

func (o *Milestone) ToFormat() *format.Milestone {
	milestone := &format.Milestone{
		Common:      format.Common{Index: o.ID},
		Title:       o.Name,
		Description: o.Content,
		Created:     o.CreatedUnix.AsTime(),
		Updated:     o.UpdatedUnix.AsTimePtr(),
		State:       string(o.State()),
	}
	if o.IsClosed {
		milestone.Closed = o.ClosedDateUnix.AsTimePtr()
	}
	if o.DeadlineUnix.Year() < 9999 {
		milestone.Deadline = o.DeadlineUnix.AsTimePtr()
	}
	return milestone
}

func (o *Milestone) FromFormat(milestone *format.Milestone) {
	var deadline timeutil.TimeStamp
	if milestone.Deadline != nil {
		deadline = timeutil.TimeStamp(milestone.Deadline.Unix())
	}
	if deadline == 0 {
		deadline = timeutil.TimeStamp(time.Date(9999, 1, 1, 0, 0, 0, 0, setting.DefaultUILocation).Unix())
	}

	var closed timeutil.TimeStamp
	if milestone.Closed != nil {
		closed = timeutil.TimeStamp(milestone.Closed.Unix())
	}

	if milestone.Created.IsZero() {
		if milestone.Updated != nil {
			milestone.Created = *milestone.Updated
		} else if milestone.Deadline != nil {
			milestone.Created = *milestone.Deadline
		} else {
			milestone.Created = time.Now()
		}
	}
	if milestone.Updated == nil || milestone.Updated.IsZero() {
		milestone.Updated = &milestone.Created
	}

	*o = Milestone{
		issues_model.Milestone{
			ID:             milestone.Index,
			Name:           milestone.Title,
			Content:        milestone.Description,
			IsClosed:       milestone.State == "closed",
			CreatedUnix:    timeutil.TimeStamp(milestone.Created.Unix()),
			UpdatedUnix:    timeutil.TimeStamp(milestone.Updated.Unix()),
			ClosedDateUnix: closed,
			DeadlineUnix:   deadline,
		},
	}
}

type MilestoneProvider struct {
	g       *Gitea
	project *ProjectProvider
}

func (o *MilestoneProvider) ToFormat(milestone *Milestone) *format.Milestone {
	return milestone.ToFormat()
}

func (o *MilestoneProvider) FromFormat(m *format.Milestone) *Milestone {
	var milestone Milestone
	milestone.FromFormat(m)
	return &milestone
}

func (o *MilestoneProvider) GetObjects(user *User, project *Project, page int) []*Milestone {
	milestones, _, err := issues_model.GetMilestones(issues_model.GetMilestonesOption{
		ListOptions: db.ListOptions{Page: page, PageSize: o.g.perPage},
		RepoID:      project.GetID(),
		State:       api.StateAll,
	})
	if err != nil {
		panic(fmt.Errorf("error while listing milestones: %v", err))
	}

	r := util.ConvertMap[*issues_model.Milestone, *Milestone](([]*issues_model.Milestone)(milestones), MilestoneConverter)
	if o.project != nil {
		o.project.milestones = util.NewNameIDMap[*Milestone](r)
	}
	return r
}

func (o *MilestoneProvider) ProcessObject(user *User, project *Project, milestone *Milestone) {
}

func (o *MilestoneProvider) Get(user *User, project *Project, exemplar *Milestone) *Milestone {
	id := exemplar.GetID()
	milestone, err := issues_model.GetMilestoneByRepoID(o.g.ctx, project.GetID(), id)
	if issues_model.IsErrMilestoneNotExist(err) {
		return &Milestone{}
	}
	if err != nil {
		panic(err)
	}
	return MilestoneConverter(milestone)
}

func (o *MilestoneProvider) Put(user *User, project *Project, milestone *Milestone) *Milestone {
	m := milestone.Milestone
	m.RepoID = project.GetID()
	if err := issues_model.NewMilestone(&m); err != nil {
		panic(err)
	}
	return o.Get(user, project, MilestoneConverter(&m))
}

func (o *MilestoneProvider) Delete(user *User, project *Project, milestone *Milestone) *Milestone {
	m := o.Get(user, project, milestone)
	if !m.IsNil() {
		if err := issues_model.DeleteMilestoneByRepoID(project.GetID(), m.GetID()); err != nil {
			panic(err)
		}
	}
	return m
}
