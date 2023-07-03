// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
)

func GenerateExternalWiki(ctx context.Context, templateRepo, generateRepo *repo_model.Repository) error {
	templateUnit, err := templateRepo.GetUnit(ctx, unit.TypeExternalWiki)
	if err != nil {
		if repo_model.IsErrUnitTypeNotExist(err) {
			return nil
		}
		return err
	}
	templateCfg := templateUnit.ExternalWikiConfig()

	generateUnit := &repo_model.RepoUnit{
		RepoID: generateRepo.ID,
		Type:   unit.TypeExternalWiki,
		Config: &repo_model.ExternalWikiConfig{
			ExternalWikiURL: generateExpansion(templateCfg.ExternalWikiURL, templateRepo, generateRepo, false),
		},
	}

	return repo_model.UpdateRepositoryUnits(generateRepo, []repo_model.RepoUnit{*generateUnit}, []unit.Type{unit.TypeWiki})
}
