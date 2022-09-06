// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"

	"lab.forgefriends.org/friendlyforgeformat/gof3/forges/common"
	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"

	"xorm.io/builder"
)

type Reaction struct {
	issues_model.Reaction
}

func ReactionConverter(f *issues_model.Reaction) *Reaction {
	return &Reaction{
		Reaction: *f,
	}
}

func (o Reaction) GetID() int64 {
	return o.ID
}

func (o *Reaction) SetID(id int64) {
	o.ID = id
}

func (o *Reaction) IsNil() bool {
	return o.ID == 0
}

func (o *Reaction) Equals(other *Reaction) bool {
	return o.UserID == other.UserID && o.Type == other.Type
}

func (o *Reaction) ToFormat() *format.Reaction {
	return &format.Reaction{
		Common:   format.Common{Index: o.ID},
		UserID:   o.User.ID,
		UserName: o.User.Name,
		Content:  o.Type,
	}
}

func (o *Reaction) FromFormat(reaction *format.Reaction) {
	*o = Reaction{
		Reaction: issues_model.Reaction{
			ID:     reaction.GetID(),
			UserID: reaction.UserID,
			User: &user_model.User{
				ID:   reaction.UserID,
				Name: reaction.UserName,
			},
			Type: reaction.Content,
		},
	}
}

type ReactionProvider struct {
	g *Gitea
}

func (o *ReactionProvider) ToFormat(reaction *Reaction) *format.Reaction {
	return reaction.ToFormat()
}

func (o *ReactionProvider) FromFormat(m *format.Reaction) *Reaction {
	var reaction Reaction
	reaction.FromFormat(m)
	return &reaction
}

//
// Although it would be possible to use a higher level logic instead of the database,
// as of September 2022 (1.18 dev)
// (i) models/issues/reaction.go imposes a significant overhead
// (ii) is fragile and bugous https://github.com/go-gitea/gitea/issues/20860
//

func (o *ReactionProvider) GetObjects(user *User, project *Project, parents []common.ContainerObjectInterface, page int) []*Reaction {
	cond := builder.NewCond()
	switch l := parents[len(parents)-1].(type) {
	case *Issue:
		cond = cond.And(builder.Eq{"reaction.issue_id": l.ID})
		cond = cond.And(builder.Eq{"reaction.comment_id": 0})
	case *Comment:
		cond = cond.And(builder.Eq{"reaction.comment_id": l.ID})
	default:
		panic(fmt.Errorf("unexpected type %T", parents[len(parents)-1]))
	}
	sess := db.GetEngine(o.g.ctx).Where(cond)
	if page > 0 {
		sess = db.SetSessionPagination(sess, &db.ListOptions{Page: page, PageSize: o.g.perPage})
	}
	reactions := make([]*issues_model.Reaction, 0, 10)
	if err := sess.Find(&reactions); err != nil {
		panic(err)
	}
	_, err := (issues_model.ReactionList)(reactions).LoadUsers(o.g.ctx, nil)
	if err != nil {
		panic(err)
	}
	return util.ConvertMap[*issues_model.Reaction, *Reaction](reactions, ReactionConverter)
}

func (o *ReactionProvider) ProcessObject(user *User, project *Project, parents []common.ContainerObjectInterface, reaction *Reaction) {
}

func (o *ReactionProvider) Get(user *User, project *Project, parents []common.ContainerObjectInterface, exemplar *Reaction) *Reaction {
	reaction := &Reaction{}
	has, err := db.GetEngine(o.g.ctx).ID(exemplar.GetID()).Get(&reaction.Reaction)
	if err != nil {
		panic(err)
	} else if !has {
		return &Reaction{}
	}
	if _, err := (issues_model.ReactionList{&reaction.Reaction}).LoadUsers(o.g.ctx, nil); err != nil {
		panic(err)
	}
	return reaction
}

func (o *ReactionProvider) Put(user *User, project *Project, parents []common.ContainerObjectInterface, reaction *Reaction) *Reaction {
	r := &issues_model.Reaction{
		Type:   reaction.Type,
		UserID: o.g.GetDoer().ID,
	}
	switch l := parents[len(parents)-1].(type) {
	case *Issue:
		r.IssueID = l.ID
		r.CommentID = 0
	case *Comment:
		i, ok := parents[len(parents)-2].(*Issue)
		if !ok {
			panic(fmt.Errorf("unexpected type %T", parents[len(parents)-2]))
		}
		r.IssueID = i.ID
		r.CommentID = l.ID
	default:
		panic(fmt.Errorf("unexpected type %T", parents[len(parents)-1]))
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		panic(err)
	}
	defer committer.Close()

	if _, err := db.GetEngine(ctx).Insert(r); err != nil {
		panic(err)
	}

	if err := committer.Commit(); err != nil {
		panic(err)
	}
	return ReactionConverter(r)
}

func (o *ReactionProvider) Delete(user *User, project *Project, parents []common.ContainerObjectInterface, reaction *Reaction) *Reaction {
	r := o.Get(user, project, parents, reaction)
	if !r.IsNil() {
		if _, err := db.GetEngine(o.g.ctx).Delete(&reaction.Reaction); err != nil {
			panic(err)
		}
		return reaction
	}
	return r
}
