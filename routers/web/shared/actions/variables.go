// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

func SetVariablesContext(ctx *context.Context, ownerID, repoID int64) {
	variables, err := actions_model.FindVariables(ctx, actions_model.FindVariablesOpts{
		OwnerID: ownerID,
		RepoID:  repoID,
	})
	if err != nil {
		ctx.ServerError("FindVariables", err)
		return
	}
	ctx.Data["Variables"] = variables
}

// some regular expression of `variables` and `secrets`
// reference to:
// https://docs.github.com/en/actions/learn-github-actions/variables#naming-conventions-for-configuration-variables
// https://docs.github.com/en/actions/security-guides/encrypted-secrets#naming-your-secrets
var (
	titleRx           = regexp.MustCompile("(?i)^[A-Z_][A-Z0-9_]*$")
	forbiddenPrefixRx = regexp.MustCompile("(?i)^GIT(EA|HUB)_")
)

func TitleRegexMatch(ctx *context.Context, title, redirectURL string) error {
	if !titleRx.MatchString(title) || forbiddenPrefixRx.MatchString(title) {
		log.Error("Title %s, regex match error", title)
		return errors.New("title has invaild character")
	}
	return nil
}

func CreateVariable(ctx *context.Context, ownerID, repoID int64, redirectURL string) {
	form := web.GetForm(ctx).(*forms.EditVariableForm)

	if err := TitleRegexMatch(ctx, form.Title, redirectURL); err != nil {
		ctx.Flash.Error(ctx.Tr("actions.variables.creation.failed"))
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"redirect": redirectURL,
		})
		return
	}

	v, err := actions_model.InsertVariable(ctx, ownerID, repoID, form.Title, reserveLineBreakForTextarea(form.Content))
	if err != nil {
		log.Error("InsertVariable error: %v", err)
		ctx.Flash.Error(ctx.Tr("actions.variables.creation.failed"))
	} else {
		ctx.Flash.Success(ctx.Tr("actions.variables.creation.success", v.Title))
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": redirectURL,
	})
}

func UpdateVariable(ctx *context.Context, redirectURL string) {
	id := ctx.ParamsInt64(":variable_id")
	form := web.GetForm(ctx).(*forms.EditVariableForm)

	if err := TitleRegexMatch(ctx, form.Title, redirectURL); err != nil {
		ctx.Flash.Error(ctx.Tr("actions.variables.creation.failed"))
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"redirect": redirectURL,
		})
		return
	}

	ok, err := actions_model.UpdateVariable(ctx, &actions_model.ActionVariable{
		ID:      id,
		Title:   strings.ToUpper(form.Title),
		Content: reserveLineBreakForTextarea(form.Content),
	})
	if err != nil || !ok {
		log.Error("UpdateVariable error: %v", err)
		ctx.Flash.Error(ctx.Tr("actions.variables.update.failed"))
	} else {
		ctx.Flash.Success(ctx.Tr("actions.variables.update.success"))
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": redirectURL,
	})
}

func DeleteVariable(ctx *context.Context, redirectURL string) {
	id := ctx.ParamsInt64(":variable_id")

	if _, err := db.DeleteByBean(ctx, &actions_model.ActionVariable{ID: id}); err != nil {
		log.Error("Delete variable [%d] failed: %v", id, err)
		ctx.Flash.Error(ctx.Tr("actions.variables.deletion.failed"))
	} else {
		ctx.Flash.Success(ctx.Tr("actions.variables.deletion.success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": redirectURL,
	})
}

func reserveLineBreakForTextarea(input string) string {
	// Since the content is from a form which is a textarea, the line endings are \r\n.
	// It's a standard behavior of HTML.
	// But we want to store them as \n like what GitHub does.
	// And users are unlikely to really need to keep the \r.
	// Other than this, we should respect the original content, even leading or trailing spaces.
	return strings.ReplaceAll(input, "\r\n", "\n")
}
