// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	awshttp "github.com/aws/smithy-go/transport/http"
)

var (
	_ ObjectStorage = &MinioStorage{}

	quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
)

// s3Object wraps the S3 object to implement the Object interface with seeking support
type s3Object struct {
	s3Client *s3.Client
	ctx      context.Context
	bucket   string
	key      string
	size     int64
	lastMod  time.Time
	offset   int64
	body     io.ReadCloser
}

func (o *s3Object) Read(p []byte) (n int, err error) {
	if o.offset >= o.size {
		return 0, io.EOF
	}

	// If we don't have a body or need to re-fetch (after seek), get one
	if o.body == nil {
		rangeHeader := fmt.Sprintf("bytes=%d-", o.offset)
		resp, err := o.s3Client.GetObject(o.ctx, &s3.GetObjectInput{
			Bucket: aws.String(o.bucket),
			Key:    aws.String(o.key),
			Range:  aws.String(rangeHeader),
		})
		if err != nil {
			return 0, convertS3Err(err)
		}
		o.body = resp.Body
	}

	n, err = o.body.Read(p)
	o.offset += int64(n)
	return n, err
}

func (o *s3Object) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = o.offset + offset
	case io.SeekEnd:
		newOffset = o.size + offset
	default:
		return 0, errors.New("Seek: invalid whence")
	}

	if newOffset < 0 {
		return 0, errors.New("Seek: invalid offset")
	}
	if newOffset > o.size {
		return 0, errors.New("Seek: invalid offset")
	}

	// If seeking to a different position, close current body so Read will re-fetch
	if newOffset != o.offset && o.body != nil {
		o.body.Close()
		o.body = nil
	}
	o.offset = newOffset
	return o.offset, nil
}

func (o *s3Object) Close() error {
	if o.body != nil {
		return o.body.Close()
	}
	return nil
}

func (o *s3Object) Stat() (os.FileInfo, error) {
	return &s3FileInfo{
		key:     o.key,
		size:    o.size,
		lastMod: o.lastMod,
	}, nil
}

// MinioStorage returns a minio bucket storage
type MinioStorage struct {
	cfg      *setting.MinioStorageConfig
	ctx      context.Context
	client   *s3.Client
	bucket   string
	basePath string
}

func convertS3Err(err error) error {
	if err == nil {
		return nil
	}

	// Check for specific S3 error types
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return os.ErrNotExist
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return os.ErrNotExist
	}

	// Check HTTP response errors
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.HTTPStatusCode() {
		case http.StatusNotFound:
			return os.ErrNotExist
		case http.StatusForbidden:
			return os.ErrPermission
		}
	}

	return err
}

var getBucketVersioning = func(ctx context.Context, client *s3.Client, bucket string) error {
	_, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucket),
	})
	return err
}

// NewMinioStorage returns a minio storage
func NewMinioStorage(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	config := cfg.MinioConfig
	if config.ChecksumAlgorithm != "" && config.ChecksumAlgorithm != "default" && config.ChecksumAlgorithm != "md5" {
		return nil, fmt.Errorf("invalid minio checksum algorithm: %s", config.ChecksumAlgorithm)
	}

	log.Info("Creating Minio storage at %s:%s with base path %s", config.Endpoint, config.Bucket, config.BasePath)

	// Build the endpoint URL
	var endpointURL string
	if config.UseSSL {
		endpointURL = "https://" + config.Endpoint
	} else {
		endpointURL = "http://" + config.Endpoint
	}

	// Build custom HTTP client with TLS settings and timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify},
		},
	}

	// Build credentials provider chain
	credProvider := buildS3CredentialsProvider(config)

	// Build AWS config directly without LoadDefaultConfig to avoid
	// background network calls (e.g., EC2 metadata discovery)
	awsCfg := aws.Config{
		Region:      config.Location,
		Credentials: credProvider,
		HTTPClient:  httpClient,
	}

	// Determine path style based on bucket lookup type
	usePathStyle := false
	switch config.BucketLookUpType {
	case "auto", "":
		// For Minio compatibility, default to path style
		usePathStyle = true
	case "dns":
		usePathStyle = false
	case "path":
		usePathStyle = true
	default:
		return nil, fmt.Errorf("invalid minio bucket lookup type: %s", config.BucketLookUpType)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpointURL)
		o.UsePathStyle = usePathStyle
	})

	// The GetBucketVersioning is only used for checking whether the Object Storage parameters are generally good.
	// It doesn't need to succeed. The assumption is that if the API returns the HTTP code 400, then the parameters
	// could be incorrect. Otherwise even if the request itself fails (403, 404, etc), the code should still continue
	// because the parameters seem "good" enough.
	err := getBucketVersioning(ctx, s3Client, config.Bucket)
	if err != nil {
		var respErr *awshttp.ResponseError
		if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusBadRequest {
			log.Error("S3 storage connection failure at %s:%s with base path %s: %v", config.Endpoint, config.Bucket, config.Location, err)
			return nil, err
		}
	}

	// Check to see if we already own this bucket
	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(config.Bucket),
	})
	if err != nil {
		var notFound *types.NotFound
		var noSuchBucket *types.NoSuchBucket
		if errors.As(err, &notFound) || errors.As(err, &noSuchBucket) {
			// Bucket doesn't exist, create it
			createInput := &s3.CreateBucketInput{
				Bucket: aws.String(config.Bucket),
			}
			// Only set LocationConstraint if not us-east-1 (AWS S3 requirement)
			if config.Location != "" && config.Location != "us-east-1" {
				createInput.CreateBucketConfiguration = &types.CreateBucketConfiguration{
					LocationConstraint: types.BucketLocationConstraint(config.Location),
				}
			}
			_, err = s3Client.CreateBucket(ctx, createInput)
			if err != nil {
				return nil, convertS3Err(err)
			}
		} else {
			return nil, convertS3Err(err)
		}
	}

	return &MinioStorage{
		cfg:      &config,
		ctx:      ctx,
		client:   s3Client,
		bucket:   config.Bucket,
		basePath: config.BasePath,
	}, nil
}

func (m *MinioStorage) buildMinioPath(p string) string {
	p = strings.TrimPrefix(util.PathJoinRelX(m.basePath, p), "/") // object store doesn't use slash for root path
	if p == "." {
		p = "" // object store doesn't use dot as relative path
	}
	return p
}

func (m *MinioStorage) buildMinioDirPrefix(p string) string {
	// ending slash is required for avoiding matching like "foo/" and "foobar/" with prefix "foo"
	p = m.buildMinioPath(p) + "/"
	if p == "/" {
		p = "" // object store doesn't use slash for root path
	}
	return p
}

// envCredentialsProvider checks a pair of environment variables for credentials.
// This is a generic provider that can be configured for different env var names.
type envCredentialsProvider struct {
	accessKeyEnv string
	secretKeyEnv string
	source       string
}

func (p envCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	accessKey := os.Getenv(p.accessKeyEnv)
	secretKey := os.Getenv(p.secretKeyEnv)
	if accessKey == "" || secretKey == "" {
		return aws.Credentials{}, fmt.Errorf("%s or %s not set", p.accessKeyEnv, p.secretKeyEnv)
	}
	return aws.Credentials{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Source:          p.source,
	}, nil
}

// minioFileCredentialsProvider reads credentials from MINIO_SHARED_CREDENTIALS_FILE.
// This uses Minio's JSON config format (not AWS INI format), so we need a custom parser.
type minioFileCredentialsProvider struct{}

type minioConfigFile struct {
	Version string                    `json:"version"`
	Aliases map[string]minioAliasConf `json:"aliases"`
}

type minioAliasConf struct {
	URL       string `json:"url"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	API       string `json:"api"`
	Path      string `json:"path"`
}

func (p minioFileCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	filePath := os.Getenv("MINIO_SHARED_CREDENTIALS_FILE")
	if filePath == "" {
		return aws.Credentials{}, errors.New("MINIO_SHARED_CREDENTIALS_FILE not set")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to read minio credentials file: %w", err)
	}

	var config minioConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to parse minio credentials file: %w", err)
	}

	// Try to find s3 alias first, then use the first available alias
	var alias minioAliasConf
	if s3Alias, ok := config.Aliases["s3"]; ok {
		alias = s3Alias
	} else {
		for _, a := range config.Aliases {
			alias = a
			break
		}
	}

	if alias.AccessKey == "" || alias.SecretKey == "" {
		return aws.Credentials{}, errors.New("no valid credentials found in minio credentials file")
	}

	return aws.Credentials{
		AccessKeyID:     alias.AccessKey,
		SecretAccessKey: alias.SecretKey,
		Source:          "MinioFileCredentials",
	}, nil
}

// awsFileCredentialsProvider reads credentials from AWS_SHARED_CREDENTIALS_FILE or the default
// ~/.aws/credentials file using the AWS SDK's built-in INI parser.
type awsFileCredentialsProvider struct{}

func (p awsFileCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	// Check if AWS_SHARED_CREDENTIALS_FILE is set (matching original Minio SDK behavior)
	if os.Getenv("AWS_SHARED_CREDENTIALS_FILE") == "" {
		return aws.Credentials{}, errors.New("AWS_SHARED_CREDENTIALS_FILE not set")
	}

	// Use SDK's built-in shared credentials loading with a timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cfg, err := awsconfig.LoadDefaultConfig(timeoutCtx,
		// Disable EC2 IMDS so we only load from the credentials file
		awsconfig.WithEC2IMDSClientEnableState(imds.ClientDisabled),
	)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	creds, err := cfg.Credentials.Retrieve(timeoutCtx)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return aws.Credentials{}, errors.New("no valid credentials found in AWS credentials file")
	}

	creds.Source = "AWSFileCredentials"
	return creds, nil
}

// iamCredentialsProvider wraps EC2 role credentials from the AWS SDK.
// A thin wrapper is needed to support custom IAM endpoints (MINIO_IAM_ENDPOINT).
type iamCredentialsProvider struct {
	endpoint string
}

func (p iamCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	// Use a short timeout for IMDS - it should respond quickly if available,
	// and we don't want to hang if not running on EC2/ECS
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var provider *ec2rolecreds.Provider
	if p.endpoint != "" {
		// Create IMDS client with custom endpoint
		imdsClient := imds.New(imds.Options{
			Endpoint: p.endpoint,
		})
		provider = ec2rolecreds.New(func(o *ec2rolecreds.Options) {
			o.Client = imdsClient
		})
	} else {
		provider = ec2rolecreds.New()
	}
	return provider.Retrieve(timeoutCtx)
}

// credentialChainProvider tries multiple providers in order until one succeeds.
// AWS SDK v2 doesn't expose a public chain provider, so we implement our own.
type credentialChainProvider struct {
	providers []aws.CredentialsProvider
}

func (c credentialChainProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	var lastErr error
	for _, provider := range c.providers {
		creds, err := provider.Retrieve(ctx)
		if err == nil {
			return creds, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return aws.Credentials{}, fmt.Errorf("all credential providers failed: %w", lastErr)
	}
	return aws.Credentials{}, errors.New("no credential providers configured")
}

func buildS3CredentialsProvider(config setting.MinioStorageConfig) aws.CredentialsProvider {
	// If static credentials are provided, use those
	if config.AccessKeyID != "" {
		return credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, "")
	}

	// Otherwise, build a chain of credential providers.
	// The chain tries each provider in order until one succeeds.
	chain := credentialChainProvider{
		providers: []aws.CredentialsProvider{
			// Check MINIO_ACCESS_KEY/MINIO_SECRET_KEY (Minio-specific env vars)
			envCredentialsProvider{
				accessKeyEnv: "MINIO_ACCESS_KEY",
				secretKeyEnv: "MINIO_SECRET_KEY",
				source:       "MinioEnvCredentials",
			},
			// Check AWS_ACCESS_KEY/AWS_SECRET_KEY (Minio SDK style, without _ID suffix)
			envCredentialsProvider{
				accessKeyEnv: "AWS_ACCESS_KEY",
				secretKeyEnv: "AWS_SECRET_KEY",
				source:       "AWSEnvCredentials",
			},
			// Check AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY (standard AWS style)
			envCredentialsProvider{
				accessKeyEnv: "AWS_ACCESS_KEY_ID",
				secretKeyEnv: "AWS_SECRET_ACCESS_KEY",
				source:       "AWSEnvCredentials",
			},
			// Read credentials from MINIO_SHARED_CREDENTIALS_FILE (JSON format)
			minioFileCredentialsProvider{},
			// Read credentials from AWS_SHARED_CREDENTIALS_FILE (INI format)
			awsFileCredentialsProvider{},
			// Read IAM role from EC2 metadata endpoint if available
			iamCredentialsProvider{endpoint: config.IamEndpoint},
		},
	}

	return chain
}

// Open opens a file
func (m *MinioStorage) Open(path string) (Object, error) {
	key := m.buildMinioPath(path)

	// First get object metadata to know the size
	headResp, err := m.client.HeadObject(m.ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, convertS3Err(err)
	}

	var size int64
	if headResp.ContentLength != nil {
		size = *headResp.ContentLength
	}
	var lastMod time.Time
	if headResp.LastModified != nil {
		lastMod = *headResp.LastModified
	}

	return &s3Object{
		s3Client: m.client,
		ctx:      m.ctx,
		bucket:   m.bucket,
		key:      key,
		size:     size,
		lastMod:  lastMod,
		offset:   0,
		body:     nil, // Will be fetched on first Read
	}, nil
}

// Save saves a file to minio
func (m *MinioStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	key := m.buildMinioPath(path)

	// AWS SDK v2 requires either a seekable reader or we must buffer the content
	// to properly send Content-Length header
	var body io.ReadSeeker
	switch v := r.(type) {
	case io.ReadSeeker:
		body = v
	default:
		// Buffer the content - required for proper Content-Length handling
		data, err := io.ReadAll(r)
		if err != nil {
			return 0, fmt.Errorf("failed to read content: %w", err)
		}
		if size < 0 {
			size = int64(len(data))
		}
		body = bytes.NewReader(data)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(m.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String("application/octet-stream"),
	}

	_, err := m.client.PutObject(m.ctx, input)
	if err != nil {
		return 0, convertS3Err(err)
	}
	return size, nil
}

type s3FileInfo struct {
	key     string
	size    int64
	lastMod time.Time
}

func (m s3FileInfo) Name() string {
	return path.Base(m.key)
}

func (m s3FileInfo) Size() int64 {
	return m.size
}

func (m s3FileInfo) ModTime() time.Time {
	return m.lastMod
}

func (m s3FileInfo) IsDir() bool {
	return strings.HasSuffix(m.key, "/")
}

func (m s3FileInfo) Mode() os.FileMode {
	return os.ModePerm
}

func (m s3FileInfo) Sys() any {
	return nil
}

// Stat returns the stat information of the object
func (m *MinioStorage) Stat(path string) (os.FileInfo, error) {
	key := m.buildMinioPath(path)
	resp, err := m.client.HeadObject(m.ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, convertS3Err(err)
	}

	var size int64
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}
	var lastMod time.Time
	if resp.LastModified != nil {
		lastMod = *resp.LastModified
	}

	return &s3FileInfo{
		key:     key,
		size:    size,
		lastMod: lastMod,
	}, nil
}

// Delete delete a file
func (m *MinioStorage) Delete(path string) error {
	_, err := m.client.DeleteObject(m.ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(m.buildMinioPath(path)),
	})
	return convertS3Err(err)
}

// URL gets the redirect URL to a file. The presigned link is valid for 5 minutes.
func (m *MinioStorage) URL(storePath, name, method string, _ url.Values) (*url.URL, error) {
	// Here we might not know the real filename, and it's quite inefficient to detect the mime type by pre-fetching the object head.
	// So we just do a quick detection by extension name, at least if works for the "View Raw File" for an LFS file on the Web UI.
	// Detect content type by extension name, only support the well-known safe types for inline rendering.
	// TODO: OBJECT-STORAGE-CONTENT-TYPE: need a complete solution and refactor for Azure in the future
	ext := path.Ext(name)
	inlineExtMimeTypes := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".avif": "image/avif",
		// ATTENTION! Don't support unsafe types like HTML/SVG due to security concerns: they can contain JS code, and maybe they need proper Content-Security-Policy
		// HINT: PDF-RENDER-SANDBOX: PDF won't render in sandboxed context, it seems fine to render it inline
		".pdf": "application/pdf",

		// TODO: refactor with "modules/public/mime_types.go", for example: "DetectWellKnownSafeInlineMimeType"
	}

	var contentType, contentDisposition string
	if mimeType, ok := inlineExtMimeTypes[ext]; ok {
		contentType = mimeType
		contentDisposition = "inline"
	} else {
		contentDisposition = fmt.Sprintf(`attachment; filename="%s"`, quoteEscaper.Replace(name))
	}

	expires := 5 * time.Minute
	key := m.buildMinioPath(storePath)
	presignClient := s3.NewPresignClient(m.client)

	if method == http.MethodHead {
		presignReq, err := presignClient.PresignHeadObject(m.ctx, &s3.HeadObjectInput{
			Bucket:                     aws.String(m.bucket),
			Key:                        aws.String(key),
			ResponseContentDisposition: aws.String(contentDisposition),
			ResponseContentType:        aws.String(contentType),
		}, s3.WithPresignExpires(expires))
		if err != nil {
			return nil, convertS3Err(err)
		}
		return url.Parse(presignReq.URL)
	}

	presignReq, err := presignClient.PresignGetObject(m.ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(m.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(contentDisposition),
		ResponseContentType:        aws.String(contentType),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return nil, convertS3Err(err)
	}
	return url.Parse(presignReq.URL)
}

// IterateObjects iterates across the objects in the miniostorage
func (m *MinioStorage) IterateObjects(dirName string, fn func(path string, obj Object) error) error {
	prefix := m.buildMinioDirPrefix(dirName)

	paginator := s3.NewListObjectsV2Paginator(m.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(m.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(m.ctx)
		if err != nil {
			return convertS3Err(err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			key := *obj.Key

			var size int64
			if obj.Size != nil {
				size = *obj.Size
			}
			var lastMod time.Time
			if obj.LastModified != nil {
				lastMod = *obj.LastModified
			}

			s3Obj := &s3Object{
				s3Client: m.client,
				ctx:      m.ctx,
				bucket:   m.bucket,
				key:      key,
				size:     size,
				lastMod:  lastMod,
				offset:   0,
				body:     nil,
			}

			if err := func() error {
				defer s3Obj.Close()
				return fn(strings.TrimPrefix(key, m.basePath), s3Obj)
			}(); err != nil {
				return convertS3Err(err)
			}
		}
	}

	return nil
}

func init() {
	RegisterStorageType(setting.MinioStorageType, NewMinioStorage)
}
