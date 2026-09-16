// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"golang.org/x/sync/errgroup"
)

const azureBlobAPIVersion = "2025-11-05" // must not exceed the Azurite version used in CI

type azureBlobError string

func (e azureBlobError) Error() string { return string(e) }

type azureBlobObject struct {
	storage *AzureBlobStorage
	blobURL *url.URL
	info    *objectFileInfo
	offset  int64
	closed  bool

	respBody io.ReadCloser
}

func (a *azureBlobObject) resetRespBody() {
	// close resp, the next attempt reopens at the current offset
	if a.respBody != nil {
		_ = a.respBody.Close()
		a.respBody = nil
	}
}

func (a *azureBlobObject) Read(p []byte) (int, error) {
	if a.closed {
		return 0, fs.ErrClosed
	}
	if a.offset >= a.info.size {
		return 0, io.EOF
	}
	for retry := 0; ; retry++ {
		if a.respBody == nil {
			reqHeader := http.Header{"X-Ms-Range": {fmt.Sprintf("bytes=%d-", a.offset)}}
			_, body, err := a.storage.do(a.storage.ctx, http.MethodGet, a.blobURL, reqHeader, nil)
			if err != nil {
				return 0, err
			}
			a.respBody = body
		}
		n, err := io.ReadFull(a.respBody, p[:min(int64(len(p)), a.info.size-a.offset)])
		a.offset += int64(n)
		if err != nil {
			a.resetRespBody()
		}
		if err == nil || n > 0 {
			return n, nil
		}
		if retry == 3 {
			return 0, util.Iif(err == io.EOF, io.ErrUnexpectedEOF, err)
		}
	}
}

func (a *azureBlobObject) Close() (err error) {
	a.closed = true
	if a.respBody != nil {
		err = a.respBody.Close()
		a.respBody = nil
	}
	return err
}

func (a *azureBlobObject) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += a.offset
	case io.SeekEnd:
		offset = a.info.size + offset
	default:
		return 0, errors.New("seek: invalid whence")
	}

	if offset < 0 || offset > a.info.size {
		return 0, errors.New("seek: invalid offset")
	}
	a.resetRespBody()
	a.offset = offset
	return a.offset, nil
}

func (a *azureBlobObject) Stat() (os.FileInfo, error) {
	return a.info, nil
}

type AzureBlobStorage struct {
	cfg         *setting.AzureBlobStorageConfig
	ctx         context.Context
	client      *http.Client
	endpoint    *url.URL
	key         []byte
	blockSize   int
	concurrency int
	retryDelay  time.Duration
}

func NewAzureBlobStorage(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	config := cfg.AzureBlobConfig

	log.Info("Creating Azure Blob storage at %s:%s with base path %s", config.Endpoint, config.Container, config.BasePath)

	key, err := base64.StdEncoding.DecodeString(config.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("invalid azure blob account key: %w", err)
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConnsPerHost: 10,
	}
	a := &AzureBlobStorage{
		cfg:         &config,
		ctx:         ctx,
		client:      &http.Client{Transport: transport},
		endpoint:    endpoint,
		key:         key,
		blockSize:   4 * 1024 * 1024,
		concurrency: 4,
		retryDelay:  200 * time.Millisecond,
	}
	_, _, err = a.do(ctx, http.MethodPut, a.url(config.Container, url.Values{"restype": {"container"}}), nil, nil)
	if err != nil && !errors.Is(err, azureBlobError("ContainerAlreadyExists")) {
		return nil, err
	}
	return a, nil
}

func (a *AzureBlobStorage) url(name string, query url.Values) *url.URL {
	u := *a.endpoint
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + name
	u.RawQuery = query.Encode()
	return &u
}

func (a *AzureBlobStorage) blobName(p string) string {
	return a.cfg.Container + "/" + buildObjectStorePath(a.cfg.BasePath, p)
}

func (a *AzureBlobStorage) signString(s string) string {
	mac := hmac.New(sha256.New, a.key)
	_, _ = mac.Write([]byte(s))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// https://learn.microsoft.com/rest/api/storageservices/authorize-with-shared-key
func (a *AzureBlobStorage) signRequest(req *http.Request) string {
	lines := []string{
		req.Method,
		req.Header.Get("Content-Encoding"),
		req.Header.Get("Content-Language"),
		util.Iif(req.ContentLength > 0, strconv.FormatInt(req.ContentLength, 10), ""),
	}
	for _, name := range []string{
		"Content-MD5", "Content-Type", "Date",
		"If-Modified-Since", "If-Match", "If-None-Match", "If-Unmodified-Since", "Range",
	} {
		lines = append(lines, req.Header.Get(name))
	}
	msHeaders := map[string]string{}
	for key, values := range req.Header {
		if key = strings.ToLower(key); strings.HasPrefix(key, "x-ms-") {
			msHeaders[key] = key + ":" + strings.Join(values, ",")
		}
	}
	for _, key := range slices.Sorted(maps.Keys(msHeaders)) {
		lines = append(lines, msHeaders[key])
	}
	lines = append(lines, "/"+a.cfg.AccountName+req.URL.EscapedPath())
	query := req.URL.Query()
	for _, key := range slices.Sorted(maps.Keys(query)) {
		lines = append(lines, strings.ToLower(key)+":"+strings.Join(slices.Sorted(slices.Values(query[key])), ","))
	}
	return a.signString(strings.Join(lines, "\n"))
}

// only GET returns the body, the caller closes it
func (a *AzureBlobStorage) do(ctx context.Context, method string, u *url.URL, header http.Header, body []byte) (http.Header, io.ReadCloser, error) {
	const maxDelay = 3 * time.Second
	for retry := 0; ; retry++ {
		req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
		if err != nil {
			return nil, nil, err
		}
		maps.Copy(req.Header, header)
		req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))
		req.Header.Set("x-ms-version", azureBlobAPIVersion)
		req.Header.Set("Authorization", "SharedKey "+a.cfg.AccountName+":"+a.signRequest(req))

		resp, err := a.client.Do(req)
		if retry < 3 && (err != nil || slices.Contains([]int{408, 429, 500, 502, 503, 504}, resp.StatusCode)) {
			if err == nil {
				_ = resp.Body.Close()
			}
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(min(a.retryDelay<<retry, maxDelay)):
			}
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode < http.StatusBadRequest && method == http.MethodGet {
			return resp.Header, resp.Body, nil
		}
		_ = resp.Body.Close()
		if resp.StatusCode < http.StatusBadRequest {
			return resp.Header, nil, nil
		}
		code := cmp.Or(resp.Header.Get("x-ms-error-code"), resp.Status)
		if code == "BlobNotFound" {
			return nil, nil, fs.ErrNotExist
		}
		return nil, nil, azureBlobError(code)
	}
}

func (a *AzureBlobStorage) Open(path string) (Object, error) {
	info, err := a.stat(path)
	if err != nil {
		return nil, err
	}
	return &azureBlobObject{storage: a, blobURL: a.url(a.blobName(path), nil), info: info}, nil
}

func (a *AzureBlobStorage) Save(path string, r io.Reader, _ int64) (int64, error) {
	name := a.blobName(path)
	block := make([]byte, a.blockSize)
	n, err := util.ReadAtMost(r, block)
	if err != nil {
		return 0, err
	}
	if n < a.blockSize {
		_, _, err := a.do(a.ctx, http.MethodPut, a.url(name, nil), http.Header{"X-Ms-Blob-Type": {"BlockBlob"}}, block[:n])
		return int64(n), err
	}

	g, ctx := errgroup.WithContext(a.ctx)
	g.SetLimit(a.concurrency)
	blockList := bytes.NewBufferString(`<?xml version="1.0" encoding="utf-8"?><BlockList>`)
	idPrefix := rand.Text() // keeps concurrent uploads to the same blob from overwriting each other's blocks
	var total int64
	for blockNum := 0; n > 0 && ctx.Err() == nil; blockNum++ {
		id := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s%05d", idPrefix, blockNum))
		blockList.WriteString("<Latest>" + id + "</Latest>")
		total += int64(n)
		data := block[:n]
		g.Go(func() error {
			_, _, err := a.do(ctx, http.MethodPut, a.url(name, url.Values{"comp": {"block"}, "blockid": {id}}), nil, data)
			return err
		})
		if n < a.blockSize {
			break
		}
		block = make([]byte, a.blockSize)
		if n, err = util.ReadAtMost(r, block); err != nil {
			break
		}
	}
	if err = errors.Join(g.Wait(), err); err != nil {
		return 0, err
	}
	blockList.WriteString("</BlockList>")
	_, _, err = a.do(a.ctx, http.MethodPut, a.url(name, url.Values{"comp": {"blocklist"}}), nil, blockList.Bytes())
	return total, err
}

func (a *AzureBlobStorage) stat(p string) (*objectFileInfo, error) {
	header, _, err := a.do(a.ctx, http.MethodHead, a.url(a.blobName(p), nil), nil, nil)
	if err != nil {
		return nil, err
	}
	size, sizeErr := strconv.ParseInt(header.Get("Content-Length"), 10, 64)
	modTime, timeErr := http.ParseTime(header.Get("Last-Modified"))
	return &objectFileInfo{path.Base(p), size, modTime}, errors.Join(sizeErr, timeErr)
}

func (a *AzureBlobStorage) Stat(path string) (os.FileInfo, error) {
	info, err := a.stat(path)
	return info, err
}

func (a *AzureBlobStorage) Delete(path string) error {
	_, _, err := a.do(a.ctx, http.MethodDelete, a.url(a.blobName(path), nil), nil, nil)
	return err
}

// https://learn.microsoft.com/rest/api/storageservices/create-service-sas
func (a *AzureBlobStorage) ServeDirectURL(storePath, name, method string, reqParams *ServeDirectOptions) (*url.URL, error) {
	permissions := util.Iif(method == http.MethodPut, "w", "r")
	param := prepareServeDirectOptions(reqParams, name)
	startTime := time.Now().UTC()
	start, expiry := startTime.Format(time.RFC3339), startTime.Add(5*time.Minute).Format(time.RFC3339)
	canonicalName := "/blob/" + a.cfg.AccountName + "/" + a.blobName(storePath)
	signature := a.signString(strings.Join([]string{permissions, start, expiry, canonicalName, "", "", "", azureBlobAPIVersion, "b", "", "", "", param.ContentDisposition, "", "", param.ContentType}, "\n"))

	query := url.Values{"sv": {azureBlobAPIVersion}, "st": {start}, "se": {expiry}, "sr": {"b"}, "sp": {permissions}, "sig": {signature}}
	if param.ContentDisposition != "" {
		query.Set("rscd", param.ContentDisposition)
	}
	if param.ContentType != "" {
		query.Set("rsct", param.ContentType)
	}
	return a.url(a.blobName(storePath), query), nil
}

func (a *AzureBlobStorage) IterateObjects(dirName string, fn func(path string, obj Object) error) error {
	basePrefix := buildObjectStorePathPrefix(a.cfg.BasePath, "")
	query := url.Values{"restype": {"container"}, "comp": {"list"}, "prefix": {buildObjectStorePathPrefix(a.cfg.BasePath, dirName)}}
	for {
		_, body, err := a.do(a.ctx, http.MethodGet, a.url(a.cfg.Container, query), nil, nil)
		if err != nil {
			return err
		}
		var result struct {
			Blobs []struct {
				Name          string `xml:"Name"`
				ContentLength int64  `xml:"Properties>Content-Length"`
				LastModified  string `xml:"Properties>Last-Modified"`
			} `xml:"Blobs>Blob"`
			NextMarker string `xml:"NextMarker"`
		}
		err = xml.NewDecoder(body).Decode(&result)
		_ = body.Close()
		if err != nil {
			return err
		}
		for _, blob := range result.Blobs {
			modTime, err := http.ParseTime(blob.LastModified)
			if err != nil {
				return err
			}
			object := &azureBlobObject{
				storage: a,
				blobURL: a.url(a.cfg.Container+"/"+blob.Name, nil),
				info:    &objectFileInfo{path.Base(blob.Name), blob.ContentLength, modTime},
			}
			err = fn(strings.TrimPrefix(blob.Name, basePrefix), object)
			_ = object.Close()
			if err != nil {
				return err
			}
		}
		if result.NextMarker == "" {
			return nil
		}
		query.Set("marker", result.NextMarker)
	}
}

func init() {
	RegisterStorageType(setting.AzureBlobStorageType, NewAzureBlobStorage)
}
