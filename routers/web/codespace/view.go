// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"
	"strings"

	"gitea.dev/models/organization"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

type openEndpointModalData struct {
	Label    string
	Access   string
	OpenPath string
}

const (
	tplCodespaceList   templates.TplName = "codespace/list"
	tplCodespaceDetail templates.TplName = "codespace/detail"
	tplCodespaceState  templates.TplName = "codespace/state"
)

// List renders the current user's Codespaces.
func List(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	orgs, err := organization.GetUserOrgsList(ctx, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserOrgsList", err)
		return
	}
	ctx.Data["Orgs"] = orgs
	ctx.Data["ContextUser"] = ctx.Doer

	ownerName := strings.TrimSpace(ctx.FormString("owner"))
	var repoOwnerID int64
	if ownerName != "" {
		var selectedOrg *organization.Organization
		for _, org := range orgs {
			if strings.EqualFold(org.Name, ownerName) {
				selectedOrg = org
				break
			}
		}
		if selectedOrg == nil {
			ctx.NotFound(nil)
			return
		}
		ownerName = selectedOrg.Name
		repoOwnerID = selectedOrg.ID
		ctx.Data["ContextUser"] = selectedOrg.AsUser()
	}

	page := max(ctx.FormInt("page"), 1)
	pageSize := setting.UI.User.RepoPagingNum
	result, err := codespace_service.ListCreatorCodespaces(ctx, codespace_service.CreatorListOptions{
		UserID:      ctx.Doer.ID,
		RepoOwnerID: repoOwnerID,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		ctx.ServerError("ListCreatorCodespaces", err)
		return
	}
	if page > 1 && len(result.Rows) == 0 && result.Total > 0 {
		lastPage := int((result.Total + int64(pageSize) - 1) / int64(pageSize))
		ctx.Redirect(setting.AppSubURL+codespaceListPath(ownerName, lastPage), http.StatusSeeOther)
		return
	}

	ctx.Data["Title"] = ctx.Tr("codespace.title")
	ctx.Data["PageIsCodespaces"] = true
	ctx.Data["Codespaces"] = result.Rows
	ctx.Data["CodespaceOwner"] = ownerName
	ctx.Data["CodespaceListReturnTo"] = setting.AppSubURL + codespaceListPath(ownerName, page)
	pager := context.NewPagination(result.Total, pageSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplCodespaceList)
}

// Detail renders the current user's single Codespace page.
func Detail(ctx *context.Context) {
	view, ok := loadCreatorDetail(ctx)
	if !ok {
		return
	}
	ctx.Data["Title"] = "Codespace"
	ctx.Data["PageIsCodespaces"] = true
	ctx.Data["Codespace"] = view
	setCreatorDetailTab(ctx, view)
	if endpointID := strings.TrimSpace(ctx.FormString("open_endpoint")); endpointID != "" {
		var endpoint *codespace_service.CreatorEndpointView
		if endpointID == "workspace" {
			endpoint = view.Workspace
		} else {
			for i := range view.Endpoints {
				if view.Endpoints[i].EndpointID == endpointID {
					endpoint = &view.Endpoints[i]
					break
				}
			}
		}
		if endpoint != nil && endpoint.CanOpen {
			label := endpoint.Label
			if endpoint.EndpointID == "workspace" {
				label = string(ctx.Tr("codespace.workspace"))
			}
			access := string(ctx.Tr("codespace.authenticated_endpoint"))
			if endpoint.Public {
				access = string(ctx.Tr("codespace.public_endpoint"))
			}
			ctx.Data["OpenEndpointModal"] = &openEndpointModalData{
				Label:    label,
				Access:   access,
				OpenPath: endpoint.OpenPath,
			}
		} else {
			ctx.Data["OpenEndpointError"] = true
		}
	}
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.HTML(http.StatusOK, tplCodespaceDetail)
}

// State renders the live state fragment for a single Codespace.
func State(ctx *context.Context) {
	view, ok := loadCreatorDetail(ctx)
	if !ok {
		return
	}
	ctx.Data["Codespace"] = view
	setCreatorDetailTab(ctx, view)
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.HTML(http.StatusOK, tplCodespaceState)
}

func setCreatorDetailTab(ctx *context.Context, view *codespace_service.CreatorCodespaceView) {
	tab := strings.TrimSpace(ctx.FormString("tab"))
	explicit := tab == codespace_service.DetailModeOverview || tab == codespace_service.DetailModeLogs
	if !explicit {
		tab = view.DetailMode
	}
	ctx.Data["CodespaceTab"] = tab
	ctx.Data["CodespaceTabExplicit"] = explicit
}

func loadCreatorDetail(ctx *context.Context) (*codespace_service.CreatorCodespaceView, bool) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return nil, false
	}
	codespaceID, ok := codespaceIDParam(ctx)
	if !ok {
		return nil, false
	}
	view, err := codespace_service.GetCreatorCodespace(ctx, codespace_service.CreatorDetailOptions{
		UserID:      ctx.Doer.ID,
		CodespaceID: codespaceID,
	})
	if err != nil {
		switch {
		case errors.Is(err, codespace_service.ErrViewNotFound), errors.Is(err, codespace_service.ErrViewPermissionDenied):
			ctx.NotFound(nil)
		default:
			ctx.ServerError("GetCreatorCodespace", err)
		}
		return nil, false
	}
	return view, true
}
