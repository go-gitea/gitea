// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"
	"time"

	project_model "gitea.dev/models/project"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
)

func projectTemplateTypeToString(t project_model.TemplateType) string {
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

func projectCardTypeToString(t project_model.CardType) string {
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

func projectTypeToString(t project_model.Type) string {
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

// loadProjectCreators batch-fetches the creators of the given projects and columns, keyed by
// user ID. Enrichment is best-effort: on a lookup failure, or for creators that no longer
// exist, the creator field stays nil rather than failing the whole conversion.
func loadProjectCreators(ctx context.Context, projects []*project_model.Project, columns []*project_model.Column) map[int64]*user_model.User {
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
		return nil
	}
	creators, err := user_model.GetUsersMapByIDs(ctx, idSet.Values())
	if err != nil {
		log.Error("GetUsersMapByIDs: %v", err)
		return nil
	}
	return creators
}

// timeStampPtr returns nil for the zero timestamp, so a missing timestamp is not
// reported to API clients as the unix epoch.
func timeStampPtr(ts timeutil.TimeStamp) *time.Time {
	if ts == 0 {
		return nil
	}
	return ts.AsTimePtr()
}

// ToProject converts a project_model.Project to api.Project.
// Caller is expected to preload p.Repo / p.Owner to avoid N+1 lookups.
func ToProject(ctx context.Context, p *project_model.Project, doer *user_model.User) *api.Project {
	creators := loadProjectCreators(ctx, []*project_model.Project{p}, nil)
	return toProject(ctx, p, doer, creators)
}

func toProject(ctx context.Context, p *project_model.Project, doer *user_model.User, creators map[int64]*user_model.User) *api.Project {
	state, closedAt := api.StateOpen, (*time.Time)(nil)
	if p.IsClosed {
		// changeProjectStatus stamps ClosedDateUnix on reopen too, so it only means
		// anything while the project is closed
		state, closedAt = api.StateClosed, timeStampPtr(p.ClosedDateUnix)
	}

	project := &api.Project{
		ID:              p.ID,
		Title:           p.Title,
		Description:     p.Description,
		OwnerID:         p.OwnerID,
		RepoID:          p.RepoID,
		CreatorID:       p.CreatorID, //nolint:staticcheck // deprecated but useful to API response
		State:           state,
		IsClosed:        p.IsClosed, //nolint:staticcheck // deprecated but useful to API response
		TemplateType:    projectTemplateTypeToString(p.TemplateType),
		CardType:        projectCardTypeToString(p.CardType),
		Type:            projectTypeToString(p.Type),
		NumOpenIssues:   p.NumOpenIssues,
		NumClosedIssues: p.NumClosedIssues,
		NumIssues:       p.NumIssues,
		CreatedAt:       p.CreatedUnix.AsTime(),
		UpdatedAt:       timeStampPtr(p.UpdatedUnix),
		ClosedAt:        closedAt,
	}

	if creator, ok := creators[p.CreatorID]; ok {
		project.Creator = ToUser(ctx, creator, doer)
	}

	// the caller preloads Repo/Owner, so Link stays free of lazy lookups
	if link := p.Link(ctx); link != "" {
		project.HTMLURL = httplib.MakeAbsoluteURL(ctx, link)
	}

	return project
}

func ToProjectColumn(ctx context.Context, column *project_model.Column, doer *user_model.User) *api.ProjectColumn {
	creators := loadProjectCreators(ctx, nil, []*project_model.Column{column})
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
		CreatedAt: column.CreatedUnix.AsTime(),
		UpdatedAt: timeStampPtr(column.UpdatedUnix),
	}
	if creator, ok := creators[column.CreatorID]; ok {
		apiColumn.Creator = ToUser(ctx, creator, doer)
	}
	return apiColumn
}

func ToProjectList(ctx context.Context, projects []*project_model.Project, doer *user_model.User) []*api.Project {
	creators := loadProjectCreators(ctx, projects, nil)
	result := make([]*api.Project, len(projects))
	for i, p := range projects {
		result[i] = toProject(ctx, p, doer, creators)
	}
	return result
}

func ToProjectColumnList(ctx context.Context, columns []*project_model.Column, doer *user_model.User) []*api.ProjectColumn {
	creators := loadProjectCreators(ctx, nil, columns)
	result := make([]*api.ProjectColumn, len(columns))
	for i, column := range columns {
		result[i] = toProjectColumn(ctx, column, doer, creators)
	}
	return result
}
