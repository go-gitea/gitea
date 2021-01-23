/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2020 MinIO, Inc.
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

package sse

import "encoding/xml"

// ApplySSEByDefault defines default encryption configuration, KMS or SSE. To activate
// KMS, SSEAlgoritm needs to be set to "aws:kms"
// Minio currently does not support Kms.
type ApplySSEByDefault struct {
	KmsMasterKeyID string `xml:"KMSMasterKeyID,omitempty"`
	SSEAlgorithm   string `xml:"SSEAlgorithm"`
}

// Rule layer encapsulates default encryption configuration
type Rule struct {
	Apply ApplySSEByDefault `xml:"ApplyServerSideEncryptionByDefault"`
}

// Configuration is the default encryption configuration structure
type Configuration struct {
	XMLName xml.Name `xml:"ServerSideEncryptionConfiguration"`
	Rules   []Rule   `xml:"Rule"`
}

// NewConfigurationSSES3 initializes a new SSE-S3 configuration
func NewConfigurationSSES3() *Configuration {
	return &Configuration{
		Rules: []Rule{
			{
				Apply: ApplySSEByDefault{
					SSEAlgorithm: "AES256",
				},
			},
		},
	}
}

// NewConfigurationSSEKMS initializes a new SSE-KMS configuration
func NewConfigurationSSEKMS(kmsMasterKey string) *Configuration {
	return &Configuration{
		Rules: []Rule{
			{
				Apply: ApplySSEByDefault{
					KmsMasterKeyID: kmsMasterKey,
					SSEAlgorithm:   "aws:kms",
				},
			},
		},
	}
}
