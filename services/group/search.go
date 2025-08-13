package group

import (
	"context"
	"slices"

	"code.gitea.io/gitea/models/git"
	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/services/convert"
	repo_service "code.gitea.io/gitea/services/repository"
	commitstatus_service "code.gitea.io/gitea/services/repository/commitstatus"
)

type WebSearchGroup struct {
	Group                    *structs.Group                      `json:"group,omitempty"`
	LatestCommitStatus       *git.CommitStatus                   `json:"latest_commit_status"`
	LocaleLatestCommitStatus string                              `json:"locale_latest_commit_status"`
	Subgroups                []*WebSearchGroup                   `json:"subgroups"`
	Repos                    []*repo_service.WebSearchRepository `json:"repos"`
}

type WebSearchResult struct {
	OK   bool            `json:"ok"`
	Data *WebSearchGroup `json:"data"`
}

type WebSearchOptions struct {
	Ctx       context.Context
	Locale    translation.Locale
	Recurse   bool
	Actor     *user_model.User
	RepoOpts  repo_model.SearchRepoOptions
	GroupOpts *group_model.FindGroupsOptions
	OrgID     int64
}

// results for root-level queries //

type WebSearchGroupRoot struct {
	Groups []*WebSearchGroup
	Repos  []*repo_service.WebSearchRepository
}

type WebSearchGroupRootResult struct {
	OK   bool                `json:"ok"`
	Data *WebSearchGroupRoot `json:"data"`
}

func ToWebSearchRepo(ctx context.Context, repo *repo_model.Repository) *repo_service.WebSearchRepository {
	return &repo_service.WebSearchRepository{
		Repository: &structs.Repository{
			ID:             repo.ID,
			FullName:       repo.FullName(),
			Fork:           repo.IsFork,
			Private:        repo.IsPrivate,
			Template:       repo.IsTemplate,
			Mirror:         repo.IsMirror,
			Stars:          repo.NumStars,
			HTMLURL:        repo.HTMLURL(ctx),
			Link:           repo.Link(),
			Internal:       !repo.IsPrivate && repo.Owner.Visibility == structs.VisibleTypePrivate,
			GroupSortOrder: repo.GroupSortOrder,
			GroupID:        repo.GroupID,
		},
	}
}

func (w *WebSearchGroup) doLoadChildren(opts *WebSearchOptions) error {
	opts.RepoOpts.OwnerID = opts.OrgID
	opts.RepoOpts.GroupID = 0
	opts.GroupOpts.OwnerID = opts.OrgID
	opts.GroupOpts.ParentGroupID = 0

	if w.Group != nil {
		opts.RepoOpts.GroupID = w.Group.ID
		opts.RepoOpts.ListAll = true
		opts.GroupOpts.ParentGroupID = w.Group.ID
		opts.GroupOpts.ListAll = true
	}
	repos, _, err := repo_model.SearchRepository(opts.Ctx, opts.RepoOpts)
	if err != nil {
		return err
	}
	slices.SortStableFunc(repos, func(a, b *repo_model.Repository) int {
		return a.GroupSortOrder - b.GroupSortOrder
	})
	latestCommitStatuses, err := commitstatus_service.FindReposLastestCommitStatuses(opts.Ctx, repos)
	if err != nil {
		log.Error("FindReposLastestCommitStatuses: %v", err)
		return err
	}
	latestIdx := -1
	for i, r := range repos {
		wsr := ToWebSearchRepo(opts.Ctx, r)
		if latestCommitStatuses[i] != nil {
			wsr.LatestCommitStatus = latestCommitStatuses[i]
			wsr.LocaleLatestCommitStatus = latestCommitStatuses[i].LocaleString(opts.Locale)
			if latestIdx > -1 {
				if latestCommitStatuses[i].UpdatedUnix.AsLocalTime().Unix() > int64(latestCommitStatuses[latestIdx].UpdatedUnix.AsLocalTime().Unix()) {
					latestIdx = i
				}
			} else {
				latestIdx = i
			}
		}
		w.Repos = append(w.Repos, wsr)
	}
	if w.Group != nil && latestIdx > -1 {
		w.LatestCommitStatus = latestCommitStatuses[latestIdx]
	}
	w.Subgroups = make([]*WebSearchGroup, 0)
	groups, err := group_model.FindGroupsByCond(opts.Ctx, opts.GroupOpts, group_model.AccessibleGroupCondition(opts.Actor, unit.TypeInvalid))
	if err != nil {
		return err
	}
	for _, g := range groups {
		toAppend, err := ToWebSearchGroup(g, opts)
		if err != nil {
			return err
		}
		w.Subgroups = append(w.Subgroups, toAppend)
	}

	if opts.Recurse {
		for _, sg := range w.Subgroups {
			err = sg.doLoadChildren(opts)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ToWebSearchGroup(group *group_model.Group, opts *WebSearchOptions) (*WebSearchGroup, error) {
	res := new(WebSearchGroup)

	res.Repos = make([]*repo_service.WebSearchRepository, 0)
	res.Subgroups = make([]*WebSearchGroup, 0)
	var err error
	if group != nil {
		if res.Group, err = convert.ToAPIGroup(opts.Ctx, group, opts.Actor); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func SearchRepoGroupWeb(group *group_model.Group, opts *WebSearchOptions) (*WebSearchResult, error) {
	var res *WebSearchGroup
	var err error
	res, err = ToWebSearchGroup(group, opts)
	if err != nil {
		return nil, err
	}
	err = res.doLoadChildren(opts)
	if err != nil {
		return nil, err
	}
	return &WebSearchResult{
		Data: res,
		OK:   true,
	}, nil
}
