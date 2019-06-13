// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"fmt"
	gotemplate "html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
)

const (
	tplSettingsLFS     base.TplName = "repo/settings/lfs"
	tplSettingsLFSFile base.TplName = "repo/settings/lfs_file"
)

// LFSFiles shows a repository's LFS files
func LFSFiles(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSFiles", nil)
		return
	}
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}
	total, err := ctx.Repo.Repository.CountLFSMetaObjects()
	if err != nil {
		ctx.ServerError("LFSFiles", err)
		return
	}

	pager := context.NewPagination(int(total), setting.UI.ExplorePagingNum, page, 5)
	ctx.Data["Title"] = ctx.Tr("repo.settings.lfs")
	ctx.Data["PageIsSettingsLFS"] = true
	lfsMetaObjects, err := ctx.Repo.Repository.GetLFSMetaObjects(pager.Paginater.Current(), setting.UI.ExplorePagingNum)
	if err != nil {
		ctx.ServerError("LFSFiles", err)
		return
	}
	ctx.Data["LFSFiles"] = lfsMetaObjects
	ctx.Data["Page"] = pager
	ctx.HTML(200, tplSettingsLFS)
}

// LFSFileGet serves a single LFS file
func LFSFileGet(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSFileGet", nil)
		return
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"
	oid := ctx.Params("oid")
	ctx.Data["Title"] = oid
	ctx.Data["PageIsSettingsLFS"] = true
	meta, err := ctx.Repo.Repository.GetLFSMetaObjectByOid(oid)
	if err != nil {
		ctx.ServerError("LFSFileGet", err)
		return
	}
	ctx.Data["LFSFile"] = meta
	dataRc, err := lfs.ReadMetaObject(meta)
	if err != nil {
		ctx.ServerError("LFSFileGet", err)
		return
	}
	defer dataRc.Close()
	buf := make([]byte, 1024)
	n, err := dataRc.Read(buf)
	if err != nil {
		ctx.ServerError("Data", err)
		return
	}
	buf = buf[:n]

	isTextFile := base.IsTextFile(buf)
	ctx.Data["IsTextFile"] = isTextFile

	fileSize := meta.Size
	ctx.Data["FileSize"] = meta.Size
	ctx.Data["RawFileLink"] = fmt.Sprintf("%s%s.git/info/lfs/objects/%s/%s", setting.AppURL, ctx.Repo.Repository.FullName(), meta.Oid, "direct")
	switch {
	case isTextFile:
		if fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		d, _ := ioutil.ReadAll(dataRc)
		buf = templates.ToUTF8WithFallback(append(buf, d...))

		// Building code view blocks with line number on server side.
		var fileContent string
		if content, err := templates.ToUTF8WithErr(buf); err != nil {
			log.Error("ToUTF8WithErr: %v", err)
			fileContent = string(buf)
		} else {
			fileContent = content
		}

		var output bytes.Buffer
		lines := strings.Split(fileContent, "\n")
		//Remove blank line at the end of file
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for index, line := range lines {
			line = gotemplate.HTMLEscapeString(line)
			if index != len(lines)-1 {
				line += "\n"
			}
			output.WriteString(fmt.Sprintf(`<li class="L%d" rel="L%d">%s</li>`, index+1, index+1, line))
		}
		ctx.Data["FileContent"] = gotemplate.HTML(output.String())

		output.Reset()
		for i := 0; i < len(lines); i++ {
			output.WriteString(fmt.Sprintf(`<span id="L%d">%d</span>`, i+1, i+1))
		}
		ctx.Data["LineNums"] = gotemplate.HTML(output.String())

	case base.IsPDFFile(buf):
		ctx.Data["IsPDFFile"] = true
	case base.IsVideoFile(buf):
		ctx.Data["IsVideoFile"] = true
	case base.IsAudioFile(buf):
		ctx.Data["IsAudioFile"] = true
	case base.IsImageFile(buf):
		ctx.Data["IsImageFile"] = true
	}
	ctx.HTML(200, tplSettingsLFSFile)
}

// LFSDelete disassociates the provided oid from the repository and if the lfs file is no longer associated with any repositories - deletes it
func LFSDelete(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSFileGet", nil)
		return
	}
	oid := ctx.Params("oid")
	count, err := ctx.Repo.Repository.RemoveLFSMetaObjectByOid(oid)
	if err != nil {
		ctx.ServerError("LFSDelete", err)
		return
	}
	// FIXME: Warning: the LFS store is not locked - and can't be locked - there could be a race condition here
	// Please note a similar condition happens in models/repo.go DeleteRepository
	if count == 0 {
		oidPath := filepath.Join(oid[0:2], oid[2:4], oid[4:])
		err = os.Remove(filepath.Join(setting.LFS.ContentPath, oidPath))
		if err != nil {
			ctx.ServerError("LFSDelete", err)
			return
		}
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs")
}
