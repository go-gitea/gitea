// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package compare

import (
	"fmt"
	"path"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func SetPathsCompareContext(ctx *context.Context, base *git.Commit, head *git.Commit, headTarget string) {
	sourcePath := setting.AppSubURL + "/%s/src/commit/%s"
	rawPath := setting.AppSubURL + "/%s/raw/commit/%s"

	ctx.Data["SourcePath"] = fmt.Sprintf(sourcePath, headTarget, head.ID)
	ctx.Data["RawPath"] = fmt.Sprintf(rawPath, headTarget, head.ID)
	if base != nil {
		baseTarget := path.Join(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
		ctx.Data["BeforeSourcePath"] = fmt.Sprintf(sourcePath, baseTarget, base.ID)
		ctx.Data["BeforeRawPath"] = fmt.Sprintf(rawPath, baseTarget, base.ID)
	}
}

func SetImageCompareContext(ctx *context.Context, base *git.Commit, head *git.Commit) {
	ctx.Data["IsImageFile"] = head.IsImageFile
	ctx.Data["ImageInfoBase"] = func(name string) *git.ImageMetaData {
		if base == nil {
			return nil
		}
		result, err := base.ImageInfo(name)
		if err != nil {
			log.Error("ImageInfo failed: %v", err)
			return nil
		}
		return result
	}
	ctx.Data["FileExistsInBaseCommit"] = func(filename string) bool {
		if base == nil {
			return false
		}
		result, err := base.HasFile(filename)
		if err != nil {
			log.Error(
				"Error while checking if file \"%s\" exists in base commit \"%s\" (repo: %s): %v",
				filename,
				base,
				ctx.Repo.GitRepo.Path,
				err)
			return false
		}
		return result
	}
	ctx.Data["ImageInfo"] = func(name string) *git.ImageMetaData {
		result, err := head.ImageInfo(name)
		if err != nil {
			log.Error("ImageInfo failed: %v", err)
			return nil
		}
		return result
	}
}
