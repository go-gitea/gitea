// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

type languageResponse []*models.LanguageStat

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

	langs, err := ctx.Repo.Repository.GetLanguageStats()
	if err != nil {
		log.Error("GetLanguageStats failed: %v", err)
		ctx.InternalServerError(err)
		return
	}

	resp := make(languageResponse, len(langs))
	for i, v := range langs {
		resp[i] = v
	}

	ctx.JSON(http.StatusOK, resp)
}
