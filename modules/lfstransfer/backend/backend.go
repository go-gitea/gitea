// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"

	"github.com/charmbracelet/git-lfs-transfer/transfer"
)

// Version is the git-lfs-transfer protocol version number.
const Version = "1"

// Capabilities is a list of Git LFS capabilities supported by this package.
var Capabilities = []string{
	"version=" + Version,
	"locking",
}

var _ transfer.Backend = (*GiteaBackend)(nil)

// GiteaBackend is an adapter between git-lfs-transfer library and Gitea's internal LFS API
type GiteaBackend struct {
	ctx          context.Context
	server       *url.URL
	op           string
	authToken    string
	internalAuth string
	logger       transfer.Logger
}

func New(ctx context.Context, repo, op, token string, logger transfer.Logger) (transfer.Backend, error) {
	// runServ guarantees repo will be in form [owner]/[name].git
	server, err := url.Parse(setting.LocalURL)
	if err != nil {
		return nil, err
	}
	server = server.JoinPath("api/internal/repo", repo, "info/lfs")
	return &GiteaBackend{ctx: ctx, server: server, op: op, authToken: token, internalAuth: fmt.Sprintf("Bearer %s", setting.InternalToken), logger: logger}, nil
}

// Batch implements transfer.Backend
func (g *GiteaBackend) Batch(_ string, pointers []transfer.BatchItem, args transfer.Args) ([]transfer.BatchItem, error) {
	reqBody := lfs.BatchRequest{Operation: g.op}
	if transfer, ok := args[argTransfer]; ok {
		reqBody.Transfers = []string{transfer}
	}
	if ref, ok := args[argRefname]; ok {
		reqBody.Ref = &lfs.Reference{Name: ref}
	}
	reqBody.Objects = make([]lfs.Pointer, len(pointers))
	for i := range pointers {
		reqBody.Objects[i].Oid = pointers[i].Oid
		reqBody.Objects[i].Size = pointers[i].Size
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		g.logger.Log("json marshal error", err)
		return nil, err
	}
	headers := map[string]string{
		headerAuthorization:     g.authToken,
		headerGiteaInternalAuth: g.internalAuth,
		headerAccept:            mimeGitLFS,
		headerContentType:       mimeGitLFS,
	}
	req := newInternalRequestLFS(g.ctx, g.server.JoinPath("objects/batch").String(), http.MethodPost, headers, bodyBytes)
	resp, err := req.Response()
	if err != nil {
		g.logger.Log("http request error", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		g.logger.Log("http statuscode error", resp.StatusCode, statusCodeToErr(resp.StatusCode))
		return nil, statusCodeToErr(resp.StatusCode)
	}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		g.logger.Log("http read error", err)
		return nil, err
	}
	var respBody lfs.BatchResponse
	err = json.Unmarshal(respBytes, &respBody)
	if err != nil {
		g.logger.Log("json umarshal error", err)
		return nil, err
	}

	// rebuild slice, we can't rely on order in resp being the same as req
	pointers = pointers[:0]
	opNum := opMap[g.op]
	for _, obj := range respBody.Objects {
		pointer := transfer.Pointer{Oid: obj.Pointer.Oid, Size: obj.Pointer.Size}
		item := transfer.BatchItem{Pointer: pointer, Args: map[string]string{}}
		switch opNum {
		case opDownload:
			if action, ok := obj.Actions[actionDownload]; ok {
				item.Present = true
				idMap := obj.Actions
				idMapBytes, err := json.Marshal(idMap)
				if err != nil {
					g.logger.Log("json marshal error", err)
					return nil, err
				}
				idMapStr := base64.StdEncoding.EncodeToString(idMapBytes)
				item.Args[argID] = idMapStr
				if authHeader, ok := action.Header[headerAuthorization]; ok {
					authHeaderB64 := base64.StdEncoding.EncodeToString([]byte(authHeader))
					item.Args[argToken] = authHeaderB64
				}
				if action.ExpiresAt != nil {
					item.Args[argExpiresAt] = action.ExpiresAt.String()
				}
			} else {
				// must be an error, but the SSH protocol can't propagate individual errors
				g.logger.Log("object not found", obj.Pointer.Oid, obj.Pointer.Size)
				item.Present = false
			}
		case opUpload:
			if action, ok := obj.Actions[actionUpload]; ok {
				item.Present = false
				idMap := obj.Actions
				idMapBytes, err := json.Marshal(idMap)
				if err != nil {
					g.logger.Log("json marshal error", err)
					return nil, err
				}
				idMapStr := base64.StdEncoding.EncodeToString(idMapBytes)
				item.Args[argID] = idMapStr
				if authHeader, ok := action.Header[headerAuthorization]; ok {
					authHeaderB64 := base64.StdEncoding.EncodeToString([]byte(authHeader))
					item.Args[argToken] = authHeaderB64
				}
				if action.ExpiresAt != nil {
					item.Args[argExpiresAt] = action.ExpiresAt.String()
				}
			} else {
				item.Present = true
			}
		}
		pointers = append(pointers, item)
	}
	return pointers, nil
}

// Download implements transfer.Backend. The returned reader must be closed by the caller.
func (g *GiteaBackend) Download(oid string, args transfer.Args) (io.ReadCloser, int64, error) {
	idMapStr, exists := args[argID]
	if !exists {
		return nil, 0, ErrMissingID
	}
	idMapBytes, err := base64.StdEncoding.DecodeString(idMapStr)
	if err != nil {
		g.logger.Log("base64 decode error", err)
		return nil, 0, transfer.ErrCorruptData
	}
	idMap := map[string]*lfs.Link{}
	err = json.Unmarshal(idMapBytes, &idMap)
	if err != nil {
		g.logger.Log("json unmarshal error", err)
		return nil, 0, transfer.ErrCorruptData
	}
	action, exists := idMap[actionDownload]
	if !exists {
		g.logger.Log("argument id incorrect")
		return nil, 0, transfer.ErrCorruptData
	}
	headers := map[string]string{
		headerAuthorization:     g.authToken,
		headerGiteaInternalAuth: g.internalAuth,
		headerAccept:            mimeOctetStream,
	}
	req := newInternalRequestLFS(g.ctx, toInternalLFSURL(action.Href), http.MethodGet, headers, nil)
	resp, err := req.Response()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get response: %w", err)
	}
	// no need to close the body here by "defer resp.Body.Close()", see below
	if resp.StatusCode != http.StatusOK {
		return nil, 0, statusCodeToErr(resp.StatusCode)
	}

	respSize, err := strconv.ParseInt(resp.Header.Get("X-Gitea-LFS-Content-Length"), 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse content length: %w", err)
	}
	// transfer.Backend will check io.Closer interface and close this Body reader
	return resp.Body, respSize, nil
}

// Upload implements transfer.Backend.
func (g *GiteaBackend) Upload(oid string, size int64, r io.Reader, args transfer.Args) error {
	idMapStr, exists := args[argID]
	if !exists {
		return ErrMissingID
	}
	idMapBytes, err := base64.StdEncoding.DecodeString(idMapStr)
	if err != nil {
		g.logger.Log("base64 decode error", err)
		return transfer.ErrCorruptData
	}
	idMap := map[string]*lfs.Link{}
	err = json.Unmarshal(idMapBytes, &idMap)
	if err != nil {
		g.logger.Log("json unmarshal error", err)
		return transfer.ErrCorruptData
	}
	action, exists := idMap[actionUpload]
	if !exists {
		g.logger.Log("argument id incorrect")
		return transfer.ErrCorruptData
	}
	headers := map[string]string{
		headerAuthorization:     g.authToken,
		headerGiteaInternalAuth: g.internalAuth,
		headerContentType:       mimeOctetStream,
		headerContentLength:     strconv.FormatInt(size, 10),
	}

	req := newInternalRequestLFS(g.ctx, toInternalLFSURL(action.Href), http.MethodPut, headers, nil)
	req.Body(r)
	resp, err := req.Response()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statusCodeToErr(resp.StatusCode)
	}
	return nil
}

// Verify implements transfer.Backend.
func (g *GiteaBackend) Verify(oid string, size int64, args transfer.Args) (transfer.Status, error) {
	reqBody := lfs.Pointer{Oid: oid, Size: size}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return transfer.NewStatus(transfer.StatusInternalServerError), err
	}
	idMapStr, exists := args[argID]
	if !exists {
		return transfer.NewStatus(transfer.StatusBadRequest, "missing argument: id"), ErrMissingID
	}
	idMapBytes, err := base64.StdEncoding.DecodeString(idMapStr)
	if err != nil {
		g.logger.Log("base64 decode error", err)
		return transfer.NewStatus(transfer.StatusBadRequest, "corrupt argument: id"), transfer.ErrCorruptData
	}
	idMap := map[string]*lfs.Link{}
	err = json.Unmarshal(idMapBytes, &idMap)
	if err != nil {
		g.logger.Log("json unmarshal error", err)
		return transfer.NewStatus(transfer.StatusBadRequest, "corrupt argument: id"), transfer.ErrCorruptData
	}
	action, exists := idMap[actionVerify]
	if !exists {
		// the server sent no verify action
		return transfer.SuccessStatus(), nil
	}
	headers := map[string]string{
		headerAuthorization:     g.authToken,
		headerGiteaInternalAuth: g.internalAuth,
		headerAccept:            mimeGitLFS,
		headerContentType:       mimeGitLFS,
	}
	req := newInternalRequestLFS(g.ctx, toInternalLFSURL(action.Href), http.MethodPost, headers, bodyBytes)
	resp, err := req.Response()
	if err != nil {
		return transfer.NewStatus(transfer.StatusInternalServerError), err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return transfer.NewStatus(uint32(resp.StatusCode), http.StatusText(resp.StatusCode)), statusCodeToErr(resp.StatusCode)
	}
	return transfer.SuccessStatus(), nil
}

// LockBackend implements transfer.Backend.
func (g *GiteaBackend) LockBackend(_ transfer.Args) transfer.LockBackend {
	return newGiteaLockBackend(g)
}
