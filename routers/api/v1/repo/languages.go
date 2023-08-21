package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
)

// GetPrimaryLanguageList returns a list of primary languages along with their color
func GetPrimaryLanguageList(ctx *context.APIContext) {
	// swagger:operation GET /repos/languages repository repoGetPrimaryLanguages
	// ---
	// summary: Get primary languages and their color
	// produces:
	//   - application/json
	// parameters:
	// - name: ids
	//   in: query
	//   description: Repo IDs (empty for all)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/PrimaryLanguageList"

	ids := ctx.FormInt64s("ids")
	var repos repo_model.RepositoryList

	if ids != nil || len(ids) > 0 {
		repos = make([]*repo_model.Repository, len(ids))
		for i, id := range ids {
			repos[i] = &repo_model.Repository{ID: id}
		}
	} else {
		repos = nil
	}

	langs, err := repo_model.GetPrimaryRepoLanguageList(db.DefaultContext, repos)
	if err != nil {
		ctx.InternalServerError(err)
	}

	list := map[string]string{}

	for _, i := range langs {
		list[i.Language] = i.Color
	}

	ctx.JSON(http.StatusOK, list)
}
