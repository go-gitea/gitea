// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/json"
	lfslock "code.gitea.io/gitea/modules/structs"

	"github.com/charmbracelet/git-lfs-transfer/transfer"
)

var _ transfer.LockBackend = &giteaLockBackend{}

type giteaLockBackend struct {
	ctx          context.Context
	g            *GiteaBackend
	server       *url.URL
	authToken    string
	internalAuth string
	logger       transfer.Logger
}

func newGiteaLockBackend(g *GiteaBackend) transfer.LockBackend {
	server := g.server.JoinPath("locks")
	return &giteaLockBackend{ctx: g.ctx, g: g, server: server, authToken: g.authToken, internalAuth: g.internalAuth, logger: g.logger}
}

// Create implements transfer.LockBackend
func (g *giteaLockBackend) Create(path, refname string) (transfer.Lock, error) {
	reqBody := lfslock.LFSLockRequest{Path: path}

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
	req := newInternalRequestLFS(g.ctx, g.server.String(), http.MethodPost, headers, bodyBytes)
	resp, err := req.Response()
	if err != nil {
		g.logger.Log("http request error", err)
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		g.logger.Log("http read error", err)
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		g.logger.Log("http statuscode error", resp.StatusCode, statusCodeToErr(resp.StatusCode))
		return nil, statusCodeToErr(resp.StatusCode)
	}
	var respBody lfslock.LFSLockResponse
	err = json.Unmarshal(respBytes, &respBody)
	if err != nil {
		g.logger.Log("json umarshal error", err)
		return nil, err
	}

	if respBody.Lock == nil {
		g.logger.Log("api returned nil lock")
		return nil, fmt.Errorf("api returned nil lock")
	}
	respLock := respBody.Lock
	owner := userUnknown
	if respLock.Owner != nil {
		owner = respLock.Owner.Name
	}
	lock := newGiteaLock(g, respLock.ID, respLock.Path, respLock.LockedAt, owner)
	return lock, nil
}

// Unlock implements transfer.LockBackend
func (g *giteaLockBackend) Unlock(lock transfer.Lock) error {
	reqBody := lfslock.LFSLockDeleteRequest{}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		g.logger.Log("json marshal error", err)
		return err
	}
	headers := map[string]string{
		headerAuthorization:     g.authToken,
		headerGiteaInternalAuth: g.internalAuth,
		headerAccept:            mimeGitLFS,
		headerContentType:       mimeGitLFS,
	}
	req := newInternalRequestLFS(g.ctx, g.server.JoinPath(lock.ID(), "unlock").String(), http.MethodPost, headers, bodyBytes)
	resp, err := req.Response()
	if err != nil {
		g.logger.Log("http request error", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		g.logger.Log("http statuscode error", resp.StatusCode, statusCodeToErr(resp.StatusCode))
		return statusCodeToErr(resp.StatusCode)
	}
	// no need to read response

	return nil
}

// FromPath implements transfer.LockBackend
func (g *giteaLockBackend) FromPath(path string) (transfer.Lock, error) {
	v := url.Values{
		argPath: []string{path},
	}

	respLocks, _, err := g.queryLocks(v)
	if err != nil {
		return nil, err
	}

	if len(respLocks) == 0 {
		return nil, transfer.ErrNotFound
	}
	return respLocks[0], nil
}

// FromID implements transfer.LockBackend
func (g *giteaLockBackend) FromID(id string) (transfer.Lock, error) {
	v := url.Values{
		argID: []string{id},
	}

	respLocks, _, err := g.queryLocks(v)
	if err != nil {
		return nil, err
	}

	if len(respLocks) == 0 {
		return nil, transfer.ErrNotFound
	}
	return respLocks[0], nil
}

// Range implements transfer.LockBackend
func (g *giteaLockBackend) Range(cursor string, limit int, iter func(transfer.Lock) error) (string, error) {
	v := url.Values{
		argLimit: []string{strconv.FormatInt(int64(limit), 10)},
	}
	if cursor != "" {
		v[argCursor] = []string{cursor}
	}

	respLocks, cursor, err := g.queryLocks(v)
	if err != nil {
		return "", err
	}

	for _, lock := range respLocks {
		err := iter(lock)
		if err != nil {
			return "", err
		}
	}
	return cursor, nil
}

func (g *giteaLockBackend) queryLocks(v url.Values) ([]transfer.Lock, string, error) {
	serverURLWithQuery := g.server.JoinPath() // get a copy
	serverURLWithQuery.RawQuery = v.Encode()
	headers := map[string]string{
		headerAuthorization:     g.authToken,
		headerGiteaInternalAuth: g.internalAuth,
		headerAccept:            mimeGitLFS,
		headerContentType:       mimeGitLFS,
	}
	req := newInternalRequestLFS(g.ctx, serverURLWithQuery.String(), http.MethodGet, headers, nil)
	resp, err := req.Response()
	if err != nil {
		g.logger.Log("http request error", err)
		return nil, "", err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		g.logger.Log("http read error", err)
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		g.logger.Log("http statuscode error", resp.StatusCode, statusCodeToErr(resp.StatusCode))
		return nil, "", statusCodeToErr(resp.StatusCode)
	}
	var respBody lfslock.LFSLockList
	err = json.Unmarshal(respBytes, &respBody)
	if err != nil {
		g.logger.Log("json umarshal error", err)
		return nil, "", err
	}

	respLocks := make([]transfer.Lock, 0, len(respBody.Locks))
	for _, respLock := range respBody.Locks {
		owner := userUnknown
		if respLock.Owner != nil {
			owner = respLock.Owner.Name
		}
		lock := newGiteaLock(g, respLock.ID, respLock.Path, respLock.LockedAt, owner)
		respLocks = append(respLocks, lock)
	}
	return respLocks, respBody.Next, nil
}

var _ transfer.Lock = &giteaLock{}

type giteaLock struct {
	g        *giteaLockBackend
	id       string
	path     string
	lockedAt time.Time
	owner    string
}

func newGiteaLock(g *giteaLockBackend, id, path string, lockedAt time.Time, owner string) transfer.Lock {
	return &giteaLock{g: g, id: id, path: path, lockedAt: lockedAt, owner: owner}
}

// Unlock implements transfer.Lock
func (g *giteaLock) Unlock() error {
	return g.g.Unlock(g)
}

// ID implements transfer.Lock
func (g *giteaLock) ID() string {
	return g.id
}

// Path implements transfer.Lock
func (g *giteaLock) Path() string {
	return g.path
}

// FormattedTimestamp implements transfer.Lock
func (g *giteaLock) FormattedTimestamp() string {
	return g.lockedAt.UTC().Format(time.RFC3339)
}

// OwnerName implements transfer.Lock
func (g *giteaLock) OwnerName() string {
	return g.owner
}

func (g *giteaLock) CurrentUser() (string, error) {
	return userSelf, nil
}

// AsLockSpec implements transfer.Lock
func (g *giteaLock) AsLockSpec(ownerID bool) ([]string, error) {
	msgs := []string{
		fmt.Sprintf("lock %s", g.ID()),
		fmt.Sprintf("path %s %s", g.ID(), g.Path()),
		fmt.Sprintf("locked-at %s %s", g.ID(), g.FormattedTimestamp()),
		fmt.Sprintf("ownername %s %s", g.ID(), g.OwnerName()),
	}
	if ownerID {
		user, err := g.CurrentUser()
		if err != nil {
			return nil, fmt.Errorf("error getting current user: %w", err)
		}
		who := "theirs"
		if user == g.OwnerName() {
			who = "ours"
		}
		msgs = append(msgs, fmt.Sprintf("owner %s %s", g.ID(), who))
	}
	return msgs, nil
}

// AsArguments implements transfer.Lock
func (g *giteaLock) AsArguments() []string {
	return []string{
		fmt.Sprintf("id=%s", g.ID()),
		fmt.Sprintf("path=%s", g.Path()),
		fmt.Sprintf("locked-at=%s", g.FormattedTimestamp()),
		fmt.Sprintf("ownername=%s", g.OwnerName()),
	}
}
