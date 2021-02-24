/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2020 MinIO, Inc.
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
	"net/url"

	"github.com/minio/minio-go/v7/pkg/s3utils"
)

// SetBucketVersioning sets a bucket versioning configuration
func (c Client) SetBucketVersioning(ctx context.Context, bucketName string, config BucketVersioningConfiguration) error {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return err
	}

	buf, err := xml.Marshal(config)
	if err != nil {
		return err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("versioning", "")

	reqMetadata := requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentBody:      bytes.NewReader(buf),
		contentLength:    int64(len(buf)),
		contentMD5Base64: sumMD5Base64(buf),
		contentSHA256Hex: sum256Hex(buf),
	}

	// Execute PUT to set a bucket versioning.
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
	return nil
}

// EnableVersioning - enable object versioning in given bucket.
func (c Client) EnableVersioning(ctx context.Context, bucketName string) error {
	return c.SetBucketVersioning(ctx, bucketName, BucketVersioningConfiguration{Status: "Enabled"})
}

// SuspendVersioning - suspend object versioning in given bucket.
func (c Client) SuspendVersioning(ctx context.Context, bucketName string) error {
	return c.SetBucketVersioning(ctx, bucketName, BucketVersioningConfiguration{Status: "Suspended"})
}

// BucketVersioningConfiguration is the versioning configuration structure
type BucketVersioningConfiguration struct {
	XMLName   xml.Name `xml:"VersioningConfiguration"`
	Status    string   `xml:"Status"`
	MFADelete string   `xml:"MfaDelete,omitempty"`
}

// Various supported states
const (
	Enabled = "Enabled"
	// Disabled  State = "Disabled" only used by MFA Delete not supported yet.
	Suspended = "Suspended"
)

// Enabled returns true if bucket versioning is enabled
func (b BucketVersioningConfiguration) Enabled() bool {
	return b.Status == Enabled
}

// Suspended returns true if bucket versioning is suspended
func (b BucketVersioningConfiguration) Suspended() bool {
	return b.Status == Suspended
}

// GetBucketVersioning gets the versioning configuration on
// an existing bucket with a context to control cancellations and timeouts.
func (c Client) GetBucketVersioning(ctx context.Context, bucketName string) (BucketVersioningConfiguration, error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return BucketVersioningConfiguration{}, err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("versioning", "")

	// Execute GET on bucket to get the versioning configuration.
	resp, err := c.executeMethod(ctx, http.MethodGet, requestMetadata{
		bucketName:  bucketName,
		queryValues: urlValues,
	})

	defer closeResponse(resp)
	if err != nil {
		return BucketVersioningConfiguration{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return BucketVersioningConfiguration{}, httpRespToErrorResponse(resp, bucketName, "")
	}

	versioningConfig := BucketVersioningConfiguration{}
	if err = xmlDecoder(resp.Body, &versioningConfig); err != nil {
		return versioningConfig, err
	}

	return versioningConfig, nil
}
