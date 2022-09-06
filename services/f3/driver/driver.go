// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"fmt"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/migrations"

	"lab.forgefriends.org/friendlyforgeformat/gof3"
	"lab.forgefriends.org/friendlyforgeformat/gof3/forges/common"
	"lab.forgefriends.org/friendlyforgeformat/gof3/forges/driver"
	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
)

type Options struct {
	gof3.Options

	Doer *user_model.User
}

type Gitea struct {
	perPage int
	ctx     context.Context
	options *Options
}

func (o *Gitea) GetPerPage() int {
	return o.perPage
}

func (o *Gitea) GetOptions() gof3.OptionsInterface {
	return o.options
}

func (o *Gitea) SetOptions(options gof3.OptionsInterface) {
	var ok bool
	o.options, ok = options.(*Options)
	if !ok {
		panic(fmt.Errorf("unexpected type %T", options))
	}
}

func (o *Gitea) GetLogger() *gof3.Logger {
	return o.GetOptions().GetLogger()
}

func (o *Gitea) Init(options gof3.OptionsInterface) {
	o.SetOptions(options)
	o.perPage = setting.ItemsPerPage
}

func (o *Gitea) GetDirectory() string {
	return o.options.GetDirectory()
}

func (o *Gitea) GetDoer() *user_model.User {
	return o.options.Doer
}

func (o *Gitea) GetNewMigrationHTTPClient() gof3.NewMigrationHTTPClientFun {
	return migrations.NewMigrationHTTPClient
}

func (o *Gitea) SupportGetRepoComments() bool {
	return false
}

func (o *Gitea) SetContext(ctx context.Context) {
	o.ctx = ctx
}

func (o *Gitea) GetProvider(name string, parent common.ProviderInterface) common.ProviderInterface {
	var parentImpl any
	if parent != nil {
		parentImpl = parent.GetImplementation()
	}
	switch name {
	case driver.ProviderUser:
		return &driver.Provider[UserProvider, *UserProvider, User, *User, format.User, *format.User]{Impl: &UserProvider{g: o}}
	case driver.ProviderProject:
		return &driver.ProviderWithParentOne[ProjectProvider, *ProjectProvider, Project, *Project, format.Project, *format.Project, User, *User]{Impl: &ProjectProvider{g: o}}
	case driver.ProviderMilestone:
		return &driver.ProviderWithParentOneTwo[MilestoneProvider, *MilestoneProvider, Milestone, *Milestone, format.Milestone, *format.Milestone, User, *User, Project, *Project]{Impl: &MilestoneProvider{g: o, project: parentImpl.(*ProjectProvider)}}
	case driver.ProviderIssue:
		return &driver.ProviderWithParentOneTwo[IssueProvider, *IssueProvider, Issue, *Issue, format.Issue, *format.Issue, User, *User, Project, *Project]{Impl: &IssueProvider{g: o, project: parentImpl.(*ProjectProvider)}}
	case driver.ProviderPullRequest:
		return &driver.ProviderWithParentOneTwo[PullRequestProvider, *PullRequestProvider, PullRequest, *PullRequest, format.PullRequest, *format.PullRequest, User, *User, Project, *Project]{Impl: &PullRequestProvider{g: o, project: parentImpl.(*ProjectProvider)}}
	case driver.ProviderReview:
		return &driver.ProviderWithParentOneTwoThree[ReviewProvider, *ReviewProvider, Review, *Review, format.Review, *format.Review, User, *User, Project, *Project, PullRequest, *PullRequest]{Impl: &ReviewProvider{g: o}}
	case driver.ProviderRepository:
		return &driver.ProviderWithParentOneTwo[RepositoryProvider, *RepositoryProvider, Repository, *Repository, format.Repository, *format.Repository, User, *User, Project, *Project]{Impl: &RepositoryProvider{g: o}}
	case driver.ProviderTopic:
		return &driver.ProviderWithParentOneTwo[TopicProvider, *TopicProvider, Topic, *Topic, format.Topic, *format.Topic, User, *User, Project, *Project]{Impl: &TopicProvider{g: o}}
	case driver.ProviderLabel:
		return &driver.ProviderWithParentOneTwo[LabelProvider, *LabelProvider, Label, *Label, format.Label, *format.Label, User, *User, Project, *Project]{Impl: &LabelProvider{g: o, project: parentImpl.(*ProjectProvider)}}
	case driver.ProviderRelease:
		return &driver.ProviderWithParentOneTwo[ReleaseProvider, *ReleaseProvider, Release, *Release, format.Release, *format.Release, User, *User, Project, *Project]{Impl: &ReleaseProvider{g: o}}
	case driver.ProviderAsset:
		return &driver.ProviderWithParentOneTwoThree[AssetProvider, *AssetProvider, Asset, *Asset, format.ReleaseAsset, *format.ReleaseAsset, User, *User, Project, *Project, Release, *Release]{Impl: &AssetProvider{g: o}}
	case driver.ProviderComment:
		return &driver.ProviderWithParentOneTwoThreeInterface[CommentProvider, *CommentProvider, Comment, *Comment, format.Comment, *format.Comment, User, *User, Project, *Project]{Impl: &CommentProvider{g: o}}
	case driver.ProviderCommentReaction:
		return &driver.ProviderWithParentOneTwoRest[ReactionProvider, *ReactionProvider, Reaction, *Reaction, format.Reaction, *format.Reaction, User, *User, Project, *Project]{Impl: &ReactionProvider{g: o}}
	case driver.ProviderIssueReaction:
		return &driver.ProviderWithParentOneTwoRest[ReactionProvider, *ReactionProvider, Reaction, *Reaction, format.Reaction, *format.Reaction, User, *User, Project, *Project]{Impl: &ReactionProvider{g: o}}
	default:
		panic(fmt.Sprintf("unknown provider name %s", name))
	}
}

func (o Gitea) Finish() {
}
