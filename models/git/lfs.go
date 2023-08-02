// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrLFSLockNotExist represents a "LFSLockNotExist" kind of error.
type ErrLFSLockNotExist struct {
	ID     int64
	RepoID int64
	Path   string
}

// IsErrLFSLockNotExist checks if an error is a ErrLFSLockNotExist.
func IsErrLFSLockNotExist(err error) bool {
	_, ok := err.(ErrLFSLockNotExist)
	return ok
}

func (err ErrLFSLockNotExist) Error() string {
	return fmt.Sprintf("lfs lock does not exist [id: %d, rid: %d, path: %s]", err.ID, err.RepoID, err.Path)
}

func (err ErrLFSLockNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrLFSUnauthorizedAction represents a "LFSUnauthorizedAction" kind of error.
type ErrLFSUnauthorizedAction struct {
	RepoID   int64
	UserName string
	Mode     perm.AccessMode
}

// IsErrLFSUnauthorizedAction checks if an error is a ErrLFSUnauthorizedAction.
func IsErrLFSUnauthorizedAction(err error) bool {
	_, ok := err.(ErrLFSUnauthorizedAction)
	return ok
}

func (err ErrLFSUnauthorizedAction) Error() string {
	if err.Mode == perm.AccessModeWrite {
		return fmt.Sprintf("User %s doesn't have write access for lfs lock [rid: %d]", err.UserName, err.RepoID)
	}
	return fmt.Sprintf("User %s doesn't have read access for lfs lock [rid: %d]", err.UserName, err.RepoID)
}

func (err ErrLFSUnauthorizedAction) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrLFSLockAlreadyExist represents a "LFSLockAlreadyExist" kind of error.
type ErrLFSLockAlreadyExist struct {
	RepoID int64
	Path   string
}

// IsErrLFSLockAlreadyExist checks if an error is a ErrLFSLockAlreadyExist.
func IsErrLFSLockAlreadyExist(err error) bool {
	_, ok := err.(ErrLFSLockAlreadyExist)
	return ok
}

func (err ErrLFSLockAlreadyExist) Error() string {
	return fmt.Sprintf("lfs lock already exists [rid: %d, path: %s]", err.RepoID, err.Path)
}

func (err ErrLFSLockAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrLFSFileLocked represents a "LFSFileLocked" kind of error.
type ErrLFSFileLocked struct {
	RepoID   int64
	Path     string
	UserName string
}

// IsErrLFSFileLocked checks if an error is a ErrLFSFileLocked.
func IsErrLFSFileLocked(err error) bool {
	_, ok := err.(ErrLFSFileLocked)
	return ok
}

func (err ErrLFSFileLocked) Error() string {
	return fmt.Sprintf("File is lfs locked [repo: %d, locked by: %s, path: %s]", err.RepoID, err.UserName, err.Path)
}

func (err ErrLFSFileLocked) Unwrap() error {
	return util.ErrPermissionDenied
}

// LFSMetaObject stores metadata for LFS tracked files.
type LFSMetaObject struct {
	ID           int64 `xorm:"pk autoincr"`
	lfs.Pointer  `xorm:"extends"`
	RepositoryID int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Existing     bool               `xorm:"-"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix  timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(LFSMetaObject))
}

// LFSTokenResponse defines the JSON structure in which the JWT token is stored.
// This structure is fetched via SSH and passed by the Git LFS client to the server
// endpoint for authorization.
type LFSTokenResponse struct {
	Header map[string]string `json:"header"`
	Href   string            `json:"href"`
}

// ErrLFSObjectNotExist is returned from lfs models functions in order
// to differentiate between database and missing object errors.
var ErrLFSObjectNotExist = db.ErrNotExist{Resource: "LFS Meta object"}

// NewLFSMetaObject stores a given populated LFSMetaObject structure in the database
// if it is not already present.
func NewLFSMetaObject(ctx context.Context, m *LFSMetaObject) (*LFSMetaObject, error) {
	var err error

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	has, err := db.GetByBean(ctx, m)
	if err != nil {
		return nil, err
	}

	if has {
		m.Existing = true
		return m, committer.Commit()
	}

	if err = db.Insert(ctx, m); err != nil {
		return nil, err
	}

	return m, committer.Commit()
}

// GetLFSMetaObjectByOid selects a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error. If the error is nil,
// the returned pointer is a valid LFSMetaObject.
func GetLFSMetaObjectByOid(ctx context.Context, repoID int64, oid string) (*LFSMetaObject, error) {
	if len(oid) == 0 {
		return nil, ErrLFSObjectNotExist
	}

	m := &LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}, RepositoryID: repoID}
	has, err := db.GetEngine(ctx).Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSObjectNotExist
	}
	return m, nil
}

// RemoveLFSMetaObjectByOid removes a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error.
func RemoveLFSMetaObjectByOid(ctx context.Context, repoID int64, oid string) (int64, error) {
	return RemoveLFSMetaObjectByOidFn(ctx, repoID, oid, nil)
}

// RemoveLFSMetaObjectByOidFn removes a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error. It will run Fn with the current count within the transaction
func RemoveLFSMetaObjectByOidFn(ctx context.Context, repoID int64, oid string, fn func(count int64) error) (int64, error) {
	if len(oid) == 0 {
		return 0, ErrLFSObjectNotExist
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return 0, err
	}
	defer committer.Close()

	m := &LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}, RepositoryID: repoID}
	if _, err := db.DeleteByBean(ctx, m); err != nil {
		return -1, err
	}

	count, err := db.CountByBean(ctx, &LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}})
	if err != nil {
		return count, err
	}

	if fn != nil {
		if err := fn(count); err != nil {
			return count, err
		}
	}

	return count, committer.Commit()
}

// GetLFSMetaObjects returns all LFSMetaObjects associated with a repository
func GetLFSMetaObjects(ctx context.Context, repoID int64, page, pageSize int) ([]*LFSMetaObject, error) {
	sess := db.GetEngine(ctx)

	if page >= 0 && pageSize > 0 {
		start := 0
		if page > 0 {
			start = (page - 1) * pageSize
		}
		sess.Limit(pageSize, start)
	}
	lfsObjects := make([]*LFSMetaObject, 0, pageSize)
	return lfsObjects, sess.Find(&lfsObjects, &LFSMetaObject{RepositoryID: repoID})
}

// CountLFSMetaObjects returns a count of all LFSMetaObjects associated with a repository
func CountLFSMetaObjects(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Count(&LFSMetaObject{RepositoryID: repoID})
}

// LFSObjectAccessible checks if a provided Oid is accessible to the user
func LFSObjectAccessible(ctx context.Context, user *user_model.User, oid string) (bool, error) {
	if user.IsAdmin {
		count, err := db.GetEngine(ctx).Count(&LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}})
		return count > 0, err
	}
	cond := repo_model.AccessibleRepositoryCondition(user, unit.TypeInvalid)
	count, err := db.GetEngine(ctx).Where(cond).Join("INNER", "repository", "`lfs_meta_object`.repository_id = `repository`.id").Count(&LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}})
	return count > 0, err
}

// ExistsLFSObject checks if a provided Oid exists within the DB
func ExistsLFSObject(ctx context.Context, oid string) (bool, error) {
	return db.GetEngine(ctx).Exist(&LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}})
}

// LFSAutoAssociate auto associates accessible LFSMetaObjects
func LFSAutoAssociate(ctx context.Context, metas []*LFSMetaObject, user *user_model.User, repoID int64) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	oids := make([]any, len(metas))
	oidMap := make(map[string]*LFSMetaObject, len(metas))
	for i, meta := range metas {
		oids[i] = meta.Oid
		oidMap[meta.Oid] = meta
	}

	if !user.IsAdmin {
		newMetas := make([]*LFSMetaObject, 0, len(metas))
		cond := builder.In(
			"`lfs_meta_object`.repository_id",
			builder.Select("`repository`.id").From("repository").Where(repo_model.AccessibleRepositoryCondition(user, unit.TypeInvalid)),
		)
		err = sess.Cols("oid").Where(cond).In("oid", oids...).GroupBy("oid").Find(&newMetas)
		if err != nil {
			return err
		}
		if len(newMetas) != len(oidMap) {
			return fmt.Errorf("unable collect all LFS objects from database, expected %d, actually %d", len(oidMap), len(newMetas))
		}
		for i := range newMetas {
			newMetas[i].Size = oidMap[newMetas[i].Oid].Size
			newMetas[i].RepositoryID = repoID
		}
		if err = db.Insert(ctx, newMetas); err != nil {
			return err
		}
	} else {
		// admin can associate any LFS object to any repository, and we do not care about errors (eg: duplicated unique key),
		// even if error occurs, it won't hurt users and won't make things worse
		for i := range metas {
			p := lfs.Pointer{Oid: metas[i].Oid, Size: metas[i].Size}
			_, err = sess.Insert(&LFSMetaObject{
				Pointer:      p,
				RepositoryID: repoID,
			})
			if err != nil {
				log.Warn("failed to insert LFS meta object %-v for repo_id: %d into database, err=%v", p, repoID, err)
			}
		}
	}
	return committer.Commit()
}

// CopyLFS copies LFS data from one repo to another
func CopyLFS(ctx context.Context, newRepo, oldRepo *repo_model.Repository) error {
	var lfsObjects []*LFSMetaObject
	if err := db.GetEngine(ctx).Where("repository_id=?", oldRepo.ID).Find(&lfsObjects); err != nil {
		return err
	}

	for _, v := range lfsObjects {
		v.ID = 0
		v.RepositoryID = newRepo.ID
		if err := db.Insert(ctx, v); err != nil {
			return err
		}
	}

	return nil
}

// GetRepoLFSSize return a repository's lfs files size
func GetRepoLFSSize(ctx context.Context, repoID int64) (int64, error) {
	lfsSize, err := db.GetEngine(ctx).Where("repository_id = ?", repoID).SumInt(new(LFSMetaObject), "size")
	if err != nil {
		return 0, fmt.Errorf("updateSize: GetLFSMetaObjects: %w", err)
	}
	return lfsSize, nil
}

// IterateRepositoryIDsWithLFSMetaObjects iterates across the repositories that have LFSMetaObjects
func IterateRepositoryIDsWithLFSMetaObjects(ctx context.Context, f func(ctx context.Context, repoID, count int64) error) error {
	batchSize := setting.Database.IterateBufferSize
	sess := db.GetEngine(ctx)
	id := int64(0)
	type RepositoryCount struct {
		RepositoryID int64
		Count        int64
	}
	for {
		counts := make([]*RepositoryCount, 0, batchSize)
		sess.Select("repository_id, COUNT(id) AS count").
			Table("lfs_meta_object").
			Where("repository_id > ?", id).
			GroupBy("repository_id").
			OrderBy("repository_id ASC")

		if err := sess.Limit(batchSize, 0).Find(&counts); err != nil {
			return err
		}
		if len(counts) == 0 {
			return nil
		}

		for _, count := range counts {
			if err := f(ctx, count.RepositoryID, count.Count); err != nil {
				return err
			}
		}
		id = counts[len(counts)-1].RepositoryID
	}
}

// IterateLFSMetaObjectsForRepoOptions provides options for IterateLFSMetaObjectsForRepo
type IterateLFSMetaObjectsForRepoOptions struct {
	OlderThan                 timeutil.TimeStamp
	UpdatedLessRecentlyThan   timeutil.TimeStamp
	OrderByUpdated            bool
	LoopFunctionAlwaysUpdates bool
}

// IterateLFSMetaObjectsForRepo provides a iterator for LFSMetaObjects per Repo
func IterateLFSMetaObjectsForRepo(ctx context.Context, repoID int64, f func(context.Context, *LFSMetaObject, int64) error, opts *IterateLFSMetaObjectsForRepoOptions) error {
	var start int
	batchSize := setting.Database.IterateBufferSize
	engine := db.GetEngine(ctx)
	type CountLFSMetaObject struct {
		Count         int64
		LFSMetaObject `xorm:"extends"`
	}

	id := int64(0)

	for {
		beans := make([]*CountLFSMetaObject, 0, batchSize)
		sess := engine.Table("lfs_meta_object").Select("`lfs_meta_object`.*, COUNT(`l1`.oid) AS `count`").
			Join("INNER", "`lfs_meta_object` AS l1", "`lfs_meta_object`.oid = `l1`.oid").
			Where("`lfs_meta_object`.repository_id = ?", repoID)
		if !opts.OlderThan.IsZero() {
			sess.And("`lfs_meta_object`.created_unix < ?", opts.OlderThan)
		}
		if !opts.UpdatedLessRecentlyThan.IsZero() {
			sess.And("`lfs_meta_object`.updated_unix < ?", opts.UpdatedLessRecentlyThan)
		}
		sess.GroupBy("`lfs_meta_object`.id")
		if opts.OrderByUpdated {
			sess.OrderBy("`lfs_meta_object`.updated_unix ASC")
		} else {
			sess.And("`lfs_meta_object`.id > ?", id)
			sess.OrderBy("`lfs_meta_object`.id ASC")
		}
		if err := sess.Limit(batchSize, start).Find(&beans); err != nil {
			return err
		}
		if len(beans) == 0 {
			return nil
		}
		if !opts.LoopFunctionAlwaysUpdates {
			start += len(beans)
		}

		for _, bean := range beans {
			if err := f(ctx, &bean.LFSMetaObject, bean.Count); err != nil {
				return err
			}
		}
		id = beans[len(beans)-1].ID
	}
}

// MarkLFSMetaObject updates the updated time for the provided LFSMetaObject
func MarkLFSMetaObject(ctx context.Context, id int64) error {
	obj := &LFSMetaObject{
		UpdatedUnix: timeutil.TimeStampNow(),
	}
	count, err := db.GetEngine(ctx).ID(id).Update(obj)
	if count != 1 {
		log.Error("Unexpectedly updated %d LFSMetaObjects with ID: %d", count, id)
	}
	return err
}
