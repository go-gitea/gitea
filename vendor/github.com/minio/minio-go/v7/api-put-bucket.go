/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2015-2020 MinIO, Inc.
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
	"bytes"
	"context"
	"encoding/xml"
	"net/http"

	"github.com/minio/minio-go/v7/pkg/s3utils"
)

/// Bucket operations
func (c Client) makeBucket(ctx context.Context, bucketName string, opts MakeBucketOptions) (err error) {
	// Validate the input arguments.
	if err := s3utils.CheckValidBucketNameStrict(bucketName); err != nil {
		return err
	}

	err = c.doMakeBucket(ctx, bucketName, opts.Region, opts.ObjectLocking)
	if err != nil && (opts.Region == "" || opts.Region == "us-east-1") {
		if resp, ok := err.(ErrorResponse); ok && resp.Code == "AuthorizationHeaderMalformed" && resp.Region != "" {
			err = c.doMakeBucket(ctx, bucketName, resp.Region, opts.ObjectLocking)
		}
	}
	return err
}

func (c Client) doMakeBucket(ctx context.Context, bucketName string, location string, objectLockEnabled bool) (err error) {
	defer func() {
		// Save the location into cache on a successful makeBucket response.
		if err == nil {
			c.bucketLocCache.Set(bucketName, location)
		}
	}()

	// If location is empty, treat is a default region 'us-east-1'.
	if location == "" {
		location = "us-east-1"
		// For custom region clients, default
		// to custom region instead not 'us-east-1'.
		if c.region != "" {
			location = c.region
		}
	}
	// PUT bucket request metadata.
	reqMetadata := requestMetadata{
		bucketName:     bucketName,
		bucketLocation: location,
	}

	if objectLockEnabled {
		headers := make(http.Header)
		headers.Add("x-amz-bucket-object-lock-enabled", "true")
		reqMetadata.customHeader = headers
	}

	// If location is not 'us-east-1' create bucket location config.
	if location != "us-east-1" && location != "" {
		createBucketConfig := createBucketConfiguration{}
		createBucketConfig.Location = location
		var createBucketConfigBytes []byte
		createBucketConfigBytes, err = xml.Marshal(createBucketConfig)
		if err != nil {
			return err
		}
		reqMetadata.contentMD5Base64 = sumMD5Base64(createBucketConfigBytes)
		reqMetadata.contentSHA256Hex = sum256Hex(createBucketConfigBytes)
		reqMetadata.contentBody = bytes.NewReader(createBucketConfigBytes)
		reqMetadata.contentLength = int64(len(createBucketConfigBytes))
	}

	// Execute PUT to create a new bucket.
	resp, err := c.executeMethod(ctx, http.MethodPut, reqMetadata)
	defer closeResponse(resp)
	if err != nil {
		return err
	}

	if resp != nil {
		if resp.StatusCode != http.StatusOK {
			return httpRespToErrorResponse(resp, bucketName, "")
		}
	}

	// Success.
	return nil
}

// MakeBucketOptions holds all options to tweak bucket creation
type MakeBucketOptions struct {
	// Bucket location
	Region string
	// Enable object locking
	ObjectLocking bool
}

// MakeBucket creates a new bucket with bucketName with a context to control cancellations and timeouts.
//
// Location is an optional argument, by default all buckets are
// created in US Standard Region.
//
// For Amazon S3 for more supported regions - http://docs.aws.amazon.com/general/latest/gr/rande.html
// For Google Cloud Storage for more supported regions - https://cloud.google.com/storage/docs/bucket-locations
func (c Client) MakeBucket(ctx context.Context, bucketName string, opts MakeBucketOptions) (err error) {
	return c.makeBucket(ctx, bucketName, opts)
}
