/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2017-2020 MinIO, Inc.
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

package notification

// Indentity represents the user id, this is a compliance field.
type identity struct {
	PrincipalID string `json:"principalId"`
}

//  event bucket metadata.
type bucketMeta struct {
	Name          string   `json:"name"`
	OwnerIdentity identity `json:"ownerIdentity"`
	ARN           string   `json:"arn"`
}

//  event object metadata.
type objectMeta struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size,omitempty"`
	ETag         string            `json:"eTag,omitempty"`
	ContentType  string            `json:"contentType,omitempty"`
	UserMetadata map[string]string `json:"userMetadata,omitempty"`
	VersionID    string            `json:"versionId,omitempty"`
	Sequencer    string            `json:"sequencer"`
}

//  event server specific metadata.
type eventMeta struct {
	SchemaVersion   string     `json:"s3SchemaVersion"`
	ConfigurationID string     `json:"configurationId"`
	Bucket          bucketMeta `json:"bucket"`
	Object          objectMeta `json:"object"`
}

// sourceInfo represents information on the client that
// triggered the event notification.
type sourceInfo struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	UserAgent string `json:"userAgent"`
}

// Event represents an Amazon an S3 bucket notification event.
type Event struct {
	EventVersion      string            `json:"eventVersion"`
	EventSource       string            `json:"eventSource"`
	AwsRegion         string            `json:"awsRegion"`
	EventTime         string            `json:"eventTime"`
	EventName         string            `json:"eventName"`
	UserIdentity      identity          `json:"userIdentity"`
	RequestParameters map[string]string `json:"requestParameters"`
	ResponseElements  map[string]string `json:"responseElements"`
	S3                eventMeta         `json:"s3"`
	Source            sourceInfo        `json:"source"`
}

// Info - represents the collection of notification events, additionally
// also reports errors if any while listening on bucket notifications.
type Info struct {
	Records []Event
	Err     error
}
