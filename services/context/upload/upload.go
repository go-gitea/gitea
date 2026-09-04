// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package upload

import (
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

// ErrFileTypeForbidden not allowed file type error
type ErrFileTypeForbidden struct {
	Type string
}

// IsErrFileTypeForbidden checks if an error is a ErrFileTypeForbidden.
func IsErrFileTypeForbidden(err error) bool {
	_, ok := err.(ErrFileTypeForbidden)
	return ok
}

func (err ErrFileTypeForbidden) Error() string {
	return "This file cannot be uploaded or modified due to a forbidden file extension or type."
}

func (err ErrFileTypeForbidden) Unwrap() error {
	return util.ErrInvalidArgument
}

var wildcardTypeRe = regexp.MustCompile(`^[a-z]+/\*$`)

// Verify validates whether a file is allowed to be uploaded. If buf is empty, it will just check if the file
// has an allowed file extension.
func Verify(buf []byte, fileName, allowedTypesStr string) error {
	allowedTypesStr = strings.ReplaceAll(allowedTypesStr, "|", ",") // compat for old config format

	allowedTypes := []string{}
	for entry := range strings.SplitSeq(allowedTypesStr, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry != "" {
			allowedTypes = append(allowedTypes, entry)
		}
	}

	if len(allowedTypes) == 0 {
		return nil // everything is allowed
	}

	fullMimeType := http.DetectContentType(buf)
	mimeType, _, err := mime.ParseMediaType(fullMimeType)
	if err != nil {
		log.Warn("Detected attachment type could not be parsed %s", fullMimeType)
		return ErrFileTypeForbidden{Type: fullMimeType}
	}
	extension := strings.ToLower(path.Ext(fileName))
	isBufEmpty := len(buf) <= 1

	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#Unique_file_type_specifiers
	for _, allowEntry := range allowedTypes {
		if allowEntry == "*/*" {
			return nil // everything allowed
		}
		if strings.HasPrefix(allowEntry, ".") && allowEntry == extension {
			return nil // extension is allowed
		}
		if isBufEmpty {
			continue // skip mime type checks if buffer is empty
		}
		if mimeType == allowEntry {
			return nil // mime type is allowed
		}
		if wildcardTypeRe.MatchString(allowEntry) && strings.HasPrefix(mimeType, allowEntry[:len(allowEntry)-1]) {
			return nil // wildcard match, e.g. image/*
		}
	}

	if !isBufEmpty {
		log.Info("Attachment with type %s blocked from upload", fullMimeType)
	}

	return ErrFileTypeForbidden{Type: fullMimeType}
}

type uploadOptions struct {
	UploadUrl       string
	UploadRemoveUrl string
	UploadLinkUrl   string
	UploadAccepts   string
	UploadMaxFiles  int
	UploadMaxSize   int64
	NeedUuidLink    bool // issue/comment Markdown editor needs the uuid link to be copiable
}

// AddUploadContext renders template values for dropzone
func AddUploadContext(ctx *context.Context, uploadType string) {
	switch uploadType {
	case "release":
		ctx.Data["UploadOptions"] = uploadOptions{
			UploadUrl:       ctx.Repo.RepoLink + "/releases/attachments",
			UploadRemoveUrl: ctx.Repo.RepoLink + "/releases/attachments/remove",
			UploadLinkUrl:   ctx.Repo.RepoLink + "/releases/attachments",
			UploadAccepts:   strings.ReplaceAll(setting.Repository.Release.AllowedTypes, "|", ","),
			UploadMaxFiles:  setting.Repository.Release.MaxFiles,
			UploadMaxSize:   setting.Repository.Release.FileMaxSize,
		}
	case "comment":
		var uploadLinkUrl string
		if len(ctx.PathParam("index")) > 0 {
			uploadLinkUrl = ctx.Repo.RepoLink + "/issues/" + url.PathEscape(ctx.PathParam("index")) + "/attachments"
		} else {
			uploadLinkUrl = ctx.Repo.RepoLink + "/issues/attachments"
		}
		ctx.Data["UploadOptions"] = uploadOptions{
			UploadUrl:       ctx.Repo.RepoLink + "/issues/attachments",
			UploadRemoveUrl: ctx.Repo.RepoLink + "/issues/attachments/remove",
			UploadLinkUrl:   uploadLinkUrl,
			UploadAccepts:   strings.ReplaceAll(setting.Attachment.AllowedTypes, "|", ","),
			UploadMaxFiles:  setting.Attachment.MaxFiles,
			UploadMaxSize:   setting.Attachment.MaxSize,
			NeedUuidLink:    true,
		}
	default:
		setting.PanicInDevOrTesting("Invalid upload type: %s", uploadType)
	}
}

func AddUploadContextForRepo(ctx reqctx.RequestContext, repo *repo_model.Repository) {
	ctxData, repoLink := ctx.GetData(), repo.Link()
	ctxData["UploadOptions"] = uploadOptions{
		UploadUrl:       repoLink + "/upload-file",
		UploadRemoveUrl: repoLink + "/upload-remove",
		UploadLinkUrl:   repoLink + "/upload-file",
		UploadAccepts:   strings.ReplaceAll(setting.Repository.Upload.AllowedTypes, "|", ","),
		UploadMaxFiles:  setting.Repository.Upload.MaxFiles,
		UploadMaxSize:   setting.Repository.Upload.FileMaxSize,
	}
}
