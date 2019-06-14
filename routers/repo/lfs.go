// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bufio"
	"bytes"
	"fmt"
	gotemplate "html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"github.com/Unknwon/com"
	"github.com/mcuadros/go-version"
)

const (
	tplSettingsLFS         base.TplName = "repo/settings/lfs"
	tplSettingsLFSFile     base.TplName = "repo/settings/lfs_file"
	tplSettingsLFSPointers base.TplName = "repo/settings/lfs_pointers"
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
		if err == models.ErrLFSObjectNotExist {
			ctx.NotFound("LFSFileGet", nil)
			return
		}
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

// LFSPointerFiles will search the repository for pointer files and report which are missing LFS files in the content store
func LFSPointerFiles(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSFileGet", nil)
		return
	}
	ctx.Data["PageIsSettingsLFS"] = true
	binVersion, err := git.BinVersion()
	if err != nil {
		log.Fatal("Error retrieving git version: %v", err)
	}
	ctx.Data["LFSFilesLink"] = ctx.Repo.RepoLink + "/settings/lfs"

	basePath := ctx.Repo.Repository.RepoPath()

	pointerChan := make(chan pointerResult)

	catFileCheckReader, catFileCheckWriter := io.Pipe()
	shasToBatchReader, shasToBatchWriter := io.Pipe()
	catFileBatchReader, catFileBatchWriter := io.Pipe()
	errChan := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(5)

	var numPointers, numAssociated, numNoExist, numAssociatable int

	go func() {
		defer wg.Done()
		pointers := make([]pointerResult, 0, 50)
		for pointer := range pointerChan {
			pointers = append(pointers, pointer)
			if pointer.InRepo {
				numAssociated++
			}
			if !pointer.Exists {
				numNoExist++
			}
			if !pointer.InRepo && pointer.Accessible {
				numAssociatable++
			}
		}
		numPointers = len(pointers)
		ctx.Data["Pointers"] = pointers
		ctx.Data["NumPointers"] = numPointers
		ctx.Data["NumAssociated"] = numAssociated
		ctx.Data["NumAssociatable"] = numAssociatable
		ctx.Data["NumNoExist"] = numNoExist
		ctx.Data["NumNotAssociated"] = numPointers - numAssociated
	}()
	go readCatFileBatch(catFileBatchReader, &wg, pointerChan, ctx.Repo.Repository, ctx.User)
	go doCatFileBatch(shasToBatchReader, catFileBatchWriter, &wg, basePath)
	go readCatFileBatchCheckAllObjects(catFileCheckReader, shasToBatchWriter, &wg)
	if !version.Compare(binVersion, "2.6.0", ">=") {
		revListReader, revListWriter := io.Pipe()
		shasToCheckReader, shasToCheckWriter := io.Pipe()
		wg.Add(2)
		go doCatFileBatchCheck(shasToCheckReader, catFileCheckWriter, &wg, basePath)
		go readRevListAllObjects(revListReader, shasToCheckWriter, &wg)
		go doRevListAllObjects(revListWriter, &wg, basePath, errChan)
	} else {
		go doCatFileBatchCheckAllObjects(catFileCheckWriter, &wg, basePath, errChan)
	}
	wg.Wait()

	select {
	case err, has := <-errChan:
		if has {
			ctx.ServerError("LFSPointerFiles", err)
		}
	default:
	}
	ctx.HTML(200, tplSettingsLFSPointers)
}

type pointerResult struct {
	SHA        string
	Oid        string
	Size       int64
	InRepo     bool
	Exists     bool
	Accessible bool
}

func doRevListAllObjects(revListWriter *io.PipeWriter, wg *sync.WaitGroup, basePath string, errChan chan<- error) {
	defer wg.Done()
	defer revListWriter.Close()

	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	cmd := git.NewCommand("rev-list", "--objects", "--all")
	if err := cmd.RunInDirPipeline(basePath, revListWriter, stderr); err != nil {
		log.Error("git rev-list --objects --all [%s]: %v - %s", basePath, err, errbuf.String())
		err = fmt.Errorf("git rev-list --objects --all [%s]: %v - %s", basePath, err, errbuf.String())
		_ = revListWriter.CloseWithError(err)
		errChan <- err
	}
}

func readRevListAllObjects(revListReader *io.PipeReader, shasToCheckWriter *io.PipeWriter, wg *sync.WaitGroup) {
	defer wg.Done()
	defer revListReader.Close()
	scanner := bufio.NewScanner(revListReader)
	defer func() {
		_ = shasToCheckWriter.CloseWithError(scanner.Err())
	}()
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		fields := strings.Split(line, " ")
		if len(fields) < 2 || len(fields[1]) == 0 {
			continue
		}
		toWrite := []byte(fields[0] + "\n")
		for len(toWrite) > 0 {
			n, err := shasToCheckWriter.Write(toWrite)
			if err != nil {
				_ = revListReader.CloseWithError(err)
				break
			}
			toWrite = toWrite[n:]
		}
	}
}

func doCatFileBatchCheck(shasToCheckReader *io.PipeReader, catFileCheckWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string) {
	defer wg.Done()
	defer shasToCheckReader.Close()
	defer catFileCheckWriter.Close()

	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	cmd := git.NewCommand("cat-file", "--batch-check")
	if err := cmd.RunInDirFullPipeline(tmpBasePath, catFileCheckWriter, stderr, shasToCheckReader); err != nil {
		_ = catFileCheckWriter.CloseWithError(fmt.Errorf("git cat-file --batch-check [%s]: %v - %s", tmpBasePath, err, errbuf.String()))
	}
}

func doCatFileBatchCheckAllObjects(catFileCheckWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string, errChan chan<- error) {
	defer wg.Done()
	defer catFileCheckWriter.Close()

	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	cmd := git.NewCommand("cat-file", "--batch-check", "--batch-all-objects")
	if err := cmd.RunInDirPipeline(tmpBasePath, catFileCheckWriter, stderr); err != nil {
		log.Error("git cat-file --batch-check --batch-all-object [%s]: %v - %s", tmpBasePath, err, errbuf.String())
		err = fmt.Errorf("git cat-file --batch-check --batch-all-object [%s]: %v - %s", tmpBasePath, err, errbuf.String())
		_ = catFileCheckWriter.CloseWithError(err)
		errChan <- err
	}
}

func readCatFileBatchCheckAllObjects(catFileCheckReader *io.PipeReader, shasToBatchWriter *io.PipeWriter, wg *sync.WaitGroup) {
	defer wg.Done()
	defer catFileCheckReader.Close()
	scanner := bufio.NewScanner(catFileCheckReader)
	defer func() {
		_ = shasToBatchWriter.CloseWithError(scanner.Err())
	}()
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		fields := strings.Split(line, " ")
		if len(fields) < 3 || fields[1] != "blob" {
			continue
		}
		size, _ := strconv.Atoi(string(fields[2]))
		if size > 1024 {
			continue
		}
		toWrite := []byte(fields[0] + "\n")
		for len(toWrite) > 0 {
			n, err := shasToBatchWriter.Write(toWrite)
			if err != nil {
				_ = catFileCheckReader.CloseWithError(err)
				break
			}
			toWrite = toWrite[n:]
		}
	}
}

func doCatFileBatch(shasToBatchReader *io.PipeReader, catFileBatchWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string) {
	defer wg.Done()
	defer shasToBatchReader.Close()
	defer catFileBatchWriter.Close()

	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	if err := git.NewCommand("cat-file", "--batch").RunInDirFullPipeline(tmpBasePath, catFileBatchWriter, stderr, shasToBatchReader); err != nil {
		_ = shasToBatchReader.CloseWithError(fmt.Errorf("git rev-list [%s]: %v - %s", tmpBasePath, err, errbuf.String()))
	}
}

func readCatFileBatch(catFileBatchReader *io.PipeReader, wg *sync.WaitGroup, pointerChan chan<- pointerResult, repo *models.Repository, user *models.User) {
	defer wg.Done()
	defer catFileBatchReader.Close()
	contentStore := lfs.ContentStore{BasePath: setting.LFS.ContentPath}

	bufferedReader := bufio.NewReader(catFileBatchReader)
	buf := make([]byte, 1025)
	for {
		// File descriptor line: sha
		sha, err := bufferedReader.ReadString(' ')
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		// Throw away the blob
		if _, err := bufferedReader.ReadString(' '); err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		sizeStr, err := bufferedReader.ReadString('\n')
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		size, err := strconv.Atoi(sizeStr[:len(sizeStr)-1])
		if err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		pointerBuf := buf[:size+1]
		if _, err := io.ReadFull(bufferedReader, pointerBuf); err != nil {
			_ = catFileBatchReader.CloseWithError(err)
			break
		}
		pointerBuf = pointerBuf[:size]
		// Now we need to check if the pointerBuf is an LFS pointer
		pointer := lfs.IsPointerFile(&pointerBuf)
		if pointer == nil {
			continue
		}

		result := pointerResult{
			SHA:  strings.TrimSpace(sha),
			Oid:  pointer.Oid,
			Size: pointer.Size,
		}

		// Then we need to check that this pointer is in the db
		if _, err := repo.GetLFSMetaObjectByOid(pointer.Oid); err != nil {
			if err != models.ErrLFSObjectNotExist {
				_ = catFileBatchReader.CloseWithError(err)
				break
			}
		} else {
			result.InRepo = true
		}

		result.Exists = contentStore.Exists(pointer)

		if result.Exists {
			if !result.InRepo {
				// Can we fix?
				// OK well that's "simple"
				// - we need to check whether current user has access to a repo that has access to the file
				result.Accessible, err = models.LFSObjectAccessible(user, result.Oid)
				if err != nil {
					_ = catFileBatchReader.CloseWithError(err)
					break
				}
			} else {
				result.Accessible = true
			}
		}
		pointerChan <- result
	}
	close(pointerChan)
}

// LFSAutoAssociate auto associates accessible lfs files
func LFSAutoAssociate(ctx *context.Context) {
	if !setting.LFS.StartServer {
		ctx.NotFound("LFSAutoAssociate", nil)
		return
	}
	oids := ctx.QueryStrings("oid")
	metas := make([]*models.LFSMetaObject, len(oids))
	for i, oid := range oids {
		idx := strings.IndexRune(oid, ' ')
		if idx < 0 || idx+1 > len(oid) {
			ctx.ServerError("LFSAutoAssociate", fmt.Errorf("Illegal oid input: %s", oid))
			return
		}
		var err error
		metas[i] = &models.LFSMetaObject{}
		metas[i].Size, err = com.StrTo(oid[idx+1:]).Int64()
		if err != nil {
			ctx.ServerError("LFSAutoAssociate", fmt.Errorf("Illegal oid input: %s %v", oid, err))
			return
		}
		metas[i].Oid = oid[:idx]
		//metas[i].RepositoryID = ctx.Repo.Repository.ID
	}
	if err := models.LFSAutoAssociate(metas, ctx.User, ctx.Repo.Repository.ID); err != nil {
		ctx.ServerError("LFSAutoAssociate", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/lfs")
}
