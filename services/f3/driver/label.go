// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"

	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type Label struct {
	issues_model.Label
}

func LabelConverter(f *issues_model.Label) *Label {
	return &Label{
		Label: *f,
	}
}

func (o Label) GetID() int64 {
	return o.ID
}

func (o Label) GetName() string {
	return o.Name
}

func (o *Label) SetID(id int64) {
	o.ID = id
}

func (o *Label) IsNil() bool {
	return o.ID == 0
}

func (o *Label) Equals(other *Label) bool {
	return o.Name == other.Name
}

func (o *Label) ToFormat() *format.Label {
	return &format.Label{
		Common:      format.Common{Index: o.ID},
		Name:        o.Name,
		Color:       o.Color,
		Description: o.Description,
	}
}

func (o *Label) FromFormat(label *format.Label) {
	*o = Label{
		Label: issues_model.Label{
			ID:          label.Index,
			Name:        label.Name,
			Description: label.Description,
			Color:       label.Color,
		},
	}
}

type LabelProvider struct {
	g       *Gitea
	project *ProjectProvider
}

func (o *LabelProvider) ToFormat(label *Label) *format.Label {
	return label.ToFormat()
}

func (o *LabelProvider) FromFormat(m *format.Label) *Label {
	var label Label
	label.FromFormat(m)
	return &label
}

func (o *LabelProvider) GetObjects(user *User, project *Project, page int) []*Label {
	labels, err := issues_model.GetLabelsByRepoID(o.g.ctx, project.GetID(), "", db.ListOptions{Page: page, PageSize: o.g.perPage})
	if err != nil {
		panic(fmt.Errorf("error while listing labels: %v", err))
	}

	r := util.ConvertMap[*issues_model.Label, *Label](labels, LabelConverter)
	if o.project != nil {
		o.project.labels = util.NewNameIDMap[*Label](r)
	}
	return r
}

func (o *LabelProvider) ProcessObject(user *User, project *Project, label *Label) {
}

func (o *LabelProvider) Get(user *User, project *Project, exemplar *Label) *Label {
	id := exemplar.GetID()
	label, err := issues_model.GetLabelInRepoByID(o.g.ctx, project.GetID(), id)
	if issues_model.IsErrRepoLabelNotExist(err) {
		return &Label{}
	}
	if err != nil {
		panic(err)
	}
	return LabelConverter(label)
}

func (o *LabelProvider) Put(user *User, project *Project, label *Label) *Label {
	l := label.Label
	l.RepoID = project.GetID()
	if err := issues_model.NewLabel(o.g.ctx, &l); err != nil {
		panic(err)
	}
	return o.Get(user, project, LabelConverter(&l))
}

func (o *LabelProvider) Delete(user *User, project *Project, label *Label) *Label {
	l := o.Get(user, project, label)
	if !l.IsNil() {
		if err := issues_model.DeleteLabel(project.GetID(), l.GetID()); err != nil {
			panic(err)
		}
	}
	return l
}
