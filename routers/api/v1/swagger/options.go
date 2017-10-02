// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	"code.gitea.io/gitea/modules/auth"
	api "code.gitea.io/sdk/gitea"
)

// not actually a response, just a hack to get go-swagger to include definitions
// of the various XYZOption structs

// swagger:response parameterBodies
type swaggerParameterBodies struct {
	AddCollaboratorOption api.AddCollaboratorOption

	CreateEmailOption api.CreateEmailOption
	DeleteEmailOption api.DeleteEmailOption

	CreateHookOption api.CreateHookOption
	EditHookOption   api.EditHookOption

	CreateIssueOption api.CreateIssueOption
	EditIssueOption   api.EditIssueOption

	CreateIssueCommentOption api.CreateIssueCommentOption
	EditIssueCommentOption   api.EditIssueCommentOption

	IssueLabelsOption api.IssueLabelsOption

	CreateKeyOption api.CreateKeyOption

	CreateLabelOption api.CreateLabelOption
	EditLabelOption   api.EditLabelOption

	MarkdownOption api.MarkdownOption

	CreateMilestoneOption api.CreateMilestoneOption
	EditMilestoneOption   api.EditMilestoneOption

	CreateOrgOption api.CreateOrgOption
	EditOrgOption   api.EditOrgOption

	CreatePullRequestOption api.CreatePullRequestOption
	EditPullRequestOption   api.EditPullRequestOption

	CreateReleaseOption api.CreateReleaseOption
	EditReleaseOption   api.EditReleaseOption

	CreateRepoOption api.CreateRepoOption
	CreateForkOption api.CreateForkOption

	CreateStatusOption api.CreateStatusOption

	CreateTeamOption api.CreateTeamOption
	EditTeamOption   api.EditTeamOption

	AddTimeOption api.AddTimeOption

	CreateUserOption api.CreateUserOption
	EditUserOption   api.EditUserOption

	MigrateRepoForm auth.MigrateRepoForm
}
