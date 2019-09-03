// Copyright 2018 The Go Cloud Development Kit Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package aws provides fundamental Wire providers for Amazon Web Services (AWS).
package aws // import "gocloud.dev/aws"

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/wire"
)

// DefaultSession is a Wire provider set that provides a *session.Session using
// the default options.
var DefaultSession = wire.NewSet(
	SessionConfig,
	ConfigCredentials,
	NewDefaultSession,
	wire.Bind(new(client.ConfigProvider), new(*session.Session)),
)

// NewDefaultSession returns a *session.Session using the default options.
func NewDefaultSession() (*session.Session, error) {
	return session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable})
}

// SessionConfig returns sess.Config.
func SessionConfig(sess *session.Session) *aws.Config {
	return sess.Config
}

// ConfigCredentials returns cfg.Credentials.
func ConfigCredentials(cfg *aws.Config) *credentials.Credentials {
	return cfg.Credentials
}

// ConfigOverrider implements client.ConfigProvider by overlaying a list of
// configurations over a base configuration provider.
type ConfigOverrider struct {
	Base    client.ConfigProvider
	Configs []*aws.Config
}

// ClientConfig calls the base provider's ClientConfig method with co.Configs
// followed by the arguments given to ClientConfig.
func (co ConfigOverrider) ClientConfig(serviceName string, cfgs ...*aws.Config) client.Config {
	cfgs = append(co.Configs[:len(co.Configs):len(co.Configs)], cfgs...)
	return co.Base.ClientConfig(serviceName, cfgs...)
}

// ConfigFromURLParams returns an aws.Config initialized based on the URL
// parameters in q. It is intended to be used by URLOpeners for AWS services.
// https://docs.aws.amazon.com/sdk-for-go/api/aws/#Config
//
// It returns an error if q contains any unknown query parameters; callers
// should remove any query parameters they know about from q before calling
// ConfigFromURLParams.
//
// The following query options are supported:
//  - region: The AWS region for requests; sets aws.Config.Region.
//  - endpoint: The endpoint URL (hostname only or fully qualified URI); sets aws.Config.Endpoint.
//  - disableSSL: A value of "true" disables SSL when sending requests; sets aws.Config.DisableSSL.
//  - s3ForcePathStyle: A value of "true" forces the request to use path-style addressing; sets aws.Config.S3ForcePathStyle.
func ConfigFromURLParams(q url.Values) (*aws.Config, error) {
	var cfg aws.Config
	for param, values := range q {
		value := values[0]
		switch param {
		case "region":
			cfg.Region = aws.String(value)
		case "endpoint":
			cfg.Endpoint = aws.String(value)
		case "disableSSL":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid value for query parameter %q: %v", param, err)
			}
			cfg.DisableSSL = aws.Bool(b)
		case "s3ForcePathStyle":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid value for query parameter %q: %v", param, err)
			}
			cfg.S3ForcePathStyle = aws.Bool(b)
		default:
			return nil, fmt.Errorf("unknown query parameter %q", param)
		}
	}
	return &cfg, nil
}
