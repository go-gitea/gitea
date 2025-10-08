// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// Subject represents a repository subject that can be shared across repositories
type Subject struct {
	ID          int64              `xorm:"pk autoincr"`
	Name        string             `xorm:"VARCHAR(255) UNIQUE NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Subject))
}

// TableName returns the table name for Subject
func (s *Subject) TableName() string {
	return "subject"
}

// GetOrCreateSubject gets an existing subject by name or creates a new one if it doesn't exist
func GetOrCreateSubject(ctx context.Context, name string) (*Subject, error) {
	if name == "" {
		return nil, nil
	}

	// Try to get existing subject
	subject := &Subject{Name: name}
	has, err := db.GetEngine(ctx).Get(subject)
	if err != nil {
		return nil, err
	}
	if has {
		return subject, nil
	}

	// Create new subject
	if err := db.Insert(ctx, subject); err != nil {
		// Handle race condition: another process might have created it
		has, err := db.GetEngine(ctx).Get(subject)
		if err != nil {
			return nil, err
		}
		if has {
			return subject, nil
		}
		return nil, fmt.Errorf("failed to create subject: %w", err)
	}

	return subject, nil
}

// GetSubjectByID gets a subject by its ID
func GetSubjectByID(ctx context.Context, id int64) (*Subject, error) {
	subject := &Subject{ID: id}
	has, err := db.GetEngine(ctx).Get(subject)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrSubjectNotExist{ID: id}
	}
	return subject, nil
}

// GetSubjectByName gets a subject by its name
func GetSubjectByName(ctx context.Context, name string) (*Subject, error) {
	subject := &Subject{Name: name}
	has, err := db.GetEngine(ctx).Get(subject)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrSubjectNotExist{Name: name}
	}
	return subject, nil
}

// UpdateSubject updates a subject's properties
func UpdateSubject(ctx context.Context, subject *Subject) error {
	_, err := db.GetEngine(ctx).ID(subject.ID).AllCols().Update(subject)
	return err
}

// DeleteSubject deletes a subject (only if no repositories reference it)
func DeleteSubject(ctx context.Context, id int64) error {
	// Check if any repositories reference this subject
	count, err := db.GetEngine(ctx).Where("subject_id = ?", id).Count(new(Repository))
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrSubjectInUse{ID: id, RepoCount: count}
	}

	_, err = db.GetEngine(ctx).ID(id).Delete(new(Subject))
	return err
}

// FindSubjects finds subjects based on options
func FindSubjects(ctx context.Context, opts FindSubjectsOptions) ([]*Subject, int64, error) {
	sess := db.GetEngine(ctx).Where(opts.ToConds())
	
	if opts.PageSize > 0 {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}

	subjects := make([]*Subject, 0, opts.PageSize)
	count, err := sess.FindAndCount(&subjects)
	return subjects, count, err
}

// FindSubjectsOptions represents options for finding subjects
type FindSubjectsOptions struct {
	db.ListOptions
	Keyword string
}

// ToConds converts options to database conditions
func (opts FindSubjectsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.Keyword != "" {
		cond = cond.And(builder.Like{"LOWER(name)", opts.Keyword})
	}
	return cond
}

// ErrSubjectNotExist represents a "SubjectNotExist" error
type ErrSubjectNotExist struct {
	ID   int64
	Name string
}

// IsErrSubjectNotExist checks if an error is ErrSubjectNotExist
func IsErrSubjectNotExist(err error) bool {
	_, ok := err.(ErrSubjectNotExist)
	return ok
}

func (err ErrSubjectNotExist) Error() string {
	if err.Name != "" {
		return fmt.Sprintf("subject does not exist [name: %s]", err.Name)
	}
	return fmt.Sprintf("subject does not exist [id: %d]", err.ID)
}

// ErrSubjectInUse represents a "SubjectInUse" error
type ErrSubjectInUse struct {
	ID        int64
	RepoCount int64
}

// IsErrSubjectInUse checks if an error is ErrSubjectInUse
func IsErrSubjectInUse(err error) bool {
	_, ok := err.(ErrSubjectInUse)
	return ok
}

func (err ErrSubjectInUse) Error() string {
	return fmt.Sprintf("subject is in use by %d repositories [id: %d]", err.RepoCount, err.ID)
}

