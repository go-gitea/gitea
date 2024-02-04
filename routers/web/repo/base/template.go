// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	issue_template "code.gitea.io/gitea/modules/issue/template"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/utils"
)

const (
	issueTemplateTitleKey = "IssueTemplateTitle"
)

// Tries to load and set an issue template. The first return value indicates if a template was loaded.
func SetTemplateIfExists(ctx *context.Context, ctxDataKey string, possibleFiles []string) (bool, map[string]error) {
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		return false, nil
	}

	templateCandidates := make([]string, 0, 1+len(possibleFiles))
	if t := ctx.FormString("template"); t != "" {
		templateCandidates = append(templateCandidates, t)
	}
	templateCandidates = append(templateCandidates, possibleFiles...) // Append files to the end because they should be fallback

	templateErrs := map[string]error{}
	for _, filename := range templateCandidates {
		if ok, _ := commit.HasFile(filename); !ok {
			continue
		}
		template, err := issue_template.UnmarshalFromCommit(commit, filename)
		if err != nil {
			templateErrs[filename] = err
			continue
		}
		ctx.Data[issueTemplateTitleKey] = template.Title
		ctx.Data[ctxDataKey] = template.Content

		if template.Type() == api.IssueTemplateTypeYaml {
			// Replace field default values by values from query
			for _, field := range template.Fields {
				fieldValue := ctx.FormString("field:" + field.ID)
				if fieldValue != "" {
					field.Attributes["value"] = fieldValue
				}
			}

			ctx.Data["Fields"] = template.Fields
			ctx.Data["TemplateFile"] = template.FileName
		}
		labelIDs := make([]string, 0, len(template.Labels))
		if repoLabels, err := issues_model.GetLabelsByRepoID(ctx, ctx.Repo.Repository.ID, "", db.ListOptions{}); err == nil {
			ctx.Data["Labels"] = repoLabels
			if ctx.Repo.Owner.IsOrganization() {
				if orgLabels, err := issues_model.GetLabelsByOrgID(ctx, ctx.Repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{}); err == nil {
					ctx.Data["OrgLabels"] = orgLabels
					repoLabels = append(repoLabels, orgLabels...)
				}
			}

			for _, metaLabel := range template.Labels {
				for _, repoLabel := range repoLabels {
					if strings.EqualFold(repoLabel.Name, metaLabel) {
						repoLabel.IsChecked = true
						labelIDs = append(labelIDs, strconv.FormatInt(repoLabel.ID, 10))
						break
					}
				}
			}

		}

		if template.Ref != "" && !strings.HasPrefix(template.Ref, "refs/") { // Assume that the ref intended is always a branch - for tags users should use refs/tags/<ref>
			template.Ref = git.BranchPrefix + template.Ref
		}
		ctx.Data["HasSelectedLabel"] = len(labelIDs) > 0
		ctx.Data["label_ids"] = strings.Join(labelIDs, ",")
		ctx.Data["Reference"] = template.Ref
		ctx.Data["RefEndName"] = git.RefName(template.Ref).ShortName()
		return true, templateErrs
	}
	return false, templateErrs
}

func RenderErrorOfTemplates(ctx *context.Context, errs map[string]error) string {
	var files []string
	for k := range errs {
		files = append(files, k)
	}
	sort.Strings(files) // keep the output stable

	var lines []string
	for _, file := range files {
		lines = append(lines, fmt.Sprintf("%s: %v", file, errs[file]))
	}

	flashError, err := ctx.RenderToString(TplAlertDetails, map[string]any{
		"Message": ctx.Tr("repo.issues.choose.ignore_invalid_templates"),
		"Summary": ctx.Tr("repo.issues.choose.invalid_templates", len(errs)),
		"Details": utils.SanitizeFlashErrorString(strings.Join(lines, "\n")),
	})
	if err != nil {
		log.Debug("render flash error: %v", err)
		flashError = ctx.Tr("repo.issues.choose.ignore_invalid_templates")
	}
	return flashError
}
