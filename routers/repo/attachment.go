// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func renderAttachmentSettings(ctx *context.Context) {
	ctx.Data["RequireDropzone"] = true
	ctx.Data["IsAttachmentEnabled"] = setting.AttachmentEnabled
	ctx.Data["AttachmentAllowedTypes"] = setting.AttachmentAllowedTypes
	ctx.Data["AttachmentMaxSize"] = setting.AttachmentMaxSize
	ctx.Data["AttachmentMaxFiles"] = setting.AttachmentMaxFiles
}

// UploadAttachment response for uploading issue's attachment
func UploadAttachment(ctx *context.Context) {
	if !setting.AttachmentEnabled {
		ctx.Error(404, "attachment is not enabled")
		return
	}

	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}
	fileType := http.DetectContentType(buf)

	allowedTypes := strings.Split(setting.AttachmentAllowedTypes, ",")
	allowed := false
	for _, t := range allowedTypes {
		t := strings.Trim(t, " ")
		if t == "*/*" || t == fileType {
			allowed = true
			break
		}
	}

	if !allowed {
		ctx.Error(400, ErrFileTypeForbidden.Error())
		return
	}

	attach, err := models.NewAttachment(header.Filename, buf, file)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("NewAttachment: %v", err))
		return
	}

	log.Trace("New attachment uploaded: %s", attach.UUID)
	ctx.JSON(200, map[string]string{
		"uuid": attach.UUID,
	})
}
