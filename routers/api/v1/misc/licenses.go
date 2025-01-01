// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"fmt"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/options"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

// Returns a list of all License templates
func ListLicenseTemplates(ctx *context.APIContext) {
	// swagger:operation GET /licenses miscellaneous listLicenseTemplates
	// ---
	// summary: Returns a list of all license templates
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/LicenseTemplateList"
	response := make([]api.LicensesTemplateListEntry, len(repo_module.Licenses))
	for i, license := range repo_module.Licenses {
		response[i] = api.LicensesTemplateListEntry{
			Key:  license,
			Name: license,
			URL:  fmt.Sprintf("%sapi/v1/licenses/%s", setting.AppURL, url.PathEscape(license)),
		}
	}
	ctx.JSON(http.StatusOK, response)
}

// Returns information about a gitignore template
func GetLicenseTemplateInfo(ctx *context.APIContext) {
	// swagger:operation GET /licenses/{name} miscellaneous getLicenseTemplateInfo
	// ---
	// summary: Returns information about a license template
	// produces:
	// - application/json
	// parameters:
	// - name: name
	//   in: path
	//   description: name of the license
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/LicenseTemplateInfo"
	//   "404":
	//     "$ref": "#/responses/notFound"
	name := util.PathJoinRelX(ctx.PathParam("name"))

	text, err := options.License(name)
	if err != nil {
		ctx.NotFound()
		return
	}

	response := api.LicenseTemplateInfo{
		Key:  name,
		Name: name,
		URL:  fmt.Sprintf("%sapi/v1/licenses/%s", setting.AppURL, url.PathEscape(name)),
		Body: string(text),
		// This is for combatibilty with the GitHub API. This Text is for some reason added to each License response.
		Implementation: "Create a text file (typically named LICENSE or LICENSE.txt) in the root of your source code and copy the text of the license into the file",
	}

	ctx.JSON(http.StatusOK, response)
}
