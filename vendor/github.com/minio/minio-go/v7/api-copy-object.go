/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2017, 2018 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package minio

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
)

// CopyObject - copy a source object into a new object
func (c Client) CopyObject(ctx context.Context, dst CopyDestOptions, src CopySrcOptions) (UploadInfo, error) {
	if err := src.validate(); err != nil {
		return UploadInfo{}, err
	}

	if err := dst.validate(); err != nil {
		return UploadInfo{}, err
	}

	header := make(http.Header)
	dst.Marshal(header)
	src.Marshal(header)

	resp, err := c.executeMethod(ctx, http.MethodPut, requestMetadata{
		bucketName:   dst.Bucket,
		objectName:   dst.Object,
		customHeader: header,
	})
	if err != nil {
		return UploadInfo{}, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return UploadInfo{}, httpRespToErrorResponse(resp, dst.Bucket, dst.Object)
	}

	// Update the progress properly after successful copy.
	if dst.Progress != nil {
		io.Copy(ioutil.Discard, io.LimitReader(dst.Progress, dst.Size))
	}

	cpObjRes := copyObjectResult{}
	if err = xmlDecoder(resp.Body, &cpObjRes); err != nil {
		return UploadInfo{}, err
	}

	// extract lifecycle expiry date and rule ID
	expTime, ruleID := amzExpirationToExpiryDateRuleID(resp.Header.Get(amzExpiration))

	return UploadInfo{
		Bucket:           dst.Bucket,
		Key:              dst.Object,
		LastModified:     cpObjRes.LastModified,
		ETag:             trimEtag(resp.Header.Get("ETag")),
		VersionID:        resp.Header.Get(amzVersionID),
		Expiration:       expTime,
		ExpirationRuleID: ruleID,
	}, nil
}
