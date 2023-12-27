package storage

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const multipart_chunk_size int64 = 20000000
const default_expire int = 7200

type MultipartPartID struct {
	Etag  string `json:"etag"`
	Index int    `json:"index"`
}

type MultiPartCommitUpload struct {
	UploadID string            `json:"upload_id"`
	PartIDs  []MultipartPartID `json:"part_ids"`
}

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

func (hwc *HWCloudStorage) GenerateMultipartParts(path string, size int64) (parts []*structs.MultipartObjectPart, abort *structs.MultipartEndpoint, verify *structs.MultipartEndpoint, err error) {
	objectKey := hwc.buildMinioPath(path)
	taskParts := map[int64]obs.Part{}
	uploadID := ""
	//1. list all the multipart tasks
	listMultipart := &obs.ListMultipartUploadsInput{}
	listMultipart.Prefix = objectKey
	listMultipart.Bucket = hwc.bucket
	listResult, err := hwc.hwclient.ListMultipartUploads(listMultipart)
	if err != nil {
		log.Error("lfs[multipart] Failed to list existing multipart task %s and %s", hwc.bucket, objectKey)
		return nil, nil, nil, err
	}
	if len(listResult.Uploads) != 0 {
		//remove all unfinished tasks if multiple tasks are found
		if len(listResult.Uploads) > 1 {
			for _, task := range listResult.Uploads {
				abortRequest := &obs.AbortMultipartUploadInput{}
				abortRequest.Key = objectKey
				abortRequest.Bucket = hwc.bucket
				abortRequest.UploadId = task.UploadId
				_, err = hwc.hwclient.AbortMultipartUpload(abortRequest)
				if err != nil {
					log.Error("lfs[multipart] Failed to abort existing multipart task %s and %s %s", hwc.bucket, objectKey, task.UploadId)
					return nil, nil, nil, err
				}
			}
		} else {
			//find out all finished tasks
			partRequest := &obs.ListPartsInput{}
			partRequest.Key = objectKey
			partRequest.Bucket = hwc.bucket
			partRequest.UploadId = listResult.Uploads[0].UploadId
			uploadID = listResult.Uploads[0].UploadId
			parts, err := hwc.hwclient.ListParts(partRequest)
			if err != nil {
				log.Error("lfs[multipart] Failed to get existing multipart task part %s and %s %s", hwc.bucket, objectKey)
			}
			for _, content := range parts.Parts {
				taskParts[int64(content.PartNumber)] = content
			}
		}
	}
	//2. get and return all unfinished tasks, clean up the task if needed
	//TODO
	//3. Initialize multipart task
	if uploadID == "" {
		log.Trace("lfs[multipart] Starting to create multipart task %s and %s", hwc.bucket, objectKey)
		upload := obs.InitiateMultipartUploadInput{}
		upload.Key = hwc.buildMinioPath(path)
		upload.Bucket = hwc.bucket
		multipart, err := hwc.hwclient.InitiateMultipartUpload(&upload)
		uploadID = multipart.UploadId
		if err != nil {
			return nil, nil, nil, err
		}
	}
	//generate part
	currentPart := int64(0)
	for {
		if currentPart*multipart_chunk_size >= size {
			break
		}
		partSize := size - currentPart*multipart_chunk_size
		if partSize > multipart_chunk_size {
			partSize = multipart_chunk_size
		}
		//check part exists and length matches
		if value, existed := taskParts[currentPart+1]; existed {
			if value.Size == partSize {
				log.Trace("lfs[multipart] Found existing part %d for multipart task %s and %s, will add etag information", currentPart+1, hwc.bucket, objectKey)
				var part = &structs.MultipartObjectPart{
					Index:             int(currentPart) + 1,
					Pos:               currentPart * multipart_chunk_size,
					Size:              partSize,
					Etag:              strings.Trim(value.ETag, "\""),
					MultipartEndpoint: nil,
				}
				parts = append(parts, part)
				currentPart += 1
				continue
			} else {
				log.Trace("lfs[multipart] Found existing part %d while size not matched for multipart task %s and %s", currentPart+1, hwc.bucket, objectKey)
			}
		}
		request := obs.CreateSignedUrlInput{
			Method:  obs.HttpMethodPut,
			Bucket:  hwc.bucket,
			Key:     objectKey,
			Expires: default_expire,
			QueryParams: map[string]string{
				"partNumber": strconv.FormatInt(currentPart+1, 10),
				"uploadId":   uploadID,
			},
		}
		result, errorMessage := hwc.hwclient.CreateSignedUrl(&request)
		if errorMessage != nil {
			return nil, nil, nil, err
		}
		var part = &structs.MultipartObjectPart{
			Index: int(currentPart) + 1,
			Pos:   currentPart * multipart_chunk_size,
			Size:  partSize,
			MultipartEndpoint: &structs.MultipartEndpoint{
				ExpiresIn:         default_expire,
				Href:              result.SignedUrl,
				Method:            http.MethodPut,
				Headers:           nil,
				Params:            nil,
				AggregationParams: nil,
			},
		}
		parts = append(parts, part)
		currentPart += 1
	}
	//generate abort
	//TODO
	//generate verify
	verify = &structs.MultipartEndpoint{
		Params: &map[string]string{
			"upload_id": uploadID,
		},
		AggregationParams: &map[string]string{
			"key":  "part_ids",
			"type": "array",
			"item": "index,etag",
		},
	}
	return parts, nil, verify, nil
}

func (hwc *HWCloudStorage) CommitUpload(path, additionalParameter string) error {
	var param MultiPartCommitUpload
	err := json.Unmarshal([]byte(additionalParameter), &param)
	if err != nil {
		log.Error("lfs[multipart] unable to decode additional parameter", additionalParameter)
		return err
	}
	if len(param.UploadID) == 0 || len(param.PartIDs) == 0 {
		log.Error("lfs[multipart] failed to commit objects, parameter is empty %v", param)
		return errors.New("parameter is empty")
	}
	log.Trace("lfs[multipart] start to commit upload object %v", param)
	//merge multipart
	parts := make([]obs.Part, 0, len(param.PartIDs))
	for _, p := range param.PartIDs {
		parts = append(parts, obs.Part{ETag: p.Etag, PartNumber: p.Index})
	}
	complete := &obs.CompleteMultipartUploadInput{}
	complete.Bucket = hwc.bucket
	complete.Key = hwc.buildMinioPath(path)
	complete.UploadId = param.UploadID
	complete.Parts = parts
	log.Trace("lfs[multipart] Start to merge multipart task %s and %s", hwc.bucket, hwc.buildMinioPath(path))
	_, err = hwc.hwclient.CompleteMultipartUpload(complete)
	if err != nil {
		return err
	}
	//TODO notify CDN to fetch new object
	return nil

}

// URL gets the redirect URL to a file. The presigned link is valid for 5 minutes.
func (hwc *HWCloudStorage) URL(path, name string) (*url.URL, error) {
	queryParameter := map[string]string{"response-content-disposition": "attachment; filename=\"" + url.QueryEscape(quoteEscaper.Replace(name)) + "\""}
	input := &obs.CreateSignedUrlInput{}

	input.Method = obs.HttpMethodGet
	input.Bucket = hwc.bucket
	input.Key = hwc.buildMinioPath(path)
	input.Expires = default_expire
	input.QueryParams = queryParameter
	output, err := hwc.hwclient.CreateSignedUrl(input)
	if err != nil {
		return nil, err
	}

	//NOTE: it will work since CDN will replace hostname back to obs domain and that will make signed url work.
	v, err := url.Parse(output.SignedUrl)
	if err == nil {
		v.Host = hwc.bucketDomain
		v.Scheme = "https"
	}

	return v, err
}

func init() {
	RegisterStorageType(setting.HWCloudStorageType, NewMinioStorage)
}
