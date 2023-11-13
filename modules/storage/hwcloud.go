package storage

import (
	"context"
	"fmt"
	"net/url"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"

	"code.gitea.io/gitea/modules/setting"
)

// NewHWCloudStorage returns a hwcloud storage
func NewHWCloudStorage(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	m, err := NewMinioStorage(ctx, cfg)
	if err != nil {
		return nil, err
	}

	obsCfg := &cfg.MinioConfig

	cli, err := obs.New(obsCfg.AccessKeyID, obsCfg.SecretAccessKey, obsCfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("new obs client failed, err:%s", err.Error())
	}

	return &HWCloudStorage{
		hwclient:     cli,
		bucketDomain: cfg.MinioConfig.BucketDomain,
		MinioStorage: m.(*MinioStorage),
	}, nil
}

type HWCloudStorage struct {
	hwclient     *obs.ObsClient
	bucketDomain string

	*MinioStorage
}

// URL gets the redirect URL to a file. The presigned link is valid for 5 minutes.
func (hwc *HWCloudStorage) URL(path, name string) (*url.URL, error) {
	input := &obs.CreateSignedUrlInput{}

	input.Method = obs.HttpMethodGet
	input.Bucket = hwc.bucket
	input.Key = hwc.buildMinioPath(path)
	input.Expires = 3600

	output, err := hwc.hwclient.CreateSignedUrl(input)
	if err != nil {
		return nil, err
	}

	v, err := url.Parse(output.SignedUrl)
	if err == nil {
		v.Host = hwc.bucketDomain
		v.Scheme = "http"
	}

	return v, err
}

func init() {
	RegisterStorageType(setting.HWCloudStorageType, NewMinioStorage)
}
