// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"bytes"
	"fmt"
	gotemplate "html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/pipeline"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

const (
	tplSettingsLFS         templates.TplName = "repo/settings/lfs"
	tplSettingsLFSLocks    templates.TplName = "repo/settings/lfs_locks"
	tplSettingsLFSFile     templates.TplName = "repo/settings/lfs_file"
	tplSettingsLFSFileFind templates.TplName = "repo/settings/lfs_file_find"
	tplSettingsLFSPointers templates.TplName = "repo/settings/lfs_pointers"
)

// LFSFiles shows a repository's LFS files
func LFSFiles(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	total, err := git_model.CountLFSMetaObjects(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("LFSFiles", err)
		return
	}
	ctx.Data["Total"] = total

	pager := context.NewPagination(int(total), setting.UI.ExplorePagingNum, page, 5)
	ctx.Data["Title"] = ctx.Tr("repo.settings.lfs")
	ctx.Data["PageIsSettingsLFS"] = true
	lfsMetaObjects, err := git_model.GetLFSMetaObjects(ctx, ctx.Repo.Repository.ID, pager.Paginater.Current(), setting.UI.ExplorePagingNum)
	if err != nil {
		ctx.ServerError("LFSFiles", err)
		return
	}
	ctx.Data["LFSFiles"] = lfsMetaObjects
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplSettingsLFS)
}

// LFSLocks shows a repository's LFS locks
func LFSLocks(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	total, err := git_model.CountLFSLockByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("LFSLocks", err)
		return
	}
	ctx.Data["Total"] = total

	pager := context.NewPagination(int(total), setting.UI.ExplorePagingNum, page, 5)
	ctx.Data["Title"] = ctx.Tr("repo.settings.lfs_locks")
	ctx.Data["PageIsSettingsLFS"] = true
	lfsLocks, err := git_model.GetLFSLockByRepoID(ctx, ctx.Repo.Repository.ID, pager.Paginater.Current(), setting.UI.ExplorePagingNum)
	if err != nil {
		ctx.ServerError("LFSLocks", err)
		return
	}
	if err := lfsLocks.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LFSLocks", err)
		return
	}

	ctx.Data["LFSLocks"] = lfsLocks

	if len(lfsLocks) == 0 {
		ctx.Data["Page"] = pager
		ctx.HTML(http.StatusOK, tplSettingsLFSLocks)
		return
	}

	// Clone base repo.
	tmpBasePath, err := repo_module.CreateTemporaryPath("locks")
	if err != nil {
		log.Error("Failed to create temporary path: %v", err)
		ctx.ServerError("LFSLocks", err)
		return
	}
	defer func() {
		if err := repo_module.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("LFSLocks: RemoveTemporaryPath: %v", err)
		}
	}()

	if err := git.Clone(ctx, ctx.Repo.Repository.RepoPath(), tmpBasePath, git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
	}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", ctx.Repo.Repository.FullName(), err)
		ctx.ServerError("LFSLocks", fmt.Errorf("failed to clone repository: %s (%w)", ctx.Repo.Repository.FullName(), err))
		return
	}

	gitRepo, err := git.OpenRepository(ctx, tmpBasePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", tmpBasePath, err)
		ctx.ServerError("LFSLocks", fmt.Errorf("failed to open new temporary repository in: %s %w", tmpBasePath, err))
		return
	}
	defer gitRepo.Close()

	filenames := make([]string, len(lfsLocks))

	for i, lock := range lfsLocks {
		filenames[i] = lock.Path
	}

	if err := gitRepo.ReadTreeToIndex(ctx.Repo.Repository.DefaultBranch); err != nil {
		log.Error("Unable to read the default branch to the index: %s (%v)", ctx.Repo.Repository.DefaultBranch, err)
		ctx.ServerError("LFSLocks", fmt.Errorf("unable to read the default branch to the index: %s (%w)", ctx.Repo.Repository.DefaultBranch, err))
		return
	}

	name2attribute2info, err := gitRepo.CheckAttribute(git.CheckAttributeOpts{
		Attributes: []string{"lockable"},
		Filenames:  filenames,
		CachedOnly: true,
	})
	if err != nil {
		log.Error("Unable to check attributes in %s (%v)", tmpBasePath, err)
		ctx.ServerError("LFSLocks", err)
		return
	}

	lockables := make([]bool, len(lfsLocks))
	for i, lock := range lfsLocks {
		attribute2info, has := name2attribute2info[lock.Path]
		if !has {
			continue
		}
		if attribute2info["lockable"] != "set" {
			continue
		}
		lockables[i] = true
	}
	ctx.Data["Lockables"] = lockables

	filelist, err := gitRepo.LsFiles(filenames...)
	if err != nil {
		log.Error("Unable to lsfiles in %s (%v)", tmpBasePath, err)
		ctx.ServerError("LFSLocks", err)
		return
	}

	fileset := make(container.Set[string], len(filelist))
	fileset.AddMultiple(filelist...)

	linkable := make([]bool, len(lfsLocks))
	for i, lock := range lfsLocks {
		linkable[i] = fileset.Contains(lock.Path)
	}
	ctx.Data["Linkable"] = linkable

	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplSettingsLFSLocks)
}

// LFSLockFile locks a file
func LFSLockFile(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	originalPath := ctx.FormString("path")
	lockPath := originalPath
	if len(lockPath) == 0 {
		ctx.Flash.Error(ctx.Tr("repo.settings.lfs_invalid_locking_path", originalPath))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
		return
	}
	if lockPath[len(lockPath)-1] == '/' {
		ctx.Flash.Error(ctx.Tr("repo.settings.lfs_invalid_lock_directory", originalPath))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
		return
	}
	lockPath = util.PathJoinRel(lockPath)
	if len(lockPath) == 0 {
		ctx.Flash.Error(ctx.Tr("repo.settings.lfs_invalid_locking_path", originalPath))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
		return
	}

	_, err := git_model.CreateLFSLock(ctx, ctx.Repo.Repository, &git_model.LFSLock{
		Path:    lockPath,
		OwnerID: ctx.Doer.ID,
	})
	if err != nil {
		if git_model.IsErrLFSLockAlreadyExist(err) {
			ctx.Flash.Error(ctx.Tr("repo.settings.lfs_lock_already_exists", originalPath))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
			return
		}
		ctx.ServerError("LFSLockFile", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
}

// LFSUnlock forcibly unlocks an LFS lock
func LFSUnlock(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	_, err := git_model.DeleteLFSLockByID(ctx, ctx.PathParamInt64("lid"), ctx.Repo.Repository, ctx.Doer, true)
	if err != nil {
		ctx.ServerError("LFSUnlock", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs/locks")
}

// LFSFileGet serves a single LFS file
func LFSFileGet(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"
	oid := ctx.PathParam("oid")

	p := lfs.Pointer{Oid: oid}
	if !p.IsValid() {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["Title"] = oid
	ctx.Data["PageIsSettingsLFS"] = true
	meta, err := git_model.GetLFSMetaObjectByOid(ctx, ctx.Repo.Repository.ID, oid)
	if err != nil {
		if err == git_model.ErrLFSObjectNotExist {
			ctx.NotFound(nil)
			return
		}
		ctx.ServerError("LFSFileGet", err)
		return
	}
	ctx.Data["LFSFile"] = meta
	dataRc, err := lfs.ReadMetaObject(meta.Pointer)
	if err != nil {
		ctx.ServerError("LFSFileGet", err)
		return
	}
	defer dataRc.Close()
	buf := make([]byte, 1024)
	n, err := util.ReadAtMost(dataRc, buf)
	if err != nil {
		ctx.ServerError("Data", err)
		return
	}
	buf = buf[:n]

	st := typesniffer.DetectContentType(buf)
	ctx.Data["IsTextFile"] = st.IsText()
	ctx.Data["FileSize"] = meta.Size
	ctx.Data["RawFileLink"] = fmt.Sprintf("%s%s/%s.git/info/lfs/objects/%s/%s", setting.AppURL, url.PathEscape(ctx.Repo.Repository.OwnerName), url.PathEscape(ctx.Repo.Repository.Name), url.PathEscape(meta.Oid), "direct")
	switch {
	case st.IsRepresentableAsText():
		if meta.Size >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		if st.IsSvgImage() {
			ctx.Data["IsImageFile"] = true
		}

		rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})

		// Building code view blocks with line number on server side.
		// FIXME: the logic is not right here: it first calls EscapeControlReader then calls HTMLEscapeString: double-escaping
		escapedContent := &bytes.Buffer{}
		ctx.Data["EscapeStatus"], _ = charset.EscapeControlReader(rd, escapedContent, ctx.Locale)

		var output bytes.Buffer
		lines := strings.Split(escapedContent.String(), "\n")
		// Remove blank line at the end of file
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

	case st.IsPDF():
		ctx.Data["IsPDFFile"] = true
	case st.IsVideo():
		ctx.Data["IsVideoFile"] = true
	case st.IsAudio():
		ctx.Data["IsAudioFile"] = true
	case st.IsImage() && (setting.UI.SVG.Enabled || !st.IsSvgImage()):
		ctx.Data["IsImageFile"] = true
	default:
		// TODO: the logic is not the same as "renderFile" in "view.go"
	}
	ctx.HTML(http.StatusOK, tplSettingsLFSFile)
}

// LFSDelete disassociates the provided oid from the repository and if the lfs file is no longer associated with any repositories - deletes it
func LFSDelete(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	oid := ctx.PathParam("oid")
	p := lfs.Pointer{Oid: oid}
	if !p.IsValid() {
		ctx.NotFound(nil)
		return
	}

	count, err := git_model.RemoveLFSMetaObjectByOid(ctx, ctx.Repo.Repository.ID, oid)
	if err != nil {
		ctx.ServerError("LFSDelete", err)
		return
	}
	// FIXME: Warning: the LFS store is not locked - and can't be locked - there could be a race condition here
	// Please note a similar condition happens in models/repo.go DeleteRepository
	if count == 0 {
		oidPath := path.Join(oid[0:2], oid[2:4], oid[4:])
		err = storage.LFS.Delete(oidPath)
		if err != nil {
			ctx.ServerError("LFSDelete", err)
			return
		}
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs")
}

// LFSFileFind guesses a sha for the provided oid (or uses the provided sha) and then finds the commits that contain this sha
func LFSFileFind(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	oid := ctx.FormString("oid")
	size := ctx.FormInt64("size")
	if len(oid) == 0 || size == 0 {
		ctx.NotFound(nil)
		return
	}
	sha := ctx.FormString("sha")
	ctx.Data["Title"] = oid
	ctx.Data["PageIsSettingsLFS"] = true
	objectFormat := ctx.Repo.GetObjectFormat()
	var objectID git.ObjectID
	if len(sha) == 0 {
		pointer := lfs.Pointer{Oid: oid, Size: size}
		objectID = git.ComputeBlobHash(objectFormat, []byte(pointer.StringContent()))
		sha = objectID.String()
	} else {
		objectID = git.MustIDFromString(sha)
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"
	ctx.Data["Oid"] = oid
	ctx.Data["Size"] = size
	ctx.Data["SHA"] = sha

	results, err := pipeline.FindLFSFile(ctx.Repo.GitRepo, objectID)
	if err != nil && err != io.EOF {
		log.Error("Failure in FindLFSFile: %v", err)
		ctx.ServerError("LFSFind: FindLFSFile.", err)
		return
	}

	ctx.Data["Results"] = results
	ctx.HTML(http.StatusOK, tplSettingsLFSFileFind)
}

// LFSPointerFiles will search the repository for pointer files and report which are missing LFS files in the content store
func LFSPointerFiles(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	ctx.Data["PageIsSettingsLFS"] = true
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"

	var err error
	err = func() error {
		pointerChan := make(chan lfs.PointerBlob)
		errChan := make(chan error, 1)
		go lfs.SearchPointerBlobs(ctx, ctx.Repo.GitRepo, pointerChan, errChan)

		numPointers := 0
		var numAssociated, numNoExist, numAssociatable int

		type pointerResult struct {
			SHA          string
			Oid          string
			Size         int64
			InRepo       bool
			Exists       bool
			Accessible   bool
			Associatable bool
		}

		results := []pointerResult{}

		contentStore := lfs.NewContentStore()
		repo := ctx.Repo.Repository

		for pointerBlob := range pointerChan {
			numPointers++

			result := pointerResult{
				SHA:  pointerBlob.Hash,
				Oid:  pointerBlob.Oid,
				Size: pointerBlob.Size,
			}

			if _, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, pointerBlob.Oid); err != nil {
				if err != git_model.ErrLFSObjectNotExist {
					return err
				}
			} else {
				result.InRepo = true
			}

			result.Exists, err = contentStore.Exists(pointerBlob.Pointer)
			if err != nil {
				return err
			}

			if result.Exists {
				if !result.InRepo {
					// Can we fix?
					// OK well that's "simple"
					// - we need to check whether current user has access to a repo that has access to the file
					result.Associatable, err = git_model.LFSObjectAccessible(ctx, ctx.Doer, pointerBlob.Oid)
					if err != nil {
						return err
					}
					if !result.Associatable {
						associated, err := git_model.ExistsLFSObject(ctx, pointerBlob.Oid)
						if err != nil {
							return err
						}
						result.Associatable = !associated
					}
				}
			}

			result.Accessible = result.InRepo || result.Associatable

			if result.InRepo {
				numAssociated++
			}
			if !result.Exists {
				numNoExist++
			}
			if result.Associatable {
				numAssociatable++
			}

			results = append(results, result)
		}

		err, has := <-errChan
		if has {
			return err
		}

		ctx.Data["Pointers"] = results
		ctx.Data["NumPointers"] = numPointers
		ctx.Data["NumAssociated"] = numAssociated
		ctx.Data["NumAssociatable"] = numAssociatable
		ctx.Data["NumNoExist"] = numNoExist
		ctx.Data["NumNotAssociated"] = numPointers - numAssociated

		return nil
	}()
	if err != nil {
		ctx.ServerError("LFSPointerFiles", err)
		return
	}

	ctx.HTML(http.StatusOK, tplSettingsLFSPointers)
}

// LFSAutoAssociate auto associates accessible lfs files
func LFSAutoAssociate(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound(nil)
		return
	}
	oids := ctx.FormStrings("oid")
	metas := make([]*git_model.LFSMetaObject, len(oids))
	for i, oid := range oids {
		idx := strings.IndexRune(oid, ' ')
		if idx < 0 || idx+1 > len(oid) {
			ctx.ServerError("LFSAutoAssociate", fmt.Errorf("illegal oid input: %s", oid))
			return
		}
		var err error
		metas[i] = &git_model.LFSMetaObject{}
		metas[i].Size, err = strconv.ParseInt(oid[idx+1:], 10, 64)
		if err != nil {
			ctx.ServerError("LFSAutoAssociate", fmt.Errorf("illegal oid input: %s %w", oid, err))
			return
		}
		metas[i].Oid = oid[:idx]
		// metas[i].RepositoryID = ctx.Repo.Repository.ID
	}
	if err := git_model.LFSAutoAssociate(ctx, metas, ctx.Doer, ctx.Repo.Repository.ID); err != nil {
		ctx.ServerError("LFSAutoAssociate", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs")
}
