package repo

import "code.gitea.io/gitea/modules/context"

// CreateCheckRun Create a new check run
func CreateCheckRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/check-runs checkRuns checkRunsCreate
	// ---
	// summary: Create a new check run
	// produces:
	// - application/json
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateCheckRunOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/CheckRun"
	//   "400":
	//     "$ref": "#/responses/error"
}

// GetCheckRun Get a check run
func GetCheckRun(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/check-runs/{check_run_id} checkRuns checkRunsGet
	// ---
	// summary: Get a check run
	// produces:
	// - application/json
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
	// - name: check_run_id
	//   in: path
	//   description: id of the check run
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CheckRun"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

// UpdateCheckRun Update a check run
func UpdateCheckRun(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/check-runs/{check_run_id} checkRuns checkRunsUpdate
	// ---
	// summary: Update a check run
	// produces:
	// - application/json
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
	// - name: check_run_id
	//   in: path
	//   description: id of the check run
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateCheckRunOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/CheckRun"
	//   "404":
	//     "$ref": "#/responses/notFound"
}

// ListCheckRun List check runs for a Git reference
func ListCheckRun(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{ref}/check-runs checkRuns checkRunsList
	// ---
	// summary: ListCheckRun List check runs for a Git reference
	// produces:
	// - application/json
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
	// - name: ref
	//   in: path
	//   description: name of branch/tag/commit
	//   type: string
	//   required: true
	// - name: sort
	//   in: query
	//   description: type of sort
	//   type: string
	//   enum: [oldest, recentupdate, leastupdate, leastindex, highestindex]
	//   required: false
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommitStatusList"
	//   "400":
	//     "$ref": "#/responses/error"
}
