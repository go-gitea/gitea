// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/services/activitypub"
)

// Note function returns the Note object for a comment to an issue or PR
func Note(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/note/{username}/{reponame}/{id}/{noteid} activitypub activitypubNote
	// ---
	// summary: Returns the Note object for a comment to an issue or PR
	// produces
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: reponame
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: ID number of the issue or PR
	//   type: string
	//   required: true
	// - name: noteid
	//   in: path
	//   description: ID number of the comment
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	index, err := strconv.ParseInt(ctx.Params("noteid"), 10, 64)
	if err != nil {
		ctx.ServerError("ParseInt", err)
		return
	}
	// TODO: index can be spoofed!!!
	comment, err := issues_model.GetCommentByID(ctx, index)
	if err != nil {
		ctx.ServerError("GetCommentByID", err)
		return
	}
	note, err := activitypub.Note(comment)
	if err != nil {
		ctx.ServerError("Note", err)
		return
	}
	response(ctx, note)
}
