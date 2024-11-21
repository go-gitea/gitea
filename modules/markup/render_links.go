// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type Links struct {
	AbsolutePrefix bool   // add absolute URL prefix to auto-resolved links like "#issue", but not for pre-provided links and medias
	Base           string // base prefix for pre-provided links and medias (images, videos), usually it is the path to the repo
	BranchPath     string // actually it is the ref path, eg: "branch/features/feat-12", "tag/v1.0"
	TreePath       string // the dir of the file, eg: "doc" if the file "doc/CHANGE.md" is being rendered
}

func (l *Links) Prefix() string {
	if l.AbsolutePrefix {
		return setting.AppURL
	}
	return setting.AppSubURL
}

func (l *Links) HasBranchInfo() bool {
	return l.BranchPath != ""
}

func (l *Links) SrcLink() string {
	return util.URLJoin(l.Base, "src", l.BranchPath, l.TreePath)
}

func (l *Links) MediaLink() string {
	return util.URLJoin(l.Base, "media", l.BranchPath, l.TreePath)
}

func (l *Links) RawLink() string {
	return util.URLJoin(l.Base, "raw", l.BranchPath, l.TreePath)
}

func (l *Links) WikiLink() string {
	return util.URLJoin(l.Base, "wiki")
}

func (l *Links) WikiRawLink() string {
	return util.URLJoin(l.Base, "wiki/raw")
}

func (l *Links) ResolveMediaLink(isWiki bool) string {
	if isWiki {
		return l.WikiRawLink()
	} else if l.HasBranchInfo() {
		return l.MediaLink()
	}
	return l.Base
}
