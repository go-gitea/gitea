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

package minio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/minio/minio-go/v7/pkg/s3utils"
)

// SetBucketNotification saves a new bucket notification with a context to control cancellations and timeouts.
func (c Client) SetBucketNotification(ctx context.Context, bucketName string, config notification.Configuration) error {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return err
	}

	// Get resources properly escaped and lined up before
	// using them in http request.
	urlValues := make(url.Values)
	urlValues.Set("notification", "")

	notifBytes, err := xml.Marshal(&config)
	if err != nil {
		return err
	}

	notifBuffer := bytes.NewReader(notifBytes)
	reqMetadata := requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentBody:      notifBuffer,
		contentLength:    int64(len(notifBytes)),
		contentMD5Base64: sumMD5Base64(notifBytes),
		contentSHA256Hex: sum256Hex(notifBytes),
	}

	// Execute PUT to upload a new bucket notification.
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

// RemoveAllBucketNotification - Remove bucket notification clears all previously specified config
func (c Client) RemoveAllBucketNotification(ctx context.Context, bucketName string) error {
	return c.SetBucketNotification(ctx, bucketName, notification.Configuration{})
}

// GetBucketNotification returns current bucket notification configuration
func (c Client) GetBucketNotification(ctx context.Context, bucketName string) (bucketNotification notification.Configuration, err error) {
	// Input validation.
	if err := s3utils.CheckValidBucketName(bucketName); err != nil {
		return notification.Configuration{}, err
	}
	return c.getBucketNotification(ctx, bucketName)
}

// Request server for notification rules.
func (c Client) getBucketNotification(ctx context.Context, bucketName string) (notification.Configuration, error) {
	urlValues := make(url.Values)
	urlValues.Set("notification", "")

	// Execute GET on bucket to list objects.
	resp, err := c.executeMethod(ctx, http.MethodGet, requestMetadata{
		bucketName:       bucketName,
		queryValues:      urlValues,
		contentSHA256Hex: emptySHA256Hex,
	})

	defer closeResponse(resp)
	if err != nil {
		return notification.Configuration{}, err
	}
	return processBucketNotificationResponse(bucketName, resp)

}

// processes the GetNotification http response from the server.
func processBucketNotificationResponse(bucketName string, resp *http.Response) (notification.Configuration, error) {
	if resp.StatusCode != http.StatusOK {
		errResponse := httpRespToErrorResponse(resp, bucketName, "")
		return notification.Configuration{}, errResponse
	}
	var bucketNotification notification.Configuration
	err := xmlDecoder(resp.Body, &bucketNotification)
	if err != nil {
		return notification.Configuration{}, err
	}
	return bucketNotification, nil
}

// ListenNotification listen for all events, this is a MinIO specific API
func (c Client) ListenNotification(ctx context.Context, prefix, suffix string, events []string) <-chan notification.Info {
	return c.ListenBucketNotification(ctx, "", prefix, suffix, events)
}

// ListenBucketNotification listen for bucket events, this is a MinIO specific API
func (c Client) ListenBucketNotification(ctx context.Context, bucketName, prefix, suffix string, events []string) <-chan notification.Info {
	notificationInfoCh := make(chan notification.Info, 1)
	const notificationCapacity = 4 * 1024 * 1024
	notificationEventBuffer := make([]byte, notificationCapacity)
	// Only success, start a routine to start reading line by line.
	go func(notificationInfoCh chan<- notification.Info) {
		defer close(notificationInfoCh)

		// Validate the bucket name.
		if bucketName != "" {
			if err := s3utils.CheckValidBucketName(bucketName); err != nil {
				select {
				case notificationInfoCh <- notification.Info{
					Err: err,
				}:
				case <-ctx.Done():
				}
				return
			}
		}

		// Check ARN partition to verify if listening bucket is supported
		if s3utils.IsAmazonEndpoint(*c.endpointURL) || s3utils.IsGoogleEndpoint(*c.endpointURL) {
			select {
			case notificationInfoCh <- notification.Info{
				Err: errAPINotSupported("Listening for bucket notification is specific only to `minio` server endpoints"),
			}:
			case <-ctx.Done():
			}
			return
		}

		// Continuously run and listen on bucket notification.
		// Create a done channel to control 'ListObjects' go routine.
		retryDoneCh := make(chan struct{}, 1)

		// Indicate to our routine to exit cleanly upon return.
		defer close(retryDoneCh)

		// Prepare urlValues to pass into the request on every loop
		urlValues := make(url.Values)
		urlValues.Set("prefix", prefix)
		urlValues.Set("suffix", suffix)
		urlValues["events"] = events

		// Wait on the jitter retry loop.
		for range c.newRetryTimerContinous(time.Second, time.Second*30, MaxJitter, retryDoneCh) {
			// Execute GET on bucket to list objects.
			resp, err := c.executeMethod(ctx, http.MethodGet, requestMetadata{
				bucketName:       bucketName,
				queryValues:      urlValues,
				contentSHA256Hex: emptySHA256Hex,
			})
			if err != nil {
				select {
				case notificationInfoCh <- notification.Info{
					Err: err,
				}:
				case <-ctx.Done():
				}
				return
			}

			// Validate http response, upon error return quickly.
			if resp.StatusCode != http.StatusOK {
				errResponse := httpRespToErrorResponse(resp, bucketName, "")
				select {
				case notificationInfoCh <- notification.Info{
					Err: errResponse,
				}:
				case <-ctx.Done():
				}
				return
			}

			// Initialize a new bufio scanner, to read line by line.
			bio := bufio.NewScanner(resp.Body)

			// Use a higher buffer to support unexpected
			// caching done by proxies
			bio.Buffer(notificationEventBuffer, notificationCapacity)
			var json = jsoniter.ConfigCompatibleWithStandardLibrary

			// Unmarshal each line, returns marshaled values.
			for bio.Scan() {
				var notificationInfo notification.Info
				if err = json.Unmarshal(bio.Bytes(), &notificationInfo); err != nil {
					// Unexpected error during json unmarshal, send
					// the error to caller for actionable as needed.
					select {
					case notificationInfoCh <- notification.Info{
						Err: err,
					}:
					case <-ctx.Done():
						return
					}
					closeResponse(resp)
					continue
				}
				// Send notificationInfo
				select {
				case notificationInfoCh <- notificationInfo:
				case <-ctx.Done():
					closeResponse(resp)
					return
				}
			}

			if err = bio.Err(); err != nil {
				select {
				case notificationInfoCh <- notification.Info{
					Err: err,
				}:
				case <-ctx.Done():
					return
				}
			}

			// Close current connection before looping further.
			closeResponse(resp)

		}
	}(notificationInfoCh)

	// Returns the notification info channel, for caller to start reading from.
	return notificationInfoCh
}
