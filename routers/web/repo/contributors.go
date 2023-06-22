package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplContributors base.TplName = "repo/contributors"
)

// Contributors render the page to show repository contributors graph
func Contributors(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.contributors")
	ctx.Data["PageIsContributors"] = true

	ctx.Data["ContributionType"] = ctx.Params("contribution_type")
	if ctx.Data["ContributionType"] == "" {
		ctx.Data["ContributionType"] = "commits"
	}
	ctx.PageData["contributionType"] = ctx.Data["ContributionType"]

	ctx.Data["ContributionTypeText"] = ctx.Tr("repo.contributors.contribution_type." + ctx.Data["ContributionType"].(string))

	ctx.PageData["repoLink"] = ctx.Repo.RepoLink

	ctx.HTML(http.StatusOK, tplContributors)
}
