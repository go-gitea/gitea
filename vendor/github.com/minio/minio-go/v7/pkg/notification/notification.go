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

package notification

import (
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/minio/minio-go/v7/pkg/set"
)

// EventType is a S3 notification event associated to the bucket notification configuration
type EventType string

// The role of all event types are described in :
// 	http://docs.aws.amazon.com/AmazonS3/latest/dev/NotificationHowTo.html#notification-how-to-event-types-and-destinations
const (
	ObjectCreatedAll                     EventType = "s3:ObjectCreated:*"
	ObjectCreatedPut                               = "s3:ObjectCreated:Put"
	ObjectCreatedPost                              = "s3:ObjectCreated:Post"
	ObjectCreatedCopy                              = "s3:ObjectCreated:Copy"
	ObjectCreatedCompleteMultipartUpload           = "s3:ObjectCreated:CompleteMultipartUpload"
	ObjectAccessedGet                              = "s3:ObjectAccessed:Get"
	ObjectAccessedHead                             = "s3:ObjectAccessed:Head"
	ObjectAccessedAll                              = "s3:ObjectAccessed:*"
	ObjectRemovedAll                               = "s3:ObjectRemoved:*"
	ObjectRemovedDelete                            = "s3:ObjectRemoved:Delete"
	ObjectRemovedDeleteMarkerCreated               = "s3:ObjectRemoved:DeleteMarkerCreated"
	ObjectReducedRedundancyLostObject              = "s3:ReducedRedundancyLostObject"
	BucketCreatedAll                               = "s3:BucketCreated:*"
	BucketRemovedAll                               = "s3:BucketRemoved:*"
)

// FilterRule - child of S3Key, a tag in the notification xml which
// carries suffix/prefix filters
type FilterRule struct {
	Name  string `xml:"Name"`
	Value string `xml:"Value"`
}

// S3Key - child of Filter, a tag in the notification xml which
// carries suffix/prefix filters
type S3Key struct {
	FilterRules []FilterRule `xml:"FilterRule,omitempty"`
}

// Filter - a tag in the notification xml structure which carries
// suffix/prefix filters
type Filter struct {
	S3Key S3Key `xml:"S3Key,omitempty"`
}

// Arn - holds ARN information that will be sent to the web service,
// ARN desciption can be found in http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html
type Arn struct {
	Partition string
	Service   string
	Region    string
	AccountID string
	Resource  string
}

// NewArn creates new ARN based on the given partition, service, region, account id and resource
func NewArn(partition, service, region, accountID, resource string) Arn {
	return Arn{Partition: partition,
		Service:   service,
		Region:    region,
		AccountID: accountID,
		Resource:  resource}
}

// String returns the string format of the ARN
func (arn Arn) String() string {
	return "arn:" + arn.Partition + ":" + arn.Service + ":" + arn.Region + ":" + arn.AccountID + ":" + arn.Resource
}

// Config - represents one single notification configuration
// such as topic, queue or lambda configuration.
type Config struct {
	ID     string      `xml:"Id,omitempty"`
	Arn    Arn         `xml:"-"`
	Events []EventType `xml:"Event"`
	Filter *Filter     `xml:"Filter,omitempty"`
}

// NewConfig creates one notification config and sets the given ARN
func NewConfig(arn Arn) Config {
	return Config{Arn: arn, Filter: &Filter{}}
}

// AddEvents adds one event to the current notification config
func (t *Config) AddEvents(events ...EventType) {
	t.Events = append(t.Events, events...)
}

// AddFilterSuffix sets the suffix configuration to the current notification config
func (t *Config) AddFilterSuffix(suffix string) {
	if t.Filter == nil {
		t.Filter = &Filter{}
	}
	newFilterRule := FilterRule{Name: "suffix", Value: suffix}
	// Replace any suffix rule if existing and add to the list otherwise
	for index := range t.Filter.S3Key.FilterRules {
		if t.Filter.S3Key.FilterRules[index].Name == "suffix" {
			t.Filter.S3Key.FilterRules[index] = newFilterRule
			return
		}
	}
	t.Filter.S3Key.FilterRules = append(t.Filter.S3Key.FilterRules, newFilterRule)
}

// AddFilterPrefix sets the prefix configuration to the current notification config
func (t *Config) AddFilterPrefix(prefix string) {
	if t.Filter == nil {
		t.Filter = &Filter{}
	}
	newFilterRule := FilterRule{Name: "prefix", Value: prefix}
	// Replace any prefix rule if existing and add to the list otherwise
	for index := range t.Filter.S3Key.FilterRules {
		if t.Filter.S3Key.FilterRules[index].Name == "prefix" {
			t.Filter.S3Key.FilterRules[index] = newFilterRule
			return
		}
	}
	t.Filter.S3Key.FilterRules = append(t.Filter.S3Key.FilterRules, newFilterRule)
}

// EqualEventTypeList tells whether a and b contain the same events
func EqualEventTypeList(a, b []EventType) bool {
	if len(a) != len(b) {
		return false
	}
	setA := set.NewStringSet()
	for _, i := range a {
		setA.Add(string(i))
	}

	setB := set.NewStringSet()
	for _, i := range b {
		setB.Add(string(i))
	}

	return setA.Difference(setB).IsEmpty()
}

// EqualFilterRuleList tells whether a and b contain the same filters
func EqualFilterRuleList(a, b []FilterRule) bool {
	if len(a) != len(b) {
		return false
	}

	setA := set.NewStringSet()
	for _, i := range a {
		setA.Add(fmt.Sprintf("%s-%s", i.Name, i.Value))
	}

	setB := set.NewStringSet()
	for _, i := range b {
		setB.Add(fmt.Sprintf("%s-%s", i.Name, i.Value))
	}

	return setA.Difference(setB).IsEmpty()
}

// Equal returns whether this `Config` is equal to another defined by the passed parameters
func (t *Config) Equal(events []EventType, prefix, suffix string) bool {
	if t == nil {
		return false
	}

	// Compare events
	passEvents := EqualEventTypeList(t.Events, events)

	// Compare filters
	var newFilterRules []FilterRule
	if prefix != "" {
		newFilterRules = append(newFilterRules, FilterRule{Name: "prefix", Value: prefix})
	}
	if suffix != "" {
		newFilterRules = append(newFilterRules, FilterRule{Name: "suffix", Value: suffix})
	}

	var currentFilterRules []FilterRule
	if t.Filter != nil {
		currentFilterRules = t.Filter.S3Key.FilterRules
	}

	passFilters := EqualFilterRuleList(currentFilterRules, newFilterRules)
	return passEvents && passFilters
}

// TopicConfig carries one single topic notification configuration
type TopicConfig struct {
	Config
	Topic string `xml:"Topic"`
}

// QueueConfig carries one single queue notification configuration
type QueueConfig struct {
	Config
	Queue string `xml:"Queue"`
}

// LambdaConfig carries one single cloudfunction notification configuration
type LambdaConfig struct {
	Config
	Lambda string `xml:"CloudFunction"`
}

// Configuration - the struct that represents the whole XML to be sent to the web service
type Configuration struct {
	XMLName       xml.Name       `xml:"NotificationConfiguration"`
	LambdaConfigs []LambdaConfig `xml:"CloudFunctionConfiguration"`
	TopicConfigs  []TopicConfig  `xml:"TopicConfiguration"`
	QueueConfigs  []QueueConfig  `xml:"QueueConfiguration"`
}

// AddTopic adds a given topic config to the general bucket notification config
func (b *Configuration) AddTopic(topicConfig Config) bool {
	newTopicConfig := TopicConfig{Config: topicConfig, Topic: topicConfig.Arn.String()}
	for _, n := range b.TopicConfigs {
		// If new config matches existing one
		if n.Topic == newTopicConfig.Arn.String() && newTopicConfig.Filter == n.Filter {

			existingConfig := set.NewStringSet()
			for _, v := range n.Events {
				existingConfig.Add(string(v))
			}

			newConfig := set.NewStringSet()
			for _, v := range topicConfig.Events {
				newConfig.Add(string(v))
			}

			if !newConfig.Intersection(existingConfig).IsEmpty() {
				return false
			}
		}
	}
	b.TopicConfigs = append(b.TopicConfigs, newTopicConfig)
	return true
}

// AddQueue adds a given queue config to the general bucket notification config
func (b *Configuration) AddQueue(queueConfig Config) bool {
	newQueueConfig := QueueConfig{Config: queueConfig, Queue: queueConfig.Arn.String()}
	for _, n := range b.QueueConfigs {
		if n.Queue == newQueueConfig.Arn.String() && newQueueConfig.Filter == n.Filter {

			existingConfig := set.NewStringSet()
			for _, v := range n.Events {
				existingConfig.Add(string(v))
			}

			newConfig := set.NewStringSet()
			for _, v := range queueConfig.Events {
				newConfig.Add(string(v))
			}

			if !newConfig.Intersection(existingConfig).IsEmpty() {
				return false
			}
		}
	}
	b.QueueConfigs = append(b.QueueConfigs, newQueueConfig)
	return true
}

// AddLambda adds a given lambda config to the general bucket notification config
func (b *Configuration) AddLambda(lambdaConfig Config) bool {
	newLambdaConfig := LambdaConfig{Config: lambdaConfig, Lambda: lambdaConfig.Arn.String()}
	for _, n := range b.LambdaConfigs {
		if n.Lambda == newLambdaConfig.Arn.String() && newLambdaConfig.Filter == n.Filter {

			existingConfig := set.NewStringSet()
			for _, v := range n.Events {
				existingConfig.Add(string(v))
			}

			newConfig := set.NewStringSet()
			for _, v := range lambdaConfig.Events {
				newConfig.Add(string(v))
			}

			if !newConfig.Intersection(existingConfig).IsEmpty() {
				return false
			}
		}
	}
	b.LambdaConfigs = append(b.LambdaConfigs, newLambdaConfig)
	return true
}

// RemoveTopicByArn removes all topic configurations that match the exact specified ARN
func (b *Configuration) RemoveTopicByArn(arn Arn) {
	var topics []TopicConfig
	for _, topic := range b.TopicConfigs {
		if topic.Topic != arn.String() {
			topics = append(topics, topic)
		}
	}
	b.TopicConfigs = topics
}

// ErrNoConfigMatch is returned when a notification configuration (sqs,sns,lambda) is not found when trying to delete
var ErrNoConfigMatch = errors.New("no notification configuration matched")

// RemoveTopicByArnEventsPrefixSuffix removes a topic configuration that match the exact specified ARN, events, prefix and suffix
func (b *Configuration) RemoveTopicByArnEventsPrefixSuffix(arn Arn, events []EventType, prefix, suffix string) error {
	removeIndex := -1
	for i, v := range b.TopicConfigs {
		// if it matches events and filters, mark the index for deletion
		if v.Topic == arn.String() && v.Config.Equal(events, prefix, suffix) {
			removeIndex = i
			break // since we have at most one matching config
		}
	}
	if removeIndex >= 0 {
		b.TopicConfigs = append(b.TopicConfigs[:removeIndex], b.TopicConfigs[removeIndex+1:]...)
		return nil
	}
	return ErrNoConfigMatch
}

// RemoveQueueByArn removes all queue configurations that match the exact specified ARN
func (b *Configuration) RemoveQueueByArn(arn Arn) {
	var queues []QueueConfig
	for _, queue := range b.QueueConfigs {
		if queue.Queue != arn.String() {
			queues = append(queues, queue)
		}
	}
	b.QueueConfigs = queues
}

// RemoveQueueByArnEventsPrefixSuffix removes a queue configuration that match the exact specified ARN, events, prefix and suffix
func (b *Configuration) RemoveQueueByArnEventsPrefixSuffix(arn Arn, events []EventType, prefix, suffix string) error {
	removeIndex := -1
	for i, v := range b.QueueConfigs {
		// if it matches events and filters, mark the index for deletion
		if v.Queue == arn.String() && v.Config.Equal(events, prefix, suffix) {
			removeIndex = i
			break // since we have at most one matching config
		}
	}
	if removeIndex >= 0 {
		b.QueueConfigs = append(b.QueueConfigs[:removeIndex], b.QueueConfigs[removeIndex+1:]...)
		return nil
	}
	return ErrNoConfigMatch
}

// RemoveLambdaByArn removes all lambda configurations that match the exact specified ARN
func (b *Configuration) RemoveLambdaByArn(arn Arn) {
	var lambdas []LambdaConfig
	for _, lambda := range b.LambdaConfigs {
		if lambda.Lambda != arn.String() {
			lambdas = append(lambdas, lambda)
		}
	}
	b.LambdaConfigs = lambdas
}

// RemoveLambdaByArnEventsPrefixSuffix removes a topic configuration that match the exact specified ARN, events, prefix and suffix
func (b *Configuration) RemoveLambdaByArnEventsPrefixSuffix(arn Arn, events []EventType, prefix, suffix string) error {
	removeIndex := -1
	for i, v := range b.LambdaConfigs {
		// if it matches events and filters, mark the index for deletion
		if v.Lambda == arn.String() && v.Config.Equal(events, prefix, suffix) {
			removeIndex = i
			break // since we have at most one matching config
		}
	}
	if removeIndex >= 0 {
		b.LambdaConfigs = append(b.LambdaConfigs[:removeIndex], b.LambdaConfigs[removeIndex+1:]...)
		return nil
	}
	return ErrNoConfigMatch
}
