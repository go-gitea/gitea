// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"

	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	api "code.gitea.io/gitea/modules/structs"
)

func ProjectTemplateTypeToString(t project_model.TemplateType) string {
	switch t {
	case project_model.TemplateTypeBasicKanban:
		return "basic_kanban"
	case project_model.TemplateTypeBugTriage:
		return "bug_triage"
	default:
		return "none"
	}
}

func ProjectTemplateTypeFromString(s string) (project_model.TemplateType, error) {
	switch s {
	case "", "none":
		return project_model.TemplateTypeNone, nil
	case "basic_kanban":
		return project_model.TemplateTypeBasicKanban, nil
	case "bug_triage":
		return project_model.TemplateTypeBugTriage, nil
	default:
		return 0, fmt.Errorf("invalid template_type %q (expected none, basic_kanban, bug_triage)", s)
	}
}

func ProjectCardTypeToString(t project_model.CardType) string {
	switch t {
	case project_model.CardTypeImagesAndText:
		return "images_and_text"
	default:
		return "text_only"
	}
}

func ProjectCardTypeFromString(s string) (project_model.CardType, error) {
	switch s {
	case "", "text_only":
		return project_model.CardTypeTextOnly, nil
	case "images_and_text":
		return project_model.CardTypeImagesAndText, nil
	default:
		return 0, fmt.Errorf("invalid card_type %q (expected text_only, images_and_text)", s)
	}
}

func ProjectTypeToString(t project_model.Type) string {
	switch t {
	case project_model.TypeIndividual:
		return "individual"
	case project_model.TypeRepository:
		return "repository"
	case project_model.TypeOrganization:
		return "organization"
	default:
		return ""
	}
}

// loadProjectCreators batch-fetches creators for the given projects + columns and
// returns a map keyed by user ID. Errors are surfaced; missing users are silently
// skipped (their creator field stays nil), matching the convention of other list
// converters that tolerate deleted users.
func loadProjectCreators(ctx context.Context, projects []*project_model.Project, columns []*project_model.Column) (map[int64]*user_model.User, error) {
	idSet := container.Set[int64]{}
	for _, p := range projects {
		if p.CreatorID > 0 {
			idSet.Add(p.CreatorID)
		}
	}
	for _, c := range columns {
		if c.CreatorID > 0 {
			idSet.Add(c.CreatorID)
		}
	}
	if len(idSet) == 0 {
		return map[int64]*user_model.User{}, nil
	}
	return user_model.GetUsersMapByIDs(ctx, idSet.Values())
}

// ToProject converts a project_model.Project to api.Project.
// Caller is expected to preload p.Repo / p.Owner to avoid N+1 lookups.
func ToProject(ctx context.Context, p *project_model.Project, doer *user_model.User) *api.Project {
	creators, _ := loadProjectCreators(ctx, []*project_model.Project{p}, nil)
	return toProject(ctx, p, doer, creators)
}

func toProject(ctx context.Context, p *project_model.Project, doer *user_model.User, creators map[int64]*user_model.User) *api.Project {
	state := api.StateOpen
	if p.IsClosed {
		state = api.StateClosed
	}

	project := &api.Project{
		ID:              p.ID,
		Title:           p.Title,
		Description:     p.Description,
		OwnerID:         p.OwnerID,
		RepoID:          p.RepoID,
		State:           state,
		TemplateType:    ProjectTemplateTypeToString(p.TemplateType),
		CardType:        ProjectCardTypeToString(p.CardType),
		Type:            ProjectTypeToString(p.Type),
		NumOpenIssues:   p.NumOpenIssues,
		NumClosedIssues: p.NumClosedIssues,
		NumIssues:       p.NumIssues,
		CreatedAt:       p.CreatedUnix.AsTime(),
		UpdatedAt:       p.UpdatedUnix.AsTime(),
	}

	if p.ClosedDateUnix > 0 {
		t := p.ClosedDateUnix.AsTime()
		project.ClosedAt = &t
	}

	if creator, ok := creators[p.CreatorID]; ok {
		project.Creator = ToUser(ctx, creator, doer)
	}

	if p.Type == project_model.TypeRepository && p.Repo != nil {
		project.HTMLURL = p.Repo.HTMLURL() + fmt.Sprintf("/projects/%d", p.ID)
	} else if p.Owner != nil {
		project.HTMLURL = p.Owner.HTMLURL(ctx) + fmt.Sprintf("/-/projects/%d", p.ID)
	}

	return project
}

func ToProjectColumn(ctx context.Context, column *project_model.Column, doer *user_model.User) *api.ProjectColumn {
	creators, _ := loadProjectCreators(ctx, nil, []*project_model.Column{column})
	return toProjectColumn(ctx, column, doer, creators)
}

func toProjectColumn(ctx context.Context, column *project_model.Column, doer *user_model.User, creators map[int64]*user_model.User) *api.ProjectColumn {
	apiColumn := &api.ProjectColumn{
		ID:        column.ID,
		Title:     column.Title,
		Default:   column.Default,
		Sorting:   int(column.Sorting),
		Color:     column.Color,
		ProjectID: column.ProjectID,
		NumIssues: column.NumIssues,
		CreatedAt: column.CreatedUnix.AsTime(),
		UpdatedAt: column.UpdatedUnix.AsTime(),
	}
	if creator, ok := creators[column.CreatorID]; ok {
		apiColumn.Creator = ToUser(ctx, creator, doer)
	}
	return apiColumn
}

func ToProjectList(ctx context.Context, projects []*project_model.Project, doer *user_model.User) []*api.Project {
	creators, _ := loadProjectCreators(ctx, projects, nil)
	result := make([]*api.Project, len(projects))
	for i, p := range projects {
		result[i] = toProject(ctx, p, doer, creators)
	}
	return result
}

func ToProjectColumnList(ctx context.Context, columns []*project_model.Column, doer *user_model.User) []*api.ProjectColumn {
	creators, _ := loadProjectCreators(ctx, nil, columns)
	result := make([]*api.ProjectColumn, len(columns))
	for i, column := range columns {
		result[i] = toProjectColumn(ctx, column, doer, creators)
	}
	return result
}
