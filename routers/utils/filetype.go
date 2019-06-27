// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

// VerifyAllowedContentType validates a file is allwoed to be uploaded.
func VerifyAllowedContentType(buf []byte, allowedTypes []string) error {
	fileType := http.DetectContentType(buf)

	allowed := false
	for _, t := range allowedTypes {
		t := strings.Trim(t, " ")
		if t == "*/*" || t == fileType {
			allowed = true
			break
		}
	}

	if !allowed {
		log.Info("Attachment with type %s blocked from upload", fileType)
		return models.ErrFileTypeForbidden{Type: fileType}
	}

	return nil
}
