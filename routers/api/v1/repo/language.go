// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"net/http"
	"strconv"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
)

type languageResponse []*repo_model.LanguageStat

func (l languageResponse) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.WriteString("{"); err != nil {
		return nil, err
	}
	for i, lang := range l {
		if i > 0 {
			if _, err := buf.WriteString(","); err != nil {
				return nil, err
			}
		}
		if _, err := buf.WriteString(strconv.Quote(lang.Language)); err != nil {
			return nil, err
		}
		if _, err := buf.WriteString(":"); err != nil {
			return nil, err
		}
		if _, err := buf.WriteString(strconv.FormatInt(lang.Size, 10)); err != nil {
			return nil, err
		}
	}
	if _, err := buf.WriteString("}"); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GetLanguages returns languages and number of bytes of code written
func GetLanguages(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/languages repository repoGetLanguages
	// ---
	// summary: Get languages and number of bytes of code written
	// produces:
	//   - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "200":
	//     "$ref": "#/responses/LanguageStatistics"

	langs, err := repo_model.GetLanguageStats(ctx, ctx.Repo.Repository)
	if err != nil {
		log.Error("GetLanguageStats failed: %v", err)
		ctx.APIErrorInternal(err)
		return
	}

	resp := make(languageResponse, len(langs))
	copy(resp, langs)

	ctx.JSON(http.StatusOK, resp)
}
