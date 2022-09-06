// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"

	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type Topic struct {
	repo_model.Topic
}

func TopicConverter(f *repo_model.Topic) *Topic {
	return &Topic{
		Topic: *f,
	}
}

func (o Topic) GetID() int64 {
	return o.ID
}

func (o *Topic) SetID(id int64) {
	o.ID = id
}

func (o *Topic) IsNil() bool {
	return o.ID == 0
}

func (o *Topic) Equals(other *Topic) bool {
	return o.Name == other.Name
}

func (o *Topic) ToFormat() *format.Topic {
	return &format.Topic{
		Common: format.Common{Index: o.ID},
		Name:   o.Name,
	}
}

func (o *Topic) FromFormat(topic *format.Topic) {
	*o = Topic{
		Topic: repo_model.Topic{
			ID:   topic.Index,
			Name: topic.Name,
		},
	}
}

type TopicProvider struct {
	g *Gitea
}

func (o *TopicProvider) ToFormat(topic *Topic) *format.Topic {
	return topic.ToFormat()
}

func (o *TopicProvider) FromFormat(m *format.Topic) *Topic {
	var topic Topic
	topic.FromFormat(m)
	return &topic
}

func (o *TopicProvider) GetObjects(user *User, project *Project, page int) []*Topic {
	topics, _, err := repo_model.FindTopics(&repo_model.FindTopicOptions{
		ListOptions: db.ListOptions{Page: page, PageSize: o.g.perPage},
		RepoID:      project.GetID(),
	})
	if err != nil {
		panic(err)
	}

	return util.ConvertMap[*repo_model.Topic, *Topic](topics, TopicConverter)
}

func (o *TopicProvider) ProcessObject(user *User, project *Project, topic *Topic) {
}

func (o *TopicProvider) Get(user *User, project *Project, exemplar *Topic) *Topic {
	id := exemplar.GetID()
	topic, err := repo_model.GetRepoTopicByID(o.g.ctx, project.GetID(), id)
	if repo_model.IsErrTopicNotExist(err) {
		return &Topic{}
	}
	if err != nil {
		panic(err)
	}
	return TopicConverter(topic)
}

func (o *TopicProvider) Put(user *User, project *Project, topic *Topic) *Topic {
	t, err := repo_model.AddTopic(project.GetID(), topic.Name)
	if err != nil {
		panic(err)
	}
	return o.Get(user, project, TopicConverter(t))
}

func (o *TopicProvider) Delete(user *User, project *Project, topic *Topic) *Topic {
	t := o.Get(user, project, topic)
	if !t.IsNil() {
		t, err := repo_model.DeleteTopic(project.GetID(), t.Name)
		if err != nil {
			panic(err)
		}
		return TopicConverter(t)
	}
	return t
}
