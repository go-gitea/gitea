package group

import (
	"errors"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	group_service "code.gitea.io/gitea/services/group"
)

func toSearchRepoOptions(ctx *context.Context) *repo_model.SearchRepoOptions {
	page := ctx.FormInt("page")
	opts := &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		Actor:              ctx.Doer,
		Keyword:            ctx.FormTrim("q"),
		OwnerID:            ctx.FormInt64("uid"),
		PriorityOwnerID:    ctx.FormInt64("priority_owner_id"),
		TeamID:             ctx.FormInt64("team_id"),
		TopicOnly:          ctx.FormBool("topic"),
		Collaborate:        optional.None[bool](),
		Private:            ctx.IsSigned && (ctx.FormString("private") == "" || ctx.FormBool("private")),
		Template:           optional.None[bool](),
		StarredByID:        ctx.FormInt64("starredBy"),
		IncludeDescription: ctx.FormBool("includeDesc"),
	}
	if ctx.FormString("template") != "" {
		opts.Template = optional.Some(ctx.FormBool("template"))
	}

	if ctx.FormBool("exclusive") {
		opts.Collaborate = optional.Some(false)
	}

	mode := ctx.FormString("mode")
	switch mode {
	case "source":
		opts.Fork = optional.Some(false)
		opts.Mirror = optional.Some(false)
	case "fork":
		opts.Fork = optional.Some(true)
	case "mirror":
		opts.Mirror = optional.Some(true)
	case "collaborative":
		opts.Mirror = optional.Some(false)
		opts.Collaborate = optional.Some(true)
	case "":
	default:
		estr := fmt.Sprintf("Invalid search mode: \"%s\"", mode)
		ctx.Status(http.StatusUnprocessableEntity)
		ctx.ServerError("toSearchRepoOptions", errors.New(estr))
		return nil
	}

	if ctx.FormString("archived") != "" {
		opts.Archived = optional.Some(ctx.FormBool("archived"))
	}

	if ctx.FormString("is_private") != "" {
		opts.IsPrivate = optional.Some(ctx.FormBool("is_private"))
	}

	sortMode := ctx.FormString("sort")
	if len(sortMode) > 0 {
		sortOrder := ctx.FormString("order")
		if len(sortOrder) == 0 {
			sortOrder = "asc"
		}
		if searchModeMap, ok := repo_model.OrderByMap[sortOrder]; ok {
			if orderBy, ok := searchModeMap[sortMode]; ok {
				opts.OrderBy = orderBy
			} else {
				estr := fmt.Errorf("Invalid sort mode: \"%s\"", sortMode)
				ctx.Status(http.StatusUnprocessableEntity)
				ctx.ServerError("toSearchRepoOptions", estr)
				return nil
			}
		} else {
			estr := fmt.Errorf("Invalid sort order: \"%s\"", sortOrder)
			ctx.Status(http.StatusUnprocessableEntity)
			ctx.ServerError("toSearchRepoOptions", estr)
			return nil
		}
	}
	return opts
}

func SearchGroup(ctx *context.Context) {
	gid := ctx.FormInt64("group_id")
	var (
		group  *group_model.Group
		err    error
		canSee = true
		oid    = ctx.FormInt64("uid")
	)
	if gid > 0 {
		group, err = group_model.GetGroupByID(ctx, gid)
		if err != nil && !group_model.IsErrGroupNotExist(err) {
			ctx.ServerError("GetGroupByID", err)
			return
		}
	}
	if group != nil {
		canSee, err = group.CanAccess(ctx, ctx.Doer)
		if err != nil {
			ctx.ServerError("GroupCanAccess", err)
			return
		}
		oid = group.OwnerID
	}
	if !canSee {
		ctx.NotFound(nil)
		return
	}

	subgroupOpts := &group_model.FindGroupsOptions{
		ParentGroupID: gid,
		ActorID:       ctx.Doer.ID,
		OwnerID:       oid,
	}
	if gid == 0 {
		page := ctx.FormInt("page")
		if page <= 0 {
			page = 1
		}
		subgroupOpts.ListOptions = db.ListOptions{
			Page:     page,
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		}
	}
	sro := toSearchRepoOptions(ctx)
	rgw, err := group_service.SearchRepoGroupWeb(group, &group_service.WebSearchOptions{
		OrgID:     oid,
		GroupOpts: subgroupOpts,
		RepoOpts:  *sro,
		Actor:     ctx.Doer,
		Recurse:   ctx.FormBool("recurse"),
		Ctx:       ctx,
		Locale:    ctx.Locale,
	})
	if err != nil {
		ctx.ServerError("SearchRepoGroupWeb", err)
		return
	}
	ctx.JSON(http.StatusOK, rgw)
}
