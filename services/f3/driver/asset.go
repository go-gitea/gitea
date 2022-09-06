// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"
	"io"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/attachment"

	"github.com/google/uuid"
	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	"lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type Asset struct {
	repo_model.Attachment
	DownloadFunc func() io.ReadCloser
}

func AssetConverter(f *repo_model.Attachment) *Asset {
	return &Asset{
		Attachment: *f,
	}
}

func (o Asset) GetID() int64 {
	return o.ID
}

func (o *Asset) SetID(id int64) {
	o.ID = id
}

func (o *Asset) IsNil() bool {
	return o.ID == 0
}

func (o *Asset) Equals(other *Asset) bool {
	return o.Name == other.Name
}

func (o *Asset) ToFormat() *format.ReleaseAsset {
	return &format.ReleaseAsset{
		Common:        format.Common{Index: o.ID},
		Name:          o.Name,
		Size:          int(o.Size),
		DownloadCount: int(o.DownloadCount),
		Created:       o.CreatedUnix.AsTime(),
		DownloadURL:   o.DownloadURL(),
		DownloadFunc:  o.DownloadFunc,
	}
}

func (o *Asset) FromFormat(asset *format.ReleaseAsset) {
	*o = Asset{
		Attachment: repo_model.Attachment{
			ID:            asset.GetID(),
			Name:          asset.Name,
			Size:          int64(asset.Size),
			DownloadCount: int64(asset.DownloadCount),
			CreatedUnix:   timeutil.TimeStamp(asset.Created.Unix()),
		},
		DownloadFunc: asset.DownloadFunc,
	}
}

type AssetProvider struct {
	g *Gitea
}

func (o *AssetProvider) ToFormat(asset *Asset) *format.ReleaseAsset {
	httpClient := o.g.GetNewMigrationHTTPClient()()
	a := asset.ToFormat()
	a.DownloadFunc = func() io.ReadCloser {
		o.g.GetLogger().Debug("download from %s", asset.DownloadURL())
		req, err := http.NewRequest("GET", asset.DownloadURL(), nil)
		if err != nil {
			panic(err)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			panic(fmt.Errorf("while downloading %s %w", asset.DownloadURL(), err))
		}

		// resp.Body is closed by the consumer
		return resp.Body
	}
	return a
}

func (o *AssetProvider) FromFormat(p *format.ReleaseAsset) *Asset {
	var asset Asset
	asset.FromFormat(p)
	return &asset
}

func (o *AssetProvider) ProcessObject(user *User, project *Project, release *Release, asset *Asset) {
}

func (o *AssetProvider) GetObjects(user *User, project *Project, release *Release, page int) []*Asset {
	if page > 1 {
		return []*Asset{}
	}
	r, err := repo_model.GetReleaseByID(o.g.ctx, release.GetID())
	if err != nil {
		panic(err)
	}
	if err := r.LoadAttributes(); err != nil {
		panic(fmt.Errorf("error while listing assets: %v", err))
	}

	return util.ConvertMap[*repo_model.Attachment, *Asset](r.Attachments, AssetConverter)
}

func (o *AssetProvider) Get(user *User, project *Project, release *Release, exemplar *Asset) *Asset {
	id := exemplar.GetID()
	asset, err := repo_model.GetAttachmentByID(o.g.ctx, id)
	if repo_model.IsErrAttachmentNotExist(err) {
		return &Asset{}
	}
	if err != nil {
		panic(err)
	}
	return AssetConverter(asset)
}

func (o *AssetProvider) Put(user *User, project *Project, release *Release, asset *Asset) *Asset {
	asset.ID = 0
	asset.UploaderID = user.GetID()
	asset.RepoID = project.GetID()
	asset.ReleaseID = release.GetID()
	asset.UUID = uuid.New().String()

	download := asset.DownloadFunc()
	defer download.Close()
	a, err := attachment.NewAttachment(&asset.Attachment, download)
	if err != nil {
		panic(err)
	}
	return o.Get(user, project, release, AssetConverter(a))
}

func (o *AssetProvider) Delete(user *User, project *Project, release *Release, asset *Asset) *Asset {
	a := o.Get(user, project, release, asset)
	if !a.IsNil() {
		err := repo_model.DeleteAttachment(&a.Attachment, true)
		if err != nil {
			panic(err)
		}
	}
	return a
}
