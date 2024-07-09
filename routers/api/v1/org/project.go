package org

import (
	"log"
	"net/http"

	project_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
)

func CreateProject(ctx *context.APIContext) {
	form := web.GetForm(ctx).(*api.CreateProjectOption)

	log.Println(ctx.ContextUser.ID)

	project := &project_model.Project{
		OwnerID:      ctx.ContextUser.ID,
		Title:        form.Title,
		Description:  form.Content,
		CreatorID:    ctx.Doer.ID,
		TemplateType: project_model.TemplateType(form.TemplateType),
		CardType:     project_model.CardType(form.CardType),
	}

	if ctx.ContextUser.IsOrganization() {
		project.Type = project_model.TypeOrganization
	} else {
		project.Type = project_model.TypeIndividual
	}

	if err := project_model.NewProject(ctx, project); err != nil {
		ctx.Error(http.StatusInternalServerError, "NewProject", err)
		return
	}

	ctx.JSON(http.StatusCreated, map[string]int64{"id": project.ID})
}
