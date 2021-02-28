// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"encoding/json"
  "bytes"
	"fmt"
	"io"
	"net/http"
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

type BatchRequest struct {
	Operation string                       `json:"operation"`
	Transfers []string                     `json:"transfers,omitempty"`
  Ref       *Reference                   `json:"ref,omitempty"`
  Objects   []*models.LFSMetaObjectBasic `json:"objects"`
}

type Reference struct {
  Name string `json:"name"`
}

func packbatch(operation string, transfers []string, ref *Reference, metaObjects []*models.LFSMetaObject) (string, error) {
  metaObjectsBasic := []*models.LFSMetaObjectBasic{}
	for _, meta := range metaObjects {
		metaBasic := &LFSMetaObjectBasic{meta.oid, meta.size}
		metaObjectsBasic = append(metaObjectsBasic, metaBasic)
	}

  reqobj := &BatchRequest{operation, transfers, ref, metaObjectsBasic}

  buf := &bytes.Buffer{}
  if err := json.NewEncoder(buf).Encode(reqobj); err != nil {
    return nil, fmt.Errorf("Failed to encode BatchRequest as json. Error: %v", err)
	}
  return buf, nil
}

func BasicTransferAdapter(href string, size int64) (io.ReadCloser, error) {
  req, err := http.NewRequestWithContext(ctx, http.MethodGet, href, nil)
  if err != nil {
		return err
	}
	req.Header.Set("Content-type", "application/octet-stream")
	req.Header.Set("Content-Length", size)

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
    return decodeJSONError(resp).Err
	}
  return resp.Body
}

func FetchLFSFilesToContentStore(ctx *context.Context, metaObjects []*models.LFSMetaObject, userName string, repo *models.Repository, LFSServer string) error {
  client := http.DefaultClient

  rv, err := packbatch("download", nil, nil, metaObjects)
  if err != nil {
    return err
  }
  batchAPIURL := LFSServer + "/objects/batch"
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
    return decodeJSONError(resp).Err
	}

  var respBatch BatchResponse
  err = json.NewDecoder(resp.Body).Decode(&respBatch)
  if err != nil {
    return err
  }

  if respBatch.Transfer == nil {
		respBatch.Transfer = "basic"
	}

  contentStore := &lfs.ContentStore{ObjectStorage: storage.LFS}

  for _, rep := range respBatch.Objects {
    rc := BasicTransferAdapter(rep.Actions["download"].Href, rep.Size)
    meta, err := GetLFSMetaObjectByOid(rep.Oid)
    if err != nil {
  		log.Error("Unable to get LFS OID[%s] Error: %v", rep.Oid, err)
  		return err
  	}

    // put LFS file to contentStore
    exist, err := contentStore.Exists(meta)
    if err != nil {
      log.Error("Unable to check if LFS OID[%s] exist on %s/%s. Error: %v", meta.Oid, userName, repo.Name, err)
      return err
    }

    if exist {
      // remove collision
      if _, err := repo.RemoveLFSMetaObjectByOid(meta.Oid); err != nil {
        return fmt.Errorf("Error whilst removing matched LFS object %s: %v", meta.Oid, err)
      }
    }

    if err := contentStore.Put(meta, rc); err != nil {
      if _, err2 := repo.RemoveLFSMetaObjectByOid(meta.Oid); err2 != nil {
        return fmt.Errorf("Error whilst removing failed inserted LFS object %s: %v (Prev Error: %v)", meta.Oid, err2, err)
      }
      return err
    }
  }
}
