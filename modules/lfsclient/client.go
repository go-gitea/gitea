// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

const (
	metaMediaType = "application/vnd.git-lfs+json"
)

// BatchRequest encodes json object using in a lfs batch api request
type BatchRequest struct {
	Operation string                       `json:"operation"`
	Transfers []string                     `json:"transfers,omitempty"`
	Ref       *Reference                   `json:"ref,omitempty"`
	Objects   []*models.LFSMetaObjectBasic `json:"objects"`
}

// Reference is a reference field of BatchRequest
type Reference struct {
	Name string `json:"name"`
}

// packbatch packs lfs batch request to json encoded as bytes
func packbatch(operation string, transfers []string, ref *Reference, metaObjects []*models.LFSMetaObject) (*bytes.Buffer, error) {
	metaObjectsBasic := []*models.LFSMetaObjectBasic{}
	for _, meta := range metaObjects {
		metaBasic := &models.LFSMetaObjectBasic{Oid: meta.Oid, Size: meta.Size}
		metaObjectsBasic = append(metaObjectsBasic, metaBasic)
	}

	reqobj := &BatchRequest{operation, transfers, ref, metaObjectsBasic}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(reqobj); err != nil {
		return buf, fmt.Errorf("Failed to encode BatchRequest as json. Error: %v", err)
	}
	return buf, nil
}

// BasicTransferAdapter makes request to lfs server and returns io.ReadCLoser
func BasicTransferAdapter(ctx context.Context, client *http.Client, href string, size int64) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, href, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-type", "application/octet-stream")
	req.Header.Set("Content-Length", strconv.Itoa(int(size)))

	resp, err := client.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to query BasicTransferAdapter with response: %s", resp.Status)
	}
	return resp.Body, nil
}

// FetchLFSFilesToContentStore downloads []LFSMetaObject from lfsServer to ContentStore
func FetchLFSFilesToContentStore(ctx context.Context, metaObjects []*models.LFSMetaObject, userName string, repo *models.Repository, lfsServer string, contentStore *models.ContentStore) error {
	client := http.DefaultClient

	rv, err := packbatch("download", nil, nil, metaObjects)
	if err != nil {
		return err
	}
	batchAPIURL := lfsServer + "/objects/batch"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, batchAPIURL, rv)
	if err != nil {
		return err
	}
	req.Header.Set("Content-type", metaMediaType)
	req.Header.Set("Accept", metaMediaType)

	resp, err := client.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to query Batch with response: %s", resp.Status)
	}

	var respBatch models.BatchResponse
	err = json.NewDecoder(resp.Body).Decode(&respBatch)
	if err != nil {
		return err
	}

	if len(respBatch.Transfer) == 0 {
		respBatch.Transfer = "basic"
	}

	for _, rep := range respBatch.Objects {
		rc, err := BasicTransferAdapter(ctx, client, rep.Actions["download"].Href, rep.Size)
		if err != nil {
			log.Error("Unable to use BasicTransferAdapter. Error: %v", err)
			return err
		}
		meta, err := repo.GetLFSMetaObjectByOid(rep.Oid)
		if err != nil {
			log.Error("Unable to get LFS OID[%s] Error: %v", rep.Oid, err)
			return err
		}

		// put LFS file to contentStore
		if err := contentStore.Put(meta, rc); err != nil {
			if _, err2 := repo.RemoveLFSMetaObjectByOid(meta.Oid); err2 != nil {
				return fmt.Errorf("Error whilst removing failed inserted LFS object %s: %v (Prev Error: %v)", meta.Oid, err2, err)
			}
			return err
		}
	}
	return nil
}
